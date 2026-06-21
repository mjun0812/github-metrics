// Package engine wires together the core plugin runner and the active
// template into a single Compute call. The engine owns no business
// logic of its own; it sequences pieces from internal/plugins,
// internal/templates, and internal/config.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/dataprovider"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/core"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// Request groups the per-call inputs the engine consumes.
type Request struct {
	// Login is the GitHub login the engine should compute metrics for.
	Login string
	// Repo is the repository name when Template == "repository" (M7).
	// Empty for the classic user / organization templates. Combined
	// with Login as `<Login>/<Repo>` to form the GitHub identifier.
	Repo string
	// Template is the registered template name to invoke for output.
	// Pass "noop" (or any registered no-op template) to wire the
	// pipeline without actually rendering anything (M1 default).
	Template string
	// Format is "svg" / "png" / "jpeg" / "json"; the active Template
	// validates it via Check.
	Format string
	// Account hints the kind we are computing for; engine falls back
	// to AccountUser when empty.
	Account plugins.AccountKind
	// Inputs is the normalized input map produced by
	// config.NewInputs(...).ForAction (or ForWeb / ForData).
	Inputs map[string]any
	// Parallel caps the number of plugin goroutines core.RunPlugins
	// spawns. Zero falls back to runtime.GOMAXPROCS.
	Parallel int
	// Die selects the per-plugin failure semantics: when true, the
	// first plugin error aborts the request; when false, errors are
	// collected in Result.Data.Plugins[name] and Compute still returns
	// nil (matching upstream `die=false`).
	Die bool
}

// Result is the engine's output.
type Result struct {
	// Data is the fully populated internal state. M2 onward consumes
	// it via the JSON marshaller and template render path.
	Data *plugins.Data
	// Errors aggregates non-fatal plugin errors when Die == false.
	Errors []error
	// Output is the serialized payload in the requested format.
	// Populated by Compute after the dispatch stage runs. Valid IANA
	// values for the corresponding MIME (see [Result.MIME]):
	//
	//   application/json   when Request.Format == "json"
	//   image/svg+xml      when Request.Format == "svg"  (Template.Run
	//                                                     output after
	//                                                     the M3
	//                                                     decoration
	//                                                     pipeline +
	//                                                     chromedp
	//                                                     svg.Resize)
	//   image/png          when Request.Format == "png"  (real PNG
	//                                                     bytes from
	//                                                     chromedp
	//                                                     page.CaptureScreenshot,
	//                                                     M3+)
	//   image/jpeg         when Request.Format == "jpeg" (same as PNG
	//                                                     with JPEG
	//                                                     format)
	//
	// On Renderer failure for png / jpeg paths the dispatch returns
	// (nil, "") and appends the chromedp error to Result.Errors so
	// callers can detect the failure via the empty Output (FR-018).
	// For svg the dispatch falls back to the un-resized decorated
	// SVG bytes.
	Output []byte
	// MIME is the IANA type that matches Output. Never empty when
	// Output is set; never set when Output is empty.
	MIME string
}

// Deps groups the long-lived collaborators the engine reuses across
// Compute calls. Construct one per process.
type Deps struct {
	Settings   *config.Settings
	Metadata   *config.MetadataLoader
	Logger     *slog.Logger
	HTTPClient *httpx.Client
	REST       *githubapi.REST
	GraphQL    *githubapi.GraphQL
	// Render performs the chromedp-backed SVG resize / convert and is
	// consumed only when Request.Format ∈ {"svg","png","jpeg"}. Nil
	// is permitted: when needed, Compute lazily allocates a default
	// *render.Browser on first use and tears it down at the end of
	// the call. Tests should inject a *render.FakeRenderer so they
	// never start chromium. JSON-format requests never read this
	// field.
	Render render.Renderer
}

