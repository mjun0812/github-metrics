package chrome

// Native-SVG layout primitives shared by partials migrated off the
// foreignObject HTML flow (#409 Phase B1 onward). They emit absolutely
// positioned SVG (`<text>` / `<g>` / nested `<svg>` / `<rect>`) sized
// with the fontmetrics package, so the Go writer can report the pixel
// height each block consumes instead of leaning on a browser to lay
// HTML out.
//
// The geometry constants mirror the flex/CSS metrics the equivalent
// HTML `.field` / `.avatar` rows used, so a converted partial lines up
// visually with the still-HTML partials it coexists with until Phase C.

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
)

const (
	// CardWidth is the fixed classic / repository card width in px. The
	// native-SVG partials lay out against this so they align with the
	// 480px outer envelope.
	CardWidth = 480

	// FieldPitch is the vertical space one icon+label field row
	// consumes, including the 2px margin-bottom the HTML `.field` had.
	FieldPitch = 20

	// svgSectionInset mirrors `section > .field { margin-left: 5px }`.
	svgSectionInset = 5
	// svgIconMargin mirrors `.field svg { margin: 0 8px }`.
	svgIconMargin = 8
	// svgIconSize mirrors `.octicon { width/height: 16px }`.
	svgIconSize = 16

	// svgBodyFont / svgBodyFill are the default field text style
	// (`svg { font-size: 14px; color: #777777 }`).
	svgBodyFont = 14.0
	svgBodyFill = "#777777"
	// svgIconFill mirrors `.field svg { fill: #959da5 }`.
	svgIconFill = "#959da5"

	// baselineRatio approximates the distance from a line's vertical
	// center to the text baseline as a fraction of the font size. It is
	// deliberately coarse (measurement, not typesetting) — enough to
	// vertically center a label against its icon.
	baselineRatio = 0.32
)

// ellipsis is the single-rune terminator TruncateToWidth appends.
const ellipsis = "…"

