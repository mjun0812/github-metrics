package chrome

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
)

// TestSVGText_Basic renders a start-anchored body-color line and checks
// the coordinate / style attributes and escaped content.
func TestSVGText_Basic(t *testing.T) {
	t.Parallel()
	got := SVGText(10, 20, "a < b & c", SVGTextOpts{})
	for _, want := range []string{
		`x="10"`, `y="20"`, `font-size="14"`, `fill="#777777"`,
		`>a &lt; b &amp; c</text>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "font-weight") || strings.Contains(got, "text-anchor") {
		t.Errorf("zero-value opts should not emit weight/anchor: %q", got)
	}
}

// TestSVGText_BoldAnchorFill exercises the non-default options.
func TestSVGText_BoldAnchorFill(t *testing.T) {
	t.Parallel()
	got := SVGText(0, 0, "Hi", SVGTextOpts{
		Size: 20, Weight: fontmetrics.Bold, Fill: "#0366d6", Anchor: "middle",
	})
	for _, want := range []string{
		`font-size="20"`, `fill="#0366d6"`, `font-weight="bold"`, `text-anchor="middle"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// TestSVGText_Empty returns "".
func TestSVGText_Empty(t *testing.T) {
	t.Parallel()
	if got := SVGText(0, 0, "", SVGTextOpts{}); got != "" {
		t.Errorf("SVGText(\"\") = %q, want \"\"", got)
	}
}

// TestSVGText_Truncates clips a long line to MaxWidth with an ellipsis.
func TestSVGText_Truncates(t *testing.T) {
	t.Parallel()
	got := SVGText(0, 0, "The quick brown fox jumps over the lazy dog", SVGTextOpts{MaxWidth: 40})
	if !strings.Contains(got, ellipsis) {
		t.Errorf("expected ellipsis in truncated output: %q", got)
	}
	if strings.Contains(got, "lazy dog") {
		t.Errorf("tail should have been trimmed: %q", got)
	}
}

// TestTruncateToWidth covers the fits / trims / lone-ellipsis paths.
func TestTruncateToWidth(t *testing.T) {
	t.Parallel()
	const size = 14.0
	full := "Contributed to 42 repositories"

	// Fits: wide budget returns the string verbatim.
	if got := TruncateToWidth(full, size, fontmetrics.Regular, 400); got != full {
		t.Errorf("fits: got %q, want %q", got, full)
	}
	// maxWidth<=0 disables truncation.
	if got := TruncateToWidth(full, size, fontmetrics.Regular, 0); got != full {
		t.Errorf("maxWidth=0: got %q, want %q", got, full)
	}
	// Trims: a tight budget yields an ellipsis-terminated prefix that
	// measures within budget.
	got := TruncateToWidth(full, size, fontmetrics.Regular, 60)
	if !strings.HasSuffix(got, ellipsis) {
		t.Errorf("trimmed output should end with ellipsis: %q", got)
	}
	if w := fontmetrics.Width(got, size); w > 60 {
		t.Errorf("trimmed width %.1f exceeds budget 60 (%q)", w, got)
	}
	if len(got) >= len(full) {
		t.Errorf("trimmed output should be shorter: %q", got)
	}
}

// TestSVGField positions the icon placeholder and returns FieldPitch.
func TestSVGField(t *testing.T) {
	t.Parallel()
	m, h := SVGField(0, 34, CardWidth/2, ":octicon-clock:", "Joined GitHub 18 years ago")
	if h != FieldPitch {
		t.Errorf("height = %v, want %v", h, FieldPitch)
	}
	// Icon: gray-filled group carrying the placeholder at the left inset
	// (colX 0 + inset 5 + icon margin 8 = 13; row top 34 + (20-16)/2 = 36).
	if !strings.Contains(m, `<g transform="translate(13,36)" fill="#959da5">:octicon-clock:</g>`) {
		t.Errorf("icon group missing/mispositioned: %q", m)
	}
	// Text: starts past the icon column (13 + 16 + 8 = 37).
	if !strings.Contains(m, `<text x="37"`) {
		t.Errorf("label x should be 37: %q", m)
	}
	if !strings.Contains(m, "Joined GitHub 18 years ago") {
		t.Errorf("label text missing: %q", m)
	}
}

// TestSVGColumn stacks rows and tracks the cursor / height.
func TestSVGColumn(t *testing.T) {
	t.Parallel()
	c := NewSVGColumn(0, CardWidth/2, 34)
	if !c.Empty() {
		t.Fatal("new column should be Empty")
	}
	c.Field(":octicon-clock:", "one")
	c.Field(":octicon-people:", "two")
	if c.Empty() {
		t.Fatal("column with rows should not be Empty")
	}
	if c.Height() != 2*FieldPitch {
		t.Errorf("height = %v, want %v", c.Height(), 2*FieldPitch)
	}
	// Second row must sit one pitch below the first.
	if !strings.Contains(c.Markup(), `<text x="37" y="48"`) {
		t.Errorf("first row baseline expected at y=48: %q", c.Markup())
	}
	if !strings.Contains(c.Markup(), `<text x="37" y="68"`) {
		t.Errorf("second row baseline expected at y=68: %q", c.Markup())
	}
}

// TestSVGCalendarRow emits one positioned rect per day, with the shared
// calendar-grid marker; an empty slice is a no-op.
func TestSVGCalendarRow(t *testing.T) {
	t.Parallel()
	if m, h := SVGCalendarRow(240, 34, CardWidth/2, nil); m != "" || h != 0 {
		t.Errorf("empty days = (%q, %v), want (\"\", 0)", m, h)
	}
	days := []plugins.ContributionDay{
		{Color: "#196127"},
		{}, // no color → empty-cell fill
		{Color: "#40c463"},
	}
	m, h := SVGCalendarRow(240, 34, CardWidth/2, days)
	if h != FieldPitch {
		t.Errorf("height = %v, want %v", h, FieldPitch)
	}
	if !strings.Contains(m, `data-block="calendar-grid"`) {
		t.Errorf("calendar-grid marker missing: %q", m)
	}
	if n := strings.Count(m, "<rect"); n != 3 {
		t.Errorf("want 3 day cells, got %d: %q", n, m)
	}
	if !strings.Contains(m, `fill="#196127"`) || !strings.Contains(m, `fill="#ebedf0"`) {
		t.Errorf("expected literal + empty-cell fills: %q", m)
	}
	// First cell starts at the icon column (240 + 5 + 8 = 253); the third
	// is two pitches (15px) further right.
	if !strings.Contains(m, `x="253"`) || !strings.Contains(m, `x="283"`) {
		t.Errorf("cell x positions off: %q", m)
	}
}

// TestSVGAvatar covers the circular (user) and rounded-square (org)
// clips plus the href attribute the image-inline stage rewrites.
func TestSVGAvatar(t *testing.T) {
	t.Parallel()
	circ := SVGAvatar(11, 10, 20, "https://x/a.png?a=1&b=2", "hc", true)
	for _, want := range []string{
		`<clipPath id="hc"><circle cx="21" cy="20" r="10"/></clipPath>`,
		`href="https://x/a.png?a=1&amp;b=2"`,
		`clip-path="url(#hc)"`,
	} {
		if !strings.Contains(circ, want) {
			t.Errorf("circular avatar missing %q: %q", want, circ)
		}
	}
	rect := SVGAvatar(11, 10, 20, "https://x/o.png", "ho", false)
	if !strings.Contains(rect, `<rect x="11" y="10" width="20" height="20" rx="3" ry="3"/>`) {
		t.Errorf("org avatar should clip to a 15%% rounded square: %q", rect)
	}
}

func TestCalendarLevelColor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level int
		want  string
	}{
		{0, "#ebedf0"},
		{1, "#9be9a8"},
		{2, "#40c463"},
		{3, "#30a14e"},
		{4, "#216e39"},
		{5, "#ebedf0"},  // out of range → empty-cell fallback
		{-1, "#ebedf0"}, // out of range → empty-cell fallback
	} {
		if got := CalendarLevelColor(tc.level); got != tc.want {
			t.Errorf("CalendarLevelColor(%d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestSVGSubHeader(t *testing.T) {
	t.Parallel()
	m, h := SVGSubHeader(120, 54, 224, "Total stargazers")
	if h != SubHeaderPitch {
		t.Errorf("height = %v, want %v", h, SubHeaderPitch)
	}
	for _, want := range []string{
		`text-anchor="middle"`,
		`fill="#0366d6"`,
		`font-size="14"`,
		`>Total stargazers</text>`,
	} {
		if !strings.Contains(m, want) {
			t.Errorf("sub-header missing %q: %q", want, m)
		}
	}
}

func TestSVGVBars(t *testing.T) {
	t.Parallel()
	if m, h := SVGVBars(8, 54, 224, nil); m != "" || h != 0 {
		t.Errorf("empty bars = (%q, %v), want (\"\", 0)", m, h)
	}
	bars := []VBar{
		{Value: "1", Label: "1", Caption: "Apr.", Share: 0.05, Level: 1},
		{Value: "3", Label: "2", Share: 1.0, Level: 4},
	}
	m, h := SVGVBars(8, 54, 224, bars)
	if !strings.Contains(m, `data-block="chart-bars"`) {
		t.Errorf("chart-bars marker missing: %q", m)
	}
	if n := strings.Count(m, "<rect"); n != 2 {
		t.Errorf("want 2 bars, got %d: %q", n, m)
	}
	// Literal contribution-graph fills, no CSS var() references.
	if !strings.Contains(m, `fill="#9be9a8"`) || !strings.Contains(m, `fill="#216e39"`) {
		t.Errorf("expected literal L1/L4 fills: %q", m)
	}
	if strings.Contains(m, "var(") {
		t.Errorf("must not emit CSS var() references: %q", m)
	}
	// The month caption reserves an extra label line, so a captioned chart
	// is one label-line taller than an uncaptioned one.
	_, hNoCaption := SVGVBars(8, 54, 224, []VBar{{Label: "1", Share: 0.5, Level: 2}})
	if h <= hNoCaption {
		t.Errorf("captioned height %v should exceed uncaptioned %v", h, hNoCaption)
	}
}
