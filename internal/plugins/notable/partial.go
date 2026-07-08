package notable

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// notableOcticon is the upstream `<%- octicon "rocket" %>` 16x16 path
// used in the notable section header (notable.ejs line 4).
const notableOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1 2.5A2.5 2.5 0 013.5 0h8.75a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0V1.5h-8a1 1 0 00-1 1v6.708A2.492 2.492 0 013.5 9h3.25a.75.75 0 010 1.5H3.5a1 1 0 100 2h5.75a.75.75 0 010 1.5H3.5A2.5 2.5 0 011 11.5v-9zm13.23 7.79a.75.75 0 001.06-1.06l-2.505-2.505a.75.75 0 00-1.06 0L9.22 9.229a.75.75 0 001.06 1.061l1.225-1.224v6.184a.75.75 0 001.5 0V9.066l1.224 1.224z"></path></svg>`

// Gauge icon octicons rendered next to each indepth gauge, mirroring the
// upstream notable.ejs commit / star / issue / pull markup.
const (
	commitsIcon = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`
	starsIcon   = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`
	issuesIcon  = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`
	pullsIcon   = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`
)

// Chip geometry. Each chip is a rounded-border box laid out as native SVG
// (mirroring `.contribution.organization`: 1px border, 6px radius, 2px/6px
// padding with padding-left:0, 2px margin, 12px font). Elements are
// vertically centered on the chip (`align-items: center`).
const (
	chipBorder      = 1.0
	chipAvMargin    = 4.0 // `.organization.avatar { margin: 0 4px }`
	chipAvSize      = 16.0
	chipNameFont    = 12.0
	chipGaugeMargin = 4.0 // `.contribution .gauge { margin: 0 4px }`
	chipGaugeSize   = 16.0
	chipIconMargin  = 2.0  // `.contribution .icon { margin-left: 2px }`
	chipIconSize    = 10.0 // `.contribution .icon { width/height: 10px }`
	chipIconScale   = "0.625"
	chipPadRight    = 6.0
	chipHeight      = 22.0 // content 16 + padding 2*2 + border 1*2
	chipMarginX     = 2.0
	chipMarginY     = 2.0
	chipRowPitch    = chipHeight + 2*chipMarginY

	// chipStatAdvance is the horizontal space one indepth stat (gauge +
	// its type icon) consumes: gauge margins (4+4) + gauge (16) + icon
	// margin (2) + icon (10).
	chipStatAdvance = 2*chipGaugeMargin + chipGaugeSize + chipIconMargin + chipIconSize

	// contribInset mirrors `.organization.contributions { margin: 0 8px }`.
	contribInset = 8.0

	// baselineRatio approximates the center-to-baseline offset for the
	// chip label, matching the chrome.SVG* primitives.
	baselineRatio = 0.32
)

// itoa formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func itoa(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// chipStyle holds the literal colors one contribution-level chip resolves
// to. Upstream leans on `currentColor`/CSS class chains for these; resvg
// does not resolve that, so the writer emits literal fills (#409 Phase B).
type chipStyle struct {
	text, bg, bgOpacity, border string
}

// chipColors mirrors the per-level `.contribution.organization.{s,a,b,c}`
// rules (text color, background at ~15%/12.5% alpha, border color). The
// default (no level) is the neutral grey box the base rule sets.
func chipColors(level string) chipStyle {
	switch level {
	case "s":
		return chipStyle{"#EB355E", "#EB355E", "0.149", "#EB355E"}
	case "a":
		return chipStyle{"#D79533", "#E7BD69", "0.149", "#E7BD69"}
	case "b":
		return chipStyle{"#9D8FFF", "#9E91FF", "0.149", "#9E91FF"}
	case "c":
		return chipStyle{"#58A6FF", "#58A6FF", "0.149", "#58A6FF"}
	default:
		return chipStyle{"#777777", "#959DA5", "0.125", "#959DA5"}
	}
}

// darken halves each RGB channel, standing in for the `.contribution .icon
// { filter: brightness(.5) }` the type icons carry (resvg does not apply
// CSS filters).
func darken(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return hex
	}
	r := (v >> 16) & 0xFF
	g := (v >> 8) & 0xFF
	b := v & 0xFF
	return fmt.Sprintf("#%02X%02X%02X", r/2, g/2, b/2)
}

