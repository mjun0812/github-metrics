package stargazers_test

import (
	"context"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func runWith(t *testing.T, inputs map[string]any) *stargazers.Result {
	t.Helper()
	data := plugins.NewData()
	pc := &plugins.PluginContext{Data: data, Inputs: inputs}
	out, err := stargazers.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*stargazers.Result)
}

func TestRun_AlwaysSkippedInM4(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !r.Skipped {
		t.Errorf("M4 stargazers should always be Skipped; got %+v", r)
	}
}

// TestRun_WorldmapPopulatedFromStargazerLocations exercises the
// end-to-end offline pipeline: the GraphQL mock supplies stargazers
// with declared locations, the geocoder resolves them, and the
// resulting Worldmap.Points slice carries one entry per unique
// coordinate with counts aggregated. Includes an unresolvable location
// to guarantee misses are dropped silently rather than crashing.
func TestRun_WorldmapPopulatedFromStargazerLocations(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, `{
		"data": {"viewer": {"repositories": {"totalCount": 1, "nodes": [
			{"nameWithOwner": "octo/hello", "stargazerCount": 5,
			 "stargazers": {"totalCount": 5, "edges": [
				{"starredAt": "2026-05-02T00:00:00Z", "node": {"login": "alice", "location": "Tokyo, Japan"}},
				{"starredAt": "2026-05-03T00:00:00Z", "node": {"login": "bob",   "location": "Tokyo"}},
				{"starredAt": "2026-05-04T00:00:00Z", "node": {"login": "carol", "location": "London"}},
				{"starredAt": "2026-05-05T00:00:00Z", "node": {"login": "dave",  "location": "qqqqzzzz-not-a-place"}},
				{"starredAt": "2026-05-06T00:00:00Z", "node": {"login": "eve",   "location": ""}}
			 ]}}
		]}}}
	}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"plugin_stargazers":          true,
			"plugin_stargazers_worldmap": true,
		}),
	)
	out, err := stargazers.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*stargazers.Result)
	if r.Worldmap == nil {
		t.Fatalf("Worldmap should be populated when plugin_stargazers_worldmap=true")
	}
	if len(r.Worldmap.Points) != 2 {
		t.Fatalf("expected 2 deduped points (Tokyo, London); got %d: %+v", len(r.Worldmap.Points), r.Worldmap.Points)
	}
	// Sorted by descending Count, so Tokyo (2) comes before London (1).
	tokyo := r.Worldmap.Points[0]
	if tokyo.Count != 2 {
		t.Errorf("Tokyo count = %d, want 2 (alice + bob)", tokyo.Count)
	}
	if tokyo.Lat < 30 || tokyo.Lat > 40 || tokyo.Lng < 130 || tokyo.Lng > 145 {
		t.Errorf("Tokyo coords out of range: %+v", tokyo)
	}
	london := r.Worldmap.Points[1]
	if london.Count != 1 {
		t.Errorf("London count = %d, want 1", london.Count)
	}
}

func TestRun_WorldmapNilWhenNotRequested(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, `{"data":{"viewer":{"repositories":{"totalCount":1,"nodes":[{"nameWithOwner":"octocat/hello-world","stargazerCount":2,"stargazers":{"totalCount":2,"edges":[{"starredAt":"2026-05-02T00:00:00Z"},{"starredAt":"2026-04-01T00:00:00Z"}]}}]}}}}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"plugin_stargazers":             true,
			"plugin_stargazers_charts_type": "graph",
		}),
	)
	out, err := stargazers.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*stargazers.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if r.Worldmap != nil {
		t.Fatalf("Worldmap = %+v, want nil when plugin_stargazers_worldmap is not requested", r.Worldmap)
	}
}

