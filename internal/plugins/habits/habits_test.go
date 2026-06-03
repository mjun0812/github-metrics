package habits_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/habits"
	"github.com/mjun0812/github-metrics/internal/templates"
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

func newREST(t *testing.T, body string) *githubapi.REST {
	t.Helper()
	mux := githubapi.NewMockTransport()
	mux.SetJSON("GET", "/users/octocat/events*", body)
	r, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return r
}

// newRESTWithRoutes builds a REST client whose mock transport serves the
// given route -> JSON body map (method+path keys, wildcards supported via
// trailing "*"). Used by the linguist tests that need commit/compare
// endpoints alongside the events feed.
func newRESTWithRoutes(t *testing.T, routes map[string]string) *githubapi.REST {
	t.Helper()
	mux := githubapi.NewMockTransport()
	for path, body := range routes {
		mux.SetJSON("GET", path, body)
	}
	r, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return r
}

func pcWith(t *testing.T, body string, inputs map[string]any) *plugins.PluginContext {
	t.Helper()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"user": "octocat"},
		REST:   newREST(t, body),
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

func ev(typ string, when time.Time) string {
	return `{"type":"` + typ + `","created_at":"` + when.UTC().Format(time.RFC3339) + `"}`
}

func TestRun_NoEvents_Skipped(t *testing.T) {
	t.Parallel()
	pc := pcWith(t, `[]`, nil)
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if !r.Skipped {
		t.Errorf("expected Skipped for empty events")
	}
}

func TestRun_PushEventsBuildHistograms(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 14, 30, 0, 0, time.UTC) // Friday 14h
	body := `[` + ev("PushEvent", now) + `,` + ev("PushEvent", now.Add(-3*time.Hour)) + `]`
	pc := pcWith(t, body, nil)
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped")
	}
	if r.Charts.Hours[14] != 1 {
		t.Errorf("Hours[14] = %d, want 1", r.Charts.Hours[14])
	}
	if r.Charts.Hours[11] != 1 {
		t.Errorf("Hours[11] = %d, want 1", r.Charts.Hours[11])
	}
}

func TestRun_NonPushFiltered(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 14, 30, 0, 0, time.UTC)
	body := `[` + ev("WatchEvent", now) + `,` + ev("PullRequestEvent", now) + `]`
	pc := pcWith(t, body, nil)
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if !r.Skipped {
		t.Errorf("Non-PushEvents only should yield Skipped")
	}
}

func TestRun_CommitsPerDay(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	body := `[` + ev("PushEvent", now) + `,` + ev("PushEvent", now.Add(-1*time.Hour)) + `]`
	pc := pcWith(t, body, map[string]any{"plugin_habits_days": 14})
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	wantCPD := 2.0 / 14.0
	if r.Facts.CommitsPerDay < wantCPD-0.001 || r.Facts.CommitsPerDay > wantCPD+0.001 {
		t.Errorf("CommitsPerDay = %f, want ~%f", r.Facts.CommitsPerDay, wantCPD)
	}
}

func TestRun_DefaultSectionToggles(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 14, 30, 0, 0, time.UTC)
	body := `[` + ev("PushEvent", now) + `]`
	pc := pcWith(t, body, nil)
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if !r.FactsEnabled {
		t.Errorf("FactsEnabled = false, want true")
	}
	if !r.ChartsEnabled {
		t.Errorf("ChartsEnabled = false, want true")
	}
}

func TestRun_SectionTogglesReadInputs(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 14, 30, 0, 0, time.UTC)
	body := `[` + ev("PushEvent", now) + `]`
	for _, tc := range []struct {
		name       string
		inputs     map[string]any
		wantFacts  bool
		wantCharts bool
	}{
		{
			name:       "facts yes charts no",
			inputs:     map[string]any{"plugin_habits_facts": "yes", "plugin_habits_charts": "no"},
			wantFacts:  true,
			wantCharts: false,
		},
		{
			name:       "facts no charts yes",
			inputs:     map[string]any{"plugin_habits_facts": "no", "plugin_habits_charts": "yes"},
			wantFacts:  false,
			wantCharts: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pc := pcWith(t, body, tc.inputs)
			out, _ := habits.Plugin.Run(context.Background(), pc)
			r := out.(*habits.Result)
			if r.FactsEnabled != tc.wantFacts {
				t.Errorf("FactsEnabled = %v, want %v", r.FactsEnabled, tc.wantFacts)
			}
			if r.ChartsEnabled != tc.wantCharts {
				t.Errorf("ChartsEnabled = %v, want %v", r.ChartsEnabled, tc.wantCharts)
			}
		})
	}
}

