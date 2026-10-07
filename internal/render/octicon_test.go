package render

import (
	"regexp"
	"strings"
	"testing"
)

// TestReplace_Known asserts the happy path: a known icon produces an SVG
// fragment carrying the `octicon` class, with the 16px variant as the
// default and hyphenated names coexisting with the 24px size suffix.
func TestReplace_Known(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		size string
	}{
		{":octicon-star-16:", "16"},
		{":octicon-star:", "16"},
		{":octicon-chevron-down-24:", "24"},
	} {
		got, err := ReplaceOcticons(tc.in)
		if err != nil {
			t.Fatalf("ReplaceOcticons(%q): %v", tc.in, err)
		}
		if !strings.Contains(got, "<svg") {
			t.Fatalf("%q: output should contain an <svg> element; got %q", tc.in, got)
		}
		if !strings.Contains(got, `class="octicon"`) && !strings.Contains(got, `class="octicon `) {
			t.Errorf("%q: output should carry the octicon class; got %q", tc.in, got)
		}
		if !strings.Contains(got, `width="`+tc.size+`"`) || !strings.Contains(got, `height="`+tc.size+`"`) {
			t.Errorf("%q: output should report %spx dimensions; got %q", tc.in, tc.size, got)
		}
	}
}

// TestReplace_UnknownPasses confirms the contract that unknown icons (and
// sizes other than 16 / 24) pass through verbatim — no panic, no escape,
// no partial replacement.
func TestReplace_UnknownPasses(t *testing.T) {
	t.Parallel()
	for _, in := range []string{":octicon-doesnotexist:", ":octicon-star-32:"} {
		got, err := ReplaceOcticons(in)
		if err != nil {
			t.Fatalf("ReplaceOcticons(%q): %v", in, err)
		}
		if got != in {
			t.Errorf("%q should pass through; got %q", in, got)
		}
	}
}

// TestReplace_MultipleInOneString exercises the global replacement:
// every match in the string is substituted.
func TestReplace_MultipleInOneString(t *testing.T) {
	t.Parallel()
	in := "A :octicon-star: B :octicon-repo-24: C"
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if strings.Contains(got, ":octicon-") {
		t.Errorf("no placeholders should remain after replacement; got %q", got)
	}
	matches := regexp.MustCompile(`<svg[^>]*class="octicon"`).FindAllStringIndex(got, -1)
	if len(matches) != 2 {
		t.Errorf("expected 2 octicon SVGs in output, found %d in %q", len(matches), got)
	}
}

// TestReplace_TextNodePreservesLiteral asserts that an octicon token
// inside a <text> node is left verbatim — user-provided display names
// and bios must never trigger a nested <svg> injection.
func TestReplace_TextNodePreservesLiteral(t *testing.T) {
	t.Parallel()
	in := `<text>:octicon-heart:</text>`
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if got != in {
		t.Errorf("token inside <text> should stay literal; got %q", got)
	}
}

// TestReplace_OutsideTextStillExpands confirms the fix is scoped: a
// token outside any <text> node is still substituted even when a
// literal-carrying <text> node is present in the same document.
func TestReplace_OutsideTextStillExpands(t *testing.T) {
	t.Parallel()
	in := `:octicon-star:<text>:octicon-heart:</text>`
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if !strings.Contains(got, `<text>:octicon-heart:</text>`) {
		t.Errorf("token inside <text> should stay literal; got %q", got)
	}
	if strings.Contains(got, `>:octicon-star:`) || strings.HasPrefix(got, ":octicon-star:") {
		t.Errorf("token outside <text> should be expanded; got %q", got)
	}
	if !strings.Contains(got, `class="octicon"`) {
		t.Errorf("expanded token should carry the octicon class; got %q", got)
	}
}

// TestInjectOcticonClass_PreservesExistingClass keeps the
// class-merging invariant clear: a fragment that already declares a
// class attribute should end up with `octicon` appended, not replaced.
func TestInjectOcticonClass_PreservesExistingClass(t *testing.T) {
	t.Parallel()
	in := `<svg class="x" width="16"><path/></svg>`
	out := injectOcticonClass(in)
	if !strings.Contains(out, `class="octicon x"`) && !strings.Contains(out, `class="x octicon"`) {
		t.Errorf("existing class should be preserved alongside octicon; got %q", out)
	}
}
