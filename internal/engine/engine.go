// Package engine wires together the core plugin runner and the active
// template into a single Compute call. The engine owns no business
// logic of its own; it sequences pieces from internal/plugins,
// internal/templates, internal/dataprovider, and internal/config.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	// Provider is the dataprovider the engine constructed for the
	// request, returned so callers (integration tests, downstream
	// services) can resolve the canonical user / organization /
	// repository payloads after Compute finishes. The lifetime matches
	// the request scope; callers MUST NOT use it past ctx cancellation.
	Provider plugins.Provider
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
	// goroutines collapse onto a single GraphQL call per resource. The
	// repo argument is set only for the M7 repository template
	// (req.Account == AccountRepository); the Provider uses it to
	// switch the Repositories/RepositorySummary accessors over to the
	// single-repo synthesized result.
	repoInput := repoFromInputs(inputs)
	providerRepo := ""
	if data.Account == plugins.AccountRepository {
		providerRepo = repoInput
	}
	provider := dataprovider.New(
		pluginutil.LoginFromInputs(inputs),
		providerRepo,
		deps.GraphQL,
		deps.REST,
		deps.Logger,
		dataprovider.Options{
			// repositories_skip_private (#656) is a cross-plugin filter
			// honoured by the account-wide paging fetch in fetch.go.
			// Repo-mode (synthesizeRepoResult) bypasses it.
			SkipPrivate: pluginutil.TruthyInput(inputs, "repositories_skip_private"),
		},
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

	// Repository-template subject hydration (#605): when the request
	// targets a single repository the engine resolves the Repo payload
	// up-front and stores it on Data so the RepoRef()-based mode gates
	// (plugins.RequireUserMode / RequireRepoMode) and the repository
	// template partials reading pc.Data.Repo see the same value the
	// Provider memoized. This is NOT eager hydration of aggregated /
	// computed downstream data — Data.Repo is the request subject (the
	// resource being rendered), the same way Data.Account is. All other
	// shared profile/repository fields stay lazy and Provider-only.
	if data.Account == plugins.AccountRepository {
		if providerRepo == "" {
			return nil, xerrors.NewInputError("repo",
				errors.New("engine: repository template requires non-empty `repo` input"))
		}
		repo, err := provider.Repo(ctx)
		if err != nil {
			return nil, fmt.Errorf("engine: repo: %w", err)
		}
		data.SetRepo(repo)
	}

	// Stage 1: core plugin pulls the global config into Data.Config /
	// Data.Computed and is responsible for the per-request setup that
	// the parallel runner expects.
	if _, err := core.Plugin.Run(ctx, pc); err != nil {
		return nil, fmt.Errorf("engine: core: %w", err)
	}

	// Stage 2: the rest of the registered plugins, in parallel. The
	// runner records per-plugin errors under data.Plugins; we surface
	// them via Result.Errors.
	if err := core.RunPlugins(ctx, pc, req.Parallel); err != nil {
		return nil, fmt.Errorf("engine: run-plugins: %w", err)
	}

	res := &Result{Data: data, Provider: provider}
	res.Errors = collectPluginErrors(data)
	if req.Die && len(res.Errors) > 0 {
		return nil, res.Errors[0]
	}

	// Stage 3: dispatch the requested output format. M1 was a noop
	// here; M2 implements [dispatchOutput] which routes "json" to the
	// engine.Marshal path and "svg"/"png"/"jpeg" to Template.Run. The
	// Provider is propagated so partials can resolve the user /
	// organization / repository via the shared singleflight cache.
	pcPartial := &templates.PartialContext{
		Settings: deps.Settings,
		Inputs:   pc.Inputs,
		Logger:   deps.Logger,
		Data:     data,
		Metadata: deps.Metadata,
		Provider: provider,
	}
	output, mime, err := dispatchOutput(ctx, req, deps, tmpl, data, pcPartial, res)
	if err != nil {
		return nil, err
	}
	res.Output = output
	res.MIME = mime

	return res, nil
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

// repoFromInputs returns the `repo` input value, or "" when unset.
// Mirrors the helper the legacy base plugin used; lives here now that
// the engine drives the Provider's repo-mode switch.
func repoFromInputs(inputs map[string]any) string {
	if inputs == nil {
		return ""
	}
	v, ok := inputs["repo"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
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
