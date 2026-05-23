# Contract: tests/visual/ file shape

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002 + E-004

Codifies the structure of the new `tests/visual/` test layer. Two
file types: the shared `visual_test.go` (TestMain + helpers, written
once in US1) and per-plugin `<slug>_test.go` (one per adopted plugin,
following the same template).

---

## 1. `tests/visual/visual_test.go` (shared harness, US1 deliverable)

```go
// Package visual hosts the chromedp-driven visual regression tests
// for the 19 adopted plugin partials. Each plugin has a sibling
// <slug>_test.go that exercises 3-5 DOM-level assertions against
// the plugin's rendered SVG via a real Chromium tab — closing the
// gap that byte-equality goldens cannot cover.
//
// Spec: specs/011-plugin-rendering-parity/spec.md FR-004
// Contract: specs/011-plugin-rendering-parity/contracts/visual-test-shape.md
package visual

import (
    "context"
    "encoding/base64"
    "fmt"
    "os"
    "os/exec"
    "testing"
    "time"

    "github.com/chromedp/chromedp"

    "github.com/mjun0812/github-metrics/internal/render"
)

var sharedBrowser *render.Browser

func TestMain(m *testing.M) {
    // Per spec Q5 clarification: in CI (CI=true) or when strict mode
    // is explicitly requested (METRICS_VISUAL_STRICT=1), chromium
    // missing is a non-zero failure so the FR-005 PR gate stays
    // honest. In local dev (neither env var set), skip cleanly so
    // contributor go-test runs stay green when they don't have
    // chromium installed.
    strict := os.Getenv("CI") == "true" || os.Getenv("METRICS_VISUAL_STRICT") != ""
    if _, err := exec.LookPath("chromium"); err != nil {
        if _, err := exec.LookPath("google-chrome"); err != nil {
            fmt.Fprintln(os.Stderr, "visual: chromium not found in PATH")
            if strict {
                fmt.Fprintln(os.Stderr, "visual: strict mode (CI or METRICS_VISUAL_STRICT) — failing the suite")
                os.Exit(1)
            }
            fmt.Fprintln(os.Stderr, "visual: skipping suite (local dev)")
            os.Exit(0)
        }
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    var err error
    sharedBrowser, err = render.NewBrowser(ctx, render.BrowserOptions{})
    if err != nil {
        fmt.Fprintf(os.Stderr, "visual: NewBrowser: %v\n", err)
        os.Exit(2)
    }
    defer sharedBrowser.Close()
    os.Exit(m.Run())
}

// renderForVisualTest renders the given plugin's SVG via the
// existing internal/action pipeline with --dryrun, returning the
// raw SVG string. Caller passes the plugin slug + any extra inputs
// (e.g., "plugin_languages_indepth=yes" for indepth variants).
//
// The returned SVG is the same bytes that would be emitted by the
// CLI / Action, modulo dynamic strings.
func renderForVisualTest(t *testing.T, plugin string, extraInputs map[string]string) string {
    t.Helper()
    // Implementation: invokes the action.Run() in dryrun mode with
    // INPUT_USER set to a fixture user (matches the existing M4 P3
    // test pattern). Returns the in-memory SVG string.
    // ...
    return "" // stub for contract documentation
}

// evalInBrowser opens the SVG in about:blank as an <img src=...>
// (mimicking github.com's render path) and evaluates the given JS
// expression against the document. Returns the result.
//
// Per R-002, this is the foundation for all 3-5 per-plugin assertions.
func evalInBrowser(t *testing.T, svg string, jsExpr string) any {
    t.Helper()
    tab, err := sharedBrowser.NewTab(context.Background())
    if err != nil {
        t.Fatalf("evalInBrowser: NewTab: %v", err)
    }
    defer tab.Close()

    // Wrap SVG in HTML with <img> tag so it renders in <img> context
    // (matches github.com's <img src=...svg> path).
    encoded := base64.StdEncoding.EncodeToString([]byte(svg))
    html := fmt.Sprintf(`<!DOCTYPE html><html><body><img src="data:image/svg+xml;base64,%s" id="metrics-svg"></body></html>`, encoded)
    dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

    var result any
    if err := chromedp.Run(tab.Context(),
        chromedp.Navigate(dataURL),
        chromedp.WaitVisible("#metrics-svg", chromedp.ByID),
        chromedp.Sleep(500*time.Millisecond), // settle for SVG layout
        chromedp.Evaluate(jsExpr, &result),
    ); err != nil {
        t.Fatalf("evalInBrowser: chromedp.Run: %v", err)
    }
    return result
}

// assertElementExists is convenience helper for R-002 A1: assert the
// plugin's rendered SVG contains at least minCount elements matching
// the CSS selector.
func assertElementExists(t *testing.T, svg string, selector string, minCount int) {
    t.Helper()
    expr := fmt.Sprintf(`document.querySelector('#metrics-svg').contentDocument?.querySelectorAll(%q).length ?? 0`, selector)
    got := evalInBrowser(t, svg, expr)
    if n, ok := got.(float64); !ok || int(n) < minCount {
        t.Errorf("assertElementExists(%q): want >= %d, got %v", selector, minCount, got)
    }
}

// assertBoundingBoxNonZero is convenience helper for R-002 A2: assert
// the first element matching the selector has non-zero rendered
// width AND height — catches the bare-<g> invisible-render bug.
func assertBoundingBoxNonZero(t *testing.T, svg string, selector string) {
    t.Helper()
    expr := fmt.Sprintf(`(() => {
        const doc = document.querySelector('#metrics-svg').contentDocument;
        if (!doc) return {width: 0, height: 0};
        const el = doc.querySelector(%q);
        if (!el) return {width: 0, height: 0};
        const r = el.getBoundingClientRect();
        return {width: r.width, height: r.height};
    })()`, selector)
    got := evalInBrowser(t, svg, expr)
    m, ok := got.(map[string]any)
    if !ok {
        t.Fatalf("assertBoundingBoxNonZero(%q): unexpected result shape: %T %v", selector, got, got)
    }
    if w, _ := m["width"].(float64); w <= 0 {
        t.Errorf("assertBoundingBoxNonZero(%q): rendered width %v <= 0", selector, m["width"])
    }
    if h, _ := m["height"].(float64); h <= 0 {
        t.Errorf("assertBoundingBoxNonZero(%q): rendered height %v <= 0", selector, m["height"])
    }
}

// assertTextContent is convenience helper for R-002 A4: assert the
// plugin's rendered SVG contains the given text substring.
func assertTextContent(t *testing.T, svg string, substring string) {
    t.Helper()
    expr := fmt.Sprintf(`document.querySelector('#metrics-svg').contentDocument?.body?.innerText?.includes(%q) ?? false`, substring)
    got := evalInBrowser(t, svg, expr)
    if b, _ := got.(bool); !b {
        t.Errorf("assertTextContent(%q): substring not found in rendered output", substring)
    }
}
```

