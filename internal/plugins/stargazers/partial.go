package stargazers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// starOcticon is the upstream `<%- octicon "star" %>` 16x16 path used
// in the stargazers section header (EJS line 4).
const starOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// upstreamMonths mirrors `plugins.stargazers.months` in
// `source/plugins/stargazers/index.mjs:64`. Indexed 1..12 (slot 0 unused)
// so the month captions rendered under the chart bars match upstream
// byte-for-byte (`Apr.` not `Apr`, `May` not `May.`, full `June`/`July`).
var upstreamMonths = [13]string{"", "Jan.", "Feb.", "Mar.", "Apr.", "May", "June", "July", "Aug.", "Sep.", "Oct.", "Nov.", "Dec."}

// Native-SVG layout metrics for the classic two-column / graph section.
const (
	chartColumns      = 2
	chartColumnPad    = 8.0  // `.chart { padding: 0 8px }`
	chartMarginBottom = 16.0 // `.margin-bottom { margin-bottom: 16px }`
	graphHeight       = 180.0
)

// bgLevel maps a 0..1 share to the contribution-graph L{1..4} ramp via
// Math.ceil(p/0.25) (matching upstream's per-bar color).
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

// Partial renders the classic SVG fragment for the stargazers plugin as
// native SVG (#409 Phase B5). Mirrors upstream
// org_repo/source/templates/classic/partials/stargazers.ejs.
//
// Output: a `<g data-section="stargazers">` anchor wrapping a
// nested `<svg>`. The star-octicon section header sits above either:
//
//   - classic: two side-by-side `.chart-bars` columns rendered as native
//     `<rect>` bars ("Total stargazers" cumulative + "New stargazers per
//     day" increments), each bar carrying the day-of-month tick and, on
//     month boundaries, a month caption; or
//   - graph: two stacked native line/area charts (`stargazers-graph`).
//
// Returns the markup and the pixel height it consumes.
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
	series := r.Charts.Series
	if len(series) == 0 && len(r.List) == 0 {
		return "", 0, nil
	}

	header, hh := chrome.SVGSectionHeader(starOcticon, "Stargazers")

	var body strings.Builder
	body.WriteString(header)

	height := int(hh)
	if len(series) > 0 {
		if r.Charts.Type == chartsTypeGraph {
			height = int(writeGraphSection(&body, series, hh))
		} else {
			height = int(writeClassicSection(&body, series, hh))
		}
	}
	return chrome.WrapSection("stargazers", height, body.String()), height, nil
}

// writeClassicSection lays the two chart-bars columns out side by side
// starting at y=top and returns the total section height (including the
// bottom margin). Column values are normalised independently, matching
// upstream's `(value-min)/(max-min)` ramp.
func writeClassicSection(b *strings.Builder, series []ChartPoint, top float64) float64 {
	totals, news := chartValues(series)

	prevTotal := 0
	totalLabel := func(_, cur int) string {
		if cur == prevTotal {
			return ""
		}
		prevTotal = cur
		return partials.FormatCount(int64(cur))
	}
	incLabel := func(_, cur int) string {
		if cur == 0 {
			return ""
		}
		return "+" + partials.FormatCount(int64(cur))
	}

	colW := float64(chrome.CardWidth) / chartColumns
	innerW := colW - 2*chartColumnPad

	subMk0, subH := chrome.SVGSubHeader(colW*0.5, top, innerW, "Total stargazers")
	subMk1, _ := chrome.SVGSubHeader(colW*1.5, top, innerW, "New stargazers per day")
	b.WriteString(subMk0)
	b.WriteString(subMk1)

	barsTop := top + subH
	bars0, h0 := chrome.SVGVBars(chartColumnPad, barsTop, innerW, buildColumnBars(series, totals, totalLabel))
	bars1, h1 := chrome.SVGVBars(colW+chartColumnPad, barsTop, innerW, buildColumnBars(series, news, incLabel))
	b.WriteString(bars0)
	b.WriteString(bars1)

	barsH := h0
	if h1 > barsH {
		barsH = h1
	}
	return top + subH + barsH + chartMarginBottom
}

