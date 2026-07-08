package habits

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// brainOcticon is the upstream `<%- octicon "light-bulb" %>`-style 16x16
// path used in the habits section header (EJS line 4).
const brainOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 01-1.484.211c-.04-.282-.163-.547-.37-.847a8.695 8.695 0 00-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.75.75 0 01-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75zM6 15.25a.75.75 0 01.75-.75h2.5a.75.75 0 010 1.5h-2.5a.75.75 0 01-.75-.75zM5.75 12a.75.75 0 000 1.5h4.5a.75.75 0 000-1.5h-4.5z"></path></svg>`

// dayNames mirrors upstream's `["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]`.
var dayNames = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// Native-SVG layout metrics. The facts header mirrors the CSS
// `.field`/`h2` box (section-inset 5px, 8px icon margins, 16px octicon,
// 16px blue label) so the icon+label line up with the shared section
// headers; the fact list indents 37px (`.habits .facts { padding-left:
// 37px }`). The chart metrics mirror the `.chart` column padding.
const (
	hdrInset         = 5.0
	hdrIconGap       = 8.0
	hdrIconSize      = 16.0
	hdrTop           = 8.0
	hdrBand          = 18.0
	hdrFont          = 16.0
	hdrFill          = "#0366d6"
	hdrIconFill      = "#959da5" // `.field svg { fill: #959da5 }`
	hdrDetailFont    = 11.0      // `.h-details { font-size: 0.8rem }`
	hdrBaselineRatio = 0.32

	factIndent    = 37.0 // `.habits .facts { padding-left: 37px }`
	factFont      = 14.0
	factFill      = "#777777"
	factLineTop   = 4.0
	factLinePitch = 20.0

	chartPad = 8.0 // `.chart { padding: 0 8px }`

	// Language-activity horizontal-bar metrics (`.chart-bars.horizontal`):
	// a right-aligned 34%-width name column, a 7px-tall share-scaled bar
	// with 6px side margins, then the percentage value.
	langTopGap       = 8.0 // `.chart-bars { margin-top: 8px }`
	langRowPitch     = 18.0
	langBarHeight    = 7.0
	langBarRadius    = 3.0
	langNameFrac     = 0.34
	langBarMargin    = 6.0
	langValueReserve = 40.0
	langFont         = 10.0
	langFill         = "#666666"
)

// itoa formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func itoa(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// maxIntSlice returns the largest entry in a slice/array of ints; 0
// when empty so callers divide safely.
func maxIntSlice(xs []int) int {
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

// dominantHourIdx returns the hour (0-23) with the largest commit count,
// matching upstream's `plugins.habits.commits.hour`. Returns -1 when
// all entries are zero (mirrors upstream's `Number.isNaN` guard).
func dominantHourIdx(hours [24]int) int {
	best, bestIdx := 0, -1
	for i, n := range hours {
		if n > best {
			best, bestIdx = n, i
		}
	}
	return bestIdx
}

// dominantDayName returns the weekday name with the largest commit
// count, or "" when all entries are zero.
func dominantDayName(days [7]int) string {
	best, bestIdx := 0, -1
	for i, n := range days {
		if n > best {
			best, bestIdx = n, i
		}
	}
	if bestIdx < 0 {
		return ""
	}
	return dayNames[bestIdx]
}

// bgLevel maps a 0..1 share to the contribution-graph L{1..4} ramp via
// `Math.ceil(p/0.25)` (clamped to 1..4).
func bgLevel(p float64) int {
	if p <= 0 {
		return 1
	}
	lvl := int(math.Ceil(p / 0.25))
	if lvl < 1 {
		lvl = 1
	}
	if lvl > 4 {
		lvl = 4
	}
	return lvl
}

// Partial renders the classic SVG fragment for the habits plugin as
// native SVG (#409 Phase B5). Mirrors upstream
// org_repo/source/templates/classic/partials/habits.ejs.
//
// Output: a `<section data-section="habits">` anchor wrapping a nested
// `<svg>` with two stacked blocks:
//
//  1. facts — the light-bulb section header + a list of `<text>` facts
//     ("Uses spaces for indentation", "Mostly active on Fri", etc.).
//  2. charts — `<rect>` chart-bars for commit activity per hour of day
//     (full width) and, side by side, per day of week plus the
//     horizontal language-activity chart.
//
// Bar colors are emitted as literal contribution-graph hex (resvg does
// not resolve the CSS `var()` ramp). Returns the markup and the pixel
// height it consumes.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", 0, nil
	}
	if !r.FactsEnabled && !r.ChartsEnabled {
		return "", 0, nil
	}

	hourIdx := dominantHourIdx(r.Charts.Hours)
	dayName := dominantDayName(r.Charts.Days)
	langAvailable := r.Linguist.Available && len(r.Linguist.Ordered) > 0
	factsHasContent := r.Facts.IndentStyle != "" ||
		r.Facts.CharsPerLine > 0 ||
		hourIdx >= 0 ||
		dayName != ""
	chartsHasContent := hourIdx >= 0 || dayName != "" || langAvailable
	if (!r.FactsEnabled || !factsHasContent) && (!r.ChartsEnabled || !chartsHasContent) {
		return "", 0, nil
	}

	var body strings.Builder
	y := 0.0

	if r.FactsEnabled && factsHasContent {
		y = writeFactsBlock(&body, r, hourIdx, dayName, y)
	}

	if r.ChartsEnabled && chartsHasContent {
		if hourIdx >= 0 {
			y = writeHourChart(&body, r.Charts.Hours, r.Trim, y)
		}
		// Upstream wraps the day chart and the language-activity chart in
		// a single `<div class="row largeable">` so they sit side by side.
		present := 0
		if dayName != "" {
			present++
		}
		if langAvailable {
			present++
		}
		if present > 0 {
			colW := float64(chrome.CardWidth) / float64(present)
			rowTop := y
			rowH := 0.0
			col := 0.0
			if dayName != "" {
				h := writeDayChart(&body, r.Charts.Days, col*colW, rowTop, colW)
				rowH = math.Max(rowH, h-rowTop)
				col++
			}
			if langAvailable {
				h := writeLanguageChart(&body, r.Linguist.Ordered, col*colW, rowTop, colW)
				rowH = math.Max(rowH, h-rowTop)
			}
			y = rowTop + rowH
		}
	}

	height := int(y)
	return chrome.WrapSection("habits", height, body.String()), height, nil
}

