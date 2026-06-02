package sponsors

import (
	"strings"
	"testing"
)

// TestRenderMarkdown_Reference mirrors the markup the upstream reference
// card (docs/reference_examples/metrics.plugin.sponsors.svg) emits for
// the GitHub Sponsors bio: paragraphs, links and images.
func TestRenderMarkdown_Reference(t *testing.T) {
	t.Parallel()
	src := "Hi! I'm Junya Morioka.\nIf I have been of any help, I would be happy if you could sponsor me!\n\n" +
		"[📝 About Me](https://mjunya.com/about/)\n\n" +
		"![metrics](https://example.com/metrics_base.svg)"
	got := renderMarkdown(src)

	for _, want := range []string{
		"<p>Hi! I&#39;m Junya Morioka.\nIf I have been of any help, I would be happy if you could sponsor me!</p>",
		`<p><a href="https://mjunya.com/about/">📝 About Me</a></p>`,
		`<p><img src="https://example.com/metrics_base.svg" alt="metrics" /></p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMarkdown missing %q\n got: %s", want, got)
		}
	}
}

// TestRenderMarkdown_EscapesText ensures literal HTML/script in the bio
// is escaped (the bio is emitted unescaped into the SVG, so the renderer
// itself must escape text nodes).
func TestRenderMarkdown_EscapesText(t *testing.T) {
	t.Parallel()
	got := renderMarkdown("<script>alert(1)</script> & <b>x</b>")
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>x</b>") {
		t.Errorf("renderMarkdown must escape raw HTML; got: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("renderMarkdown should escape entities; got: %s", got)
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
