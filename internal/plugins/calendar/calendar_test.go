package calendar_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
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

// makeCal builds a synthetic ContributionCalendar where day k has
// dayFn(year, month, day) contributions.
func makeCal(years []int) *plugins.ContributionCalendar {
	cal := &plugins.ContributionCalendar{}
	for _, year := range years {
		// One week per month, day per month — 12 weeks total per year.
		for month := 1; month <= 12; month++ {
			week := plugins.ContributionWeek{FirstDay: fmt.Sprintf("%04d-%02d-01", year, month)}
			for d := 1; d <= 7; d++ {
				week.Days = append(week.Days, plugins.ContributionDay{
					Date:              fmt.Sprintf("%04d-%02d-%02d", year, month, d),
					ContributionCount: month,
				})
			}
			cal.Weeks = append(cal.Weeks, week)
		}
	}
	return cal
}

func run(t *testing.T, cal *plugins.ContributionCalendar, inputs map[string]any) *calendar.Result {
	t.Helper()
	data := plugins.NewData()
	data.Computed.ContributionCalendar = cal
	pc := &plugins.PluginContext{Data: data, Inputs: inputs}
	out, err := calendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*calendar.Result)
}

func TestRun_NoCalendar_Skipped(t *testing.T) {
	t.Parallel()
	r := run(t, nil, nil)
	if !r.Skipped {
		t.Errorf("nil calendar should be Skipped")
	}
}

func TestRun_SingleYear(t *testing.T) {
	t.Parallel()
	r := run(t, makeCal([]int{2026}), nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped")
	}
	if len(r.Years) != 1 {
		t.Errorf("Years len = %d, want 1", len(r.Years))
	}
	if r.Years[0].Year != 2026 {
		t.Errorf("Year = %d, want 2026", r.Years[0].Year)
	}
}

func TestRun_MultiYear(t *testing.T) {
	t.Parallel()
	r := run(t, makeCal([]int{2023, 2024, 2025, 2026}), nil)
	if len(r.Years) != 4 {
		t.Errorf("Years len = %d, want 4", len(r.Years))
	}
	if r.Years[0].Year != 2023 || r.Years[3].Year != 2026 {
		t.Errorf("years not ascending: %+v", r.Years)
	}
}

func TestRun_LimitTruncates(t *testing.T) {
	t.Parallel()
	r := run(t, makeCal([]int{2023, 2024, 2025, 2026}), map[string]any{
		"plugin_calendar_limit": 2,
	})
	if len(r.Years) != 2 {
		t.Errorf("Years len = %d, want 2", len(r.Years))
	}
	// Most-recent two: 2025, 2026
	if r.Years[0].Year != 2025 || r.Years[1].Year != 2026 {
		t.Errorf("limit should keep most-recent 2; got %+v", r.Years)
	}
}

func TestRun_MonthHistogram(t *testing.T) {
	t.Parallel()
	r := run(t, makeCal([]int{2026}), nil)
	if r.Years[0].Months[0] != 7 {
		t.Errorf("January should have 7 contributions (7 days × 1); got %d", r.Years[0].Months[0])
	}
	if r.Years[0].Months[11] != 7*12 {
		t.Errorf("December should have 84 contributions (7 days × 12); got %d", r.Years[0].Months[11])
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &calendar.Result{
		Years: []calendar.YearCalendar{
			{Year: 2026, Total: 365, Months: [12]int{30, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 32}},
		},
		Limit: 0,
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "calendar.json")
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