// writeFactsBlock emits the light-bulb section header (with the optional
// "(computed from last N commits)" detail) and the facts list, starting
// at y=top. Returns the y cursor after the block.
func writeFactsBlock(b *strings.Builder, r *Result, hourIdx int, dayName string, top float64) float64 {
	iconX := hdrInset + hdrIconGap
	iconY := top + hdrTop + (hdrBand-hdrIconSize)/2
	textX := iconX + hdrIconSize + hdrIconGap
	baseline := top + hdrTop + hdrBand/2 + hdrFont*hdrBaselineRatio

	b.WriteString(chrome.SVGIcon(iconX, iconY, hdrIconFill, brainOcticon))
	b.WriteString(chrome.SVGText(textX, baseline, "Recent coding habits", chrome.SVGTextOpts{Size: hdrFont, Fill: hdrFill}))
	if r.From > 0 {
		detailX := textX + fontmetrics.Width("Recent coding habits", hdrFont) + 4
		detail := fmt.Sprintf("(computed from last %d commit%s)", r.From, pluginutil.Plural(r.From))
		b.WriteString(chrome.SVGText(detailX, baseline, detail, chrome.SVGTextOpts{Size: hdrDetailFont, Fill: hdrFill}))
	}

	y := top + chrome.SectionHeaderPitch
	writeFact := func(text string) {
		b.WriteString(chrome.SVGText(factIndent, y+factLineTop+factFont, text, chrome.SVGTextOpts{Size: factFont, Fill: factFill}))
		y += factLinePitch
	}
	if r.Facts.IndentStyle != "" {
		writeFact(fmt.Sprintf("Uses %s for indentation", r.Facts.IndentStyle))
	}
	if r.Facts.CharsPerLine > 0 {
		writeFact(fmt.Sprintf("Has approximately %.1f characters per line of code written", r.Facts.CharsPerLine))
	}
	if hourIdx >= 0 {
		writeFact(fmt.Sprintf("Mostly pushes code around %d:00", hourIdx))
	}
	if dayName != "" {
		writeFact(fmt.Sprintf("Mostly active on %s", dayName))
	}
	return y
}

