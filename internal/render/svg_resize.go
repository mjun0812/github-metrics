package render

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// jsPrepareTemplate runs the pre-measurement preparation step: it
// executes the caller-supplied scripts and disables the SVG
// animations so the upcoming settle sleep can shake out into a stable
// layout. We split this away from the measurement JS so chromedp can
// drive the settle delay via its own Sleep action rather than relying
// on the Runtime.evaluate awaitPromise behaviour, which has been
// flaky against modern desktop Chrome (137+) when the IIFE returns a
// JSON-stringified payload.
//
// %s slots: padding JSON, scripts JSON.
const jsPrepareTemplate = `(() => {
  const scripts = JSON.parse(%s);
  for (const script of scripts) {
    try {
      new Function("document", script)(document);
    } catch (e) {
      console.debug("script error: " + e);
    }
  }
  const svg = document.querySelector("svg");
  if (!svg) {
    throw new Error("no <svg> root in document");
  }
  if (!svg.classList.contains("no-animations")) {
    svg.classList.add("no-animations");
    svg.dataset.metricsAnimationsToggled = "1";
  }
  return true;
})()`

// jsMeasureTemplate runs after the settle sleep: it looks up the
// `#metrics-end` anchor, applies padding, rewrites the SVG's `height`
// attribute (unless it is `auto`), and returns the serialized SVG
// plus the post-padding dimensions as a JSON STRING. Returning a
// JSON-encoded string survives Runtime.evaluate's deep serialization
// without hitting the await-promise pitfall.
//
// %s slots: padding JSON. %d slot: ignored — the settle delay is
// owned by chromedp.Sleep outside this template.
const jsMeasureTemplate = `(() => {
  const padding = JSON.parse(%s);
  const svg = document.querySelector("svg");
  if (!svg) {
    throw new Error("no <svg> root in document at measurement time");
  }
  const endNode = svg.querySelector("#metrics-end");
  if (!endNode) {
    throw new Error("missing #metrics-end measurement anchor");
  }
  // Upstream uses #metrics-end.getBoundingClientRect() but for an
  // empty <g> the bbox width is 0. We pull width from the <svg> root
  // (which carries the intrinsic content width) and height from the
  // anchor's vertical position.
  const svgRect = svg.getBoundingClientRect();
  const endRect = endNode.getBoundingClientRect();
  let height = endRect.y - svgRect.y;
  let width  = svgRect.width;
  height = Math.max(1, Math.ceil(height * padding.height + padding.absoluteHeight));
  width  = Math.max(1, Math.ceil(width  * padding.width  + padding.absoluteWidth));
  if (svg.getAttribute("height") !== "auto") svg.setAttribute("height", String(height));
  if (svg.dataset.metricsAnimationsToggled === "1") {
    svg.classList.remove("no-animations");
    delete svg.dataset.metricsAnimationsToggled;
  }
  return JSON.stringify({
    resized: new XMLSerializer().serializeToString(svg),
    width: width,
    height: height,
  });
})()`

