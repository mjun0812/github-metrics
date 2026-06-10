package stargazers

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// starOcticon is the upstream `<%- octicon "star" %>` 16x16 path used
// in the stargazers section header (EJS line 4).
const starOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// bgLevel maps a 0..1 share to the upstream
// `var(--color-calendar-graph-day-L{1..4}-bg)` ramp via Math.ceil(p/0.25).
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

// Partial renders the classic SVG fragment for the stargazers plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/stargazers.ejs.
//
// Returns "" until stargazers.go's Run is wired with chart data (current
// M4 path stays Skipped). When data arrives, emits the upstream two
// chart-bars columns ("Total stargazers" + "New stargazers") for the
// classic chart type, or the single line plot for the graph type.
//
// Output structure (classic):
//
//	<section data-section="stargazers">
//	  <h2 class="field"><svg star/>Stargazers</h2>
//	  <div class="row margin-bottom">
//	    <section class="column chart">
//	      <h3>Total stargazers</h3>
//	      <div class="chart-bars">…cumulative…</div>
//	    </section>
//	    <section class="column chart">
//	      <h3>New stargazers per month</h3>
//	      <div class="chart-bars">…per-bucket increments…</div>
//	    </section>
//	  </div>
//	</section>
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", nil
	}
	series := r.Charts.Series
	if len(series) == 0 && len(r.List) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="stargazers">`)
	fmt.Fprintf(&b, `<h2 class="field">%sStargazers</h2>`, starOcticon)

	if len(series) > 0 {
		b.WriteString(`<div class="row margin-bottom">`)
		if r.Charts.Type == chartsTypeGraph {
			totals, news := chartValues(series)
			b.WriteString(`<section class="column chart fill-width">`)
			b.WriteString(`<h3>Total stargazers</h3>`)
			writeGraphChart(&b, series, totals, "Total stargazers graph")
			b.WriteString(`<h3>New stargazers per day</h3>`)
			writeGraphChart(&b, series, news, "New stargazers per day graph")
			b.WriteString(`</section>`)
		} else {
			// The classic chart mirrors upstream's two side-by-side
			// chart-bars columns (cumulative + per-bucket new stars).
			writeClassicCharts(&b, series)
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeClassicCharts renders the two upstream chart-bars columns:
// cumulative "Total stargazers" and per-bucket "New stargazers". The Go
// series is bucketed by month (stargazers.buildSeries), so the second
// column is labelled "per month" rather than upstream's daily "per day",
// and each bar's x-axis label is the month (the date is always the 1st
// of its month, which previously rendered as a meaningless "1").
func writeClassicCharts(b *strings.Builder, series []ChartPoint) {
	totals, news := chartValues(series)
	writeChartColumn(b, "Total stargazers", series, totals)
	writeChartColumn(b, "New stargazers per day", series, news)
}

func chartValues(series []ChartPoint) ([]int, []int) {
	totals := make([]int, len(series))
	news := make([]int, len(series))
	for i, pt := range series {
		totals[i] = pt.Count
		news[i] = pt.New
	}
	return totals, news
}

// writeChartColumn emits one `<section class="column chart">` containing
// a chart-bars block whose bar heights are normalised within `values`
// (matching upstream's `(value-min)/(max-min)` ramp). Each bar is
// labelled with its month; a zero value renders an empty `.value`.
func writeChartColumn(b *strings.Builder, title string, series []ChartPoint, values []int) {
	b.WriteString(`<section class="column chart">`)
	fmt.Fprintf(b, `<h3>%s</h3>`, title)

	minV, maxV := values[0], values[0]
	for _, v := range values[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	denom := maxV - minV
	if denom == 0 {
		denom = 1
	}

	// Thin the annotations so month names don't overlap on the narrow
	// half-width columns (same stride idea as the graph chart's labels):
	// every bar still draws, but only ~maxLabels of them carry a month
	// label + value. The month label is emitted as bare text — like
	// upstream's `<%= d %>` — NOT inside `<span class="label">`, which is
	// the unrelated blue pill-badge class (.label{background;padding;…})
	// that previously turned each x-axis tick into a wide rounded chip
	// and overflowed the column.
	const maxLabels = 7
	stride := 1
	if len(series) > maxLabels {
		stride = (len(series) + maxLabels - 1) / maxLabels
	}

	b.WriteString(`<div class="chart-bars">`)
	for i, pt := range series {
		v := values[i]
		share := 0.05 + 0.95*float64(v-minV)/float64(denom)
		labeled := i == 0 || i == len(series)-1 || i%stride == 0
		valueStr := ""
		label := ""
		if labeled {
			if v != 0 {
				valueStr = partials.FormatCount(int64(v))
			}
			label = pt.Date.UTC().Format("2")
		}
		fmt.Fprintf(
			b,
			`<div class="entry"><span class="value">%s</span><div class="bar" style="height: %.0fpx; background-color: var(--color-calendar-graph-day-L%d-bg)"></div>%s</div>`,
			valueStr, share*50, bgLevel(share), label,
		)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
}

func writeGraphChart(b *strings.Builder, series []ChartPoint, values []int, label string) {
	const (
		width  = 480.0
		height = 180.0
		left   = 32.0
		top    = 12.0
		right  = 14.0
		bottom = 54.0
	)
	plotW := width - left - right
	plotH := height - top - bottom
	minV, maxV := values[0], values[0]
	for _, v := range values[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	denom := maxV - minV
	if denom == 0 {
		denom = 1
	}

	points := make([][2]float64, len(series))
	for i := range series {
		x := left + plotW/2
		if len(series) > 1 {
			x = left + plotW*float64(i)/float64(len(series)-1)
		}
		y := top + plotH - plotH*float64(values[i]-minV)/float64(denom)
		points[i] = [2]float64{x, y}
	}

	// Thin out per-point labels so they don't overlap. Upstream's D3
	// chart lets the layout engine drop ticks; here we keep at most
	// ~maxLabels evenly spaced labels (always including the first/last).
	const maxLabels = 12
	stride := 1
	if len(series) > maxLabels {
		stride = (len(series) + maxLabels - 1) / maxLabels
	}
	labelAt := func(i int) bool {
		return i == 0 || i == len(series)-1 || i%stride == 0
	}

	fmt.Fprintf(b, `<svg class="stargazers-graph" xmlns="http://www.w3.org/2000/svg" width="480" height="180" viewBox="0 0 480 180" role="img" aria-label="%s">`, partials.EscapeXML(label))
	// Y axis (dashed) + X axis (solid baseline), matching upstream's
	// faint grey rgba(127,127,127) axis colors.
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="rgba(127, 127, 127, .4)" stroke-dasharray="2,2"></line>`, left, top, left, top+plotH)
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="rgba(127, 127, 127, .8)"></line>`, left, top+plotH, left+plotW, top+plotH)
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="rgba(127, 127, 127, .8)" text-anchor="end" font-size="10">%d</text>`, left-4, top+4, maxV)
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="rgba(127, 127, 127, .8)" text-anchor="end" font-size="10">%d</text>`, left-4, top+plotH, minV)

	// Area fill first so the line + markers paint on top of it.
	b.WriteString(`<path d="`)
	writeAreaPath(b, points, top+plotH)
	b.WriteString(`" fill="rgba(88, 166, 255, .1)"></path>`)
	b.WriteString(`<path d="`)
	writeLinePath(b, points)
	b.WriteString(`" fill="transparent" stroke="#87ceeb" stroke-width="2"></path>`)

	for i := range series {
		p := points[i]
		fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="2" fill="#106cbc"></circle>`, p[0], p[1])
		if labelAt(i) {
			anchor := "middle"
			if i == 0 {
				anchor = "start"
			} else if i == len(series)-1 {
				anchor = "end"
			}
			fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="rgba(127, 127, 127, .8)" text-anchor="%s" font-size="10">%d</text>`, p[0], p[1]-6, anchor, values[i])
		}
	}
	// X-axis date labels rotated -45deg around their anchor to avoid
	// overlap (same approach as upstream's D3 `rotate(-45)` tick text).
	for i, pt := range series {
		if !labelAt(i) {
			continue
		}
		p := points[i]
		label := pt.Date.UTC().Format("Jan 2")
		ly := top + plotH + 12
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="rgba(127, 127, 127, .8)" text-anchor="end" font-size="10" transform="rotate(-45 %.1f %.1f)">%s</text>`, p[0], ly, p[0], ly, label)
	}
	b.WriteString(`</svg>`)
}

func writeLinePath(b *strings.Builder, points [][2]float64) {
	for i, p := range points {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(b, `%s%.1f,%.1f`, cmd, p[0], p[1])
	}
}

func writeAreaPath(b *strings.Builder, points [][2]float64, baseline float64) {
	if len(points) == 0 {
		return
	}
	writeLinePath(b, points)
	for i := len(points) - 1; i >= 0; i-- {
		fmt.Fprintf(b, `L%.1f,%.1f`, points[i][0], baseline)
	}
	b.WriteString(`Z`)
}
