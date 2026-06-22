package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

// RunMode distinguishes Action mode (GITHUB_ACTIONS=true) from CLI
// mode. Set by Run / RunCLI before any business logic runs so the
// banner + log fields can record which path the invocation took.
type RunMode int

const (
	// ModeAction = GitHub Actions runtime (GITHUB_ACTIONS=true).
	ModeAction RunMode = iota
	// ModeCLI = local / scripted CLI invocation.
	ModeCLI
)

func (m RunMode) String() string {
	if m == ModeAction {
		return "action"
	}
	return "cli"
}

// Invocation is the resolved per-run state Run / RunCLI hand off to
// the engine + committer pipeline. It is built from the merged
// inputs (INPUTS JSON / INPUT_<UPPER> / CLI flags / preset / env).
type Invocation struct {
	Mode             RunMode
	Inputs           map[string]any
	Token            config.Token
	Template         string
	Login            string
	Format           string // "svg" / "png" / "jpeg" / "json"
	Dryrun           bool
	OutputAction     string
	OutputCondition  string
	OutputFilename   string
	OutputDir        string // "/renders" by default in Action mode; pwd in CLI
	UseMockedData    bool
	NoticeReleases   bool
	RepoOwner        string
	RepoName         string
	RunID            string // GITHUB_RUN_ID; used by pull-request* head branch naming
	Branch           string // committer_branch; empty = default
	CommitterMessage string
	CommitterAuthor  string
	CommitterEmail   string
	RetryPolicy      RetryPolicy
	GitHubAPIRest    string
	GitHubAPIGraphQL string
}

// Run is the Action-mode entry point. Spec FR-001 / FR-002.
//
// Pipeline:
//
//  1. Skip detection — short-circuits when GITHUB_EVENT_PATH commit
//     message contains "[Skip GitHub Action]" or "Auto-generated
//     metrics for run #N".
//  2. Input parsing — merges INPUTS JSON + INPUT_<UPPER>.
//  3. Output_action validation — fail-fast on gist / markdown-*.
//  4. Token validation — github_pat_* reject + scope check + quota
//     check. Quota insufficient → exit 0 (skipped).
//  5. Banner print (English fixed, slog-handler-agnostic).
//  6. engine.Compute (wrapped in RetryPolicy).
//  7. Write output to OutputDir/OutputFilename.
//  8. Committer.Run (commit / pull-request*) unless Dryrun.
//  9. SetOutput metrics_url + metrics_sha to $GITHUB_OUTPUT.
func Run(ctx context.Context) error {
	return runWith(ctx, runOptions{
		Mode:      ModeAction,
		Env:       os.Environ(),
		Stdout:    os.Stdout,
		EventPath: os.Getenv("GITHUB_EVENT_PATH"),
		OutputDir: defaultOutputDir(),
	})
}

// runOptions captures everything Run touches in the real world so
// tests can inject deterministic values. The exported Run / RunCLI
// callers fill defaults from os.Environ / os.Stdout / etc.
type runOptions struct {
	Mode       RunMode
	Env        []string
	Stdout     io.Writer
	EventPath  string
	OutputDir  string
	WorkingDir string                                                          // for CLI mode (defaults to pwd)
	BuildDeps  func(ctx context.Context, inv *Invocation) (engine.Deps, error) // optional override for tests
}

