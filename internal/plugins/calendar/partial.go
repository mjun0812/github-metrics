package calendar

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// calendarOcticon mirrors upstream EJS line 4 — the "calendar" octicon.
const calendarOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4.75 0a.75.75 0 01.75.75V2h5V.75a.75.75 0 011.5 0V2h1.25c.966 0 1.75.784 1.75 1.75v10.5A1.75 1.75 0 0113.25 16H2.75A1.75 1.75 0 011 14.25V3.75C1 2.784 1.784 2 2.75 2H4V.75A.75.75 0 014.75 0zm0 3.5h8.5a.25.25 0 01.25.25V6h-11V3.75a.25.25 0 01.25-.25h2zm-2.25 4v6.75c0 .138.112.25.25.25h10.5a.25.25 0 00.25-.25V7.5h-11z"></path></svg>`

// Partial renders the classic SVG fragment for the calendar plugin.
// Multi-year heatmap matching upstream
// org_repo/source/templates/classic/partials/calendar.ejs.
//
// Settings: mjun0812 uses plugin_calendar: yes, plugin_calendar_limit: 3.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Years) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="calendar">`)
	b.WriteString(`<section>`)

	// Header.
	fmt.Fprintf(&b, `<h2 class="field">%sContributions calendar</h2>`, calendarOcticon)

	b.WriteString(`<div class="row">`)
	b.WriteString(`<section>`)

	// Heatmap SVG — viewBox 0,0 795,130*Y where Y = years count.
	// Per upstream EJS line 15. width="100%" makes the inner SVG scale
	// down to the parent foreignObject's available width (otherwise it
	// renders at its intrinsic 795px width and overflows the 480px outer
	// SVG, leaving a clipped/whitespace appearance below).
	viewboxHeight := 130 * len(r.Years)
	fmt.Fprintf(
		&b,
		`<svg class="calendar" version="1.1" xmlns="http://www.w3.org/2000/svg" width="100%%" viewBox="0,0 795,%d" preserveAspectRatio="xMidYMin meet" role="img" aria-label="Multi-year contribution heatmap"><title>Multi-year contribution heatmap</title>`,
		viewboxHeight,
	)

	for r0, yearCal := range r.Years {
		// Per-year group: <g transform="translate(0, 14 + r*130)">
		// (year label at y=0, cells offset down).
		yOffset := 14 + r0*130
		fmt.Fprintf(&b, `<g transform="translate(0, %d)">`, yOffset)
		// Year label.
		fmt.Fprintf(&b, `<text x="0" y="0">%d</text>`, yearCal.Year)
		// Per-week column.
		for x, week := range yearCal.Weeks {
			fmt.Fprintf(&b, `<g transform="translate(%d, 0)">`, x*15)
			// First week may be short (year-boundary partial week);
			// shift cells down so they align with the right weekday.
			missingDays := 0
			if x == 0 && len(week.ContributionDays) < 7 {
				missingDays = 7 - len(week.ContributionDays)
			}
			for y, cell := range week.ContributionDays {
				cellY := 4 + missingDays*15 + y*15
				fmt.Fprintf(
					&b,
					`<rect class="day" x="0" y="%d" width="11" height="11" fill="%s" rx="2" ry="2"/>`,
					cellY, partials.EscapeXML(cell.Color),
				)
			}
			b.WriteString(`</g>`)
		}
		b.WriteString(`</g>`)
	}

	b.WriteString(`</svg>`)
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
