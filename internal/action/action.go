package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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

// Invocation is the resolved per-run state Run hands off to the engine
// + committer pipeline. It is built from the merged inputs (INPUT_<UPPER>
// / INPUTS JSON / --config YAML / CLI flags).
type Invocation struct {
	Inputs            map[string]any
	Token             config.Token
	Template          string
	Login             string
	Format            string // "svg" / "png" / "jpeg" / "json"
	Dryrun            bool
	OutputAction      string
	OutputCondition   string
	OutputFilename    string
	OutputDir         string
	PerPlugin         bool // true when in per-plugin SVG output mode
	UseMockedData     bool
	NoticeReleases    bool
	RepoOwner         string
	RepoName          string
	RunID             string // GITHUB_RUN_ID; used by pull-request* head branch naming
	Branch            string // committer_branch; empty = default
	CommitterMessage  string
	CommitterAuthor   string
	CommitterEmail    string
	RetryPolicy       RetryPolicy // rendering (engine.Compute) retries
	OutputRetryPolicy RetryPolicy // output_action (committer) retries
	GitHubAPIRest     string
	GitHubAPIGraphQL  string
}

// Run is the unified entry point for the metrics-cli binary. It reads
// CLI flags from args AND env vars (INPUT_<UPPER> / INPUTS JSON) every
// invocation; CLI flags take precedence on conflict.
//
// Pipeline:
//
//  1. Skip detection — short-circuits when GITHUB_EVENT_PATH commit
//     message contains "[Skip GitHub Action]" or "Auto-generated
//     metrics for run #N".
//  2. Input assembly — merges env layer (INPUT_<UPPER> + INPUTS JSON,
//     unless --no-env), --config YAML overlay, and CLI flag overrides
//     (highest priority).
//  3. Output_action + repository-template + token gates run fail-fast
//     before any GitHub API call.
//  4. engine.Compute (wrapped in RetryPolicy).
//  5. Write output to OutputDir/OutputFilename (or stdout when
//     --filename -).
//  6. Committer.Run (commit / pull-request*) unless --dryrun.
//  7. SetOutput metrics_url + metrics_sha to $GITHUB_OUTPUT (no-op
//     outside the GitHub Actions runner).
func Run(ctx context.Context, args []string) error {
	cwd, _ := os.Getwd()
	// Stdout is left nil so the unified pipeline routes the startup
	// banner to os.Stderr (the safe default — it never collides with
	// `--filename -`, which streams rendered bytes to stdout). Tests
	// inject their own opts.Stdout when they need to capture banner
	// output for assertions.
	return runWith(ctx, runOptions{
		Args:       args,
		Env:        os.Environ(),
		EventPath:  os.Getenv("GITHUB_EVENT_PATH"),
		OutputDir:  defaultOutputDir(cwd),
		WorkingDir: cwd,
	})
}

// runOptions captures everything Run touches in the real world so tests
// can inject deterministic values. Run fills these from os.Environ /
// os.Stdout / etc; tests construct them explicitly.
type runOptions struct {
	Env        []string
	Stdout     io.Writer
	Stderr     io.Writer // test seam: when non-nil, banner is written here instead of os.Stderr.
	EventPath  string
	OutputDir  string
	WorkingDir string
	Args       []string                                                        // CLI flag args; nil when no CLI invocation
	Flags      *CLIFlags                                                       // test seam: when non-nil, skips ParseFlags(opts.Args)
	BuildDeps  func(ctx context.Context, inv *Invocation) (engine.Deps, error) // optional override for tests
}