func runWith(ctx context.Context, opts runOptions) error {
	// 1. Skip detection.
	if opts.Mode == ModeAction {
		if skip, reason := shouldSkip(opts.EventPath); skip {
			slog.Info("metrics-cli skipped", "reason", reason)
			return nil
		}
	}

	// 2. Input parsing.
	env := envSliceToMap(opts.Env)
	inputs, err := ParseInputs(env)
	if err != nil {
		return fmt.Errorf("action: parse inputs: %w", err)
	}

	// 2b. Preset overlay (if config_presets is set).
	if presetPath, ok := inputs["config_presets"].(string); ok && presetPath != "" {
		preset, perr := LoadPreset(presetPath)
		if perr != nil {
			return fmt.Errorf("action: load preset: %w", perr)
		}
		preset.MergeInto(inputs)
	}

	// 3. Build invocation.
	inv, ierr := newInvocation(opts.Mode, inputs, env, opts.OutputDir)
	if ierr != nil {
		return ierr
	}

	// 4. Output_action validation — fail-fast before any API call.
	if verr := DefaultRegistry().Validate(inv.OutputAction); verr != nil {
		return verr
	}

	// 4b. M7: repository template requires the `repo` input. Validate
	// before deps so SC-003 holds (5-second exit, zero API calls).
	if verr := validateRepositoryInput(inv); verr != nil {
		return verr
	}

	// 5. Build engine deps (real or mocked).
	var deps engine.Deps
	if opts.BuildDeps != nil {
		deps, err = opts.BuildDeps(ctx, inv)
	} else {
		deps, err = defaultBuildDeps(ctx, inv)
	}
	if err != nil {
		return fmt.Errorf("action: build deps: %w", err)
	}

	// 6. Token validation.
	if !inv.UseMockedData {
		validator := &TokenValidator{
			Token:         inv.Token,
			REST:          deps.REST,
			UseMockedData: inv.UseMockedData,
			// RequiredScopes + Quota are zero by default — Phase 3 ships
			// the minimal validator; per-plugin scope/quota aggregation
			// lands incrementally as integration tests demand it.
		}
		vRes, verr := validator.Validate(ctx)
		if verr != nil {
			return verr
		}
		if !vRes.QuotaSufficient {
			slog.Info("metrics-cli skipped: insufficient GitHub API quota",
				"reset", vRes.RateState.REST.Reset)
			return nil
		}
		if len(vRes.MissingScopes) > 0 {
			slog.Warn("token is missing some scopes; affected plugins will skip",
				"missing", vRes.MissingScopes)
		}
	}

	// 7. Print banner.
	PrintBanner(opts.Stdout, BannerInfo{
		Version:     engine.Version(),
		Mode:        inv.Mode.String(),
		Template:    inv.Template,
		Plugins:     sortedTruthyPluginGates(inputs),
		TokenMasked: inv.Token.String(),
		GoVersion:   runtime.Version(),
		OSArch:      runtime.GOOS + "/" + runtime.GOARCH,
	})

	// 8. Notice — best-effort newer-version hint.
	if inv.NoticeReleases && deps.REST != nil {
		if msg := CheckLatestRelease(ctx, deps.REST, "mjun0812/github-metrics", engine.Version()); msg != "" {
			slog.Info(msg)
		}
	}

	// 9. Compute (with retry).
	var res *engine.Result
	cerr := inv.RetryPolicy.Do(ctx, func() error {
		var e error
		res, e = engine.Compute(ctx, engine.Request{
			Login:    inv.Login,
			Repo:     stringInput(inv.Inputs, "repo", ""),
			Account:  accountForTemplate(inv.Template),
			Template: inv.Template,
			Format:   inv.Format,
			Inputs:   inv.Inputs,
		}, deps)
		return e
	})
	if cerr != nil {
		return fmt.Errorf("action: engine.Compute: %w", cerr)
	}
	if res == nil {
		return errors.New("action: engine.Compute returned nil result")
	}

	// 10. Write output.
	outputPath := filepath.Join(inv.OutputDir, inv.OutputFilename)
	if werr := writeOutputFile(outputPath, res.Output); werr != nil {
		return fmt.Errorf("action: write output: %w", werr)
	}

	// 11. metrics_sha output (always set, even on dryrun + skipped Committer).
	sha, hashErr := render.Hash(string(res.Output))
	if hashErr != nil {
		slog.Warn("render.Hash failed; metrics_sha output skipped", "err", hashErr)
	} else if oerr := SetOutput("metrics_sha", sha); oerr != nil {
		slog.Warn("metrics_sha output write failed", "err", oerr)
	}

	// 12. Committer — only when not dryrun and output_action != none.
	if !inv.Dryrun && inv.OutputAction != "none" {
		committer, nerr := NewCommitter(deps.REST, inv, res.Output)
		if nerr != nil {
			return fmt.Errorf("action: committer init: %w", nerr)
		}
		if cerr := committer.Run(ctx); cerr != nil {
			// FR-016: committer failure does not block the workflow.
			slog.Warn("committer failed (action continues)", "err", cerr)
		}
		if committer.MetricsURL != "" {
			if oerr := SetOutput("metrics_url", committer.MetricsURL); oerr != nil {
				slog.Warn("metrics_url output write failed", "err", oerr)
			}
		}
	}

	return nil
}

