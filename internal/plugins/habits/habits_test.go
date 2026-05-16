package habits_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
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
}

func TestRun_GoldenShape(t *testing.T) {
	r := &habits.Result{
		Days:   14,
		Facts:  habits.HabitFacts{IndentStyle: "spaces", CommitsPerDay: 1.5},
		Charts: habits.HabitCharts{Hours: [24]int{}, Days: [7]int{2, 1, 3, 1, 0, 4, 2}},
		From:   200,
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
	_ = http.MethodGet
	_ = strings.Contains
}
