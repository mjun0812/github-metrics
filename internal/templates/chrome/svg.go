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
	"regexp"
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

	// svgHeaderFont / svgHeaderFill are the `<h2>` section-header text
	// style (`h2 { font-size: 16px; color: #0366d6; font-weight: normal }`).
	svgHeaderFont = 16.0
	svgHeaderFill = "#0366d6"
	// svgHeaderTop mirrors `h1,h2,h3 { margin: 8px 0 2px }` — the header's
	// top margin — and svgHeaderBand the content band that holds the 16px
	// icon and label.
	svgHeaderTop  = 8.0
	svgHeaderBand = 18.0

	// SectionHeaderPitch is the vertical space one `<h2 class="field">`
	// section header consumes: margin-top(8) + band(18) + margin-bottom(2).
	SectionHeaderPitch = svgHeaderTop + svgHeaderBand + 2

	// svgLineHeightRatio is the line pitch of wrapped body text as a
	// fraction of the font size.
	svgLineHeightRatio = 1.35
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

// WrapSection wraps native-SVG body markup in the `<section
// data-section>` HTML anchor (kept for the section gate / DOM diffing)
// plus a fixed-height nested `<svg>` block. The nested svg lays out as
// one block within the outer foreignObject flow, so a converted partial
// coexists with the still-HTML partials around it until Phase C.
func WrapSection(section string, height int, body string) string {
	return fmt.Sprintf(
		`<section data-section=%q><svg xmlns="http://www.w3.org/2000/svg" width="100%%" height="%d" viewBox="0 0 %d %d">%s</svg></section>`,
		section, height, CardWidth, height, body)
}

// SVGIcon positions an inline octicon `<svg>` fragment at (x, y) inside
// a `<g>` carrying the given fill (the octicon paths inherit it). Pass
// fill="" to leave the fragment's own colors untouched (e.g. a language
// dot that already sets its path fill).
func SVGIcon(x, y float64, fill, icon string) string {
	if icon == "" {
		return ""
	}
	if fill == "" {
		return fmt.Sprintf(`<g transform="translate(%s,%s)">%s</g>`, pos(x), pos(y), icon)
	}
	return fmt.Sprintf(`<g transform="translate(%s,%s)" fill=%q>%s</g>`, pos(x), pos(y), fill, icon)
}

// SVGSectionHeader renders an `<h2 class="field">` section header — a
// grey octicon followed by the 16px blue label — as native SVG. icon is
// an inline octicon `<svg>` fragment (positioned at the field inset);
// label is the header text, ellipsis-truncated to the card width.
// Returns the markup and the height consumed (SectionHeaderPitch).
func SVGSectionHeader(icon, label string) (string, float64) {
	iconX := float64(svgSectionInset + svgIconMargin)
	iconY := svgHeaderTop + (svgHeaderBand-svgIconSize)/2
	textX := iconX + svgIconSize + svgIconMargin
	baseline := svgHeaderTop + svgHeaderBand/2 + svgHeaderFont*baselineRatio

	var b strings.Builder
	b.WriteString(SVGIcon(iconX, iconY, svgIconFill, icon))
	b.WriteString(SVGText(textX, baseline, label, SVGTextOpts{
		Size:     svgHeaderFont,
		Fill:     svgHeaderFill,
		MaxWidth: CardWidth - textX - svgSectionInset,
	}))
	return b.String(), SectionHeaderPitch
}

// SVGParagraph renders word-wrapped body text as one `<text>` line per
// wrapped line, left-anchored at x with the block's top at y=top. size
// is the font size, fill the text color. Returns the markup and the
// total height the wrapped block consumes.
func SVGParagraph(x, top, maxWidth, size float64, fill, text string) (string, float64) {
	if text == "" {
		return "", 0
	}
	wrapped := fontmetrics.Wrap(text, size, maxWidth)
	if len(wrapped) == 0 {
		return "", 0
	}
	lineH := size * svgLineHeightRatio
	var b strings.Builder
	for i, ln := range wrapped {
		baseline := top + float64(i)*lineH + size
		b.WriteString(SVGText(x, baseline, ln, SVGTextOpts{Size: size, Fill: fill}))
	}
	return b.String(), lineH * float64(len(wrapped))
}