// RunCLI is the CLI-mode entry point (spec FR-019). Pipeline:
//
//  1. Parse flags (T034) + load --config YAML (T035).
//  2. Merge inputs via CLIFlags.ToInvocation (T036).
//  3. Resolve token (T037) — flag > env > error.
//  4. newInvocation + output_action validation (shared with Action).
//  5. Banner + engine.Compute + retry policy (shared).
//  6. Write output to --filename or stdout (T038).
//  7. Committer dispatch unless --dryrun.
func RunCLI(ctx context.Context, args []string) error {
	cf, err := ParseFlags(args)
	if err != nil {
		return fmt.Errorf("action: cli flags: %w", err)
	}
	cwd, _ := os.Getwd()
	return runCLIWith(ctx, cf, runOptions{
		Mode:       ModeCLI,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		OutputDir:  cwd,
		WorkingDir: cwd,
	})
}

func runCLIWith(ctx context.Context, cf *CLIFlags, opts runOptions) error {
	env := envSliceToMap(opts.Env)
	inputs, err := cf.ToInvocation(env)
	if err != nil {
		return err
	}

	// Preset overlay (mirrors Action path) — runs after --config so the
	// preset's q: map overrides defaults but not CLI flags / config.
	if presetPath, ok := inputs["config_presets"].(string); ok && presetPath != "" {
		preset, perr := LoadPreset(presetPath)
		if perr != nil {
			return fmt.Errorf("action: load preset: %w", perr)
		}
		preset.MergeInto(inputs)
	}

	// Token resolution — applied after the merge so INPUT_TOKEN serves as
	// the fallback when --token / --token-env are absent.
	inputTok, _ := inputs["token"].(string)
	tok, terr := ResolveToken(cf, os.Getenv, inputTok)
	if terr != nil {
		// Allow dryrun + mocked-data to bypass token requirement.
		if !cf.Dryrun || !boolInput(inputs, "use_mocked_data", false) {
			return terr
		}
	}
	if tok != "" {
		inputs["token"] = tok
	}

	inv, ierr := newInvocation(ModeCLI, inputs, env, opts.OutputDir)
	if ierr != nil {
		return ierr
	}

	if verr := DefaultRegistry().Validate(inv.OutputAction); verr != nil {
		return verr
	}
	if verr := validateRepositoryInput(inv); verr != nil {
		return verr
	}

	var deps engine.Deps
	if opts.BuildDeps != nil {
		deps, err = opts.BuildDeps(ctx, inv)
	} else {
		deps, err = defaultBuildDeps(ctx, inv)
	}
	if err != nil {
		return fmt.Errorf("action: build deps: %w", err)
	}

	if !inv.UseMockedData {
		validator := &TokenValidator{Token: inv.Token, REST: deps.REST, UseMockedData: inv.UseMockedData}
		vRes, verr := validator.Validate(ctx)
		if verr != nil {
			return verr
		}
		if !vRes.QuotaSufficient {
			slog.Info("metrics-cli skipped: insufficient GitHub API quota",
				"reset", vRes.RateState.REST.Reset)
			return nil
		}
		if len(vRes.MissingScopes) > 0 {
			slog.Warn("token is missing some scopes; affected plugins will skip",
				"missing", vRes.MissingScopes)
		}
	}

	// Banner goes to stderr in CLI mode so it can never contaminate the
	// rendered output. Specifically: `--filename -` streams the SVG/PNG
	// payload to stdout, and a banner prepended to that stream would
	// corrupt PNG bytes outright and pollute committed SVG diffs. stderr
	// keeps the banner human-visible in a terminal while leaving the
	// data path clean for redirection and pipelines.
	PrintBanner(os.Stderr, BannerInfo{
		Version:     engine.Version(),
		Mode:        inv.Mode.String(),
		Template:    inv.Template,
		Plugins:     sortedTruthyPluginGates(inputs),
		TokenMasked: inv.Token.String(),
		GoVersion:   runtime.Version(),
		OSArch:      runtime.GOOS + "/" + runtime.GOARCH,
	})

	var res *engine.Result
	cerr := inv.RetryPolicy.Do(ctx, func() error {
		var e error
		res, e = engine.Compute(ctx, engine.Request{
			Login:    inv.Login,
			Repo:     stringInput(inv.Inputs, "repo", ""),
			Account:  accountForTemplate(inv.Template),
			Template: inv.Template,
			Format:   inv.Format,
			Inputs:   inv.Inputs,
		}, deps)
		return e
	})
	if cerr != nil {
		return fmt.Errorf("action: engine.Compute: %w", cerr)
	}
	if res == nil {
		return errors.New("action: engine.Compute returned nil result")
	}

	w, closeFn, oerr := ResolveOutputWriter(targetOutputPath(inv), inv.Format)
	if oerr != nil {
		return fmt.Errorf("action: open output: %w", oerr)
	}
	defer func() { _ = closeFn() }()
	if _, werr := w.Write(res.Output); werr != nil {
		return fmt.Errorf("action: write output: %w", werr)
	}

	if !inv.Dryrun && inv.OutputAction != "none" {
		committer, nerr := NewCommitter(deps.REST, inv, res.Output)
		if nerr != nil {
			return fmt.Errorf("action: committer init: %w", nerr)
		}
		if cerr := committer.Run(ctx); cerr != nil {
			slog.Warn("committer failed (action continues)", "err", cerr)
		}
	}
	return nil
}

