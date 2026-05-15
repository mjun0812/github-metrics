package render

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// jsTemplate is the chromedp.Evaluate body used by Resize. It mirrors
// the upstream puppeteer routine documented in
// docs/design/13-appendix.md §G, lightly adapted so the settle
// duration is parameterized at call time (and the function only
// returns serializable values).
//
// %d is substituted with the settle delay in milliseconds.
const jsTemplate = `(async () => {
  const padding = JSON.parse(%s);
  const scripts = JSON.parse(%s);
  for (const script of scripts) {
    try {
      await new Function("document", "return (async () => {" + script + "})()")(document);
    } catch (e) {
      console.debug("script error: " + e);
    }
  }
  const svg = document.querySelector("svg");
  if (!svg) {
    throw new Error("no <svg> root in document");
  }
  const animated = !svg.classList.contains("no-animations");
  if (animated) svg.classList.add("no-animations");
  await new Promise(r => setTimeout(r, %d));
  const endNode = svg.querySelector("#metrics-end");
  if (!endNode) {
    throw new Error("missing #metrics-end measurement anchor");
  }
  const rect = endNode.getBoundingClientRect();
  let height = rect.y;
  let width  = rect.width;
  height = Math.max(1, Math.ceil(height * padding.height + padding.absoluteHeight));
  width  = Math.max(1, Math.ceil(width  * padding.width  + padding.absoluteWidth));
  if (svg.getAttribute("height") !== "auto") svg.setAttribute("height", String(height));
  if (animated) svg.classList.remove("no-animations");
  return JSON.stringify({
    resized: new XMLSerializer().serializeToString(svg),
    width,
    height,
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

	js := fmt.Sprintf(
		jsTemplate,
		quoteForJS(padJSON),
		quoteForJS(scriptsJSON),
		opts.SettleDelay.Milliseconds(),
	)

	var rawResult string
	err = chromedp.Run(
		tabCtx,
		chromedp.EmulateViewport(int64(opts.ViewportWidth), int64(opts.ViewportHeight)),
		chromedp.Navigate("about:blank"),
		setDocumentContent(in),
		chromedp.Evaluate(js, &rawResult, chromedp.EvalAsValue),
	)
	if err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: chromedp evaluate: %w", err))
	}

	var parsed jsResult
	if decodeErr := json.Unmarshal([]byte(rawResult), &parsed); decodeErr != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: decode jsResult: %w", decodeErr))
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
