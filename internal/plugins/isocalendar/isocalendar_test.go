package isocalendar_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
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

// makeCalendar builds a synthetic ContributionCalendar with `weeks`
// weeks, each containing 7 days (Mon-Sun). The dayFn callback decides
// the contribution count per (weekIndex, dayIndex).
func makeCalendar(weeks int, dayFn func(w, d int) int) *plugins.ContributionCalendar {
	cal := &plugins.ContributionCalendar{}
	for w := 0; w < weeks; w++ {
		week := plugins.ContributionWeek{FirstDay: fmt.Sprintf("2026-W%02d", w+1)}
		for d := 0; d < 7; d++ {
			c := dayFn(w, d)
			cal.TotalContributions += c
			week.Days = append(week.Days, plugins.ContributionDay{
				Date:              fmt.Sprintf("2026-W%02d-%d", w+1, d+1),
				ContributionCount: c,
				Weekday:           d,
			})
		}
		cal.Weeks = append(cal.Weeks, week)
	}
	return cal
}

func run(t *testing.T, cal *plugins.ContributionCalendar, account plugins.AccountKind, in map[string]any) *isocalendar.Result {
	t.Helper()
	data := plugins.NewData()
	data.Account = account
	data.Computed.ContributionCalendar = cal
	pc := &plugins.PluginContext{Inputs: in, Data: data}
	out, err := isocalendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*isocalendar.Result)
}

// TestRun_HalfYear26Weeks asserts the half-year duration truncates to
// the most-recent 26 weeks.
func TestRun_HalfYear26Weeks(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(52, func(w, d int) int { return w })
	r := run(t, cal, plugins.AccountUser, nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if len(r.Weeks) != 26 {
		t.Errorf("Weeks len = %d, want 26", len(r.Weeks))
	}
}

// TestRun_FullYear53Weeks asserts full-year duration keeps 53 weeks
// when the input has at least that many.
func TestRun_FullYear53Weeks(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(53, func(w, d int) int { return d })
	r := run(t, cal, plugins.AccountUser, map[string]any{
		"plugin_isocalendar_duration": "full-year",
	})
	if len(r.Weeks) != 53 {
		t.Errorf("Weeks len = %d, want 53", len(r.Weeks))
	}
}

// TestRun_StreakMax — 5 consecutive non-zero days inside an otherwise
// zero calendar yields Max=5.
func TestRun_StreakMax(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(2, func(w, d int) int {
		// week 0 days 1..5 = 1 contribution each → 5-day streak.
		if w == 0 && d >= 1 && d <= 5 {
			return 1
		}
		return 0
	})
	r := run(t, cal, plugins.AccountUser, nil)
	if r.Streak.Max != 5 {
		t.Errorf("Streak.Max = %d, want 5", r.Streak.Max)
	}
	if r.Streak.Current != 0 {
		t.Errorf("Streak.Current = %d, want 0", r.Streak.Current)
	}
}

// TestRun_StreakCurrent — last 3 days non-zero yields Current=3.
func TestRun_StreakCurrent(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(2, func(w, d int) int {
		if w == 1 && d >= 4 {
			return 1
		}
		return 0
	})
	r := run(t, cal, plugins.AccountUser, nil)
	if r.Streak.Current != 3 {
		t.Errorf("Streak.Current = %d, want 3", r.Streak.Current)
	}
}

// TestRun_OrganizationSkipped — organization account → Skipped=true.
func TestRun_OrganizationSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, makeCalendar(26, func(w, d int) int { return 1 }), plugins.AccountOrganization, nil)
	if !r.Skipped {
		t.Errorf("expected Skipped=true for organization; got %+v", r)
	}
}

// Golden tests.
func TestPartial_Isocalendar_Golden(t *testing.T) {
	r := &isocalendar.Result{
		Weeks: []isocalendar.ISOWeek{
			{FirstDay: "2026-W18", Days: [7]int{1, 2, 0, 3, 4, 0, 1}},
			{FirstDay: "2026-W19", Days: [7]int{0, 0, 1, 2, 5, 3, 0}},
		},
		Streak:   isocalendar.Streak{Max: 5, Current: 0},
		Sum:      22,
		Average:  1.5714,
		Duration: "half-year",
	}
	data := plugins.NewData()
	data.SetPlugin(isocalendar.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := isocalendar.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "isocalendar.svg")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	for _, marker := range []string{
		`class="calendar"`,
		`class="calendar-day"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

func TestRun_GoldenShape_Isocalendar(t *testing.T) {
	r := &isocalendar.Result{
		Weeks:    []isocalendar.ISOWeek{{FirstDay: "2026-W18", Days: [7]int{1, 0, 0, 0, 0, 0, 0}}},
		Streak:   isocalendar.Streak{Max: 1, Current: 0},
		Sum:      1,
		Average:  0.14,
		Duration: "half-year",
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "isocalendar.json")
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
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), string(got))
	}
}
