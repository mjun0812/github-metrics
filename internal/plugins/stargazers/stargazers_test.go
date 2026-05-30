package stargazers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log/slog"
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

func TestRun_WorldmapNilEvenWhenRequested(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_stargazers_worldmap": true})
	if r.Worldmap != nil {
		t.Errorf("worldmap should remain nil in M4; got %+v", r.Worldmap)
	}
}

func TestRun_WorldmapWarnLogEmitted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"plugin_stargazers_worldmap": true},
		Logger: logger,
	}
	_, _ = stargazers.Plugin.Run(context.Background(), pc)
	if !strings.Contains(buf.String(), "worldmap is not yet implemented") {
		t.Errorf("expected worldmap WARN log; got %q", buf.String())
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
	if len(r.Charts.Series) != 2 {
		t.Fatalf("Series len = %d, want 2", len(r.Charts.Series))
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

func TestPartial_ClassicChartBarsMaintained(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "classic")
	if !strings.Contains(got, `class="chart-bars"`) {
		t.Fatalf("classic partial missing chart-bars:\n%s", got)
	}
	if strings.Contains(got, `class="stargazers-graph"`) {
		t.Fatalf("classic partial should not render graph svg:\n%s", got)
	}
}

// TestPartial_ClassicTwoColumns asserts the classic chart renders the
// two upstream columns (cumulative Total + per-bucket New) with month
// x-axis labels rather than the old single column labelled with a
// meaningless day-of-month "1".
func TestPartial_ClassicTwoColumns(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "classic")
	for _, marker := range []string{
		`<h3>Total stargazers</h3>`,
		`<h3>New stargazers per month</h3>`,
		// Month labels are emitted as bare text after the bar (upstream
		// style), NOT as the blue pill `<span class="label">` badge.
		`</div>Apr</div>`,
		`</div>May</div>`,
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("classic partial missing %q:\n%s", marker, got)
		}
	}
	// The x-axis ticks must NOT reuse the pill-badge `.label` class.
	if strings.Contains(got, `<span class="label">`) {
		t.Errorf("chart x-axis ticks must not use the pill `.label` class:\n%s", got)
	}
	// Two chart-bars columns (one per section).
	if n := strings.Count(got, `class="chart-bars"`); n != 2 {
		t.Fatalf("want 2 chart-bars columns, got %d:\n%s", n, got)
	}
	// New-stargazers column: Apr is the first bucket (cumulative 1 →
	// +1), May adds 2 (cumulative 3 → +2). The increment "2" must appear.
	if !strings.Contains(got, `<span class="value">2</span>`) {
		t.Errorf("expected a +2 increment in the New column:\n%s", got)
	}
}

func TestPartial_GraphChart(t *testing.T) {
	t.Parallel()
	got := renderPartial(t, "graph")
	for _, marker := range []string{
		`class="stargazers-graph"`,
		`aria-label="Total stargazers graph"`,
		`stroke="#87ceeb"`,
		`Apr 2026`,
		`May 2026`,
	} {
		if !strings.Contains(got, marker) {
			t.Fatalf("graph partial missing %q:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `class="chart-bars"`) {
		t.Fatalf("graph partial should not render classic chart-bars:\n%s", got)
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
				{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Count: 1},
				{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Count: 3},
			},
		},
	})
	got, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return got
}
