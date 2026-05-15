// Package engine wires together the base plugin, the core plugin
// runner, and the active template into a single Compute call. The
// engine owns no business logic of its own; it sequences pieces from
// internal/plugins, internal/templates, and internal/config.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/plugins/core"
	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// Request groups the per-call inputs the engine consumes.
type Request struct {
	// Login is the GitHub login the engine should compute metrics for.
	Login string
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
	//   image/svg+xml      when Request.Format == "svg"
	//   image/png          when Request.Format == "png" (M2: contains
	//                                                    SVG bytes plus
	//                                                    a warn log;
	//                                                    chromedp
	//                                                    conversion
	//                                                    lands in M3)
	//   image/jpeg         when Request.Format == "jpeg" (same as PNG
	//                                                    for M2)
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

	pc := &plugins.PluginContext{
		Settings:   deps.Settings,
		Inputs:     mergeLogin(req.Inputs, req.Login),
		Logger:     deps.Logger,
		HTTPClient: deps.HTTPClient,
		REST:       deps.REST,
		GraphQL:    deps.GraphQL,
		Data:       data,
		Metadata:   deps.Metadata,
		Imports:    plugins.NewImports(data),
	}

	// Stage 1: base plugin. This populates data.User and the
	// repository totals every downstream plugin keys off of.
	if _, err := base.Plugin.Run(ctx, pc); err != nil {
		return nil, fmt.Errorf("engine: base: %w", err)
	}

	// Stage 2: core plugin pulls the global config into Data.Config /
	// Data.Computed and is responsible for the per-request setup that
	// the parallel runner expects.
	if _, err := core.Plugin.Run(ctx, pc); err != nil {
		return nil, fmt.Errorf("engine: core: %w", err)
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
	output, mime, err := dispatchOutput(ctx, req, deps, tmpl, data, pcPartial)
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

// collectPluginErrors walks Data.Plugins looking for stored errors and
// returns them in registration order.
func collectPluginErrors(d *plugins.Data) []error {
	if d == nil || len(d.Plugins) == 0 {
		return nil
	}
	var out []error
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
	return out
}