// jsResult is the shape jsTemplate JSON-stringifies before returning.
// We deserialize it Go-side so the caller can act on the typed
// dimensions and serialized SVG body.
type jsResult struct {
	Resized string `json:"resized"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

// Resize implements Renderer. It opens a fresh chromedp tab, injects
// the rendered SVG into about:blank, runs the upstream-derived
// measurement JS, optionally rasterizes to PNG/JPEG, and returns the
// final bytes plus the post-padding dimensions.
//
// Chromedp errors (start failure, evaluate failure, tab teardown)
// are wrapped as *xerrors.RetryableError so the engine dispatch can
// surface them via Result.Errors while preserving the typed shape
// callers match on with errors.As.
func (b *Browser) Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error) {
	opts, err := opts.normalize()
	if err != nil {
		return ResizeResult{}, err
	}

	tabCtx, cancel, err := b.NewTab(ctx)
	if err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: open tab: %w", err))
	}
	defer cancel()

	pad := parsePadding(opts.Padding, b.opts.Logger)
	padJSON, _ := json.Marshal(map[string]float64{
		"width":          pad.width,
		"height":         pad.height,
		"absoluteWidth":  pad.absoluteWidth,
		"absoluteHeight": pad.absoluteHeight,
	})
	scriptsJSON, _ := json.Marshal(opts.Scripts)
	if string(scriptsJSON) == "null" {
		scriptsJSON = []byte("[]")
	}

	prepareJS := fmt.Sprintf(jsPrepareTemplate, quoteForJS(scriptsJSON))
	measureJS := fmt.Sprintf(jsMeasureTemplate, quoteForJS(padJSON))

	var prepareOK bool
	var rawJSON string
	err = chromedp.Run(
		tabCtx,
		chromedp.EmulateViewport(int64(opts.ViewportWidth), int64(opts.ViewportHeight)),
		chromedp.Navigate("about:blank"),
		setDocumentContent(in),
		chromedp.Evaluate(prepareJS, &prepareOK),
		chromedp.Sleep(opts.SettleDelay),
		chromedp.Evaluate(measureJS, &rawJSON),
	)
	if err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: chromedp evaluate: %w", err))
	}
	var parsed jsResult
	if decodeErr := json.Unmarshal([]byte(rawJSON), &parsed); decodeErr != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: decode jsResult (%q): %w", rawJSON, decodeErr))
	}

	if opts.Convert == "svg" {
		return ResizeResult{
			Body:   []byte(parsed.Resized),
			Width:  parsed.Width,
			Height: parsed.Height,
			MIME:   mimeForConvert("svg"),
		}, nil
	}

	// PNG / JPEG branch: re-snapshot the page with the finalized
	// dimensions and the chosen image format.
	var imgBuf []byte
	format := page.CaptureScreenshotFormatPng
	if opts.Convert == "jpeg" {
		format = page.CaptureScreenshotFormatJpeg
	}
	err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		buf, capErr := page.CaptureScreenshot().
			WithFormat(format).
			WithClip(&page.Viewport{
				X:      0,
				Y:      0,
				Width:  float64(parsed.Width),
				Height: float64(parsed.Height),
				Scale:  1,
			}).
			WithCaptureBeyondViewport(true).
			WithOptimizeForSpeed(false).
			Do(ctx)
		if capErr != nil {
			return capErr
		}
		imgBuf = buf
		return nil
	}))
	if err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: chromedp screenshot: %w", err))
	}

	return ResizeResult{
		Body:   imgBuf,
		Width:  parsed.Width,
		Height: parsed.Height,
		MIME:   mimeForConvert(opts.Convert),
	}, nil
}

// setDocumentContent injects `in` into the active tab's document via
// the CDP `Page.setDocumentContent` command. chromedp.Navigate(data:
// URL ...) is unusable for large SVGs (URL length limit), so we use
// the documented direct command instead.
func setDocumentContent(in string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		frameTree, err := page.GetFrameTree().Do(ctx)
		if err != nil {
			return fmt.Errorf("get frame tree: %w", err)
		}
		return page.SetDocumentContent(frameTree.Frame.ID, in).Do(ctx)
	})
}

// quoteForJS turns a JSON byte slice into a JS string literal that
// can be passed positionally to the jsTemplate %s slots. The literal
// uses backticks to avoid the escaping that double quotes would
// require for embedded `"` characters.
func quoteForJS(raw []byte) string {
	// Backticks inside the source would break the template literal,
	// so we escape them defensively even though chromedp output is
	// unlikely to contain any.
	out := make([]byte, 0, len(raw)+2)
	out = append(out, '`')
	for _, b := range raw {
		if b == '`' || b == '\\' {
			out = append(out, '\\')
		}
		out = append(out, b)
	}
	out = append(out, '`')
	return string(out)
}

// Compile-time interface check: catch signature drift between the
// Renderer contract and *Browser.
var _ Renderer = (*Browser)(nil)
