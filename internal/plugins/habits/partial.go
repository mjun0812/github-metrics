package habits

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
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

// maxIntArr returns the largest entry in a slice/array of ints; 0
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

// bgLevel maps a 0..1 share to the upstream
// `var(--color-calendar-graph-day-L{1..4}-bg)` color ramp via
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

// Partial renders the classic SVG fragment for the habits plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/habits.ejs.
//
// Two sections are emitted:
//
//  1. <section class="habits"> — header + facts list
//     ("Mostly pushes code around HH:00", "Mostly active on Day", etc.)
//
//  2. <section class="habits"> — chart-bars
//     - <h3>Commit activity per hour of day</h3> + chart-bars (24 bars)
//     - <h3>Commit activity per day of week</h3> + chart-bars (7 bars)
//
// The chart-bars block emits plain HTML inside foreignObject (no bare
// SVG primitives), fixing the v1.0.0 bare-`<g>` invisibility bug.
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

	var b strings.Builder
	b.WriteString(`<section data-section="habits">`)

	// ── Section 1: header + facts list ──────────────────────────────
	if r.FactsEnabled && factsHasContent {
		b.WriteString(`<section class="habits">`)
		b.WriteString(`<h2 class="field wrap">`)
		b.WriteString(brainOcticon)
		b.WriteString(`Recent coding habits`)
		if r.From > 0 {
			fmt.Fprintf(
				&b,
				`<small class="h-details">(computed from last %d commit%s)</small>`,
				r.From, pluginutil.Plural(r.From),
			)
		}
		b.WriteString(`</h2>`)

		b.WriteString(`<div class="row"><ul class="facts">`)
		if r.Facts.IndentStyle != "" {
			fmt.Fprintf(&b, `<li>Uses %s for indentation</li>`, partials.EscapeXML(r.Facts.IndentStyle))
		}
		if r.Facts.CharsPerLine > 0 {
			fmt.Fprintf(
				&b,
				`<li>Has approximately %.1f characters per line of code written</li>`,
				r.Facts.CharsPerLine,
			)
		}
		if hourIdx >= 0 {
			fmt.Fprintf(&b, `<li>Mostly pushes code around %d:00</li>`, hourIdx)
		}
		if dayName != "" {
			fmt.Fprintf(&b, `<li>Mostly active on %s</li>`, dayName)
		}
		b.WriteString(`</ul></div>`)
		b.WriteString(`</section>`)
	}

	// ── Section 2: chart-bars (hour + day + language activity) ──────
	if r.ChartsEnabled && chartsHasContent {
		b.WriteString(`<section class="habits">`)
		if hourIdx >= 0 {
			writeHourChart(&b, r.Charts.Hours, r.Trim)
		}
		// Upstream wraps the day chart and the language-activity chart in
		// a single `<div class="row largeable">` so they sit side by side.
		if dayName != "" || langAvailable {
			b.WriteString(`<div class="row largeable">`)
			if dayName != "" {
				writeDayChart(&b, r.Charts.Days)
			}
			if langAvailable {
				writeLanguageChart(&b, r.Linguist.Ordered)
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</section>`)
	}

	b.WriteString(`</section>`)
	return b.String(), 0, nil
}

// writeHourChart emits the hour-of-day chart-bars block (EJS lines 51-72).
// When trim=true the leading + trailing zero-count hours are stripped
// (matches upstream's `displayedValues.shift()` / `pop()`).
func writeHourChart(b *strings.Builder, hours [24]int, trim bool) {
	b.WriteString(`<div class="column chart largeable">`)
	b.WriteString(`<h3>Commit activity per hour of day</h3>`)
	b.WriteString(`<div class="chart-bars">`)
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
	for h := start; h < end; h++ {
		share := float64(hours[h]) / float64(maxN)
		writeBarEntry(b, fmt.Sprintf("%02d", h), hours[h], share)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
}

// writeDayChart emits the day-of-week chart-bars block (EJS lines 75-93).
func writeDayChart(b *strings.Builder, days [7]int) {
	b.WriteString(`<section class="column chart">`)
	b.WriteString(`<h3>Commit activity per day of week</h3>`)
	b.WriteString(`<div class="chart-bars">`)
	maxN := maxIntSlice(days[:])
	if maxN == 0 {
		maxN = 1
	}
	for d := 0; d < 7; d++ {
		share := float64(days[d]) / float64(maxN)
		writeBarEntry(b, dayNames[d], days[d], share)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

// writeLanguageChart emits the "Language activity" chart-bars block
// (EJS lines 91-110, `plugins.habits.linguist`). Unlike the hour/day
// charts this uses the `horizontal` variant: each entry carries a
// `<span class="name">` label, a width-scaled bar (share*80%), and a
// `<span class="value">N%</span>` percentage. The literal heading
// "Language activity" matches upstream's EJS exactly (singular).
func writeLanguageChart(b *strings.Builder, langs []LanguageShare) {
	b.WriteString(`<section class="column chart">`)
	b.WriteString(`<h3>Language activity</h3>`)
	b.WriteString(`<div class="chart-bars horizontal">`)
	for _, ls := range langs {
		width := ls.Share * 80
		lvl := bgLevel(ls.Share)
		pct := int(math.Round(100 * ls.Share))
		fmt.Fprintf(
			b,
			`<div class="entry"><span class="name">%s</span><div class="bar" style="width: %g%%; background-color: var(--color-calendar-graph-day-L%d-bg)"></div><span class="value">%d%%</span></div>`,
			partials.EscapeXML(ls.Name), width, lvl, pct,
		)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

// writeBarEntry emits one `<div class="entry">` row in the chart-bars
// block (EJS lines 66-71, 84-89). height = share*50px, color uses the
// L1..L4 calendar-graph ramp.
func writeBarEntry(b *strings.Builder, label string, value int, share float64) {
	height := share * 50
	lvl := bgLevel(share)
	fmt.Fprintf(
		b,
		`<div class="entry"><span class="value">%d</span><div class="bar" style="height: %.0fpx; background-color: var(--color-calendar-graph-day-L%d-bg)"></div>%s</div>`,
		value, height, lvl, partials.EscapeXML(label),
	)
}
