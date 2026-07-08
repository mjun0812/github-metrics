package calendar

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// calendarOcticon mirrors upstream EJS line 4 — the "calendar" octicon.
const calendarOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4.75 0a.75.75 0 01.75.75V2h5V.75a.75.75 0 011.5 0V2h1.25c.966 0 1.75.784 1.75 1.75v10.5A1.75 1.75 0 0113.25 16H2.75A1.75 1.75 0 011 14.25V3.75C1 2.784 1.784 2 2.75 2H4V.75A.75.75 0 014.75 0zm0 3.5h8.5a.25.25 0 01.25.25V6h-11V3.75a.25.25 0 01.25-.25h2zm-2.25 4v6.75c0 .138.112.25.25.25h10.5a.25.25 0 00.25-.25V7.5h-11z"></path></svg>`

// heatmapViewBoxWidth is the intrinsic width of the multi-year heatmap
// (upstream EJS line 15 authors it against a 795-unit viewBox). The
// nested `<svg>` renders it at the 480px card width with a uniform
// `xMidYMin meet` scale, so the drawn height is 480/795 of the viewBox
// height — computed in Go so the partial can self-report it (#409 Phase B6).
const heatmapViewBoxWidth = 795.0

// yearBandHeight is the per-year viewBox band (upstream EJS line 15 uses
// 130 units per year).
const yearBandHeight = 130

// yearLabelFont / yearLabelFill are the literal year-label text style.
// Upstream renders it via `svg.calendar text { font-size: 18px; fill:
// currentColor }`; currentColor inherits the global `svg { color:
// #777777 }`. Emitted as literal attributes because resvg does not
// resolve `currentColor` through the class chain (#409 decision log).
const (
	yearLabelFont = 18
	yearLabelFill = "#777777"
)

// Partial renders the classic SVG fragment for the calendar plugin as
// native SVG (#409 Phase B6). Multi-year heatmap matching upstream
// org_repo/source/templates/classic/partials/calendar.ejs.
//
// Output (native SVG): a `<section data-section="calendar">` anchor
// wrapping a nested `<svg>` — the "Contributions calendar" section header
// above the multi-year heatmap. The heatmap keeps its intrinsic 795-unit
// viewBox and scales to the 480px card width; the partial reports the
// resulting pixel height it consumes.
//
// Settings: mjun0812 uses plugin_calendar: yes, plugin_calendar_limit: 3.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Years) == 0 {
		return "", 0, nil
	}

	header, hh := chrome.SVGSectionHeader(calendarOcticon, "Contributions calendar")

	viewboxHeight := yearBandHeight * len(r.Years)
	// width=100% scales the 795-unit viewBox down to the 480px card, so
	// the drawn height is 480/795 of the viewBox height.
	scale := float64(chrome.CardWidth) / heatmapViewBoxWidth
	heatmapHeight := int(math.Round(float64(viewboxHeight) * scale))
	heatmapTop := int(math.Round(hh))

	var body strings.Builder
	body.WriteString(header)
	fmt.Fprintf(
		&body,
		`<svg class="calendar" version="1.1" xmlns="http://www.w3.org/2000/svg" x="0" y="%d" width="%d" height="%d" viewBox="0,0 795,%d" preserveAspectRatio="xMidYMin meet" role="img" aria-label="Multi-year contribution heatmap"><title>Multi-year contribution heatmap</title>`,
		heatmapTop, chrome.CardWidth, heatmapHeight, viewboxHeight,
	)

	for r0, yearCal := range r.Years {
		// Per-year group: <g transform="translate(0, 14 + r*130)">
		// (year label at y=0, cells offset down).
		yOffset := 14 + r0*yearBandHeight
		fmt.Fprintf(&body, `<g transform="translate(0, %d)">`, yOffset)
		// Year label — literal font-size/fill so resvg renders it without
		// the `svg.calendar text` class rule or `currentColor`.
		fmt.Fprintf(&body, `<text x="0" y="0" font-size="%d" fill="%s">%d</text>`,
			yearLabelFont, yearLabelFill, yearCal.Year)
		// Per-week column.
		for x, week := range yearCal.Weeks {
			fmt.Fprintf(&body, `<g transform="translate(%d, 0)">`, x*15)
			// First week may be short (year-boundary partial week);
			// shift cells down so they align with the right weekday.
			missingDays := 0
			if x == 0 && len(week.ContributionDays) < 7 {
				missingDays = 7 - len(week.ContributionDays)
			}
			for y, cell := range week.ContributionDays {
				cellY := 4 + missingDays*15 + y*15
				fmt.Fprintf(
					&body,
					`<rect class="day" x="0" y="%d" width="11" height="11" fill="%s" rx="2" ry="2"/>`,
					cellY, partials.EscapeXML(cell.Color),
				)
			}
			body.WriteString(`</g>`)
		}
		body.WriteString(`</g>`)
	}

	body.WriteString(`</svg>`)

	height := heatmapTop + heatmapHeight
	return chrome.WrapSection("calendar", height, body.String()), height, nil
}
