package stargazers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

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

// TestRun_WorldmapNilWhenNotRequested verifies that omitting or
// disabling `plugin_stargazers_worldmap` keeps Worldmap = nil so
// stargazers.Result serializes without the field (the JSON omitempty
// contract downstream partials rely on).
func TestRun_WorldmapNilWhenNotRequested(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if r.Worldmap != nil {
		t.Errorf("worldmap should be nil when input absent; got %+v", r.Worldmap)
	}
	r = runWith(t, map[string]any{"plugin_stargazers_worldmap": false})
	if r.Worldmap != nil {
		t.Errorf("worldmap should be nil when input false; got %+v", r.Worldmap)
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

// TestRun_WorldmapAcceptsLegacyInputsSilently confirms that
// `plugin_stargazers_worldmap_token` and `plugin_stargazers_worldmap_sample`
// (both no-ops now that geocoding is offline) do not affect Run's
// outcome or produce errors.
func TestRun_WorldmapAcceptsLegacyInputsSilently(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, `{"data":{"viewer":{"repositories":{"totalCount":0,"nodes":[]}}}}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"plugin_stargazers":                 true,
			"plugin_stargazers_worldmap":        true,
			"plugin_stargazers_worldmap_token":  "irrelevant-legacy-key",
			"plugin_stargazers_worldmap_sample": 100,
		}),
	)
	if _, err := stargazers.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRun_ChartsTypeGraphInput(t *testing.T) {
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
	if r.Charts.Type != "graph" {
		t.Fatalf("Charts.Type = %q, want graph", r.Charts.Type)
	}
	if len(r.Charts.Series) != 14 {
		t.Fatalf("Series len = %d, want 14", len(r.Charts.Series))
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

func TestRun_ChartsTypeDefaultsToClassic(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if r.Charts.Type != "classic" {
		t.Fatalf("Charts.Type = %q, want classic", r.Charts.Type)
	}
}

// TestRun_ChartsTypeChartistAliasedToGraph verifies that `chartist`
// (deprecated upstream alias) produces the same Charts.Type as `graph`.
func TestRun_ChartsTypeChartistAliasedToGraph(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_stargazers_charts_type": "chartist"})
	if r.Charts.Type != "graph" {
		t.Fatalf("Charts.Type = %q, want graph (chartist is an alias of graph)", r.Charts.Type)
	}
}

// TestRun_ChartsTypeChartistOutputIdenticalToGraph verifies that
// `charts_type=chartist` produces output byte-identical to `charts_type=graph`
// when both share the same input data — satisfying the Acceptance Criteria of
// GitHub issue #395.
func TestRun_ChartsTypeChartistOutputIdenticalToGraph(t *testing.T) {
	t.Parallel()
	mux := mocks.NewGraphQLMux(t)
	const mockResponse = `{"data":{"viewer":{"repositories":{"totalCount":1,"nodes":[{"nameWithOwner":"octocat/hello-world","stargazerCount":2,"stargazers":{"totalCount":2,"edges":[{"starredAt":"2026-05-02T00:00:00Z"},{"starredAt":"2026-04-01T00:00:00Z"}]}}]}}}}`
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, mockResponse)
	mux.OnBody("ViewerStargazersRepos", http.StatusOK, mockResponse)

	pcGraph := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"plugin_stargazers":             true,
			"plugin_stargazers_charts_type": "graph",
		}),
	)
	outGraph, err := stargazers.Plugin.Run(context.Background(), pcGraph)
	if err != nil {
		t.Fatalf("Run(graph): %v", err)
	}
	rGraph := outGraph.(*stargazers.Result)

	pcChartist := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"plugin_stargazers":             true,
			"plugin_stargazers_charts_type": "chartist",
		}),
	)
	outChartist, err := stargazers.Plugin.Run(context.Background(), pcChartist)
	if err != nil {
		t.Fatalf("Run(chartist): %v", err)
	}
	rChartist := outChartist.(*stargazers.Result)

	jsonGraph, err := json.Marshal(rGraph)
	if err != nil {
		t.Fatalf("json.Marshal(graph): %v", err)
	}
	jsonChartist, err := json.Marshal(rChartist)
	if err != nil {
		t.Fatalf("json.Marshal(chartist): %v", err)
	}
	if !bytes.Equal(jsonGraph, jsonChartist) {
		t.Fatalf("chartist output differs from graph output\ngraph:    %s\nchartist: %s", jsonGraph, jsonChartist)
	}
}

// Spec 013: user-mode is now wired, so the Skipped path only fires when
// the GraphQL client is missing (test paths). The Skipped reason was
// updated from the M4 "repository account kind" message to the precise
// "GraphQL client unavailable" gating.
func TestRun_SkippedReasonMentionsRepositoryAccount(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !strings.Contains(r.SkippedReason, "GraphQL") {
		t.Errorf("SkippedReason should mention GraphQL; got %q", r.SkippedReason)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &stargazers.Result{
		Skipped: true,
		List:    []stargazers.Stargazer{},
		Charts:  stargazers.StargazersCharts{Type: "classic", Series: []stargazers.ChartPoint{}},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "stargazers.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}

func TestRun_GraphGoldenShape(t *testing.T) {
	r := &stargazers.Result{
		Mode: "user",
		List: []stargazers.Stargazer{},
		Charts: stargazers.StargazersCharts{
			Type: "graph",
			Series: []stargazers.ChartPoint{
				{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Count: 1},
				{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Count: 3},
			},
		},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "stargazers_graph.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
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

func TestPartial_ClassicChartBarsMaintained(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "classic")
	if !strings.Contains(got, `data-block="chart-bars"`) {
		t.Fatalf("classic partial missing native chart-bars block:\n%s", got)
	}
	if strings.Contains(got, `class="stargazers-graph"`) {
		t.Fatalf("classic partial should not render graph svg:\n%s", got)
	}
	// #409 Phase B5 completion condition: no CSS var() color references
	// survive the native-SVG conversion (resvg cannot resolve them).
	if strings.Contains(got, "var(") {
		t.Fatalf("classic partial must not emit CSS var() references:\n%s", got)
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
	// The chart must not fall back to any HTML chart-bars markup.
	if strings.Contains(got, `<span class="label">`) || strings.Contains(got, `<div class="chart-bars">`) {
		t.Errorf("chart must not emit HTML chart-bars markup:\n%s", got)
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
	if strings.Contains(got, `class="chart-bars"`) {
		t.Fatalf("graph partial should not render classic chart-bars:\n%s", got)
	}
	// Pin the upstream-equivalent dashed grid (#542): each of the two
	// graph charts emits one vertical Y-axis line + 5 horizontal grid
	// rows (numGrid in writeGraphChart), so the partial must contain
	// at least (1+5)*2 = 12 `stroke-dasharray="2,2"` occurrences.
	if n := strings.Count(got, `stroke-dasharray="2,2"`); n < 12 {
		t.Errorf("graph partial should carry the horizontal dashed grid (>= 12 dashed lines for 2 charts), got %d:\n%s", n, got)
	}
}

// TestPartial_ChartistOutputIdenticalToGraph pins the deprecated-alias
// contract at the partial layer (#543). parseChartsType in stargazers.go
// already normalises `chartist` to `graph`, and
// TestRun_ChartsTypeChartistOutputIdenticalToGraph guards the Result
// shape — this guard goes the full Run → Partial round-trip so any
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