// Compute drives a single end-to-end run.
func Compute(ctx context.Context, req Request, deps Deps) (*Result, error) {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if req.Login == "" {
		return nil, xerrors.NewInputError("user", errors.New("engine: login is required"))
	}

	// Template lookup happens before any expensive work. "noop" is a
	// reserved sentinel for tests; the engine does not require a
	// registered noop template, it short-circuits straight to the
	// post-plugin output path.
	var tmpl templates.Template
	if req.Template != "" && req.Template != "noop" {
		t, err := templates.MustGet(req.Template)
		if err != nil {
			return nil, fmt.Errorf("engine: template %q: %w", req.Template, err)
		}
		if err := t.Check(req.Inputs, string(req.Account), req.Format); err != nil {
			return nil, fmt.Errorf("engine: template %q check: %w", req.Template, err)
		}
		tmpl = t
	}

	data := plugins.NewData()
	if req.Account != "" {
		data.Account = req.Account
	} else {
		data.Account = plugins.AccountUser
	}

	// Startup rate-limit gate (#529): refresh /rate_limit once (quota-free)
	// and, per config_rate_limit_guard, wait for reset or fail fast when a
	// pool is below the threshold — instead of starting a render that would
	// silently degrade mid-run (#522). No-op when the mode is "off"
	// (default), the token is mocked / NOT_NEEDED, or deps are absent. On a
	// fail/wait-cap verdict the returned error aborts Compute before any
	// plugin runs, so there is no partial Data to surface it through.
	if err := runRateGate(ctx, req.Inputs, deps); err != nil {
		return nil, err
	}

	inputs := mergeLogin(req.Inputs, req.Login)

	// dataprovider (#603) lazily memoizes the user/organization profile
	// + repository paging + indepth commit calendar fetches each plugin
	// shares. Construct once per Compute so concurrent plugin
	// goroutines collapse onto a single GraphQL call per resource.
	provider := dataprovider.New(
		pluginutil.LoginFromInputs(inputs),
		deps.GraphQL,
		deps.REST,
		deps.Logger,
	)

	pc := &plugins.PluginContext{
		Settings:   deps.Settings,
		Inputs:     inputs,
		Logger:     deps.Logger,
		HTTPClient: deps.HTTPClient,
		REST:       deps.REST,
		GraphQL:    deps.GraphQL,
		Data:       data,
		Metadata:   deps.Metadata,
		Imports:    plugins.NewImports(data),
		Provider:   provider,
		Render:     deps.Render,
	}

	// Stage 1: core plugin pulls the global config into Data.Config /
	// Data.Computed and is responsible for the per-request setup that
	// the parallel runner expects.
	if _, err := core.Plugin.Run(ctx, pc); err != nil {
		return nil, fmt.Errorf("engine: core: %w", err)
	}

	// Stage 2: populate pc.Data from Provider. Plugins read Data.User /
	// Data.Organization / Data.Computed.RepositoryList to avoid
	// per-plugin Provider calls in the common case. Errors here are
	// non-fatal: if the profile is unavailable the parallel plugins will
	// see a nil Data.User and skip gracefully (matching upstream
	// degraded-payload semantics). Repositories failures are handled the
	// same way.
	if err := applyProfile(ctx, pc, req, deps); err != nil {
		deps.Logger.Warn("engine: profile fetch failed; plugins will degrade gracefully",
			"err", err)
	}

	// Stage 3: the rest of the registered plugins, in parallel. The
	// runner records per-plugin errors under data.Plugins; we surface
	// them via Result.Errors.
	if err := core.RunPlugins(ctx, pc, req.Parallel); err != nil {
		return nil, fmt.Errorf("engine: run-plugins: %w", err)
	}

	res := &Result{Data: data}
	res.Errors = collectPluginErrors(data)
	if req.Die && len(res.Errors) > 0 {
		return nil, res.Errors[0]
	}

	// Stage 4: dispatch the requested output format. M1 was a noop
	// here; M2 implements [dispatchOutput] which routes "json" to the
	// engine.Marshal path and "svg"/"png"/"jpeg" to Template.Run.
	pcPartial := &templates.PartialContext{
		Settings: deps.Settings,
		Inputs:   pc.Inputs,
		Logger:   deps.Logger,
		Data:     data,
		Metadata: deps.Metadata,
	}
	output, mime, err := dispatchOutput(ctx, req, deps, tmpl, data, pcPartial, res)
	if err != nil {
		return nil, err
	}
	res.Output = output
	res.MIME = mime

	return res, nil
}

// PerPluginResult holds the output for a single plugin's SVG render.
type PerPluginResult struct {
	PluginName string
	Output     []byte
	MIME       string
	Err        error // non-nil = this plugin failed; others may still succeed
}

