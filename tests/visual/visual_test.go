// Package visual hosts the chromedp-driven visual regression tests
// for the 19 adopted plugin partials. Each plugin has a sibling
// <slug>_test.go that exercises 3-5 DOM-level assertions against the
// plugin's rendered SVG via a real Chromium tab — closing the gap
// that byte-equality goldens cannot cover (the bare-<g>
// invisible-render bug class).
//
// Test inputs are the existing M4 byte goldens at
// tests/golden/classic/m4/<slug>.svg, wrapped in a minimal SVG
// envelope so foreignObject rendering mirrors what GitHub does for
// <img src=...svg> embeds. After a partial.go rewrite re-baselines
// the golden, the visual test automatically picks up the new bytes —
// no separate test fixture maintenance.
//
// Behavior notes: 3-retry flake absorption and CI strict mode.
package visual

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/mjun0812/github-metrics/internal/render"
)

// sharedBrowser is recycled across all plugin visual tests in a
// single `go test ./tests/visual/...` invocation (M3 R-002 pattern).
// Initialized in TestMain; closed in the deferred teardown.
var sharedBrowser *render.Browser

// TestMain bootstraps the chromedp browser once and tears it down on
// exit. Behavior on chromium-missing is conditional per spec Q5:
//
//   - CI=true (GHA auto-sets this) OR METRICS_VISUAL_STRICT=1 set ->
//     chromium missing fails the suite with os.Exit(1) so the FR-005
//     PR gate stays honest.
//   - Otherwise (local dev) -> skip the suite cleanly with os.Exit(0)
//     so contributor `go test ./...` runs stay green without chromium.
func TestMain(m *testing.M) {
	strict := os.Getenv("CI") == "true" || os.Getenv("METRICS_VISUAL_STRICT") != ""

	chromePath := os.Getenv("METRICS_CHROME_PATH")
	if chromePath == "" {
		for _, candidate := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if _, err := os.Stat(candidate); err == nil {
				chromePath = candidate
				break
			}
		}
		if chromePath == "" {
			if path, err := exec.LookPath("chromium"); err == nil {
				chromePath = path
			} else if path, err := exec.LookPath("google-chrome"); err == nil {
				chromePath = path
			}
		}
	}

	if chromePath == "" {
		fmt.Fprintln(os.Stderr, "visual: chromium / google-chrome not found")
		if strict {
			fmt.Fprintln(os.Stderr, "visual: strict mode (CI or METRICS_VISUAL_STRICT) — failing suite")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "visual: skipping suite (local dev)")
		os.Exit(0)
	}

	b, err := render.New(render.BrowserOpts{ExecPath: chromePath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "visual: render.New: %v\n", err)
		os.Exit(2)
	}
	sharedBrowser = b

	code := m.Run()
	if err := sharedBrowser.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "visual: browser close: %v\n", err)
	}
	os.Exit(code)
}

// loadGoldenSVG reads the existing M4 byte golden for the given plugin
// and wraps it in a minimal SVG + foreignObject envelope that mirrors
// the classic template's rendering structure enough to exercise the
// foreignObject HTML-parsing path. Returns the full SVG markup as a
// string ready to inject into a chromedp tab.
func loadGoldenSVG(t *testing.T, plugin string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	goldenPath := filepath.Join(root, "tests", "golden", "classic", "m4", plugin+".svg")
	fragment, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("loadGoldenSVG: read %s: %v", goldenPath, err)
	}
	// Wrap the partial fragment in a minimal SVG envelope. The structure
	// mirrors what internal/templates/classic emits at runtime: an outer
	// <svg> with a foreignObject containing the partial under an HTML
	// <div xmlns="http://www.w3.org/1999/xhtml">. This reproduces the
	// foreignObject HTML-parsing path that triggers the bare-<g> bug.
	return `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="800" id="metrics-svg" class=""><foreignObject x="0" y="0" width="100%" height="100%"><div xmlns="http://www.w3.org/1999/xhtml" class="items-wrapper">` +
		string(fragment) +
		`</div></foreignObject></svg>`
}