const (
	// `.label` pill geometry / colors: 12px medium text on a translucent
	// blue fully-rounded chip (`.label { background:#58A6FF30; color:#0366D6;
	// padding:0 10px; line-height:22px; border-radius:32px; font-size:12px }`).
	svgChipFont    = 12.0
	svgChipHeight  = 22.0
	svgChipPadX    = 10.0
	svgChipMarginX = 5.0
	svgChipMarginY = 2.0
	svgChipBG      = "#58A6FF"
	svgChipBGOpac  = "0.19" // 0x30 / 255
	svgChipFill    = "#0366D6"

	// svgChipRowPitch is the vertical space one row of pills consumes:
	// chip height (22) + top & bottom margin (2 each).
	svgChipRowPitch = svgChipHeight + 2*svgChipMarginY
)

// SVGLabelChip renders one `.label` pill (a translucent blue rounded
// chip) whose top-left is (x, top). Returns the markup and the total
// horizontal advance (chip width + right margin) so callers can flow
// chips left to right and wrap.
func SVGLabelChip(x, top float64, text string) (string, float64) {
	chipW := fontmetrics.Width(text, svgChipFont) + 2*svgChipPadX
	baseline := top + svgChipHeight/2 + svgChipFont*baselineRatio
	var b strings.Builder
	fmt.Fprintf(&b,
		`<rect x="%s" y="%s" width="%s" height="%d" rx="%d" ry="%d" fill=%q fill-opacity=%q/>`,
		pos(x), pos(top), pos(chipW), int(svgChipHeight), int(svgChipHeight/2), int(svgChipHeight/2),
		svgChipBG, svgChipBGOpac)
	b.WriteString(SVGText(x+svgChipPadX, baseline, text, SVGTextOpts{Size: svgChipFont, Fill: svgChipFill}))
	return b.String(), chipW + svgChipMarginX
}

// SVGChipFlow lays `.label` pills out left to right starting at (x, top),
// wrapping to a new row whenever the next chip would exceed maxRight
// (mirroring the `.topics { flex-wrap: wrap }` flow). Returns the markup
// and the total height consumed.
func SVGChipFlow(x, top, maxRight float64, texts []string) (string, float64) {
	if len(texts) == 0 {
		return "", 0
	}
	var b strings.Builder
	cx, rowTop, rows := x, top, 1
	for _, t := range texts {
		chipW := fontmetrics.Width(t, svgChipFont) + 2*svgChipPadX
		if cx+chipW > maxRight && cx > x {
			cx, rowTop = x, rowTop+svgChipRowPitch
			rows++
		}
		m, adv := SVGLabelChip(cx, rowTop+svgChipMarginY, t)
		b.WriteString(m)
		cx += adv
	}
	return b.String(), float64(rows) * svgChipRowPitch
}

const (
	// Repository-card colors: the name is GitHub link-blue and the
	// trailing date grey 10px (`.repository .name span:first-child {
	// color:#58a6ff }`, `span:last-child { color:#666666; font-size:10px;
	// max-width:150px }`).
	svgRepoNameFill = "#58a6ff"
	svgRepoDateFill = "#666666"
	svgRepoDateFont = 10.0
	svgRepoDateMax  = 150.0
	svgRepoNameGap  = 8.0 // `.repository .name { gap: 8px }`
)

// SVGRepoName renders a repository card's name row: the repo/fork
// octicon at the card's field inset, the blue repository name
// (ellipsis-truncated), and an optional right-aligned grey date label
// (ellipsis-truncated to 150px). cardX is the card's left edge and
// nameWidth the name area width (the `.repository .name` box). When url
// is non-empty the name is wrapped in an `<a>` so SVG viewers keep the
// link. Returns the markup and the height consumed (FieldPitch).
func SVGRepoName(cardX, top, nameWidth, nameFont float64, icon, name, url, date string) (string, float64) {
	iconX := cardX + svgSectionInset + svgIconMargin
	iconY := top + (FieldPitch-svgIconSize)/2
	nameX := iconX + svgIconSize + svgIconMargin
	rightEdge := nameX + nameWidth

	var b strings.Builder
	b.WriteString(SVGIcon(iconX, iconY, svgIconFill, icon))

	dateW := 0.0
	if date != "" {
		date = TruncateToWidth(date, svgRepoDateFont, fontmetrics.Regular, svgRepoDateMax)
		dateW = fontmetrics.Width(date, svgRepoDateFont)
		dbase := top + FieldPitch/2 + svgRepoDateFont*baselineRatio
		b.WriteString(SVGText(rightEdge, dbase, date, SVGTextOpts{
			Size: svgRepoDateFont, Fill: svgRepoDateFill, Anchor: "end",
		}))
	}

	baseline := top + FieldPitch/2 + nameFont*baselineRatio
	nameMax := rightEdge - nameX - svgRepoNameGap - dateW
	nameSVG := SVGText(nameX, baseline, name, SVGTextOpts{Size: nameFont, Fill: svgRepoNameFill, MaxWidth: nameMax})
	if url != "" {
		fmt.Fprintf(&b, `<a href=%q>%s</a>`, escapeXML(url), nameSVG)
	} else {
		b.WriteString(nameSVG)
	}
	return b.String(), FieldPitch
}

