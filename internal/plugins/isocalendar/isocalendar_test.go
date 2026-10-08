package isocalendar_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
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
	inputs := map[string]any{"plugin_isocalendar": true}
	for k, v := range in {
		inputs[k] = v
	}
	pc := &plugins.PluginContext{Inputs: inputs, Data: data}
	out, err := isocalendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*isocalendar.Result)
}

// TestRun_Streak — Max is the longest run of non-zero days; Current is the
// trailing run.
func TestRun_Streak(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		dayFn       func(w, d int) int
		wantMax     int
		wantCurrent int
	}{
		{
			// week 0 days 1..5 = 1 contribution each → 5-day streak, not trailing.
			name: "max",
			dayFn: func(w, d int) int {
				if w == 0 && d >= 1 && d <= 5 {
					return 1
				}
				return 0
			},
			wantMax:     5,
			wantCurrent: 0,
		},
		{
			name: "current",
			dayFn: func(w, d int) int {
				if w == 1 && d >= 4 {
					return 1
				}
				return 0
			},
			wantMax:     3,
			wantCurrent: 3,
		},
	} {
		r := run(t, makeCalendar(2, tc.dayFn), plugins.AccountUser, nil)
		if r.Streak.Max != tc.wantMax || r.Streak.Current != tc.wantCurrent {
			t.Errorf("%s: Streak = %+v, want Max=%d Current=%d", tc.name, r.Streak, tc.wantMax, tc.wantCurrent)
		}
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

// TestRun_DurationVariantWindow pins the only difference between the two
// documented variants (#467): the duration input selects the window
// width — half-year keeps the most-recent 26 weeks, full-year keeps 53.
// Both variants run the identical aggregation over their window, so the
// fullyear card legitimately reports a larger best-streak/different
// average purely because it spans more days, not because of any
// counting discrepancy. Daily counts are shared between the windows.
func TestRun_DurationVariantWindow(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(60, func(w, d int) int { return 1 })

	half := run(t, cal, plugins.AccountUser, nil)
	full := run(t, cal, plugins.AccountUser, map[string]any{
		"plugin_isocalendar_duration": "full-year",
	})
	if len(half.Weeks) != 26 {
		t.Errorf("half-year Weeks = %d, want 26", len(half.Weeks))
	}
	if len(full.Weeks) != 53 {
		t.Errorf("full-year Weeks = %d, want 53", len(full.Weeks))
	}
	// Same daily value everywhere → full-year sum strictly larger
	// because it spans more weeks (53*7 vs 26*7).
	if full.Sum != 53*7 || half.Sum != 26*7 {
		t.Errorf("Sum half=%d full=%d, want %d / %d", half.Sum, full.Sum, 26*7, 53*7)
	}
	// Per-day max and average are window-invariant here (constant 1).
	if half.Max != 1 || full.Max != 1 {
		t.Errorf("Max half=%d full=%d, want 1 / 1", half.Max, full.Max)
	}
	if half.Average != 1 || full.Average != 1 {
		t.Errorf("Average half=%v full=%v, want 1 / 1", half.Average, full.Average)
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
	got, _, err := isocalendar.Partial(context.Background(), pc)
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
}

// TestRun_PluginDisabled_Skipped covers the gate-off path: without a
// truthy `plugin_isocalendar` input, Run must Skip before reaching the
// dataprovider or any API client. The GraphQL mux has no handlers, so
// any GraphQL call fails the test.
func TestRun_PluginDisabled_Skipped(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	rest := mocks.NewRESTMux(t)
	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
	pc.Provider = dataprovider.New("octocat", "", pc.GraphQL, pc.REST, pc.Logger, dataprovider.Options{})

	out, err := isocalendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*isocalendar.Result)
	if !r.Skipped || r.SkippedReason != "plugin disabled" {
		t.Errorf("Skipped = %v, SkippedReason = %q; want true, %q", r.Skipped, r.SkippedReason, "plugin disabled")
	}
	if n := rest.TotalCalls(); n != 0 {
		t.Errorf("REST calls = %d, want 0", n)
	}
}
