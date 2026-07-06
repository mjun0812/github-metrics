package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// dispatchOutput routes Request.Format through the right marshaller
// and returns the (Output, MIME) pair Compute records in Result.
//
// Behavior:
//
//   - empty Format: defaults to Template.Metadata().Formats[0] when a
//     template is registered; "json" otherwise.
//   - "json": [Marshal](data) → application/json.
//   - "svg" / "png" / "jpeg": tmpl.Run + decoration stages + chromedp
//     resize. Renderer comes from deps.Render; nil triggers a lazy
//     *render.Browser construction that is owned + closed by Compute.
//   - anything else: *UnsupportedFormatError.
//
// Stage-level errors (decoration, chromedp) are appended to
// res.Errors and the call falls through to a best-effort response:
// SVG path returns the un-resized SVG, PNG/JPEG path returns
// (nil, "") so the caller can detect the failure via the empty
// Output.
func dispatchOutput(
	ctx context.Context,
	req Request,
	deps Deps,
	tmpl templates.Template,
	data *plugins.Data,
	pcPartial *templates.PartialContext,
	res *Result,
) ([]byte, string, error) {
	format := req.Format
	if format == "" {
		switch {
		case tmpl != nil && tmpl.Metadata() != nil && len(tmpl.Metadata().Formats) > 0:
			format = tmpl.Metadata().Formats[0]
		default:
			format = "json"
		}
	}

	switch format {
	case "json":
		var provider plugins.Provider
		if pcPartial != nil {
			provider = pcPartial.Provider
		}
		body, err := MarshalWithProvider(ctx, data, provider)
		if err != nil {
			return nil, "", fmt.Errorf("engine: marshal json: %w", err)
		}
		return body, "application/json", nil

	case "svg", "png", "jpeg":
		if tmpl == nil {
			return nil, "", xerrors.NewInputError("template",
				fmt.Errorf("format %q requires a registered template", format))
		}
		rendered, err := tmpl.Run(ctx, pcPartial)
		if err != nil {
			return nil, "", fmt.Errorf("engine: template %q run: %w", req.Template, err)
		}

		// Stage 2-4: decoration pipeline. US3 wires the actual
		// stages; here we keep an empty slice so the chain is
		// observable but a no-op.
		decorated, stageErrs := render.Apply(buildPipelineStages(ctx, req.Inputs, imageFetcher(deps)), rendered)
		for _, e := range stageErrs {
			deps.Logger.Warn("engine: pipeline stage error", "err", e)
			if res != nil {
				res.Errors = append(res.Errors, e)
			}
		}

		// Stage 5: resize / convert via the Renderer interface.
		renderer, closeBrowser, err := obtainRenderer(deps)
		if err != nil {
			// Log in addition to res.Errors (#666): on the svg path the
			// run exits 0 with an untrimmed height=99999 SVG, so without
			// this Warn the degradation is invisible in run logs.
			deps.Logger.Warn("engine: renderer init failed; SVG will not be resized",
				"format", format, "err", err)
			if res != nil {
				res.Errors = append(res.Errors, xerrors.NewRetryableError(
					fmt.Errorf("engine: renderer init: %w", err),
				))
			}
			if format == "svg" {
				return []byte(decorated), "image/svg+xml", nil
			}
			return nil, "", nil
		}
		if closeBrowser != nil {
			defer closeBrowser()
		}

		// Apply upstream's `config_padding` default ("0, 8 + 11%") when
		// the caller did not override — width:0 absolute (no padding),
		// height:8 absolute + 11% relative (absorbs minor measurement
		// errors from foreignObject layout so the bottom of plugin
		// content does not clip).
		// org_repo/source/plugins/core/metadata.yml line 347.
		padding := stringSliceInput(req.Inputs, "config.padding")
		if len(padding) == 0 {
			padding = []string{"0, 8 + 11%"}
		}
		out, err := renderer.Resize(ctx, decorated, render.ResizeOpts{
			Convert: format,
			Padding: padding,
			Scripts: stringSliceInput(req.Inputs, "extras.js"),
		})
		if err != nil {
			// Same visibility contract as the renderer-init branch (#666)
			// and per_plugin.go's "resize failed; using unresized SVG".
			deps.Logger.Warn("engine: resize failed; using unresized SVG",
				"format", format, "err", err)
			if res != nil {
				res.Errors = append(res.Errors, fmt.Errorf("engine: resize: %w", err))
			}
			if format == "svg" {
				return []byte(decorated), "image/svg+xml", nil
			}
			return nil, "", nil
		}
		return out.Body, out.MIME, nil

	default:
		return nil, "", xerrors.NewUnsupportedFormatError(format, errors.New("engine: dispatch"))
	}
}

// obtainRenderer returns the configured Renderer when deps.Render is
// non-nil. When nil, a fresh *render.Browser is constructed and the
// returned cleanup func cancels it at the end of the request.
func obtainRenderer(deps Deps) (render.Renderer, func(), error) {
	if deps.Render != nil {
		return deps.Render, nil, nil
	}
	b, err := render.New(render.BrowserOpts{Logger: deps.Logger})
	if err != nil {
		return nil, nil, err
	}
	return b, func() { _ = b.Close() }, nil
}