func TestRun_NilREST_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"user": "octocat"},
	}
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if !r.Skipped {
		t.Errorf("nil REST should yield Skipped")
	}
	if !r.FactsEnabled {
		t.Errorf("FactsEnabled = false, want true")
	}
	if !r.ChartsEnabled {
		t.Errorf("ChartsEnabled = false, want true")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &habits.Result{
		Days:          14,
		FactsEnabled:  true,
		ChartsEnabled: true,
		Facts:         habits.HabitFacts{IndentStyle: "spaces", CommitsPerDay: 1.5},
		Charts:        habits.HabitCharts{Hours: [24]int{}, Days: [7]int{2, 1, 3, 1, 0, 4, 2}},
		From:          200,
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "habits.json")
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

func fixedPartialResult() *habits.Result {
	return &habits.Result{
		Days:          14,
		FactsEnabled:  true,
		ChartsEnabled: true,
		Facts: habits.HabitFacts{
			IndentStyle:   "spaces",
			CharsPerLine:  82.5,
			CommitsPerDay: 1.5,
		},
		Charts: habits.HabitCharts{
			Hours: [24]int{
				0, 0, 0, 0, 0, 0,
				1, 2, 4, 6, 3, 1,
				0, 1, 3, 5, 2, 1,
				0, 0, 0, 0, 0, 0,
			},
			Days: [7]int{2, 1, 3, 1, 0, 4, 2},
		},
		From: 200,
	}
}

func renderPartial(t *testing.T, r *habits.Result) string {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin(habits.Name, r)
	got, err := habits.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return got
}

func assertPartialGolden(t *testing.T, name, got string) {
	t.Helper()
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", name)
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", werr)
		}
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
}

func TestPartial_Habits_FactsOnly_Golden(t *testing.T) {
	r := fixedPartialResult()
	r.FactsEnabled = true
	r.ChartsEnabled = false
	got := renderPartial(t, r)
	assertPartialGolden(t, "habits_facts_only.svg", got)
	for _, marker := range []string{
		`Recent coding habits`,
		`<ul class="facts">`,
		`Mostly active on Fri`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("facts-only partial missing marker %q in:\n%s", marker, got)
		}
	}
	for _, marker := range []string{
		`Commit activity per hour of day`,
		`Commit activity per day of week`,
		`class="chart-bars"`,
	} {
		if strings.Contains(got, marker) {
			t.Errorf("facts-only partial unexpectedly contains marker %q in:\n%s", marker, got)
		}
	}
}

// pcWithRoutes builds a PluginContext whose REST client serves the given
// route map, for linguist tests that need commit/compare endpoints.
func pcWithRoutes(t *testing.T, routes map[string]string, inputs map[string]any) *plugins.PluginContext {
	t.Helper()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"user": "octocat"},
		REST:   newRESTWithRoutes(t, routes),
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

func TestRun_LanguageActivity_FromCommits(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	// One in-window PushEvent carrying an explicit commit SHA. The commit
	// touches Go (90 bytes) and a small JS file (10 bytes), so Go ~= 0.9.
	events := `[{"type":"PushEvent","created_at":"` + now.Format(time.RFC3339) + `",` +
		`"repo":{"name":"octocat/repo"},` +
		`"payload":{"commits":[{"sha":"abc123"}]}}]`
	commit := `{"files":[` +
		`{"filename":"main.go","additions":60,"deletions":30},` +
		`{"filename":"app.js","additions":7,"deletions":3}]}`
	pc := pcWithRoutes(t, map[string]string{
		"/users/octocat/events*":             events,
		"/repos/octocat/repo/commits/abc123": commit,
	}, nil)
	out, err := habits.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*habits.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped")
	}
	if !r.Linguist.Available {
		t.Fatalf("Linguist.Available = false, want true")
	}
	if len(r.Linguist.Ordered) != 2 {
		t.Fatalf("Ordered len = %d, want 2: %+v", len(r.Linguist.Ordered), r.Linguist.Ordered)
	}
	if r.Linguist.Ordered[0].Name != "Go" {
		t.Errorf("dominant language = %q, want Go", r.Linguist.Ordered[0].Name)
	}
	if r.Linguist.Ordered[0].Share < 0.89 || r.Linguist.Ordered[0].Share > 0.91 {
		t.Errorf("Go share = %f, want ~0.9", r.Linguist.Ordered[0].Share)
	}
	// Section must render with the upstream literal heading.
	got := renderPartial(t, r)
	for _, marker := range []string{`Language activity`, `chart-bars horizontal`, `<span class="name">Go</span>`, `90%`} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

