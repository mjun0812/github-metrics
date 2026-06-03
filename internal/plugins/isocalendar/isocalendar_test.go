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

// TestRun_AggregationMatchesUpstream pins the contribution-count
// aggregation to upstream's isocalendar algorithm (#467). Upstream
// (source/plugins/isocalendar/index.mjs::statistics) iterates every
// ContributionDay in the windowed contributionsCollection calendar and
// computes:
//
//	values.push(day.contributionCount)
//	max          = Math.max(max, day.contributionCount)        // highest single day
//	streak.current = day.contributionCount ? current + 1 : 0   // trailing run
//	streak.max   = Math.max(streak.max, streak.current)        // forward pass
//	average      = sum(values) / values.length
//
// Our Run() must reproduce this exactly: Sum/Max/Average over every day
// in the (truncated) window and the same streak definitions. The per-day
// contributionCount is GitHub's own value (commits + issues + PRs +
// reviews, including private contributions iff the user enabled "Include
// private contributions on my profile"); the plugin does no reweighting,
// so identical daily inputs yield identical aggregates to upstream.
func TestRun_AggregationMatchesUpstream(t *testing.T) {
	t.Parallel()
	// 26 weeks so half-year keeps every day (no truncation), making the
	// expected aggregates a closed-form function of dayFn.
	// day count = weekIndex+1 (1..7 per week, week 0 has 1..7 → 0-based d).
	cal := makeCalendar(26, func(w, d int) int { return d }) // 0..6 each week

	r := run(t, cal, plugins.AccountUser, nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}

	// Recompute expected aggregates with the upstream algorithm.
	wantSum, wantMax := 0, 0
	wantStreakMax, cur := 0, 0
	total := 0
	for w := 0; w < 26; w++ {
		for d := 0; d < 7; d++ {
			c := d
			wantSum += c
			total++
			if c > wantMax {
				wantMax = c
			}
			if c > 0 {
				cur++
				if cur > wantStreakMax {
					wantStreakMax = cur
				}
			} else {
				cur = 0
			}
		}
	}
	wantAvg := float64(wantSum) / float64(total)
	// Trailing run: each week ends at d=6 (>0) but the next week starts
	// at d=0 (zero), so the only trailing non-zero run is the final
	// week's days 1..6 → current = 6.
	wantCurrent := 6

	if r.Sum != wantSum {
		t.Errorf("Sum = %d, want %d", r.Sum, wantSum)
	}
	if r.Max != wantMax {
		t.Errorf("Max = %d, want %d", r.Max, wantMax)
	}
	if r.Average != wantAvg {
		t.Errorf("Average = %v, want %v", r.Average, wantAvg)
	}
	if r.Streak.Max != wantStreakMax {
		t.Errorf("Streak.Max = %d, want %d", r.Streak.Max, wantStreakMax)
	}
	if r.Streak.Current != wantCurrent {
		t.Errorf("Streak.Current = %d, want %d", r.Streak.Current, wantCurrent)
	}
}

// TestRun_PrivateContributionsCountedFromCalendar documents the
// private-contribution policy (#467). The plugin consumes GitHub's
// contributionsCollection.contributionCalendar daily counts verbatim —
// it never inspects a separate public/private breakdown and applies no
// filtering. Whatever GitHub places in ContributionCount (which already
// folds in private contributions when the user's profile setting allows
// it) flows straight into Sum/Max/Average. This test asserts that a day
// whose count GitHub reports as N (regardless of its public/private
// origin) contributes exactly N — no doubling, no dropping.
func TestRun_PrivateContributionsCountedFromCalendar(t *testing.T) {
	t.Parallel()
	cal := makeCalendar(1, func(w, d int) int {
		if d == 3 {
			return 42 // e.g. a day dominated by private commits
		}
		return 0
	})
	r := run(t, cal, plugins.AccountUser, nil)
	if r.Sum != 42 {
		t.Errorf("Sum = %d, want 42 (GitHub-reported daily count passed through)", r.Sum)
	}
	if r.Max != 42 {
		t.Errorf("Max = %d, want 42", r.Max)
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
	// 011 v2: replaced the flat <g class="calendar"> + <rect class="calendar-day">
	// emission with the upstream-equivalent 3D isometric heatmap inside
	// <svg class="isocalendar-grid"> per upstream index.mjs lines 38-69.
	// New markers reflect the stats panel + isometric wrapper.
	for _, marker := range []string{
		`<h2 class="field">`,
		`Contributions calendar`,
		`class="isocalendar-grid"`,
		`<filter id="brightness1">`,
		`Best streak`,
		`Highest in a day at`,
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