// buildPipelineStages assembles the decoration stages applied between
// Template.Run and Renderer.Resize. The chain is fixed-order:
//
//  1. octicon       — always enabled. Replaces `:octicon-<name>(-<size>)?:`
//     placeholders with the embedded SVG fragment.
//  2. image-inline  — only when an ImageFetcher is available (i.e. an
//     HTTP client is wired). Rewrites remote `<img src="http(s)://...">`
//     avatars / icons into self-contained `data:` URIs so the SVG
//     renders on GitHub's camo proxy and offline.
//  3. css           — when the "css" optimization pass is requested.
//     Purges unused selectors and minifies the surviving rules.
//  4. xml           — when the "xml" optimization pass is requested.
//     Re-indents the document with two-space indentation.
//
// A pass is "requested" when the upstream `optimize` input lists it
// (default "css, xml") or the explicit `svg.optimize.<pass>` boolean is
// truthy — see [optimizeEnabled]. Honoring the `optimize` input here is
// what wires the upstream default through: the action / CLI loader only
// ever produces the `optimize` key, never the `svg.optimize.*` form.
//
// Each stage is best-effort: errors land in res.Errors (via Apply)
// and the input is forwarded unchanged to the next stage so a
// localized failure does not break the SVG.
func buildPipelineStages(ctx context.Context, inputs map[string]any, fetcher render.ImageFetcher) []render.PipelineStage {
	stages := []render.PipelineStage{
		{Name: "octicon", Run: render.ReplaceOcticons},
	}
	if fetcher != nil {
		stages = append(stages, render.InlineImagesStage(ctx, fetcher))
	}
	if optimizeEnabled(inputs, "css") {
		stages = append(stages, render.PipelineStage{Name: "css", Run: render.OptimizeCSS})
	}
	if optimizeEnabled(inputs, "xml") {
		stages = append(stages, render.PipelineStage{Name: "xml", Run: render.FormatXML})
	}
	return stages
}

// optimizeEnabled reports whether the named optimization pass should
// run. The wired passes are "css" and "xml"; "svg" (SVGO) is accepted
// by the input grammar but not yet implemented, so buildPipelineStages
// never queries it. It accepts two input shapes and OR's them:
//
//   - the upstream `optimize` input — a comma-separated list normalized
//     to []string (metadata default "css, xml"); this is the only form
//     the action / CLI input loader emits, so it carries the real
//     upstream default into the render pipeline.
//   - the explicit `svg.optimize.<pass>` boolean — set directly by
//     integration tests and advanced callers that bypass the loader.
//
// Either form turning the pass on is sufficient.
func optimizeEnabled(inputs map[string]any, pass string) bool {
	if asBool(inputs, "svg.optimize."+pass) {
		return true
	}
	// The `optimize` input may arrive as a normalized []string
	// ({"css","xml"}) or as a raw comma-separated string ("css,xml")
	// when it comes straight from an INPUT_OPTIMIZE env var, so split
	// each element defensively before matching.
	for _, p := range stringSliceInput(inputs, "optimize") {
		for part := range strings.SplitSeq(p, ",") {
			if strings.EqualFold(strings.TrimSpace(part), pass) {
				return true
			}
		}
	}
	return false
}

// imageFetcher adapts deps.HTTPClient to the render.ImageFetcher
// interface the inline stage consumes. It returns a nil interface when
// no HTTP client is wired (e.g. the mocked-data path), which the
// pipeline reads as "skip image inlining" — guarding against the
// typed-nil interface trap that a direct *httpx.Client assignment would
// hit.
func imageFetcher(deps Deps) render.ImageFetcher {
	if deps.HTTPClient == nil {
		return nil
	}
	return deps.HTTPClient
}

// asBool inspects the normalized Inputs map for a boolean-shaped
// value at `key`. The upstream-compatible input loader produces real
// bool values, but we also tolerate the truthy strings "true" and
// "1" so the dispatch survives test fixtures that bypass the loader.
func asBool(inputs map[string]any, key string) bool {
	if inputs == nil {
		return false
	}
	switch v := inputs[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

// stringSliceInput extracts a slice-of-strings input from the
// normalized Inputs map. The upstream input shapes can be either a
// single string or a list; we hand both forms unchanged to the
// downstream Resize call which knows how to interpret them
// (parsePadding handles both).
func stringSliceInput(inputs map[string]any, key string) []string {
	if inputs == nil {
		return nil
	}
	v, ok := inputs[key]
	if !ok {
		return nil
	}
	switch tv := v.(type) {
	case []string:
		out := make([]string, len(tv))
		copy(out, tv)
		return out
	case string:
		if tv == "" {
			return nil
		}
		return []string{tv}
	case []any:
		out := make([]string, 0, len(tv))
		for _, x := range tv {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