func TestRun_ChartsUseLast14DailyBuckets(t *testing.T) {
	restore := stargazers.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	})
	defer restore()

	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, `{"data":{"viewer":{"repositories":{"totalCount":1,"nodes":[{"nameWithOwner":"octocat/hello-world","stargazerCount":10,"stargazers":{"totalCount":10,"edges":[{"starredAt":"2026-05-01T23:59:59Z"},{"starredAt":"2026-05-02T00:00:00Z"},{"starredAt":"2026-05-14T09:00:00Z"},{"starredAt":"2026-05-15T00:00:00Z"}]}}]}}}}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{"plugin_stargazers": true}),
	)
	out, err := stargazers.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*stargazers.Result)
	if len(r.Charts.Series) != 14 {
		t.Fatalf("Series len = %d, want 14", len(r.Charts.Series))
	}
	first := r.Charts.Series[0]
	if got := first.Date.Format("2006-01-02"); got != "2026-05-01" {
		t.Fatalf("first date = %s, want 2026-05-01", got)
	}
	if first.New != 1 || first.Count != 8 {
		t.Fatalf("first bucket = %+v, want New=1 Count=8", first)
	}
	last := r.Charts.Series[13]
	if got := last.Date.Format("2006-01-02"); got != "2026-05-14" {
		t.Fatalf("last date = %s, want 2026-05-14", got)
	}
	if last.New != 1 || last.Count != 10 {
		t.Fatalf("last bucket = %+v, want New=1 Count=10", last)
	}
}

func TestRun_ChartsType(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		input map[string]any
		want  string
	}{
		{nil, "classic"},
		{map[string]any{"plugin_stargazers_charts_type": "graph"}, "graph"},
	} {
		if r := runWith(t, c.input); r.Charts.Type != c.want {
			t.Errorf("inputs %v: Charts.Type = %q, want %q", c.input, r.Charts.Type, c.want)
		}
	}
}

// TestPartial_WorldmapRendersBaseMapAndMarkers exercises the worldmap
// section end-to-end at the partial level: base country paths appear,
// markers appear, no CSS var() references leak (resvg cannot resolve
// them), and the section reports a positive height so StackSections
// places it deterministically.
func TestPartial_WorldmapRendersBaseMapAndMarkers(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin("stargazers", &stargazers.Result{
		Mode:   plugins.ModeUser,
		List:   []stargazers.Stargazer{},
		Charts: stargazers.StargazersCharts{Type: "classic", Series: []stargazers.ChartPoint{}},
		Worldmap: &stargazers.StargazersWorldmap{Points: []stargazers.WorldmapPoint{
			{Location: "Tokyo", Lat: 35.68, Lng: 139.75, Count: 3},
			{Location: "London", Lat: 51.5, Lng: -0.13, Count: 1},
		}},
	})
	got, h, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if h <= 0 {
		t.Fatalf("worldmap partial must report a positive height; got %d", h)
	}
	if !strings.Contains(got, "worldmap-countries") {
		t.Errorf("worldmap partial missing country base map:\n%s", got)
	}
	if !strings.Contains(got, "worldmap-markers") {
		t.Errorf("worldmap partial missing markers group:\n%s", got)
	}
	if !strings.Contains(got, "Stargazers origins") {
		t.Errorf("worldmap partial missing sub-header:\n%s", got)
	}
	if strings.Contains(got, "var(") {
		t.Errorf("worldmap partial must not emit CSS var() references")
	}
}

// TestPartial_ClassicTwoColumns asserts the classic chart renders the
// two upstream columns (cumulative Total + per-bucket New) as native SVG
// with day-of-month ticks plus month-boundary captions (#541), rather
// than the day-only stride-thinned labels of #508.
func TestPartial_ClassicTwoColumns(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "classic")
	for _, marker := range []string{
		`>Total stargazers</text>`,
		`>New stargazers per day</text>`,
		// Day-of-month ticks sit as bare `<text>` under each bar; the
		// first bar and any day-1 bar additionally carries a month
		// caption line (#541), matching
		// `org_repo/source/templates/classic/partials/stargazers.ejs`.
		`>Apr.</text>`,
		`>May</text>`,
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("classic partial missing %q:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `class="stargazers-graph"`) {
		t.Fatalf("classic partial should not render graph svg:\n%s", got)
	}
	// #409 Phase B5 completion condition: no CSS var() color references
	// survive the native-SVG conversion (resvg cannot resolve them).
	if strings.Contains(got, "var(") {
		t.Fatalf("classic partial must not emit CSS var() references:\n%s", got)
	}
	// Two chart-bars columns (one per section).
	if n := strings.Count(got, `data-block="chart-bars"`); n != 2 {
		t.Fatalf("want 2 chart-bars columns, got %d:\n%s", n, got)
	}
	// New-stargazers column: Apr is the first bucket (cumulative 1 →
	// +1), May adds 2 (cumulative 3 → +2). The signed increment "+2"
	// must appear (#541 switched the Increments column to upstream's
	// `f(value, {sign:true})` shape).
	if !strings.Contains(got, `>+2</text>`) {
		t.Errorf("expected a +2 increment in the New column:\n%s", got)
	}
	// Total column: the second bar (May, cumulative 3) carries the
	// raw count, exercising the "label only when the value changed"
	// rule from writeClassicSection.
	if !strings.Contains(got, `>3</text>`) {
		t.Errorf("expected the Total column to label the changed cumulative count 3:\n%s", got)
	}
}

func TestPartial_GraphChart(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "graph")
	for _, marker := range []string{
		`class="stargazers-graph"`,
		`aria-label="Total stargazers graph"`,
		`aria-label="New stargazers per day graph"`,
		`stroke="#87ceeb"`,
		`Apr 1`,
		`May 1`,
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("graph partial missing %q:\n%s", marker, got)
		}
	}
	// Pin the upstream-equivalent dashed grid (#542): each chart emits
	// one vertical Y-axis line plus one row per Y tick (at least 2).
	if n := strings.Count(got, `stroke-dasharray="2,2"`); n < 6 {
		t.Errorf("graph partial should carry the horizontal dashed grid (>= 6 dashed lines for 2 charts), got %d:\n%s", n, got)
	}
}

// TestPartial_ChartistOutputIdenticalToGraph pins the deprecated-alias
// contract at the partial layer (#543). parseChartsType in stargazers.go
// already normalises `chartist` to `graph`; this guard goes the full
// Run → Partial round-trip so any
// future Partial branch on Result.Charts.Type would be caught before
// it reaches users. The Run pass is what feeds the normalised
// Charts.Type into Partial; constructing Result manually via
// renderPartial would skip parseChartsType and is not the realistic
// production path.
func TestPartial_ChartistOutputIdenticalToGraph(t *testing.T) {
	t.Parallel()
	const mockResponse = `{"data":{"viewer":{"repositories":{"totalCount":1,"nodes":[{"nameWithOwner":"octocat/hello-world","stargazerCount":2,"stargazers":{"totalCount":2,"edges":[{"starredAt":"2026-05-02T00:00:00Z"},{"starredAt":"2026-04-01T00:00:00Z"}]}}]}}}}`

	runAndRender := func(t *testing.T, chartsType string) string {
		t.Helper()
		mux := mocks.NewGraphQLMux(t)
		mux.OnBody("ViewerStargazersRepos", http.StatusOK, mockResponse)
		pc := mocks.NewPluginContext(
			t,
			mocks.WithGraphQL(mux),
			mocks.WithInputs(map[string]any{
				"plugin_stargazers":             true,
				"plugin_stargazers_charts_type": chartsType,
			}),
		)
		out, err := stargazers.Plugin.Run(context.Background(), pc)
		if err != nil {
			t.Fatalf("Run(%s): %v", chartsType, err)
		}
		r := out.(*stargazers.Result)
		data := plugins.NewData()
		data.SetPlugin("stargazers", r)
		got, _, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
		if err != nil {
			t.Fatalf("Partial(%s): %v", chartsType, err)
		}
		return got
	}

	gotGraph := runAndRender(t, "graph")
	gotChartist := runAndRender(t, "chartist")
	if gotGraph != gotChartist {
		t.Fatalf("chartist partial output differs from graph output\ngraph:\n%s\n\nchartist:\n%s", gotGraph, gotChartist)
	}
}

func renderPartial(t *testing.T, chartsType string) string {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin("stargazers", &stargazers.Result{
		Mode: plugins.ModeUser,
		List: []stargazers.Stargazer{},
		Charts: stargazers.StargazersCharts{
			Type: chartsType,
			Series: []stargazers.ChartPoint{
				{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Count: 1, New: 1},
				{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Count: 3, New: 2},
			},
		},
	})
	got, _, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return got
}

// graphSeries mirrors the shape of the plugin-stargazers-graph sample:
// 14 daily points, small per-day increments (max 3), a flat tail.
func graphSeries() []stargazers.ChartPoint {
	news := []int{1, 0, 2, 1, 3, 2, 1, 0, 1, 2, 1, 1, 0, 0}
	start := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	total := 1832
	series := make([]stargazers.ChartPoint, len(news))
	for i, n := range news {
		total += n
		series[i] = stargazers.ChartPoint{Date: start.AddDate(0, 0, i), Count: total, New: n}
	}
	return series
}

type graphText struct {
	x, y    float64
	val     string
	anchor  string
	rotated bool
}

var (
	graphSVGRe  = regexp.MustCompile(`(?s)<svg class="stargazers-graph".*?</svg>`)
	graphTextRe = regexp.MustCompile(`<text x="([\d.]+)" y="([\d.]+)"[^>]*?text-anchor="(\w+)"[^>]*?(transform="[^"]*")?>([^<]*)</text>`)
	graphGridRe = regexp.MustCompile(`<line x1="32.0" y1="([\d.]+)" x2="466.0" y2="[\d.]+" stroke="rgba\(127, 127, 127, \.4\)"`)
	graphDotRe  = regexp.MustCompile(`<circle cx="([\d.]+)" cy="([\d.]+)"`)
)