// buildColumnBars converts a value series into vertical bars, normalising
// heights within [min,max] (upstream's per-column ramp) and captioning
// the first bar plus every month boundary (day == 1) with the month name.
// valueFn picks the per-bar label text so the Total and Increments
// columns can use different display rules without forking the loop.
func buildColumnBars(series []ChartPoint, values []int, valueFn func(i, cur int) string) []chrome.VBar {
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

	bars := make([]chrome.VBar, len(series))
	for i, pt := range series {
		v := values[i]
		share := 0.05 + 0.95*float64(v-minV)/float64(denom)
		day := pt.Date.UTC().Day()
		caption := ""
		if i == 0 || day == 1 {
			caption = upstreamMonths[int(pt.Date.UTC().Month())]
		}
		bars[i] = chrome.VBar{
			Value:   valueFn(i, v),
			Label:   strconv.Itoa(day),
			Caption: caption,
			Share:   share,
			Level:   bgLevel(share),
		}
	}
	return bars
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

// writeGraphSection stacks the two native line/area charts (each with its
// centered `<h3>` sub-title) starting at y=top and returns the total
// section height (including the bottom margin).
func writeGraphSection(b *strings.Builder, series []ChartPoint, top float64) float64 {
	totals, news := chartValues(series)
	center := float64(chrome.CardWidth) / 2
	maxW := float64(chrome.CardWidth) - 2*chartColumnPad

	y := top
	m, subH := chrome.SVGSubHeader(center, y, maxW, "Total stargazers")
	b.WriteString(m)
	y += subH
	writeGraphChart(b, series, totals, "Total stargazers graph", y)
	y += graphHeight

	m2, _ := chrome.SVGSubHeader(center, y, maxW, "New stargazers per day")
	b.WriteString(m2)
	y += subH
	writeGraphChart(b, series, news, "New stargazers per day graph", y)
	y += graphHeight

	return y + chartMarginBottom
}

// writeGraphChart renders one native line/area chart as a positioned
// nested `<svg>` at y=yOffset. The 480x180 viewBox is unchanged; only the
// vertical placement inside the section is set by yOffset.
func writeGraphChart(b *strings.Builder, series []ChartPoint, values []int, label string, yOffset float64) {
	const (
		width  = 480.0
		height = graphHeight
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

	fmt.Fprintf(b, `<svg class="stargazers-graph" xmlns="http://www.w3.org/2000/svg" width="480" height="180" viewBox="0 0 480 180" x="0" y="%.0f" role="img" aria-label="%s">`, yOffset, partials.EscapeXML(label))
	// Vertical Y axis (left dashed) + bottom solid baseline, matching
	// upstream's faint grey rgba(127,127,127) axis colors.
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="rgba(127, 127, 127, .4)" stroke-dasharray="2,2"></line>`, left, top, left, top+plotH)
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="rgba(127, 127, 127, .8)"></line>`, left, top+plotH, left+plotW, top+plotH)

	// Horizontal dashed grid (#542): upstream's chartist line chart
	// draws a stack of intermediate Y-grid rows so the reader can read
	// off plot values without guessing. The previous implementation
	// emitted only the topmost (max) and bottom (min) Y labels with no
	// gridlines in between, so the chart looked unreferenced. Five
	// evenly-spaced rows (incl. max/min) keeps the loop trivial and
	// matches the upstream visual density.
	const numGrid = 5
	for k := 0; k < numGrid; k++ {
		frac := float64(k) / float64(numGrid-1)
		y := top + plotH*frac
		val := maxV - int(float64(maxV-minV)*frac+0.5)
		fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="rgba(127, 127, 127, .4)" stroke-dasharray="2,2"></line>`, left, y, left+plotW, y)
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="rgba(127, 127, 127, .8)" text-anchor="end" font-size="10">%d</text>`, left-4, y+4, val)
	}

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