// ComputePerPlugin runs the engine for each enabled plugin and produces
// one SVG per plugin. For each plugin in pluginAllowlist (or all enabled
// plugins when allowlist is empty), it calls Compute with a modified input
// set that enables only that one plugin. This reuses the full pipeline
// and guarantees isolation between plugins.
func ComputePerPlugin(ctx context.Context, req Request, deps Deps, pluginAllowlist []string) ([]*PerPluginResult, error) {
	// Determine which plugin gates are enabled in the base inputs.
	enabledPlugins := resolveEnabledPlugins(req.Inputs, pluginAllowlist)

	results := make([]*PerPluginResult, 0, len(enabledPlugins))
	for _, name := range enabledPlugins {
		// Build a copy of inputs with only this plugin enabled.
		singleInputs := make(map[string]any, len(req.Inputs))
		for k, v := range req.Inputs {
			singleInputs[k] = v
		}
		// Disable all plugin gates, then re-enable only this one.
		disableAllPluginGates(singleInputs)
		singleInputs["plugin_"+name] = "yes"
		// Force config_order to only this plugin so the template renders
		// just this panel.
		singleInputs["config_order"] = name

		singleReq := req
		singleReq.Inputs = singleInputs

		res, err := Compute(ctx, singleReq, deps)
		pr := &PerPluginResult{PluginName: name}
		if err != nil {
			pr.Err = err
		} else if res != nil {
			// Check whether this specific plugin recorded an error. Compute
			// records per-plugin errors in Result.Errors (Die=false semantics)
			// rather than returning a non-nil error from Compute itself. We
			// treat a plugin-scoped error as a failure for this result so the
			// caller can skip writing an empty or degraded file.
			pluginErr := pluginError(res, name)
			if pluginErr != nil {
				pr.Err = pluginErr
			} else {
				pr.Output = res.Output
				pr.MIME = res.MIME
			}
		}
		results = append(results, pr)
	}
	return results, nil
}

// pluginError returns the error recorded for the named plugin in Result.Errors,
// or nil if no error was recorded for that plugin. It inspects both the typed
// plugin-slot errors (wrapped as "plugin %q: <err>") and the flat Errors slice.
func pluginError(res *Result, name string) error {
	if res == nil {
		return nil
	}
	prefix := fmt.Sprintf("plugin %q:", name)
	for _, e := range res.Errors {
		if e != nil && strings.HasPrefix(e.Error(), prefix) {
			return e
		}
	}
	return nil
}

// resolveEnabledPlugins returns the plugin names to render in per-plugin
// mode. When allowlist is non-empty it is used as-is; otherwise all plugin_*
// gates set to a truthy value in inputs are included.
func resolveEnabledPlugins(inputs map[string]any, allowlist []string) []string {
	if len(allowlist) > 0 {
		return allowlist
	}
	var names []string
	_ = plugins.Each(func(name string, _ plugins.Plugin) error {
		if name == "core" {
			return nil
		}
		gateKey := "plugin_" + name
		v, ok := inputs[gateKey]
		if !ok {
			return nil
		}
		if isTruthyValue(v) {
			names = append(names, name)
		}
		return nil
	})
	return names
}

// disableAllPluginGates sets every plugin_<name> gate in inputs to "no".
func disableAllPluginGates(inputs map[string]any) {
	for k := range inputs {
		if strings.HasPrefix(k, "plugin_") && !strings.Contains(k[7:], "_") {
			// Only top-level gates (no underscore after the prefix means
			// it is a gate like plugin_languages, not plugin_languages_limit).
			inputs[k] = "no"
		}
	}
}