const (
	// `.repository .infos` row: 13px grey text with small icons
	// (`margin-left:38px; color:#666666; font-size:13px`; icons
	// `margin-right:4px`; segments `margin-right:16px`).
	svgInfoFont    = 13.0
	svgInfoFill    = "#666666"
	svgInfoRowH    = 18.0
	svgInfoIconGap = 4.0
	svgInfoSegGap  = 16.0
)

// iconWidthRe extracts an octicon fragment's declared pixel width so a
// row can vertically center icons of mixed sizes (16px vs 11px).
var iconWidthRe = regexp.MustCompile(`width="(\d+)"`)

// SVGInfoSegment is one icon + label pair in an info row. Class, when
// set, wraps the segment in a `<g class="...">` (e.g. "language") so the
// section keeps its DOM hook.
type SVGInfoSegment struct {
	Icon  string
	Text  string
	Class string
}

// SVGInfoRow lays the repository info segments (language / license /
// stars / forks / issues / PRs) out left-to-right starting at (x, top).
// Each segment's octicon keeps the grey icon fill (a language dot's own
// path color is preserved); the label is 13px grey. Returns the markup
// and the row height.
func SVGInfoRow(x, top float64, segs []SVGInfoSegment) (string, float64) {
	baseline := top + svgInfoRowH/2 + svgInfoFont*baselineRatio
	var b strings.Builder
	cx := x
	for _, s := range segs {
		iconW := 16.0
		if m := iconWidthRe.FindStringSubmatch(s.Icon); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				iconW = float64(n)
			}
		}
		iconY := top + (svgInfoRowH-iconW)/2
		var seg strings.Builder
		seg.WriteString(SVGIcon(cx, iconY, svgIconFill, s.Icon))
		tx := cx + iconW + svgInfoIconGap
		seg.WriteString(SVGText(tx, baseline, s.Text, SVGTextOpts{Size: svgInfoFont, Fill: svgInfoFill}))
		if s.Class != "" {
			fmt.Fprintf(&b, `<g class=%q>%s</g>`, s.Class, seg.String())
		} else {
			b.WriteString(seg.String())
		}
		cx = tx + fontmetrics.Width(s.Text, svgInfoFont) + svgInfoSegGap
	}
	return b.String(), svgInfoRowH
}

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

// SVGAvatarSpec describes one avatar in a grid. IsOrg selects the
// rounded-square clip (organization) over the default circle (user).
type SVGAvatarSpec struct {
	URL   string
	IsOrg bool
}

// SVGAvatarGrid lays a list of avatars out left-to-right within availWidth,
// wrapping to a new row when the next avatar would overflow. size is the
// avatar edge length and gap the spacing between avatars; the grid's
// top-left is (x, top). clipPrefix seeds unique clipPath ids. Returns the
// markup and the height consumed (0 for an empty list).
func SVGAvatarGrid(x, top, availWidth, size, gap float64, clipPrefix string, avatars []SVGAvatarSpec) (string, float64) {
	if len(avatars) == 0 {
		return "", 0
	}
	perRow := int((availWidth + gap) / (size + gap))
	if perRow < 1 {
		perRow = 1
	}
	var b strings.Builder
	col, rows := 0, 1
	cx, cy := x, top
	for i, a := range avatars {
		if col == perRow {
			col, cx = 0, x
			cy += size + gap
			rows++
		}
		b.WriteString(SVGAvatar(cx, cy, size, a.URL, fmt.Sprintf("%s-%d", clipPrefix, i), !a.IsOrg))
		cx += size + gap
		col++
	}
	return b.String(), float64(rows)*(size+gap) - gap
}

const (
	// h3 sub-header style (`h3 { font-size: 14px; color: #0366d6; margin:
	// 8px 0 2px }`). The chart columns center their h3 (`.column {
	// align-items: center }`), so SVGSubHeader anchors on the column
	// center.
	svgSubHeaderTop  = 8.0
	svgSubHeaderBand = 16.0

	// SubHeaderPitch is the vertical space one `<h3>` consumes:
	// margin-top(8) + band(16) + margin-bottom(2).
	SubHeaderPitch = svgSubHeaderTop + svgSubHeaderBand + 2
)

