package render

import (
	"html"
	"os"
	"strings"
	"testing"
)

// TestOptimizeCSS_RealClassicStyleSheet runs the *actual* shipped
// classic style sheet through OptimizeCSS and asserts the result is
// structurally valid CSS: balanced braces, intact keyframes, and no
// leaked tokens. This is the end-to-end guard for the at-rule
// reconstruction fix — the synthetic tests cover the mechanism, this
// covers the real input the pipeline feeds it.
func TestOptimizeCSS_RealClassicStyleSheet(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../assets/templates/classic/style.css")
	if err != nil {
		t.Skipf("classic style.css not readable: %v", err)
	}
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">` +
		html.EscapeString(string(raw)) +
		`</style><h1 class="header"></h1><h2></h2><section class="field"><svg class="octicon"></svg></section>` +
		`<div class="row"><div class="avatar"></div></div>` +
		`<svg class="calendar"><div class="day"></div></svg></svg>`

	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS(real style.css): %v", err)
	}

	if open, closeB := strings.Count(out, "{"), strings.Count(out, "}"); open != closeB {
		t.Fatalf("unbalanced braces after optimize: %d open vs %d close", open, closeB)
	}

	// Keyframes must reconstruct with braces and not leak their last
	// declaration into the following rule.
	//
	// animation-rainbow (.cakeday) was removed from the real stylesheet
	// by #684 (Phase A3 dead-CSS purge) — the cakeday selector had no
	// matching Go-emitted element, so it no longer exercises this
	// reconstruction path. animation-gauge remains and still covers the
	// mechanism end-to-end against the real file.
	for _, want := range []string{
		"@keyframes animation-gauge{from{stroke-dasharray:0 329}}",
		":root{--color-calendar-graph-day-bg:#ebedf0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in optimized output", want)
		}
	}
	for _, bad := range []string{
		"@keyframes animation-gauge;",
		";};",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("optimized output contains mangled fragment %q", bad)
		}
	}
}