// contributionLevel mirrors the upstream class suffix on each chip
// (notable.ejs): maintainer="s", then percentage tiers a/b/c, else "".
func contributionLevel(c NotableContrib) string {
	switch {
	case c.Maintainer:
		return "s"
	case c.Percentage > 0.2:
		return "a"
	case c.Percentage > 0.1:
		return "b"
	case c.Percentage > 0.05:
		return "c"
	default:
		return ""
	}
}

// chipStat is one indepth gauge (commits / stars / issues / pulls).
type chipStat struct {
	fraction float64
	value    string
	icon     string
}

// chipStats collects the indepth gauge stack for a chip, each stat guarded
// on a non-zero counter (mirroring upstream's `<% if (commits) %>` guards).
// Returns nil in basic mode.
func chipStats(c NotableContrib, totalStars int, list []NotableContrib) []chipStat {
	if !c.Indepth {
		return nil
	}
	stats := make([]chipStat, 0, 4)
	if c.Commits > 0 {
		stats = append(stats, chipStat{c.Percentage, strconv.Itoa(c.Commits), commitsIcon})
	}
	if c.StargazerCount > 0 {
		stats = append(stats, chipStat{float64(c.StargazerCount) / float64(max(totalStars, 1)), partials.FormatCount(int64(c.StargazerCount)), starsIcon})
	}
	if c.Issues > 0 {
		stats = append(stats, chipStat{float64(c.Issues) / float64(max(totalIssues(list), 1)), strconv.Itoa(c.Issues), issuesIcon})
	}
	if c.Pulls > 0 {
		stats = append(stats, chipStat{float64(c.Pulls) / float64(max(totalPulls(list), 1)), strconv.Itoa(c.Pulls), pullsIcon})
	}
	return stats
}

// chipWidth is the total pixel width a chip occupies given its measured
// label width and stat count.
func chipWidth(nameW float64, nStats int) float64 {
	return 2*chipBorder + 2*chipAvMargin + chipAvSize + nameW + float64(nStats)*chipStatAdvance + chipPadRight
}

// maxBasicChipNameLen caps the chip label so the 480 px card width still
// wraps deterministically (issue #422). The chip font-size is 12 px, so a
// 36-character ceiling keeps even the widest chip narrower than the card.
const maxBasicChipNameLen = 36

// truncateName ellipsizes a chip label once it exceeds the per-chip budget
// defined by maxBasicChipNameLen, on rune boundaries so multi-byte names
// are not split mid-character.
func truncateName(name string, limit int) string {
	if limit <= 0 {
		return name
	}
	runes := []rune(name)
	if len(runes) <= limit {
		return name
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

// Partial renders the classic SVG fragment for the notable plugin as
// native SVG (#409 Phase B4), mirroring upstream
// source/templates/classic/partials/notable.ejs (issue #447).
//
// Output: a `<g data-section="notable">` anchor wrapping a nested
// `<svg>`. The rocket-octicon header sits above a `<g class="row
// organization contributions">` of flow-wrapped chips. Each chip is a
// `<g class="organization contribution <level> ">` with a rounded-border
// background, the owner avatar, the "@owner" (or "@owner/repo") label, and
// — in indepth mode — the upstream gauge stack (commits / stars / issues /
// pulls), each gauge guarded on a non-zero counter. Returns the markup and
// the pixel height it consumes.
//
// Basic mode renders only the avatar + owner chip (no star badge — the
// stray "★ N" badge from #422 was a regression). The chip width is
// content-driven, matching upstream's natural flex sizing (#557).
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.List) == 0 {
		return "", 0, nil
	}

	// total.stars normalizes the star gauge (upstream `total.stars`).
	totalStars := 0
	for _, c := range r.List {
		totalStars += c.StargazerCount
	}

	header, hh := chrome.SVGSectionHeader(notableOcticon, "Notable contributions")

	var body strings.Builder
	body.WriteString(header)
	body.WriteString(`<g class="row organization contributions">`)

	x := contribInset
	rowTop := 0.0
	rows := 1
	maxRight := float64(chrome.CardWidth) - contribInset
	for i, c := range r.List {
		level := contributionLevel(c)
		label := "@" + truncateName(c.Name, maxBasicChipNameLen)
		nameW := fontmetrics.Width(label, chipNameFont)
		stats := chipStats(c, totalStars, r.List)
		w := chipWidth(nameW, len(stats))

		if x+w > maxRight && x > contribInset {
			x = contribInset
			rowTop += chipRowPitch
			rows++
		}
		chipY := hh + rowTop + chipMarginY
		writeChip(&body, x, chipY, i, c, level, label, nameW, stats)
		x += w + chipMarginX
	}

	body.WriteString(`</g>`)
	height := int(hh + float64(rows)*chipRowPitch)
	return chrome.WrapSection("notable", height, body.String()), height, nil
}