// writeHourChart emits the full-width hour-of-day chart-bars block. When
// trim=true the leading + trailing zero-count hours are stripped (matches
// upstream's `displayedValues.shift()` / `pop()`). Returns the y cursor
// after the block.
func writeHourChart(b *strings.Builder, hours [24]int, trim bool, top float64) float64 {
	innerW := float64(chrome.CardWidth) - 2*chartPad
	m, hh := chrome.SVGSubHeader(float64(chrome.CardWidth)/2, top, innerW, "Commit activity per hour of day")
	b.WriteString(m)
	bars, bh := chrome.SVGVBars(chartPad, top+hh, innerW, buildHourBars(hours, trim))
	b.WriteString(bars)
	return top + hh + bh
}

func buildHourBars(hours [24]int, trim bool) []chrome.VBar {
	maxN := maxIntSlice(hours[:])
	if maxN == 0 {
		maxN = 1
	}
	start, end := 0, 24
	if trim {
		for start < 24 && hours[start] == 0 {
			start++
		}
		for end > start && hours[end-1] == 0 {
			end--
		}
	}
	bars := make([]chrome.VBar, 0, end-start)
	for h := start; h < end; h++ {
		share := float64(hours[h]) / float64(maxN)
		bars = append(bars, chrome.VBar{
			Value: strconv.Itoa(hours[h]),
			Label: fmt.Sprintf("%02d", h),
			Share: share,
			Level: bgLevel(share),
		})
	}
	return bars
}

// writeDayChart emits the day-of-week chart-bars column at x=colX (width
// colW), starting at y=top. Returns the y cursor after the block.
func writeDayChart(b *strings.Builder, days [7]int, colX, top, colW float64) float64 {
	innerW := colW - 2*chartPad
	m, hh := chrome.SVGSubHeader(colX+colW/2, top, innerW, "Commit activity per day of week")
	b.WriteString(m)
	bars, bh := chrome.SVGVBars(colX+chartPad, top+hh, innerW, buildDayBars(days))
	b.WriteString(bars)
	return top + hh + bh
}

func buildDayBars(days [7]int) []chrome.VBar {
	maxN := maxIntSlice(days[:])
	if maxN == 0 {
		maxN = 1
	}
	bars := make([]chrome.VBar, 0, 7)
	for d := 0; d < 7; d++ {
		share := float64(days[d]) / float64(maxN)
		bars = append(bars, chrome.VBar{
			Value: strconv.Itoa(days[d]),
			Label: dayNames[d],
			Share: share,
			Level: bgLevel(share),
		})
	}
	return bars
}

// writeLanguageChart emits the horizontal "Language activity" chart-bars
// column at x=colX (width colW), starting at y=top. Each row is a
// right-aligned name, a width-scaled bar, and a percentage value,
// mirroring upstream's `.chart-bars.horizontal` layout. Returns the y
// cursor after the block.
func writeLanguageChart(b *strings.Builder, langs []LanguageShare, colX, top, colW float64) float64 {
	innerW := colW - 2*chartPad
	m, hh := chrome.SVGSubHeader(colX+colW/2, top, innerW, "Language activity")
	b.WriteString(m)

	barsTop := top + hh + langTopGap
	x0 := colX + chartPad
	nameW := innerW * langNameFrac
	nameRight := x0 + nameW
	barStart := nameRight + langBarMargin
	barMaxW := innerW - nameW - 2*langBarMargin - langValueReserve
	if barMaxW < 0 {
		barMaxW = 0
	}
	for i, ls := range langs {
		cy := barsTop + float64(i)*langRowPitch + langRowPitch/2
		txtBaseline := cy + langFont*hdrBaselineRatio
		b.WriteString(chrome.SVGText(nameRight, txtBaseline, ls.Name, chrome.SVGTextOpts{
			Size: langFont, Fill: langFill, Anchor: "end", MaxWidth: nameW,
		}))
		barW := ls.Share * barMaxW
		fmt.Fprintf(b,
			`<rect x="%s" y="%s" width="%s" height="%d" rx="%d" ry="%d" fill=%q/>`,
			itoa(barStart), itoa(cy-langBarHeight/2), itoa(barW), int(langBarHeight),
			int(langBarRadius), int(langBarRadius), chrome.CalendarLevelColor(bgLevel(ls.Share)))
		pct := int(math.Round(100 * ls.Share))
		b.WriteString(chrome.SVGText(barStart+barW+langBarMargin, txtBaseline, fmt.Sprintf("%d%%", pct),
			chrome.SVGTextOpts{Size: langFont, Fill: langFill}))
	}
	return barsTop + float64(len(langs))*langRowPitch + 4
}
