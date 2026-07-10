package render

import (
	"regexp"
	"strings"
	"testing"
)

// TestReplace_Known16 asserts the happy path: a known icon with an
// explicit size produces an SVG fragment carrying the `octicon` class.
func TestReplace_Known16(t *testing.T) {
	t.Parallel()
	got, err := ReplaceOcticons(":octicon-star-16:")
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if !strings.Contains(got, "<svg") {
		t.Fatalf("output should contain an <svg> element; got %q", got)
	}
	if !strings.Contains(got, `class="octicon"`) && !strings.Contains(got, `class="octicon `) {
		t.Errorf("output should carry the octicon class; got %q", got)
	}
	if !strings.Contains(got, `width="16"`) || !strings.Contains(got, `height="16"`) {
		t.Errorf("output should report 16px dimensions; got %q", got)
	}
}

// TestReplace_DefaultSize asserts the `:octicon-<name>:` (no size)
// form picks the 16px variant.
func TestReplace_DefaultSize(t *testing.T) {
	t.Parallel()
	got, err := ReplaceOcticons(":octicon-star:")
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if !strings.Contains(got, `width="16"`) {
		t.Errorf("default size should be 16; got %q", got)
	}
}

// TestReplace_ChevronDown24 verifies hyphenated names ("chevron-down")
// and the 24px size suffix coexist correctly with the lazy-match
// regex.
func TestReplace_ChevronDown24(t *testing.T) {
	t.Parallel()
	got, err := ReplaceOcticons(":octicon-chevron-down-24:")
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if !strings.Contains(got, `width="24"`) {
		t.Errorf("expected width=\"24\" for the -24 suffix; got %q", got)
	}
}

// TestReplace_UnknownPasses confirms the contract that unknown icons
// pass through verbatim — no panic, no escape, no partial replacement.
func TestReplace_UnknownPasses(t *testing.T) {
	t.Parallel()
	in := ":octicon-doesnotexist:"
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if got != in {
		t.Errorf("unknown name should pass through; got %q", got)
	}
}

// TestReplace_InvalidSize covers the documented edge case where the
// requested size is neither 16 nor 24 (primer/octicons does not
// publish other sizes). The regex `(?:-(16|24))?` rejects the suffix,
// so the name expands to include the digits and the lookup misses.
func TestReplace_InvalidSize(t *testing.T) {
	t.Parallel()
	in := ":octicon-star-32:"
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if got != in {
		t.Errorf("invalid size suffix should pass through; got %q", got)
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

// TestReplace_Idempotent confirms that running ReplaceOcticons twice
// on the same input is a no-op for the second call.
func TestReplace_Idempotent(t *testing.T) {
	t.Parallel()
	once, err := ReplaceOcticons(":octicon-star-16: hello :octicon-repo:")
	if err != nil {
		t.Fatalf("ReplaceOcticons first pass: %v", err)
	}
	twice, err := ReplaceOcticons(once)
	if err != nil {
		t.Fatalf("ReplaceOcticons second pass: %v", err)
	}
	if once != twice {
		t.Errorf("ReplaceOcticons should be idempotent; round-trip differs")
	}
}

// TestReplace_EmptyInput keeps the empty-input contract: blank in =>
// blank out, no error.
func TestReplace_EmptyInput(t *testing.T) {
	t.Parallel()
	got, err := ReplaceOcticons("")
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
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

// TestReplace_TextNodeWithAttributes covers the same guarantee for a
// <text> element carrying attributes, so the tag-boundary match is not
// tripped up by attribute content.
func TestReplace_TextNodeWithAttributes(t *testing.T) {
	t.Parallel()
	in := `<text x="1" y="2">Jane :octicon-heart: Doe</text>`
	got, err := ReplaceOcticons(in)
	if err != nil {
		t.Fatalf("ReplaceOcticons: %v", err)
	}
	if got != in {
		t.Errorf("token inside <text x=...> should stay literal; got %q", got)
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
