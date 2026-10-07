package sponsors

import (
	"strings"
	"testing"
)

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