// pos formats an SVG coordinate. Layout math works in whole pixels, so
// coordinates round to the nearest integer for compact, stable output.
func pos(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// SVGTextOpts styles a SVGText call. The zero value renders 14px
// regular text in the body color (#777777), start-anchored, untruncated.
type SVGTextOpts struct {
	Size     float64            // font-size in px; 0 → svgBodyFont
	Weight   fontmetrics.Weight // Regular (default) or Bold
	Fill     string             // fill color; "" → svgBodyFill
	MaxWidth float64            // >0 truncates with a trailing ellipsis to fit
	Anchor   string             // "" (start) | "middle" | "end" → text-anchor
}

func (o SVGTextOpts) size() float64 {
	if o.Size > 0 {
		return o.Size
	}
	return svgBodyFont
}

func (o SVGTextOpts) fill() string {
	if o.Fill != "" {
		return o.Fill
	}
	return svgBodyFill
}

// SVGText renders one line of text as a `<text>` element with its
// baseline at (x, baseline). When o.MaxWidth > 0 the string is
// ellipsis-truncated to fit, measured with the matching weight. Returns
// "" for empty input.
func SVGText(x, baseline float64, s string, o SVGTextOpts) string {
	if s == "" {
		return ""
	}
	size := o.size()
	if o.MaxWidth > 0 {
		s = TruncateToWidth(s, size, o.Weight, o.MaxWidth)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<text x="%s" y="%s" font-size="%s" fill="%s"`,
		pos(x), pos(baseline), pos(size), o.fill())
	if o.Weight == fontmetrics.Bold {
		b.WriteString(` font-weight="bold"`)
	}
	if o.Anchor != "" {
		fmt.Fprintf(&b, ` text-anchor="%s"`, o.Anchor)
	}
	b.WriteString(`>`)
	b.WriteString(escapeXML(s))
	b.WriteString(`</text>`)
	return b.String()
}

// TruncateToWidth returns s clipped with a trailing ellipsis so its
// measured width at (size, weight) does not exceed maxWidth px. It
// returns s unchanged when it already fits (or when maxWidth <= 0), and
// a lone ellipsis when not even one rune fits.
func TruncateToWidth(s string, size float64, weight fontmetrics.Weight, maxWidth float64) string {
	if maxWidth <= 0 || fontmetrics.WidthWeight(s, size, weight) <= maxWidth {
		return s
	}
	ellW := fontmetrics.WidthWeight(ellipsis, size, weight)
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if fontmetrics.WidthWeight(string(runes), size, weight)+ellW <= maxWidth {
			return string(runes) + ellipsis
		}
	}
	return ellipsis
}

// SVGField renders an octicon + single-line label as one field row of a
// column whose left edge is colX and total width is colWidth, with the
// row top at y=top. iconPlaceholder is an ":octicon-…:" token left
// verbatim for the octicon decoration stage (wrapped in a positioned
// `<g fill>` so it lands at the field's left inset and inherits the icon
// color). The label follows, ellipsis-truncated to the remaining column
// width. Returns the markup and the height consumed (FieldPitch).
func SVGField(colX, top, colWidth float64, iconPlaceholder, label string) (string, float64) {
	iconX := colX + svgSectionInset + svgIconMargin
	iconY := top + (FieldPitch-svgIconSize)/2
	textX := iconX + svgIconSize + svgIconMargin
	baseline := top + FieldPitch/2 + svgBodyFont*baselineRatio
	maxW := colX + colWidth - textX - svgSectionInset

	var b strings.Builder
	if iconPlaceholder != "" {
		fmt.Fprintf(&b, `<g transform="translate(%s,%s)" fill="%s">%s</g>`,
			pos(iconX), pos(iconY), svgIconFill, iconPlaceholder)
	}
	b.WriteString(SVGText(textX, baseline, label, SVGTextOpts{MaxWidth: maxW}))
	return b.String(), FieldPitch
}

// SVGCalendarRow renders the mini contribution calendar as a row of
// positioned day cells occupying one field row of the column at
// (colX, top). Cells align with the icon column (colX + inset) and use
// each day's literal fill (resvg does not resolve the CSS
// `--color-calendar-graph-day-*` variables — #409 decision log #4;
// upstream already hands us resolved hex colors). Returns "" / 0 when no
// days are present so the caller can hide the block.
func SVGCalendarRow(colX, top, _ float64, days []plugins.ContributionDay) (string, float64) {
	if len(days) == 0 {
		return "", 0
	}
	x0 := colX + svgSectionInset + svgIconMargin
	cellY := top + (FieldPitch-calendarCellSize)/2

	var b strings.Builder
	b.WriteString(`<g data-block="calendar-grid">`)
	for i, d := range days {
		color := emptyCellColor
		if d.Color != "" {
			color = d.Color
		}
		fmt.Fprintf(&b,
			`<rect class="day" fill=%q x="%s" y="%s" width="%d" height="%d" rx="2" ry="2"/>`,
			color, pos(x0+float64(i)*calendarCellPitch), pos(cellY), calendarCellSize, calendarCellSize)
	}
	b.WriteString(`</g>`)
	return b.String(), FieldPitch
}

// SVGColumn stacks native-SVG field rows vertically within a
// fixed-width column, tracking the y cursor so callers append rows
// without managing coordinates by hand. X/Width are the column's left
// edge and width in px; Top is where the first row begins.
type SVGColumn struct {
	X, Width, Top float64

	y float64
	b strings.Builder
}

// NewSVGColumn returns a column whose first row begins at (x, top) and
// whose rows are laid out within width px.
func NewSVGColumn(x, width, top float64) *SVGColumn {
	return &SVGColumn{X: x, Width: width, Top: top, y: top}
}

// Field appends an icon + label row and advances the cursor by
// FieldPitch.
func (c *SVGColumn) Field(iconPlaceholder, label string) {
	m, h := SVGField(c.X, c.y, c.Width, iconPlaceholder, label)
	c.b.WriteString(m)
	c.y += h
}

// Calendar appends the mini contribution calendar as one row and
// advances the cursor. A nil / empty day slice is a no-op.
func (c *SVGColumn) Calendar(days []plugins.ContributionDay) {
	m, h := SVGCalendarRow(c.X, c.y, c.Width, days)
	c.b.WriteString(m)
	c.y += h
}

// Markup returns the accumulated column SVG.
func (c *SVGColumn) Markup() string { return c.b.String() }

// Height returns the total px consumed since Top.
func (c *SVGColumn) Height() float64 { return c.y - c.Top }

// Empty reports whether no rows have been appended.
func (c *SVGColumn) Empty() bool { return c.y == c.Top }

// SVGAvatar renders a profile avatar as a clipped `<image>` of the given
// square size at (x, y). The image URL is emitted as an `href` (left for
// the image-inline stage to fold into a data URI). rounded=true clips to
// a circle (user avatars); rounded=false clips to a 15%-radius rounded
// square (organization avatars), matching the `.avatar` /
// `.organization.avatar` CSS. clipID must be unique within the document.
func SVGAvatar(x, y, size float64, url, clipID string, rounded bool) string {
	var clip string
	if rounded {
		clip = fmt.Sprintf(`<clipPath id=%q><circle cx="%s" cy="%s" r="%s"/></clipPath>`,
			clipID, pos(x+size/2), pos(y+size/2), pos(size/2))
	} else {
		r := size * 0.15
		clip = fmt.Sprintf(`<clipPath id=%q><rect x="%s" y="%s" width="%s" height="%s" rx="%s" ry="%s"/></clipPath>`,
			clipID, pos(x), pos(y), pos(size), pos(size), pos(r), pos(r))
	}
	return fmt.Sprintf(
		`<defs>%s</defs><image class="avatar" href=%q x="%s" y="%s" width="%s" height="%s" clip-path="url(#%s)"/>`,
		clip, escapeXML(url), pos(x), pos(y), pos(size), pos(size), clipID)
}
