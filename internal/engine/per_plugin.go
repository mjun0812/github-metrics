package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// PerPluginResult holds the rendering output for one plugin in per-plugin mode.
type PerPluginResult struct {
	Plugin string
	Output []byte
	Error  error
}

// ComputePerPlugin runs the full plugin data-collection stage once (via
// Compute with template="noop"), then renders each plugin enabled in
// req.Inputs (plugin_<slug>=truthy) into a standalone SVG file.
// Plugin-local failures are recorded in PerPluginResult.Error and skip
// only that plugin's file; other plugins continue.
//
// Design decision: single Compute pass + partial-render per plugin (efficient).
// Running Compute N times would re-fetch GitHub API data N times.
func ComputePerPlugin(ctx context.Context, req Request, deps Deps) ([]*PerPluginResult, error) {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	// Stage 1+2: data collection without rendering (noop template + json format
	// skips the dispatchOutput template-render stage but still runs core + RunPlugins).
	dataReq := req
	dataReq.Template = "noop"
	dataReq.Format = "json"
	res, err := Compute(ctx, dataReq, deps)
	if err != nil {
		return nil, fmt.Errorf("engine: ComputePerPlugin: data collection: %w", err)
	}

	// Resolve the template used for per-plugin partial renders.
	templateName := req.Template
	if templateName == "" || templateName == "noop" {
		templateName = "classic"
	}
	tmpl, tmplErr := templates.MustGet(templateName)
	if tmplErr != nil {
		return nil, fmt.Errorf("engine: ComputePerPlugin: template %q: %w", templateName, tmplErr)
	}

	// Determine the set of plugins to render.
	targets := resolvePerPluginTargets(req.Inputs)

	// Obtain renderer once so all plugins share a single browser instance.
	renderer, closeBrowser, rerr := obtainRenderer(deps)
	if rerr != nil {
		deps.Logger.Warn("engine: ComputePerPlugin: renderer init failed; SVGs will not be resized", "err", rerr)
		// renderer stays nil; we'll use unresized SVG below.
	}
	if closeBrowser != nil {
		defer closeBrowser()
	}

	stages := buildPipelineStages(ctx, req.Inputs, imageFetcher(deps))

	results := make([]*PerPluginResult, 0, len(targets))
	for _, slug := range targets {
		pr := renderOnePlugin(ctx, slug, req, deps, tmpl, res, renderer, stages)
		results = append(results, pr)
	}
	return results, nil
}

// renderOnePlugin renders a single plugin's partial into a standalone SVG.
func renderOnePlugin(
	ctx context.Context,
	slug string,
	req Request,
	deps Deps,
	tmpl templates.Template,
	res *Result,
	renderer render.Renderer,
	stages []render.PipelineStage,
) *PerPluginResult {
	pr := &PerPluginResult{Plugin: slug}

	// Check for plugin-level error recorded during data collection.
	if raw, present := res.Data.GetPlugin(slug); present {
		if e, isErr := raw.(error); isErr {
			pr.Error = fmt.Errorf("plugin %q failed during data collection: %w", slug, e)
			return pr
		}
	}

	// Build single-plugin inputs: disable all other plugin gates,
	// disable base sections and metadata footer via base="".
	singleInputs := singlePluginInputs(req.Inputs, slug)

	pcPartial := &templates.PartialContext{
		Settings: deps.Settings,
		Inputs:   singleInputs,
		Logger:   deps.Logger,
		Data:     res.Data,
		Metadata: deps.Metadata,
		Provider: res.Provider,
	}

	rendered, rerr := tmpl.Run(ctx, pcPartial)
	if rerr != nil {
		pr.Error = fmt.Errorf("engine: per-plugin render %q: %w", slug, rerr)
		return pr
	}

	// Decoration pipeline (octicon replacement, image inlining, CSS/XML opt).
	decorated, stageErrs := render.Apply(stages, rendered)
	for _, e := range stageErrs {
		deps.Logger.Warn("engine: per-plugin pipeline stage error", "plugin", slug, "err", e)
	}

	if renderer == nil {
		// No renderer available — return unresized SVG.
		pr.Output = []byte(decorated)
		return pr
	}

	// Resize / trim via the Renderer (chromedp in production, FakeRenderer in tests).
	padding := stringSliceInput(req.Inputs, "config.padding")
	if len(padding) == 0 {
		padding = []string{"0, 8 + 11%"}
	}
	resized, resizeErr := renderer.Resize(ctx, decorated, render.ResizeOpts{
		Convert: "svg",
		Padding: padding,
		Scripts: stringSliceInput(req.Inputs, "extras.js"),
	})
	if resizeErr != nil {
		deps.Logger.Warn("engine: per-plugin resize failed; using unresized SVG", "plugin", slug, "err", resizeErr)
		pr.Output = []byte(decorated)
		return pr
	}

	pr.Output = resized.Body
	return pr
}

// resolvePerPluginTargets returns the sorted list of plugin slugs to render.
// Collects every top-level `plugin_<slug>` gate that is truthy in inputs.
func resolvePerPluginTargets(inputs map[string]any) []string {
	var out []string
	for k, v := range inputs {
		if !strings.HasPrefix(k, "plugin_") {
			continue
		}
		// Top-level gate only (plugin_<slug>, not plugin_<slug>_<opt>).
		rest := strings.TrimPrefix(k, "plugin_")
		if strings.Contains(rest, "_") {
			continue
		}
		if perPluginIsTruthy(v) {
			out = append(out, rest)
		}
	}
	sort.Strings(out)
	return out
}

// singlePluginInputs returns a copy of inputs modified so that:
//   - only the target plugin's gate (plugin_<slug>) is truthy,
//   - all other top-level plugin gates are disabled,
//   - every chrome section is explicitly off (#640) so a leaked
//     chrome_X from the caller does not pull extra chrome into the
//     per-plugin SVG.
func singlePluginInputs(inputs map[string]any, slug string) map[string]any {
	out := make(map[string]any, len(inputs)+8)
	for k, v := range inputs {
		if strings.HasPrefix(k, "plugin_") {
			rest := strings.TrimPrefix(k, "plugin_")
			if !strings.Contains(rest, "_") {
				// Top-level gate: enable only the target.
				if rest == slug {
					out[k] = true
				} else {
					out[k] = false
				}
				continue
			}
		}
		out[k] = v
	}
	// Enable the target plugin gate (in case it was absent from inputs).
	out["plugin_"+slug] = true
	// Suppress every chrome section explicitly so a caller-supplied
	// chrome_X=yes cannot leak into the per-plugin SVG.
	for _, section := range []string{
		"chrome_header", "chrome_activity", "chrome_community",
		"chrome_repositories", "chrome_metadata", "chrome_introduction",
	} {
		out[section] = false
	}
	return out
}

// perPluginIsTruthy mirrors the action package helper; duplicated here to
// avoid importing the action package (which imports engine — import cycle).
func perPluginIsTruthy(v any) bool {
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
