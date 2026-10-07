package render

import (
	"regexp"
	"strings"
	"testing"
)

// TestFormatXML_NestedIndentation verifies the two-space indentation
// per nesting level, the explicit `\n` line separator, and the
// collapsing of empty elements into self-closing tags.
func TestFormatXML_NestedIndentation(t *testing.T) {
	t.Parallel()
	in := `<svg><g><path/></g><g></g></svg>`
	out, err := FormatXML(in)
	if err != nil {
		t.Fatalf("FormatXML: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// Expect lines roughly:
	// <svg>
	//   <g>
	//     <path/>
	//   </g>
	//   <g/>
	// </svg>
	want := []string{
		"<svg>",
		"  <g>",
		"    <path/>",
		"  </g>",
		"  <g/>",
		"</svg>",
	}
	if len(lines) != len(want) {
		t.Fatalf("line count = %d, want %d\nout=%q", len(lines), len(want), out)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

// TestFormatXML_TextContent confirms whitespace-only text nodes get
// dropped and substantive text lands on its own indented line.
func TestFormatXML_TextContent(t *testing.T) {
	t.Parallel()
	in := `<svg><text>hello world</text></svg>`
	out, err := FormatXML(in)
	if err != nil {
		t.Fatalf("FormatXML: %v", err)
	}
	if !regexp.MustCompile(`(?m)^    hello world$`).MatchString(out) {
		t.Errorf("text node inside <svg><text> should be indented four spaces (depth 2); got %q", out)
	}
}

// TestFormatXML_XmlnsRoundtrip is the regression anchor for the
// "FormatXML drops xmlns" bug: the SVG root carries xmlns via the
// default namespace, and the formatter MUST keep it so the result is
// still a valid SVG document. Without the fix, the root opens as
// `<svg ...>` (no xmlns) and downstream consumers refuse the file.
func TestFormatXML_XmlnsRoundtrip(t *testing.T) {
	t.Parallel()
	in := `<svg xmlns="http://www.w3.org/2000/svg"><g/></svg>`
	out, err := FormatXML(in)
	if err != nil {
		t.Fatalf("FormatXML: %v", err)
	}
	if !strings.Contains(out, `xmlns="http://www.w3.org/2000/svg"`) {
		t.Errorf("xmlns attribute should survive FormatXML; got %q", out)
	}
	// Inner elements MUST NOT re-emit the inherited default namespace.
	innerGCount := strings.Count(out, `<g xmlns=`)
	if innerGCount != 0 {
		t.Errorf("inner element should inherit default namespace silently; got %d redundant xmlns", innerGCount)
	}
	if c := strings.Count(out, `xmlns="http://www.w3.org/2000/svg"`); c != 1 {
		t.Errorf("svg xmlns should appear exactly once, got %d; %q", c, out)
	}
	// Round-trip: feed the output back into FormatXML and check
	// xmlns survives the second pass too (idempotency on this
	// dimension).
	out2, err := FormatXML(out)
	if err != nil {
		t.Fatalf("FormatXML round 2: %v", err)
	}
	if !strings.Contains(out2, `xmlns="http://www.w3.org/2000/svg"`) {
		t.Errorf("xmlns must survive the second FormatXML pass; got %q", out2)
	}
}

// TestFormatXML_Empty preserves the empty/whitespace passthrough
// contract (FR-018 fallback expects unmodified input).
func TestFormatXML_Empty(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "   ", "\n\n"} {
		out, err := FormatXML(in)
		if err != nil {
			t.Errorf("FormatXML(%q): unexpected err %v", in, err)
		}
		if out != in {
			t.Errorf("FormatXML(%q) should passthrough; got %q", in, out)
		}
	}
}
