package engine

import (
	"context"
	"errors"
	"fmt"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// dispatchOutput routes Request.Format through the right marshaller and
// returns the (Output, MIME) pair Compute records in Result.
//
// Behavior (see specs/002-output-classic-json/contracts/result-dispatch.md):
//
//   - empty Format: defaults to Template.Metadata().Formats[0] when a
//     template is registered; "json" otherwise.
//   - "json": [Marshal](data) → application/json.
//   - "svg":  tmpl.Run(...) → image/svg+xml. Requires a template.
//   - "png" / "jpeg": M2 interim — emit SVG bytes plus a warn log so
//     callers can already plumb the value through; the real chromedp
//     conversion lands with the M3 rendering pipeline.
//   - anything else: *UnsupportedFormatError.
func dispatchOutput(
	ctx context.Context,
	req Request,
	deps Deps,
	tmpl templates.Template,
	data *plugins.Data,
	pcPartial *templates.PartialContext,
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
		body, err := Marshal(data)
		if err != nil {
			return nil, "", fmt.Errorf("engine: marshal json: %w", err)
		}
		return body, "application/json", nil

	case "svg", "png", "jpeg":
		if tmpl == nil {
			return nil, "", xerrors.NewInputError("template",
				fmt.Errorf("format %q requires a registered template", format))
		}
		out, err := tmpl.Run(ctx, pcPartial)
		if err != nil {
			return nil, "", fmt.Errorf("engine: template %q run: %w", req.Template, err)
		}
		mime := "image/svg+xml"
		switch format {
		case "png":
			mime = "image/png"
			deps.Logger.Warn("engine: png output stages SVG bytes (chromedp conversion lands in M3)",
				"format", format)
		case "jpeg":
			mime = "image/jpeg"
			deps.Logger.Warn("engine: jpeg output stages SVG bytes (chromedp conversion lands in M3)",
				"format", format)
		}
		return []byte(out), mime, nil

	default:
		return nil, "", xerrors.NewUnsupportedFormatError(format, errors.New("engine: dispatch"))
	}
}