// targetOutputPath chooses the destination for the CLI Write step.
// `-` (stdout) stays as is; otherwise the resolved filename is joined
// to OutputDir when it's relative.
func targetOutputPath(inv *Invocation) string {
	if inv.OutputFilename == "-" {
		return "-"
	}
	if filepath.IsAbs(inv.OutputFilename) {
		return inv.OutputFilename
	}
	return filepath.Join(inv.OutputDir, inv.OutputFilename)
}

// ---------- helpers ----------

// validateRepositoryInput enforces the M7 requirement that the
// `repository` template runs only when the user provided a non-empty
// `repo` input. Returns nil for non-repository templates so the
// classic + (future) other templates remain unaffected.
//
// Per spec SC-003 + research R-006, this runs before deps construction
// so we exit with code 1 in under 5 seconds without contacting the
// GitHub API.
func validateRepositoryInput(inv *Invocation) error {
	if inv == nil || inv.Template != "repository" {
		return nil
	}
	if stringInput(inv.Inputs, "repo", "") == "" {
		return &ConfigError{
			Key:   "repo",
			Value: "",
			Msg:   "template 'repository' requires --repo / INPUT_REPO (the GitHub repository name)",
		}
	}
	return nil
}

// accountForTemplate maps the action's template input to the
// plugins.AccountKind value the engine + base plugin use to branch
// between user-centric (M2/M4) and repository-centric (M7) flows.
// Unknown template names default to AccountUser to preserve the
// existing classic-template behavior.
func accountForTemplate(template string) plugins.AccountKind {
	if template == "repository" {
		return plugins.AccountRepository
	}
	return plugins.AccountUser
}

func defaultOutputDir() string {
	// Action mode writes to /renders inside the Docker container.
	// Falls back to pwd when the dir does not exist (= running
	// outside the container, e.g., go test).
	if _, err := os.Stat("/renders"); err == nil {
		return "/renders"
	}
	return "."
}

func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		out[kv[:idx]] = kv[idx+1:]
	}
	return out
}

func newInvocation(mode RunMode, inputs map[string]any, env map[string]string, outputDir string) (*Invocation, error) {
	// Materialize the `optimize` metadata default. The action / CLI
	// input layer (ParseInputs) only carries explicitly-provided
	// inputs — it does not apply metadata.yml defaults — so without
	// this the upstream default ("css, xml") never reaches the render
	// dispatch and CSS/XML optimization silently never runs (see
	// internal/engine/dispatch.go::optimizeEnabled and
	// assets/plugins/core/metadata.yml `optimize`). An explicitly empty
	// `optimize=` is preserved so callers can still opt out of it.
	if _, ok := inputs["optimize"]; !ok {
		inputs["optimize"] = []string{"css", "xml"}
	}
	inv := &Invocation{
		Mode:             mode,
		Inputs:           inputs,
		Template:         stringInput(inputs, "template", "classic"),
		Login:            stringInput(inputs, "user", ""),
		Format:           stringInput(inputs, "config_output", "svg"),
		Dryrun:           boolInput(inputs, "dryrun", false),
		OutputAction:     stringInput(inputs, "output_action", "commit"),
		OutputCondition:  stringInput(inputs, "output_condition", "always"),
		UseMockedData:    boolInput(inputs, "use_mocked_data", false),
		NoticeReleases:   boolInput(inputs, "notice_releases", true),
		Branch:           stringInput(inputs, "committer_branch", ""),
		CommitterMessage: stringInput(inputs, "committer_message", "Auto-generated metrics for run #${run}"),
		CommitterAuthor:  stringInput(inputs, "committer_author", "metrics[bot]"),
		CommitterEmail:   stringInput(inputs, "committer_email", "metrics@noreply.localhost"),
		GitHubAPIRest:    stringInput(inputs, "github_api_rest", githubapi.DefaultRESTBaseURL),
		GitHubAPIGraphQL: stringInput(inputs, "github_api_graphql", "https://api.github.com/graphql"),
		OutputDir:        outputDir,
	}
	inv.Token = config.NewToken(stringInput(inputs, "token", ""))
	inv.RetryPolicy = RetryPolicy{
		Retries: intInput(inputs, "retries", DefaultRetries),
		Delay:   durationMsInput(inputs, "retries_delay", DefaultRetryDelay),
	}

	// Resolve filename wildcard.
	rawFilename := stringInput(inputs, "filename", "github-metrics.*")
	resolved, ferr := WildcardFilename(rawFilename, inv.Format)
	if ferr != nil {
		return nil, fmt.Errorf("action: filename: %w", ferr)
	}
	inv.OutputFilename = resolved

	// Resolve repo from GITHUB_REPOSITORY + run id from GITHUB_RUN_ID.
	if mode == ModeAction {
		if repo, ok := env["GITHUB_REPOSITORY"]; ok && repo != "" {
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) == 2 {
				inv.RepoOwner = parts[0]
				inv.RepoName = parts[1]
			}
		}
		inv.RunID = env["GITHUB_RUN_ID"]
	}

	// Login fallback: GITHUB_ACTOR.
	if inv.Login == "" && mode == ModeAction {
		inv.Login = env["GITHUB_ACTOR"]
	}

	if inv.Login == "" {
		return nil, errors.New("action: input 'user' (or GITHUB_ACTOR fallback) is required")
	}

	return inv, nil
}