func runWith(ctx context.Context, opts runOptions) error {
	// 1. Skip detection.
	if opts.EventPath != "" {
		if skip, reason := shouldSkip(opts.EventPath); skip {
			slog.Info("metrics-cli skipped", "reason", reason)
			return nil
		}
	}

	// 2. Parse CLI flags (or use the test-injected ones).
	cf := opts.Flags
	if cf == nil {
		parsed, err := ParseFlags(opts.Args)
		if err != nil {
			return fmt.Errorf("action: cli flags: %w", err)
		}
		cf = parsed
	}

	// 3. Assemble inputs with source attribution.
	env := envSliceToMap(opts.Env)
	inputs, sources, err := assembleInputs(cf, env)
	if err != nil {
		return err
	}

	// 4. Build invocation.
	inv, ierr := newInvocation(inputs, env, opts.OutputDir)
	if ierr != nil {
		return ierr
	}

	// 4b. Source attribution for the GITHUB_TOKEN fallback path.
	// newInvocation seeds inputs["token"] from env["GITHUB_TOKEN"] when
	// no INPUT_TOKEN was supplied; assembleInputs has already run by
	// then so the sources map misses the attribution. Patch it now so
	// the debug log records where the token actually came from
	// (PR #651 cap-1 review SHOULD-FIX #2).
	if _, hasTokenSource := sources["token"]; !hasTokenSource {
		if _, set := inputs["token"]; set && env["GITHUB_TOKEN"] != "" {
			sources["token"] = "env(GITHUB_TOKEN)"
		}
	}

	// 6. Output_action validation — fail-fast before any API call.
	if verr := DefaultRegistry().Validate(inv.OutputAction); verr != nil {
		return verr
	}

	// 6b. M7: repository template requires the `repo` input. Validate
	// before deps so SC-003 holds (5-second exit, zero API calls).
	if verr := validateRepositoryInput(inv); verr != nil {
		return verr
	}

	// 6c. Missing-token guard (#647). Surfaces the canonical
	// "set GITHUB_TOKEN" diagnostic before deps build so the user
	// does not see the deeper "token does not match recognized prefix"
	// returned by the auth layer.
	if terr := requireTokenUnlessMocked(inv); terr != nil {
		return terr
	}

	// 7. Source attribution debug log — names the layer each resolved
	// input came from (env / inputs_json / flag / config).
	// Emitted once per invocation; only fires when slog debug level is
	// enabled, so production runs pay no cost.
	logResolvedSources(sources)

	// 7b. Print the startup banner BEFORE deps build / token validation
	// so it reaches users even when those later stages fail. Always
	// goes to stderr so it can never contaminate the rendered output
	// stream — `--filename -` streams the SVG/PNG payload to stdout,
	// and a banner prepended to that stream would corrupt PNG bytes
	// outright and pollute committed SVG diffs. stderr keeps the banner
	// human-visible in a terminal (and in GitHub Actions logs, which
	// capture both streams) while leaving the data path clean for
	// redirection and pipelines.
	emitBanner(opts, inv, inputs)

	// 8. Build engine deps (real or mocked).
	var deps engine.Deps
	if opts.BuildDeps != nil {
		deps, err = opts.BuildDeps(ctx, inv)
	} else {
		deps, err = defaultBuildDeps(ctx, inv)
	}
	if err != nil {
		return fmt.Errorf("action: build deps: %w", err)
	}

	// 9. Token validation.
	if !inv.UseMockedData {
		validator := &TokenValidator{
			Token:         inv.Token,
			REST:          deps.REST,
			UseMockedData: inv.UseMockedData,
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

	// Banner already emitted at step 7b above.

	// 11. Notice — best-effort newer-version hint.
	if inv.NoticeReleases && deps.REST != nil {
		if msg := CheckLatestRelease(ctx, deps.REST, "mjun0812/github-metrics", engine.Version()); msg != "" {
			slog.Info(msg)
		}
	}

	// 12. Compute + write output.
	if inv.PerPlugin {
		if perr := runPerPluginDispatch(ctx, inv, deps); perr != nil {
			return fmt.Errorf("action: per-plugin dispatch: %w", perr)
		}
		return nil
	}

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

	// 13. Write output. ResolveOutputWriter handles both file targets and
	// the "-" stdout sentinel.
	target := targetOutputPath(inv)
	w, closeFn, oerr := ResolveOutputWriter(target, inv.Format)
	if oerr != nil {
		return fmt.Errorf("action: write output: %w", oerr)
	}
	if _, werr := w.Write(res.Output); werr != nil {
		_ = closeFn()
		return fmt.Errorf("action: write output: %w", werr)
	}
	if cerr := closeFn(); cerr != nil {
		return fmt.Errorf("action: write output: %w", cerr)
	}

	// 14. metrics_sha output (always set, even on dryrun + skipped Committer).
	sha, hashErr := render.Hash(string(res.Output))
	if hashErr != nil {
		slog.Warn("render.Hash failed; metrics_sha output skipped", "err", hashErr)
	} else if oerr := SetOutput("metrics_sha", sha); oerr != nil {
		slog.Warn("metrics_sha output write failed", "err", oerr)
	}

	// 15. Committer — only when not dryrun and output_action != none.
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

// emitBanner writes the startup banner to opts.Stderr (preferred test
// seam) / opts.Stdout (legacy test seam) / os.Stderr (production).
// Extracted from runWith so the early-exit paths and the post-deps
// path can share the same destination logic.
func emitBanner(opts runOptions, inv *Invocation, inputs map[string]any) {
	bannerOut := io.Writer(os.Stderr)
	switch {
	case opts.Stderr != nil:
		bannerOut = opts.Stderr
	case opts.Stdout != nil:
		bannerOut = opts.Stdout
	}
	PrintBanner(bannerOut, BannerInfo{
		Version:     engine.Version(),
		Template:    inv.Template,
		Plugins:     sortedTruthyPluginGates(inputs),
		TokenMasked: inv.Token.String(),
		GoVersion:   runtime.Version(),
		OSArch:      runtime.GOOS + "/" + runtime.GOARCH,
	})
}

// runCLIWith is a test-only seam that drives runWith with a pre-built
// CLIFlags. The unified pipeline normally constructs the flags via
// ParseFlags(opts.Args); tests skip that step so they can pin
// invocation shape directly without spelling out CLI arg strings.
func runCLIWith(ctx context.Context, cf *CLIFlags, opts runOptions) error {
	opts.Flags = cf
	return runWith(ctx, opts)
}

// assembleInputs merges the three input layers — env (INPUT_<UPPER> +
// INPUTS JSON), --config YAML, CLI flags — into a single map. CLI flags
// win.
//
// The sources map records the originating layer of each resolved key so
// logResolvedSources can emit the per-key attribution debug log.
func assembleInputs(cf *CLIFlags, env map[string]string) (map[string]any, map[string]string, error) {
	inputs := map[string]any{}
	sources := map[string]string{}

	// Layer 1: env (INPUT_<UPPER> + INPUTS JSON). Suppressed by --no-env.
	if !cf.NoEnv {
		parsed, err := ParseInputs(env)
		if err != nil {
			return nil, nil, fmt.Errorf("action: parse inputs: %w", err)
		}
		for k, v := range parsed {
			// Skip empty-string env values: a real GitHub Actions runner
			// emits `INPUT_<KEY>=` for every unset workflow input
			// (~30+ keys for this action), and recording them as
			// "from env" pollutes the source-attribution debug log
			// with values the user did not actually set. Pairs with the
			// same guard in cmd/metrics-cli/main.go::hasActionInputs.
			// PR #651 cap-1 review SHOULD-FIX #1.
			if s, ok := v.(string); ok && s == "" {
				continue
			}
			inputs[k] = v
			sources[k] = "env"
		}
		// INPUTS JSON wins over INPUT_<UPPER> on collision; re-mark
		// those keys so the source log distinguishes the two channels.
		if raw, ok := env["INPUTS"]; ok && raw != "" {
			var decoded map[string]any
			if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
				for k := range decoded {
					sources[strings.ToLower(k)] = "inputs_json"
				}
			}
		}
	}

	// Layer 2: --config YAML overlay (beats env, loses to CLI flag).
	if cf.Config != "" {
		ycfg, yerr := LoadYAMLConfig(cf.Config)
		if yerr != nil {
			return nil, nil, yerr
		}
		for k, v := range ycfg {
			inputs[k] = v
			sources[k] = "config"
		}
	}

	// Layer 3: CLI flag overrides (highest priority).
	applied := cf.applyFlagsOver(inputs)
	for _, k := range applied {
		sources[k] = "flag"
	}

	return inputs, sources, nil
}

// logResolvedSources emits a single slog.Debug record whose attrs name
// the originating layer of every resolved input key. The attrs are
// sorted alphabetically by key so log diffs stay deterministic across
// runs. Skipped silently when the resolved map is empty.
func logResolvedSources(sources map[string]string) {
	if len(sources) == 0 {
		return
	}
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attrs := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		attrs = append(attrs, k, sources[k])
	}
	slog.Debug("inputs resolved", attrs...)
}

// runPerPluginDispatch runs ComputePerPlugin and writes one SVG file per
// plugin into inv.OutputDir. Plugin-local failures (PerPluginResult.Error != nil)
// log a warning and skip that plugin's file but do not abort the others.
func runPerPluginDispatch(ctx context.Context, inv *Invocation, deps engine.Deps) error {
	results, err := engine.ComputePerPlugin(ctx, engine.Request{
		Login:    inv.Login,
		Repo:     stringInput(inv.Inputs, "repo", ""),
		Account:  accountForTemplate(inv.Template),
		Template: inv.Template,
		Format:   inv.Format,
		Inputs:   inv.Inputs,
	}, deps)
	if err != nil {
		return err
	}

	if mkErr := os.MkdirAll(inv.OutputDir, 0o755); mkErr != nil { //nolint:gosec
		return fmt.Errorf("mkdir -p %s: %w", inv.OutputDir, mkErr)
	}

	for _, pr := range results {
		if pr.Error != nil {
			slog.Warn("per-plugin render failed; skipping file",
				"plugin", pr.Plugin, "err", pr.Error)
			continue
		}
		if len(pr.Output) == 0 {
			slog.Warn("per-plugin render produced empty output; skipping file",
				"plugin", pr.Plugin)
			continue
		}
		// Defense in depth: refuse to write a per-plugin file whose slug
		// could escape OutputDir. Plugin slugs coming from the in-process
		// registry are already safe; this guards against any future caller
		// that supplies a slug from an untrusted source.
		if !isValidPluginSlug(pr.Plugin) {
			slog.Warn("per-plugin slug rejected as unsafe; skipping file",
				"plugin", pr.Plugin)
			continue
		}
		path := filepath.Join(inv.OutputDir, pr.Plugin+".svg")
		if werr := os.WriteFile(path, pr.Output, 0o600); werr != nil {
			slog.Warn("per-plugin write failed", "plugin", pr.Plugin, "path", path, "err", werr)
		}
	}
	return nil
}

// targetOutputPath chooses the destination for the unified Write step.
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

// requireTokenUnlessMocked surfaces the canonical "set GITHUB_TOKEN"
// diagnostic when no token reached newInvocation through either
// inputs["token"] (= INPUT_TOKEN) or the env["GITHUB_TOKEN"] fallback.
//
// Runs before deps construction so the user sees the helpful
// "set GITHUB_TOKEN" guidance instead of the auth layer's deeper
// "token does not match recognized prefix" message (#647).
//
// `use_mocked_data` (offline demo) and MOCKED_TOKEN bypass the guard
// — the validator stage 1 path that handled this before still gates
// the deeper check on the same condition.
func requireTokenUnlessMocked(inv *Invocation) error {
	if inv == nil || inv.UseMockedData {
		return nil
	}
	if inv.Token.Reveal() != "" {
		return nil
	}
	return &InputError{
		Key: "token",
		Msg: tokenMissingMsg,
	}
}

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

func defaultOutputDir(cwd string) string {
	// Action mode writes to /renders inside the Docker container.
	// Falls back to the supplied working directory when /renders is
	// absent (= running outside the container, e.g., local CLI / tests).
	if _, err := os.Stat("/renders"); err == nil {
		return "/renders"
	}
	if cwd != "" {
		return cwd
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

func newInvocation(inputs map[string]any, env map[string]string, outputDir string) (*Invocation, error) {
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
	// The action.yml default for `config_output` is "auto" ("use the
	// template's default format"), and the Actions runner forwards
	// defaults as INPUT_CONFIG_OUTPUT=auto. Every adopted template
	// defaults to SVG, so resolve it here before the format reaches
	// filename resolution and template.CheckFormat.
	format := stringInput(inputs, "config_output", "svg")
	if format == "auto" {
		format = "svg"
	}
	inv := &Invocation{
		Inputs:           inputs,
		Template:         stringInput(inputs, "template", "classic"),
		Login:            stringInput(inputs, "user", ""),
		Format:           format,
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
	// Token resolution chain (#647):
	//
	//  1. inputs["token"] — populated from INPUT_TOKEN via ParseInputs
	//     (Action runtime: `with: token:` becomes INPUT_TOKEN by runner
	//     convention; local CLI: user can still export INPUT_TOKEN).
	//  2. env["GITHUB_TOKEN"] — the canonical convention used by gh
	//     CLI, GitHub Actions, and most tooling. The fallback covers
	//     direct CLI invocations and workflow steps that set
	//     `env: GITHUB_TOKEN:` instead of the action input.
	//
	// Neither set is not fatal here — runWith surfaces the missing-token
	// diagnostic before deps build (the auth layer's "token does not
	// match recognized prefix" message would be less helpful than the
	// canonical "set GITHUB_TOKEN" guidance).
	tokenRaw := stringInput(inputs, "token", "")
	if tokenRaw == "" {
		tokenRaw = env["GITHUB_TOKEN"]
		if tokenRaw != "" {
			inputs["token"] = tokenRaw
		}
	}
	inv.Token = config.NewToken(tokenRaw)
	inv.RetryPolicy = RetryPolicy{
		Retries: intInput(inputs, "retries", DefaultRetries),
		Delay:   durationSecInput(inputs, "retries_delay", DefaultRetryDelay),
	}
	inv.OutputRetryPolicy = RetryPolicy{
		Retries: intInput(inputs, "retries_output_action", DefaultOutputRetries),
		Delay:   durationSecInput(inputs, "retries_delay_output_action", DefaultOutputRetryDelay),
	}

	// Resolve filename wildcard.
	rawFilename := stringInput(inputs, "filename", "github-metrics.*")
	resolved, ferr := WildcardFilename(rawFilename, inv.Format)
	if ferr != nil {
		return nil, fmt.Errorf("action: filename: %w", ferr)
	}
	inv.OutputFilename = resolved

	// Per-plugin mode detection.
	// Combined mode is triggered by:
	//   1. explicit `combined: yes` input, OR
	//   2. an explicit non-default filename (i.e. the user passed --filename foo.svg), OR
	//   3. stdout mode (filename == "-").
	// Everything else defaults to per-plugin mode.
	filenameIsDefault := rawFilename == "github-metrics.*" || rawFilename == ""
	combined := boolInput(inputs, "combined", false) ||
		(!filenameIsDefault && inv.OutputFilename != "") ||
		inv.OutputFilename == "-"
	inv.PerPlugin = !combined

	// Resolve per-plugin output directory.
	if inv.PerPlugin {
		perPluginDir := stringInput(inputs, "output_dir", "")
		if perPluginDir != "" {
			inv.OutputDir = perPluginDir
		} else if inv.OutputDir == "" || inv.OutputDir == "." {
			// Default per-plugin output directory.
			inv.OutputDir = "./metrics-renders"
		}
	}

	// Resolve repo + run id from GitHub Actions runner env vars. These
	// fields populate when the env var is set (Action runtime) and stay
	// empty otherwise (local CLI); the committer reads them when present.
	if repo, ok := env["GITHUB_REPOSITORY"]; ok && repo != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			inv.RepoOwner = parts[0]
			inv.RepoName = parts[1]
		}
	}
	inv.RunID = env["GITHUB_RUN_ID"]

	// Expand committer_message placeholders: ${filename} resolves to the
	// rendered output filename (upstream parity), ${run} to the workflow
	// run id (GITHUB_RUN_ID; port-local extension for the CLI default).
	// Both defaults (action.yml + CLI) embed these, so without expansion
	// commits would carry literal ${filename} / ${run}.
	inv.CommitterMessage = expandCommitterMessage(inv.CommitterMessage, inv.OutputFilename, inv.RunID)

	// Login fallback: GITHUB_ACTOR (set by the GitHub Actions runner).
	// In local CLI invocations the env var is typically absent, so the
	// fallback no-ops and the missing-user error below fires as before.
	if inv.Login == "" {
		inv.Login = env["GITHUB_ACTOR"]
	}

	if inv.Login == "" {
		return nil, errors.New("action: input 'user' (or GITHUB_ACTOR fallback) is required")
	}

	// Per-plugin mode is incompatible with the committer (the committer is
	// single-file by construction). Fail fast so users do not silently lose
	// committed output after the default-mode flip. Users who want per-plugin
	// output must set `output_action: none` explicitly, or opt back into
	// combined mode via `combined: yes` / a non-default `filename`.
	if inv.PerPlugin && !inv.Dryrun && inv.OutputAction != "none" {
		return nil, fmt.Errorf("action: per-plugin mode is incompatible with output_action=%q; set `output_action: none` to write SVGs locally without committing, or set `combined: yes` to restore the single-file behaviour expected by the committer", inv.OutputAction)
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
	// depend on a browser. We let engine.Compute lazily allocate the
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
		REST:       rest,
		GraphQL:    gql,
		Render:     renderer,
		HTTPClient: imgClient,
	}, nil
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

// durationSecInput reads an integer input declared in seconds
// (action.yml: `retries_delay` — "Delay between each retry (in
// seconds)") and converts it to a time.Duration.
func durationSecInput(in map[string]any, key string, def time.Duration) time.Duration {
	v, ok := in[key]
	if !ok {
		return def
	}
	secs := intInput(map[string]any{key: v}, key, int(def/time.Second))
	return time.Duration(secs) * time.Second
}

// expandCommitterMessage substitutes the committer_message
// placeholders: every ${filename} becomes the resolved output filename
// (upstream's only documented placeholder) and every ${run} becomes the
// workflow run id — a port-local extension backing the CLI default
// message. Both replacements are global.
func expandCommitterMessage(msg, filename, runID string) string {
	msg = strings.ReplaceAll(msg, "${filename}", filename)
	msg = strings.ReplaceAll(msg, "${run}", runID)
	return msg
}

// writeOutputFile is the historical "mkdir -p + write 0o600" helper
// retained because helpers_test exercises it directly. The runtime
// pipeline now goes through ResolveOutputWriter to keep the stdout
// (`--filename -`) and file paths in a single code branch.
func writeOutputFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // committer-style path under inv.OutputDir
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

// pluginSlugPattern restricts user-supplied plugin slugs to a safe subset
// so that `filepath.Join(OutputDir, slug+".svg")` cannot escape OutputDir
// or write to a privileged path. Plugin registrations themselves follow
// this shape (lowercase ASCII, digits, '_', '-', leading letter).
var pluginSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func isValidPluginSlug(s string) bool {
	return pluginSlugPattern.MatchString(s)
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