---

## 2. `tests/visual/<slug>_test.go` (per-plugin, US1 = languages pilot, US2 = 18 others)

```go
package visual

import "testing"

// TestLanguages_Visual exercises the languages plugin's rendered SVG
// against R-002's DOM assertions. Per the per-plugin parity checklist
// at specs/011-plugin-rendering-parity/plugins/languages.md.
func TestLanguages_Visual(t *testing.T) {
    svg := renderForVisualTest(t, "languages", map[string]string{
        "plugin_languages":                  "yes",
        "plugin_languages_indepth":          "yes",
        "plugin_languages_sections":         "most-used,recently-used",
        "plugin_languages_analysis_timeout": "30",
    })

    t.Run("header_exists", func(t *testing.T) {
        assertElementExists(t, svg, "h2.field", 1) // "27 Languages" header
    })
    t.Run("bar_renders", func(t *testing.T) {
        assertBoundingBoxNonZero(t, svg, "rect.language-bar") // catches bare-<g> bug
    })
    t.Run("has_most_used_section", func(t *testing.T) {
        assertTextContent(t, svg, "Most used languages")
    })
    t.Run("has_recently_used_section", func(t *testing.T) {
        assertTextContent(t, svg, "Recently used languages")
    })
    t.Run("has_indepth_estimation", func(t *testing.T) {
        assertTextContent(t, svg, "estimation from")
    })
}
```

---

## 3. Per-plugin assertion count budget

Per R-002, each plugin's test file has 3-5 sub-tests. The 5-budget comes from the R-002 menu (A1-A5). Most plugins will land on 3-4 — only the most complex (languages, isocalendar, achievements) need 5.

| Plugin | Suggested assertion count | Notes |
|---|---|---|
| languages | 5 | most complex (sub-modes + indepth + recent) |
| achievements | 5 | grid of badges + per-badge progress |
| isocalendar | 4 | year heatmap + summary stats |
| activity | 4 | event list + per-event icon + dates |
| habits | 4 | habits chart (the bare-`<g>` source) |
| calendar | 4 | calendar heatmap + summary |
| starlists | 4 | list grid (the bare-`<g>` source) |
| stargazers | 4 | time-series chart |
| projects | 3 | project status pills |
| traffic | 3 | views + clones chart |
| people | 3 | per-person card grid |
| contributors | 3 | per-contributor avatar grid |
| repositories | 3 | per-repo chip list |
| sponsors | 3 | per-tier section |
| sponsorships | 3 | per-tier section |
| notable | 3 | per-entry author info |
| reactions | 3 | emoji column |
| stars | 3 | per-repo chip list |
| topics | 3 | topic chip list (chromedp-gated, mocked data) |
| **TOTAL** | **70** | average 3.7 assertions/plugin |

---

## 4. CI integration

```yaml
# .github/workflows/ci.yml addition (US3 final task)
- name: Visual regression tests
  run: go test ./tests/visual/... -timeout 15m
  env:
    METRICS_CHROME_PATH: /usr/bin/chromium
```

CI runs the visual suite on every push to PR / main. Failures block the PR (FR-005).

---

## 5. Local maintainer flow

```sh
# Run all 19 plugin visual tests against current code:
go test ./tests/visual/...

# Run one plugin:
go test ./tests/visual/ -run TestLanguages_Visual -v

# Run with verbose chromedp output for debugging:
go test ./tests/visual/ -run TestLanguages_Visual -v -chromedp-debug
```

If a visual test fails locally during a partial.go rewrite, the maintainer:

1. Reads the failure message — it names the missing element (e.g., `"rect.language-bar": rendered width 0 <= 0`)
2. Inspects the partial.go output (via `metrics-cli ... --filename out.svg --dryrun`)
3. Fixes the partial to emit the missing element with proper SVG wrapping
4. Re-runs the visual test; iterates until green
5. Re-baselines the byte golden with `go test ./internal/plugins/<slug>/ -update`
6. Captures the after-screenshot via `bash scripts/capture-plugin-screenshot.sh <slug>`
7. Commits + opens PR
