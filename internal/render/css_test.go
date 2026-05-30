package render

import (
	"fmt"
	"strings"
	"testing"
)

// TestOptimizeCSS_DropsUnusedSelectors covers the SC-006 anchor: with
// 50 selectors of which only 25 are referenced by elements in the
// body, OptimizeCSS keeps the 25 used selectors and drops the rest.
func TestOptimizeCSS_DropsUnusedSelectors(t *testing.T) {
	t.Parallel()

	// Build the SVG fixture programmatically so the test is
	// readable: 50 selectors `.s0..s49`, body references the even
	// ones (`.s0`, `.s2`, ...), and `#metrics-end` is added as a
	// guard for the ID-keep rule.
	var styles, body strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&styles, ".s%d{color:red;}", i)
		if i%2 == 0 {
			fmt.Fprintf(&body, `<g class="s%d"/>`, i)
		}
	}
	// One ID selector that's NOT referenced anywhere in the body:
	// the rule should still survive because it's an ID.
	styles.WriteString("#metrics-end{visibility:visible;}")

	in := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">%s</style>%s</svg>`,
		styles.String(), body.String())

	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}

	// Look for the selector followed by `{` so `.s1` does not
	// match `.s10` substrings.
	for i := 0; i < 50; i++ {
		needle := fmt.Sprintf(".s%d{", i)
		want := i%2 == 0
		got := strings.Contains(out, needle)
		if want && !got {
			t.Errorf("used selector .s%d should survive purge", i)
		}
		if !want && got {
			t.Errorf("unused selector .s%d should be dropped", i)
		}
	}

	if !strings.Contains(out, "#metrics-end") {
		t.Errorf("ID selector #metrics-end must survive (SC-006); out=%q", out)
	}
}

// TestOptimizeCSS_LeavesNonOptimizableStyleAlone asserts the
// `<style>` without `data-optimizable="true"` is untouched.
func TestOptimizeCSS_LeavesNonOptimizableStyleAlone(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style>.untouched{display:none;}</style></svg>`
	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}
	if !strings.Contains(out, ".untouched") {
		t.Errorf("non-optimizable style should pass through; got %q", out)
	}
}

// TestOptimizeCSS_EmptyInput keeps the empty-passthrough contract
// shared by every pipeline stage.
func TestOptimizeCSS_EmptyInput(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "   ", "\n"} {
		out, err := OptimizeCSS(in)
		if err != nil {
			t.Errorf("OptimizeCSS(%q) err = %v", in, err)
		}
		if out != in {
			t.Errorf("empty input should passthrough; got %q for %q", out, in)
		}
	}
}

// TestOptimizeCSS_PreservesAtRuleBraces guards the at-rule
// reconstruction: `@keyframes` / `@media` bodies must keep their
// braces so the minified output stays valid CSS. Regression for the
// bug where BeginAtRuleGrammar dropped the opening `{`, producing
// `@keyframes name0%,100%{…}` and leaking the last keyframe's tokens
// (e.g. `#FF0000`) into the following rule.
func TestOptimizeCSS_PreservesAtRuleBraces(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">
		@keyframes animation-gauge { 0% { stroke-dasharray: 0 329; } }
		@keyframes animation-rainbow {
			0%, 100% { color: #7f00ff; fill: #7f00ff; }
			86% { color: #FF0000; fill: #FF0000; }
		}
		.used { color: red; }
		:root { --accent: #ebedf0; }
	</style><g class="used"/></svg>`
	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}

	// The keyframes blocks must survive intact with their braces.
	for _, want := range []string{
		"@keyframes animation-gauge{0%{stroke-dasharray:0 329}}",
		"@keyframes animation-rainbow{",
		"0%,100%{",
		".used{color:red}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected minified output to contain %q\n got: %q", want, out)
		}
	}

	// The keyframe color must NOT leak onto the following selector.
	for _, bad := range []string{
		"#FF0000:root", "#ff0000:root",
		"@keyframes animation-rainbow0%", // missing brace signature
		";};",                            // mangled-brace signature
	} {
		if strings.Contains(out, bad) {
			t.Errorf("output should not contain mangled fragment %q\n got: %q", bad, out)
		}
	}

	// Validate brace balance — broken at-rule emission unbalances it.
	if open, close := strings.Count(out, "{"), strings.Count(out, "}"); open != close {
		t.Errorf("unbalanced braces: %d open vs %d close\n got: %q", open, close, out)
	}
}

// TestOptimizeCSS_DropsEmptyMediaAfterPurge confirms a `@media` whose
// only inner rule is purged collapses away entirely (matching upstream
// purgecss + csso), rather than leaving an empty `@media(...){}`.
func TestOptimizeCSS_DropsEmptyMediaAfterPurge(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">
		.used { color: red; }
		@media (max-width: 850px) { .unused-wrapper { column-count: 1; } }
	</style><g class="used"/></svg>`
	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}
	if !strings.Contains(out, ".used{color:red}") {
		t.Errorf("used selector should survive: %q", out)
	}
	if strings.Contains(out, "@media") {
		t.Errorf("empty @media should be dropped after purge: %q", out)
	}
}

// TestOptimizeCSS_KeepsMediaWithUsedRule confirms a `@media` block is
// preserved (with braces) when its inner selector is actually used.
func TestOptimizeCSS_KeepsMediaWithUsedRule(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">
		@media (max-width: 850px) { .wrap { column-count: 1; } }
	</style><g class="wrap"/></svg>`
	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}
	if !strings.Contains(out, "@media") || !strings.Contains(out, ".wrap{column-count:1}") {
		t.Errorf("used @media rule should survive with braces: %q", out)
	}
	if open, close := strings.Count(out, "{"), strings.Count(out, "}"); open != close {
		t.Errorf("unbalanced braces: %d open vs %d close\n got: %q", open, close, out)
	}
}

// TestOptimizeCSS_MinifiesSurvivingRules confirms the minify step
// runs after the purge.
func TestOptimizeCSS_MinifiesSurvivingRules(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><style data-optimizable="true">
		.used   {
			color: red ;
		}
	</style><g class="used"/></svg>`
	out, err := OptimizeCSS(in)
	if err != nil {
		t.Fatalf("OptimizeCSS: %v", err)
	}
	if !strings.Contains(out, ".used") {
		t.Errorf("used selector dropped unexpectedly: %q", out)
	}
	// The minified output should NOT carry the original tabs /
	// newlines around the declaration.
	if strings.Contains(out, "color: red ;") {
		t.Errorf("declaration not minified; got %q", out)
	}
}