func renderGraphCharts(t *testing.T) []string {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin("stargazers", &stargazers.Result{
		Mode:   plugins.ModeUser,
		List:   []stargazers.Stargazer{},
		Charts: stargazers.StargazersCharts{Type: "graph", Series: graphSeries()},
	})
	got, _, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	charts := graphSVGRe.FindAllString(got, -1)
	if len(charts) != 2 {
		t.Fatalf("want 2 graph svgs, got %d", len(charts))
	}
	return charts
}

// extent approximates the horizontal span of a 10px label (~5.6px/char).
func (g graphText) extent() (float64, float64) {
	w := 5.6 * float64(len(g.val))
	switch g.anchor {
	case "end":
		return g.x - w, g.x
	case "start":
		return g.x, g.x + w
	}
	return g.x - w/2, g.x + w/2
}

func parseGraphTexts(svg string) []graphText {
	matches := graphTextRe.FindAllStringSubmatch(svg, -1)
	out := make([]graphText, 0, len(matches))
	for _, m := range matches {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		out = append(out, graphText{x: x, y: y, val: m[5], anchor: m[3], rotated: m[4] != ""})
	}
	return out
}

// TestPartial_GraphYTicksAreNiceIntegers pins d3's `ticks()` behaviour
// used by upstream's Graph.timeline: 1/2/5 x 10^k steps inside
// [low, high], each tick label sitting on its own grid line.
func TestPartial_GraphYTicksAreNiceIntegers(t *testing.T) {
	t.Parallel()
	charts := renderGraphCharts(t)
	want := [][]string{{"1845", "1840", "1835"}, {"3", "2", "1", "0"}}
	for ci, svg := range charts {
		var ticks []graphText
		for _, tx := range parseGraphTexts(svg) {
			if tx.x == 28.0 {
				ticks = append(ticks, tx)
			}
		}
		vals := make([]string, 0, len(ticks))
		for _, tk := range ticks {
			vals = append(vals, tk.val)
		}
		if !slices.Equal(vals, want[ci]) {
			t.Errorf("chart %d y ticks = %v, want %v", ci, vals, want[ci])
		}
		grid := graphGridRe.FindAllStringSubmatch(svg, -1)
		if len(grid) != len(ticks) {
			t.Fatalf("chart %d: %d grid lines for %d ticks", ci, len(grid), len(ticks))
		}
		for i, g := range grid {
			gy, _ := strconv.ParseFloat(g[1], 64)
			if math.Abs(ticks[i].y-4-gy) > 0.11 {
				t.Errorf("chart %d tick %q at y=%.1f is off its grid line y=%.1f", ci, ticks[i].val, ticks[i].y-4, gy)
			}
		}
	}
	// New chart: the point with value 2 (index 2) lies on the "2" grid line.
	grid := graphGridRe.FindAllStringSubmatch(charts[1], -1)
	dots := graphDotRe.FindAllStringSubmatch(charts[1], -1)
	if grid[1][1] != dots[2][2] {
		t.Errorf("value-2 point cy=%s differs from tick 2 line y=%s", dots[2][2], grid[1][1])
	}
}

// TestPartial_GraphLabelsDoNotCollide checks the data labels and rotated
// X labels keep a minimum horizontal gap and stay inside the viewport.
func TestPartial_GraphLabelsDoNotCollide(t *testing.T) {
	t.Parallel()
	for ci, svg := range renderGraphCharts(t) {
		var data, xs []graphText
		for _, tx := range parseGraphTexts(svg) {
			switch {
			case tx.rotated:
				xs = append(xs, tx)
			case tx.x != 28.0:
				data = append(data, tx)
			}
		}
		for name, set := range map[string][]graphText{"data": data, "x": xs} {
			for i := 1; i < len(set); i++ {
				_, prevR := set[i-1].extent()
				curL, _ := set[i].extent()
				if curL-prevR < 2 {
					t.Errorf("chart %d %s labels %q/%q overlap or touch", ci, name, set[i-1].val, set[i].val)
				}
			}
		}
		for _, d := range data {
			if d.y < 10 {
				t.Errorf("chart %d data label %q baseline y=%.1f is clipped by the viewport top", ci, d.val, d.y)
			}
		}
	}
}