// SVGSubHeader renders an `<h3>` chart sub-title as native SVG: 14px
// blue text centered on centerX with its block top at y=top,
// ellipsis-truncated to maxWidth. Returns the markup and the height
// consumed (SubHeaderPitch).
func SVGSubHeader(centerX, top, maxWidth float64, label string) (string, float64) {
	baseline := top + svgSubHeaderTop + svgSubHeaderBand/2 + svgBodyFont*baselineRatio
	m := SVGText(centerX, baseline, label, SVGTextOpts{
		Size:     svgBodyFont,
		Fill:     svgHeaderFill,
		Anchor:   "middle",
		MaxWidth: maxWidth,
	})
	return m, SubHeaderPitch
}

const (
	// `.chart-bars` bar geometry: 7px-wide rounded bars whose height maps
	// a 0..1 share onto svgBarMaxHeight (upstream `.bar { width: 7px;
	// border-radius: 5px }` with the height set inline). The tiny value
	// label above each bar is 6px (`.value { font-size: 6px }`); the
	// x-axis label below is 10px grey (`.entry { font-size: 10px; color:
	// #666666 }`).
	svgBarWidth     = 7.0
	svgBarMaxHeight = 50.0
	svgBarRadius    = 3.0
	svgBarValueFont = 6.0
	svgBarValueGap  = 2.0
	svgBarLabelFont = 10.0
	svgBarLabelGap  = 2.0
	svgBarTopGap    = 8.0 // `.chart-bars { margin-top: 8px }`
	svgBarLabelFill = "#666666"
)

// VBar is one bar of a vertical chart-bars column. Value is the small
// label above the bar ("" hides it); Label is the x-axis tick below it;
// Caption is an optional second line under the tick (the month boundary
// caption); Share (0..1) sets the bar height; Level (1..4) picks the
// contribution-graph color.
type VBar struct {
	Value   string
	Label   string
	Caption string
	Share   float64
	Level   int
}

// SVGVBars renders a vertical `.chart-bars` block: bars flow left to
// right, evenly spaced across [x, x+width], bottoms aligned, with the
// block top at y=top. Bar fills use the literal contribution-graph ramp
// (resvg does not resolve the CSS variables). Returns the markup and the
// height the block consumes. A caption row is reserved only when some bar
// carries one.
func SVGVBars(x, top, width float64, bars []VBar) (string, float64) {
	if len(bars) == 0 {
		return "", 0
	}
	valueRoom := svgBarValueFont + svgBarValueGap
	barsTop := top + svgBarTopGap
	baseline := barsTop + valueRoom + svgBarMaxHeight
	labelBaseline := baseline + svgBarLabelGap + svgBarLabelFont
	captionBaseline := labelBaseline + svgBarLabelFont

	span := width / float64(len(bars))
	hasCaption := false

	var b strings.Builder
	b.WriteString(`<g data-block="chart-bars">`)
	for i, bar := range bars {
		center := x + span*(float64(i)+0.5)
		barH := bar.Share * svgBarMaxHeight
		if barH < 0 {
			barH = 0
		}
		barTop := baseline - barH
		if bar.Value != "" {
			b.WriteString(SVGText(center, barTop-svgBarValueGap, bar.Value, SVGTextOpts{
				Size: svgBarValueFont, Fill: svgBarLabelFill, Anchor: "middle",
			}))
		}
		fmt.Fprintf(&b,
			`<rect x="%s" y="%s" width="%d" height="%s" rx="%d" ry="%d" fill=%q/>`,
			pos(center-svgBarWidth/2), pos(barTop), int(svgBarWidth), pos(barH),
			int(svgBarRadius), int(svgBarRadius), CalendarLevelColor(bar.Level))
		b.WriteString(SVGText(center, labelBaseline, bar.Label, SVGTextOpts{
			Size: svgBarLabelFont, Fill: svgBarLabelFill, Anchor: "middle",
		}))
		if bar.Caption != "" {
			hasCaption = true
			b.WriteString(SVGText(center, captionBaseline, bar.Caption, SVGTextOpts{
				Size: svgBarLabelFont, Fill: svgBarLabelFill, Anchor: "middle",
			}))
		}
	}
	b.WriteString(`</g>`)

	height := svgBarTopGap + valueRoom + svgBarMaxHeight + svgBarLabelGap + svgBarLabelFont
	if hasCaption {
		height += svgBarLabelFont
	}
	return b.String(), height
}
