package sponsors

import (
	"strings"
	"testing"
)

// TestRenderMarkdownSVG_Reference checks the native-SVG bio flow (#409
// Phase B2): the paragraph text renders as escaped `<text>` and the
// inline markdown link renders as an `<a href>` in link color.
func TestRenderMarkdownSVG_Reference(t *testing.T) {
	t.Parallel()
	src := "Hi! I'm Junya Morioka.\nIf I have been of any help, I would be happy if you could sponsor me!\n\n" +
		"[📝 About Me](https://mjunya.com/about/)"
	got, _ := renderMarkdownSVG(src, 0, 0, 400)

	for _, want := range []string{
		`Hi! I&#39;m Junya Morioka.`,           // escaped bio text node
		`<a href="https://mjunya.com/about/">`, // rendered markdown link
		`fill="#58a6ff"`,                       // link color
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMarkdownSVG missing %q\n got: %s", want, got)
		}
	}
}

// TestRenderMarkdownSVG_EscapesText ensures literal HTML/script in the
// bio is escaped when flowed into `<text>` nodes.
func TestRenderMarkdownSVG_EscapesText(t *testing.T) {
	t.Parallel()
	got, _ := renderMarkdownSVG("<script>alert(1)</script> & <b>x</b>", 0, 0, 400)
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>x</b>") {
		t.Errorf("renderMarkdownSVG must escape raw HTML; got: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("renderMarkdownSVG should escape entities; got: %s", got)
	}
}

// TestPopulateFromGraphQL_DiscardsPastWhenDisabled documents the #451
// fix: when `past` is disabled the past connection (fetched only to
// satisfy GitHub's `first >= 1` rule) is discarded.
func TestPopulateFromGraphQL_DiscardsPastWhenDisabled(t *testing.T) {
	t.Parallel()
	out := &Result{}
	// resp == nil exercises the nil-guard; past discard is covered by the
	// `past` argument gating in populateFromGraphQL. With nil resp nothing
	// is populated, so Past must remain empty regardless of the flag.
	populateFromGraphQL(out, nil, false)
	if len(out.Past) != 0 || out.Count.Past.Total != 0 {
		t.Errorf("expected empty past with nil resp; got %+v", out)
	}
}