func defaultBuildDeps(_ context.Context, inv *Invocation) (engine.Deps, error) {
	rest, err := githubapi.NewREST(
		inv.Token,
		inv.GitHubAPIRest,
		httpx.Options{DisableRetries: true},
	)
	if err != nil {
		return engine.Deps{}, fmt.Errorf("new REST: %w", err)
	}
	gql, err := githubapi.NewGraphQL(
		inv.Token,
		inv.GitHubAPIGraphQL,
		httpx.Options{DisableRetries: true},
	)
	if err != nil {
		return engine.Deps{}, fmt.Errorf("new GraphQL: %w", err)
	}
	// Render: FakeRenderer in mocked mode, real browser otherwise. The
	// real browser is now only needed for the SVG -> PNG/JPEG resize
	// pipeline (engine.dispatch.go); topics and starlists no longer
	// depend on chromedp. We let engine.Compute lazily allocate the
	// browser when an image format is requested (engine.Deps.Render
	// nil is the documented "lazy allocate" signal).
	var renderer render.Renderer
	if inv.UseMockedData {
		renderer = render.NewFakeRenderer()
	}

	// HTTPClient feeds the render pipeline's image-inline stage, which
	// fetches avatar / icon URLs (public CDN, no auth) and embeds them
	// as base64 data URIs so the SVG renders on GitHub and offline. Left
	// nil under mocked data so hermetic test runs never touch the
	// network; the stage is then skipped.
	var imgClient *httpx.Client
	if !inv.UseMockedData {
		imgClient = httpx.New(httpx.Options{})
	}
	return engine.Deps{
		Settings:   &config.Settings{Repositories: 100},
		REST:       rest,
		GraphQL:    gql,
		Render:     renderer,
		HTTPClient: imgClient,
	}, nil
}

func writeOutputFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // committer-style path under inv.OutputDir
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func sortedTruthyPluginGates(inputs map[string]any) []string {
	var out []string
	for k, v := range inputs {
		if !strings.HasPrefix(k, "plugin_") {
			continue
		}
		// Top-level gate only — sub-options (plugin_<slug>_<opt>)
		// would clutter the banner.
		if strings.Count(k, "_") > 1 {
			continue
		}
		if isTruthy(v) {
			out = append(out, strings.TrimPrefix(k, "plugin_"))
		}
	}
	sort.Strings(out)
	return out
}

// ---------- typed input readers ----------

func stringInput(in map[string]any, key, def string) string {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case string:
		if x == "" {
			return def
		}
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

func boolInput(in map[string]any, key string, def bool) bool {
	v, ok := in[key]
	if !ok {
		return def
	}
	return isTruthy(v)
}

func intInput(in map[string]any, key string, def int) int {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		var n int
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func durationMsInput(in map[string]any, key string, def time.Duration) time.Duration {
	v, ok := in[key]
	if !ok {
		return def
	}
	ms := intInput(map[string]any{key: v}, key, int(def/time.Millisecond))
	return time.Duration(ms) * time.Millisecond
}

func isTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "yes" || s == "1" || s == "on"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}
