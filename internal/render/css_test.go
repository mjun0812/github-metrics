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