func TestRun_LanguageActivity_ViaCompare(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	// PushEvent without payload.commits → resolve via the compare API.
	events := `[{"type":"PushEvent","created_at":"` + now.Format(time.RFC3339) + `",` +
		`"repo":{"name":"octocat/repo"},` +
		`"payload":{"before":"aaa","head":"bbb"}}]`
	compare := `{"files":[{"filename":"lib.py","additions":40,"deletions":0}]}`
	pc := pcWithRoutes(t, map[string]string{
		"/users/octocat/events*":                events,
		"/repos/octocat/repo/compare/aaa...bbb": compare,
	}, nil)
	out, err := habits.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*habits.Result)
	if !r.Linguist.Available || len(r.Linguist.Ordered) != 1 {
		t.Fatalf("expected 1 ordered language, got available=%v %+v", r.Linguist.Available, r.Linguist.Ordered)
	}
	if r.Linguist.Ordered[0].Name != "Python" {
		t.Errorf("language = %q, want Python", r.Linguist.Ordered[0].Name)
	}
}

func TestRun_LanguageActivity_ThresholdAndLimit(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	events := `[{"type":"PushEvent","created_at":"` + now.Format(time.RFC3339) + `",` +
		`"repo":{"name":"octocat/repo"},` +
		`"payload":{"commits":[{"sha":"abc123"}]}}]`
	// Go 95 bytes, JS 5 bytes (5%). A 10% threshold drops JS.
	commit := `{"files":[` +
		`{"filename":"main.go","additions":95,"deletions":0},` +
		`{"filename":"app.js","additions":5,"deletions":0}]}`
	pc := pcWithRoutes(t, map[string]string{
		"/users/octocat/events*":             events,
		"/repos/octocat/repo/commits/abc123": commit,
	}, map[string]any{"plugin_habits_languages_threshold": "10%"})
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if len(r.Linguist.Ordered) != 1 || r.Linguist.Ordered[0].Name != "Go" {
		t.Fatalf("threshold filter failed: %+v", r.Linguist.Ordered)
	}
}

func TestRun_LanguageActivity_NoFiles_NotAvailable(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	events := `[{"type":"PushEvent","created_at":"` + now.Format(time.RFC3339) + `",` +
		`"repo":{"name":"octocat/repo"},` +
		`"payload":{"commits":[{"sha":"abc123"}]}}]`
	commit := `{"files":[]}`
	pc := pcWithRoutes(t, map[string]string{
		"/users/octocat/events*":             events,
		"/repos/octocat/repo/commits/abc123": commit,
	}, nil)
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if r.Linguist.Available {
		t.Errorf("Linguist.Available = true, want false when no analyzable files")
	}
}

func TestRun_LanguageActivity_ChartsDisabled_Skipped(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	events := `[{"type":"PushEvent","created_at":"` + now.Format(time.RFC3339) + `",` +
		`"repo":{"name":"octocat/repo"},` +
		`"payload":{"commits":[{"sha":"abc123"}]}}]`
	commit := `{"files":[{"filename":"main.go","additions":10,"deletions":0}]}`
	pc := pcWithRoutes(t, map[string]string{
		"/users/octocat/events*":             events,
		"/repos/octocat/repo/commits/abc123": commit,
	}, map[string]any{"plugin_habits_charts": "no"})
	out, _ := habits.Plugin.Run(context.Background(), pc)
	r := out.(*habits.Result)
	if r.Linguist.Available {
		t.Errorf("Linguist computed despite charts disabled")
	}
}

func TestPartial_Habits_ChartsOnly_Golden(t *testing.T) {
	r := fixedPartialResult()
	r.FactsEnabled = false
	r.ChartsEnabled = true
	got := renderPartial(t, r)
	assertPartialGolden(t, "habits_charts_only.svg", got)
	for _, marker := range []string{
		`Commit activity per hour of day`,
		`Commit activity per day of week`,
		`class="chart-bars"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("charts-only partial missing marker %q in:\n%s", marker, got)
		}
	}
	for _, marker := range []string{
		`Recent coding habits`,
		`<ul class="facts">`,
	} {
		if strings.Contains(got, marker) {
			t.Errorf("charts-only partial unexpectedly contains marker %q in:\n%s", marker, got)
		}
	}
}