// writeChip emits one contribution chip anchored at (chipX, chipY): a
// rounded-border background box, the owner avatar, the label, and the
// indepth gauge stack. The `organization contribution <level>` class
// (with upstream's trailing space) is preserved for the DOM hook.
func writeChip(b *strings.Builder, chipX, chipY float64, idx int, c NotableContrib, level, label string, nameW float64, stats []chipStat) {
	style := chipColors(level)
	w := chipWidth(nameW, len(stats))
	centerY := chipHeight / 2

	fmt.Fprintf(b, `<g class="organization contribution %s " transform="translate(%s,%s)">`,
		level, itoa(chipX), itoa(chipY))
	fmt.Fprintf(b,
		`<rect x="0" y="0" width="%s" height="%d" rx="6" ry="6" fill="%s" fill-opacity="%s" stroke="%s" stroke-width="1"/>`,
		itoa(w), int(chipHeight), style.bg, style.bgOpacity, style.border)

	// Avatar: organization → 15%-rounded square, user → circle.
	avX := chipBorder + chipAvMargin
	avY := centerY - chipAvSize/2
	b.WriteString(chrome.SVGAvatar(avX, avY, chipAvSize, c.AvatarURL, fmt.Sprintf("notable-%d", idx), !c.Organization))

	// Label ("@owner" or "@owner/repo"), vertically centered.
	nameX := chipBorder + 2*chipAvMargin + chipAvSize
	nameBaseline := centerY + chipNameFont*baselineRatio
	b.WriteString(chrome.SVGText(nameX, nameBaseline, label, chrome.SVGTextOpts{Size: chipNameFont, Fill: style.text}))

	// Indepth gauge stack (gauge + type icon per stat).
	iconColor := darken(style.text)
	cx := nameX + nameW
	for _, s := range stats {
		gx := cx + chipGaugeMargin
		gy := centerY - chipGaugeSize/2
		writeChipGauge(b, gx, gy, s.fraction, s.value, style.text)
		ix := gx + chipGaugeSize + chipGaugeMargin + chipIconMargin
		iy := centerY - chipIconSize/2
		fmt.Fprintf(b, `<g transform="translate(%s,%s) scale(%s)" fill="%s">%s</g>`,
			itoa(ix), itoa(iy), chipIconScale, iconColor, s.icon)
		cx = ix + chipIconSize
	}
	b.WriteString(`</g>`)
}

// writeChipGauge renders one indepth gauge as a nested 30-unit viewBox
// `<svg class="gauge">` scaled to 16px. It keeps the class names for the
// browser animation and carries literal stroke/fill attributes (the
// chip's resolved level color) so resvg renders the arc without the CSS
// `currentColor` chain. The inline `stroke-dasharray` final value is
// preserved.
func writeChipGauge(b *strings.Builder, gx, gy, fraction float64, value, color string) {
	fmt.Fprintf(b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30" width="16" height="16" x="%s" y="%s" class="gauge">`,
		itoa(gx), itoa(gy))
	fmt.Fprintf(b,
		`<circle class="gauge-base" r="12.5" cx="15" cy="15" fill="none" stroke="%s" stroke-width="4" stroke-opacity="0.2" stroke-linecap="round"></circle>`,
		color)
	fmt.Fprintf(b,
		`<circle class="gauge-arc" transform="rotate(-90 15 15)" r="12.5" cx="15" cy="15" fill="none" stroke="%s" stroke-width="4" stroke-linecap="round" stroke-dasharray="%s 329"></circle>`,
		color, formatFraction(fraction*329))
	fmt.Fprintf(b,
		`<text x="15" y="15" dominant-baseline="central" text-anchor="middle" font-size="11" fill="%s">%s</text>`,
		color, partials.EscapeXML(value))
	b.WriteString(`</svg>`)
}

func totalIssues(list []NotableContrib) int {
	total := 0
	for _, c := range list {
		total += c.Issues
	}
	return total
}

func totalPulls(list []NotableContrib) int {
	total := 0
	for _, c := range list {
		total += c.Pulls
	}
	return total
}

// formatFraction trims a float to a compact, deterministic form so the
// stroke-dasharray matches upstream's numeric output without trailing
// zeros.
func formatFraction(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