// isTruthyValue mirrors the action package's isTruthy logic for use in engine.
func isTruthyValue(v any) bool {
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

// mergeLogin injects req.Login into the inputs map so plugins that
// read inputs["user"] see the engine-supplied value. Returns a copy
// when inputs is non-nil; otherwise allocates a single-key map.
func mergeLogin(inputs map[string]any, login string) map[string]any {
	out := make(map[string]any, len(inputs)+1)
	for k, v := range inputs {
		out[k] = v
	}
	if _, present := out["user"]; !present {
		out["user"] = login
	}
	return out
}

// collectPluginErrors walks Data.Plugins looking for stored errors and
// returns them in registration order. Non-fatal errors plugins recorded
// via Data.AppendError (the M4 plumbing for degraded paths like the
// indepth GraphQL 5xx and the repositories paging batch-halving) are
// appended after the per-plugin slot errors.
func collectPluginErrors(d *plugins.Data) []error {
	if d == nil {
		return nil
	}
	var out []error
	if len(d.Plugins) > 0 {
		_ = plugins.Each(func(name string, _ plugins.Plugin) error {
			v, ok := d.GetPlugin(name)
			if !ok {
				return nil
			}
			if err, isErr := v.(error); isErr {
				out = append(out, fmt.Errorf("plugin %q: %w", name, err))
			}
			return nil
		})
	}
	out = append(out, d.SnapshotErrors()...)
	return out
}

// applyProfile fetches the account profile and repository list via
// pc.Provider and writes them into pc.Data so downstream plugins can
// read Data.User / Data.Organization / Data.Computed.RepositoryList
// without issuing per-plugin Provider calls. This replaces the eager
// population that the legacy base plugin performed synchronously before
// the parallel runner started (#605).
//
// For repository template requests (pc.Data.Account == AccountRepository),
// it also fetches Data.Repo via the GraphQL Repository query and
// synthesizes a single-element RepositoryList so user-centric plugins
// (languages, activity, stargazers, etc.) naturally produce repo-scoped
// output, mirroring upstream template.mjs:14-17.
//
// Failures are surfaced to the caller so the engine can decide whether
// to abort or degrade. Partial success (profile OK, repositories fail)
// is possible: the engine logs and continues in both cases.
func applyProfile(ctx context.Context, pc *plugins.PluginContext, req Request, deps Deps) error {
	if pc == nil || pc.Provider == nil || pc.Data == nil {
		return nil
	}

	// Repository template: fetch the single repo via GraphQL, then
	// populate Data.Repo and synthesize a one-element RepositoryList so
	// the user-centric plugins (languages, activity, etc.) operate in
	// repo scope. User profile is also fetched so Data.User is available
	// for header chrome, sponsorships, etc.
	if req.Account == plugins.AccountRepository {
		if req.Login == "" || req.Repo == "" {
			return nil
		}
		repo, err := fetchRepo(ctx, req.Login, req.Repo, deps.REST, deps.GraphQL)
		if err != nil {
			return fmt.Errorf("engine: applyProfile (repo): %w", err)
		}
		pc.Data.Account = plugins.AccountRepository

		// Populate Data.User from Provider for header + sponsorships chrome.
		if u, uerr := pc.Provider.User(ctx); uerr == nil && u != nil {
			pc.Data.User = u
			if repo.Calendar == nil {
				repo.Calendar = u.RecentContributions
			}
		}
		pc.Data.SetRepo(repo)

		// Synthesize a single-element RepositoryList mirroring upstream
		// template.mjs:14-17 so user-centric plugins produce repo-scoped output.
		syntheticRepo := plugins.Repository{
			NameWithOwner: repo.Owner + "/" + repo.Name,
			Description:   repo.Description,
			Stars:         repo.Stargazers,
			Forks:         repo.Forks,
			Watchers:      repo.Watchers,
			Languages:     append([]plugins.LanguageStat(nil), repo.Languages...),
		}
		if repo.PrimaryLanguage != "" {
			lang := plugins.LanguageStat{
				Name:  repo.PrimaryLanguage,
				Color: repo.PrimaryLanguageColor,
			}
			syntheticRepo.Language = &lang
			if len(syntheticRepo.Languages) == 0 {
				syntheticRepo.Languages = []plugins.LanguageStat{{
					Name:  repo.PrimaryLanguage,
					Color: repo.PrimaryLanguageColor,
					Size:  1,
				}}
			}
		}
		pc.Data.Computed.RepositoryList = []plugins.Repository{syntheticRepo}
		pc.Data.Computed.Repositories.Count = 1
		pc.Data.Computed.Repositories.Stargazers = repo.Stargazers
		pc.Data.Computed.Repositories.Forks = repo.Forks
		pc.Data.Computed.Repositories.Watchers = repo.Watchers
		return nil
	}

	// User / organization account: resolve profile and repository list.
	prof, err := pc.Provider.Profile(ctx)
	if err != nil {
		return fmt.Errorf("engine: applyProfile: %w", err)
	}
	switch prof.Kind {
	case plugins.ProfileKindUser:
		pc.Data.User = prof.User
		pc.Data.Account = plugins.AccountUser
	case plugins.ProfileKindOrganization:
		pc.Data.Organization = prof.Organization
		pc.Data.Account = plugins.AccountOrganization
	}

	// Populate repository list. Failures are non-fatal: plugins that
	// depend on RepositoryList will see an empty slice and skip gracefully.
	repos, reposErr := pc.Provider.Repositories(ctx)
	if reposErr == nil && len(repos) > 0 {
		pc.Data.Computed.RepositoryList = repos
		pc.Data.Computed.Repositories.Count = len(repos)
		for _, r := range repos {
			pc.Data.Computed.Repositories.Stargazers += r.Stars
			pc.Data.Computed.Repositories.Forks += r.Forks
			pc.Data.Computed.Repositories.Watchers += r.Watchers
		}
	}

	return nil
}
