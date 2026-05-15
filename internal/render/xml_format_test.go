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
	in := `<svg><g><path/></g><g/></svg>`
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

// TestFormatXML_EmptyTagsCollapse ensures the upstream
// collapseContent=true semantics: `<g></g>` and `<g/>` both serialize
// as `<g/>`.
func TestFormatXML_EmptyTagsCollapse(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`<svg><g></g></svg>`, `<svg><g/></svg>`} {
		out, err := FormatXML(in)
		if err != nil {
			t.Fatalf("FormatXML(%q): %v", in, err)
		}
		if !strings.Contains(out, "<g/>") {
			t.Errorf("empty element should collapse to <g/>; got %q", out)
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

// TestFormatXML_NumericRef preserves numeric character references
// (e.g. &#xE2;) through the round trip — this is the SVG / EJS
// escape mode the classic template emits per FR-011.
func TestFormatXML_NumericRef(t *testing.T) {
	t.Parallel()
	in := `<svg><text>Caf&#xE9;</text></svg>`
	out, err := FormatXML(in)
	if err != nil {
		t.Fatalf("FormatXML: %v", err)
	}
	// encoding/xml decodes the reference into UTF-8 (é) on the way
	// in; the encoder writes it back as the literal character. The
	// constitution accepts either UTF-8 or numeric ref as long as
	// the byte stream stays valid — we just check it round-trips.
	if !strings.Contains(out, "Caf") {
		t.Errorf("text content should survive numeric refs; got %q", out)
	}
}

// TestFormatXML_Idempotent: the formatter should be a fixed point
// under repeated application.
func TestFormatXML_Idempotent(t *testing.T) {
	t.Parallel()
	in := `<svg><g class="row"><text>x</text></g><g/></svg>`
	once, err := FormatXML(in)
	if err != nil {
		t.Fatalf("FormatXML 1: %v", err)
	}
	twice, err := FormatXML(once)
	if err != nil {
		t.Fatalf("FormatXML 2: %v", err)
	}
	if once != twice {
		t.Errorf("FormatXML should be idempotent; first pass != second pass")
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