// evalInBrowser opens the SVG inline inside an HTML document, runs the
// supplied JS expression in the tab via chromedp.Evaluate, and returns
// the result. The result type depends on the expression; callers
// type-assert to the expected shape.
//
// Retries internally up to 3 times to absorb chromedp flake per Q3
// clarification (DOMContentLoaded race, font-load race). Only the
// third consecutive failure is reported as an error.
func evalInBrowser(t *testing.T, svg, jsExpr string) any {
	t.Helper()
	// Inline-embed the SVG inside an HTML body so JS can query the SVG
	// DOM directly (`<img>` and `<object>` add origin / contentDocument
	// complications). Inline embedding still triggers the same
	// foreignObject HTML-parsing path that catches the bare-<g> bug.
	html := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>` + svg + `</body></html>`
	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		tabCtx, cancelTab, err := sharedBrowser.NewTab(context.Background())
		if err != nil {
			lastErr = fmt.Errorf("NewTab attempt %d: %w", attempt, err)
			continue
		}
		evalCtx, cancelEval := context.WithTimeout(tabCtx, 10*time.Second)
		var result any
		err = chromedp.Run(
			evalCtx,
			chromedp.Navigate(dataURL),
			chromedp.WaitVisible("#metrics-svg", chromedp.ByID),
			chromedp.Sleep(300*time.Millisecond), // settle for layout
			chromedp.Evaluate(jsExpr, &result),
		)
		cancelEval()
		cancelTab()
		if err == nil {
			return result
		}
		lastErr = fmt.Errorf("chromedp attempt %d: %w", attempt, err)
	}
	t.Fatalf("evalInBrowser: all 3 attempts failed: %v", lastErr)
	return nil
}

// assertElementExists (R-002 menu A1) asserts the rendered SVG
// contains at least minCount elements matching the CSS selector. The
// selector applies under the SVG's DOM root (which is inline in the
// document body, so `document.querySelectorAll` works directly).
//
// Retries internally per evalInBrowser; only persistent failure is
// reported.
func assertElementExists(t *testing.T, svg, selector string, minCount int) {
	t.Helper()
	expr := fmt.Sprintf(`document.querySelectorAll(%q).length`, selector)
	got := evalInBrowser(t, svg, expr)
	n, ok := got.(float64)
	if !ok {
		t.Errorf("assertElementExists(%q): unexpected result %T %v", selector, got, got)
		return
	}
	if int(n) < minCount {
		t.Errorf("assertElementExists(%q): want >= %d elements, got %d", selector, minCount, int(n))
	}
}

// assertBoundingBoxNonZero (R-002 menu A2) asserts the FIRST element
// matching the selector has a rendered width AND height > 0. This is
// the KEY assertion for catching the bare-<g> invisible-render bug:
// invisible elements have zero rendered dimensions even when present
// in the DOM.
//
// Retries internally per evalInBrowser.
func assertBoundingBoxNonZero(t *testing.T, svg, selector string) {
	t.Helper()
	expr := fmt.Sprintf(`(() => {
        const el = document.querySelector(%q);
        if (!el) return {width: -1, height: -1};
        const r = el.getBoundingClientRect();
        return {width: r.width, height: r.height};
    })()`, selector)
	got := evalInBrowser(t, svg, expr)
	m, ok := got.(map[string]any)
	if !ok {
		t.Errorf("assertBoundingBoxNonZero(%q): unexpected result %T %v", selector, got, got)
		return
	}
	w, _ := m["width"].(float64)
	h, _ := m["height"].(float64)
	if w < 0 {
		t.Errorf("assertBoundingBoxNonZero(%q): element not found in DOM", selector)
		return
	}
	if w <= 0 {
		t.Errorf("assertBoundingBoxNonZero(%q): rendered width %v <= 0 (likely bare-<g> bug — element exists in DOM but is invisible)", selector, w)
	}
	if h <= 0 {
		t.Errorf("assertBoundingBoxNonZero(%q): rendered height %v <= 0", selector, h)
	}
}

// assertTextContent (R-002 menu A4) asserts the rendered SVG body
// contains the given text substring (case-sensitive).
//
// Retries internally per evalInBrowser.
func assertTextContent(t *testing.T, svg, substring string) {
	t.Helper()
	expr := fmt.Sprintf(`document.body.innerText.includes(%q)`, substring)
	got := evalInBrowser(t, svg, expr)
	b, ok := got.(bool)
	if !ok {
		t.Errorf("assertTextContent(%q): unexpected result %T %v", substring, got, got)
		return
	}
	if !b {
		t.Errorf("assertTextContent(%q): substring not found in rendered output", substring)
	}
}
