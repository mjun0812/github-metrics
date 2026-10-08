package sponsors

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
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

var mdTextRe = regexp.MustCompile(`<text x="([0-9.]+)"[^>]*>([^<]*)</text>`)

// mdTexts returns the x and content of each emitted `<text>`.
func mdTexts(t *testing.T, markup string) (xs []float64, texts []string) {
	t.Helper()
	for _, m := range mdTextRe.FindAllStringSubmatch(markup, -1) {
		x, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		xs = append(xs, x)
		texts = append(texts, m[2])
	}
	return xs, texts
}

// TestRenderMarkdownSVG_AdjacentLinksDoNotOverlap guards the emoji-link
// row: a link whose label starts with a full-width emoji must advance
// the next link past the whole emoji, not half of it.
func TestRenderMarkdownSVG_AdjacentLinksDoNotOverlap(t *testing.T) {
	t.Parallel()
	got, _ := renderMarkdownSVG("[📝 About Me (JA)](https://a.example) [📝 About Me (EN)](https://b.example)", 0, 0, 400)
	xs, texts := mdTexts(t, got)
	if len(xs) != 2 {
		t.Fatalf("want 2 <text>, got %d: %s", len(xs), got)
	}
	// The emoji is about 1em wide; everything else is measurable Latin.
	minW := mdFont + fontmetrics.Width(" About Me (JA)", mdFont)
	if xs[1]-xs[0] < minW {
		t.Errorf("second link starts %.1f px after the first, want >= %.1f (%q)", xs[1]-xs[0], minW, texts[0])
	}
}

// TestRenderMarkdownSVG_NoSpaceBeforeAdjacentPunctuation: whitespace is
// inserted only where the source has it.
func TestRenderMarkdownSVG_NoSpaceBeforeAdjacentPunctuation(t *testing.T) {
	t.Parallel()
	got, _ := renderMarkdownSVG("My project [wheels](https://a.example), which is [x](https://b.example) ok", 0, 0, 400)
	xs, texts := mdTexts(t, got)
	if len(xs) != 5 {
		t.Fatalf("want 5 <text>, got %d: %s", len(xs), got)
	}
	// texts: "My project", "wheels", ", which is", "x", "ok"
	if want := xs[1] + fontmetrics.Width(texts[1], mdFont); math.Abs(xs[2]-want) > 1 {
		t.Errorf("text after link at x=%.2f, want %.2f (no gap before %q)", xs[2], want, texts[2])
	}
	if want := xs[0] + fontmetrics.Width(texts[0], mdFont) + fontmetrics.Width(" ", mdFont); math.Abs(xs[1]-want) > 1 {
		t.Errorf("link at x=%.2f, want %.2f (one space after %q)", xs[1], want, texts[0])
	}
	if want := xs[3] + fontmetrics.Width(texts[3], mdFont) + fontmetrics.Width(" ", mdFont); math.Abs(xs[4]-want) > 1 {
		t.Errorf("text after spaced link at x=%.2f, want %.2f", xs[4], want)
	}
}
