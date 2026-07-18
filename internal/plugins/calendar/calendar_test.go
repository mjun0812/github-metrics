package calendar_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
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
	// limit=0 means "all years" (zero: disable); pass it explicitly to opt out
	// of the metadata default (1) and exercise the multi-year path.
	r := run(t, makeCal([]int{2023, 2024, 2025, 2026}), map[string]any{
		"plugin_calendar_limit": 0,
	})
	if len(r.Years) != 4 {
		t.Errorf("Years len = %d, want 4", len(r.Years))
	}
	if r.Years[0].Year != 2026 || r.Years[3].Year != 2023 {
		t.Errorf("years not newest-first: %+v", r.Years)
	}
}

// TestRun_DefaultLimitSingleYear is the regression for #444: with no explicit
// plugin_calendar_limit, the metadata default (1) must be applied so only the
// most-recent year is rendered (matching upstream), not all years.
func TestRun_DefaultLimitSingleYear(t *testing.T) {
	t.Parallel()
	r := run(t, makeCal([]int{2025, 2026}), nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped")
	}
	if len(r.Years) != 1 {
		t.Fatalf("default limit should keep 1 year; got %d years: %+v", len(r.Years), r.Years)
	}
	if r.Years[0].Year != 2026 {
		t.Errorf("default limit should keep most-recent year 2026; got %d", r.Years[0].Year)
	}
	if r.Limit != 1 {
		t.Errorf("Limit should default to 1; got %d", r.Limit)
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
	// Most-recent two, newest first: 2026, 2025.
	if r.Years[0].Year != 2026 || r.Years[1].Year != 2025 {
		t.Errorf("limit should keep most-recent 2 newest-first; got %+v", r.Years)
	}
}

func TestRun_FetchesFullCalendarYears(t *testing.T) {
	restore := calendar.SetNowForTest(func() time.Time {
		return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	})
	defer restore()

	data := plugins.NewData()
	data.User = &plugins.User{
		Login:     "octocat",
		CreatedAt: time.Date(2024, 3, 2, 9, 8, 7, 0, time.UTC),
	}
	seenFrom := []string{}
	seenTo := []string{}
	mux := mocks.NewGraphQLMux(t)
	// Each calendar year is fetched as consecutive week-aligned windows to
	// stay under GitHub's per-request contributionsCollection resource
	// limit. The mock keys its response day off the window's own `from`
	// date so chunks never double-count and every day lands in the right
	// year. The exact window count depends on the (deliberately
	// conservative) chunk width, so the assertions below check the
	// invariants — coverage from account creation to now, correct years —
	// rather than a brittle call count.
	mux.OnFunc("UserIsocalendar", func(vars map[string]any) (int, string) {
		from := fmt.Sprint(vars["from"])
		seenFrom = append(seenFrom, from)
		seenTo = append(seenTo, fmt.Sprint(vars["to"]))
		date := from[:10]
		return 200, fmt.Sprintf(`{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[{"firstDay":"%[1]s","contributionDays":[{"date":"%[1]s","contributionCount":1,"weekday":1,"color":"#9be9a8"}]}]}}}}}`, date)
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"user": "octocat", "plugin_calendar": true, "plugin_calendar_limit": 0}),
	)
	out, err := calendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*calendar.Result)
	// Three years covered, each split into more than one window.
	if got := mux.Calls("UserIsocalendar"); got <= 3 {
		t.Fatalf("UserIsocalendar calls = %d, want each year windowed into multiple calls", got)
	}
	if len(r.Years) != 3 || r.Years[0].Year != 2026 || r.Years[2].Year != 2024 {
		t.Fatalf("fetched years should render newest-first; got %+v", r.Years)
	}
	if seenFrom[0] != "2024-03-02T09:08:07Z" {
		t.Fatalf("first window should start at account creation; got %s", seenFrom[0])
	}
	if seenTo[len(seenTo)-1] != "2026-06-10T12:00:00Z" {
		t.Fatalf("final window should end at now; got %s", seenTo[len(seenTo)-1])
	}
}

func TestRun_FetchesLimitedCalendarYears(t *testing.T) {
	restore := calendar.SetNowForTest(func() time.Time {
		return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	})
	defer restore()

	for _, tc := range []struct {
		name          string
		createdAt     time.Time
		limit         int
		minYears      int
		wantFirstFrom string
	}{
		{
			// limit=2 keeps 2025+2026; both years are windowed, so more
			// than two calls fire.
			name:          "limit",
			createdAt:     time.Date(2024, 3, 2, 9, 8, 7, 0, time.UTC),
			limit:         2,
			minYears:      2,
			wantFirstFrom: "2025-01-01T00:00:00Z",
		},
		{
			// clamped to account creation: 2024+2025+2026, each windowed.
			name:          "created-at-clamp",
			createdAt:     time.Date(2024, 3, 2, 9, 8, 7, 0, time.UTC),
			limit:         10,
			minYears:      3,
			wantFirstFrom: "2024-03-02T09:08:07Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := plugins.NewData()
			data.User = &plugins.User{Login: "octocat", CreatedAt: tc.createdAt}
			seenFrom := []string{}
			mux := mocks.NewGraphQLMux(t)
			mux.OnFunc("UserIsocalendar", func(vars map[string]any) (int, string) {
				from := fmt.Sprint(vars["from"])
				seenFrom = append(seenFrom, from)
				date := from[:10]
				return 200, fmt.Sprintf(`{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[{"firstDay":"%[1]s","contributionDays":[{"date":"%[1]s","contributionCount":1,"weekday":1,"color":"#9be9a8"}]}]}}}}}`, date)
			})
			pc := mocks.NewPluginContext(
				t,
				mocks.WithGraphQL(mux),
				mocks.WithData(data),
				mocks.WithInputs(map[string]any{"user": "octocat", "plugin_calendar": true, "plugin_calendar_limit": tc.limit}),
			)
			if _, err := calendar.Plugin.Run(context.Background(), pc); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := mux.Calls("UserIsocalendar"); got < tc.minYears {
				t.Fatalf("UserIsocalendar calls = %d, want at least %d (one window per year, likely more)", got, tc.minYears)
			}
			if seenFrom[0] != tc.wantFirstFrom {
				t.Fatalf("first from = %s, want %s", seenFrom[0], tc.wantFirstFrom)
			}
		})
	}
}

// TestRun_ChunkBoundaryWeekStaysWhole reproduces the #781 heatmap
// regression: a calendar year split across windows must not fracture the
// boundary week. A range-aware mock emulates GitHub (Sunday-aligned weeks
// clipped to the queried range); with week-aligned chunk boundaries the
// week straddling a chunk cut is returned whole in exactly one window and
// renders as a single 7-cell column. A mid-week boundary (the bug) would
// return it as two partial weeks and calendar.go would emit two broken
// columns.
func TestRun_ChunkBoundaryWeekStaysWhole(t *testing.T) {
	restore := calendar.SetNowForTest(func() time.Time {
		return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	})
	defer restore()

	data := plugins.NewData()
	data.User = &plugins.User{
		Login:     "octocat",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	mux := mocks.NewGraphQLMux(t)
	mux.OnFunc("UserIsocalendar", func(vars map[string]any) (int, string) {
		fromDate := fmt.Sprint(vars["from"])[:10]
		toDate := fmt.Sprint(vars["to"])[:10]
		return 200, sundayWeeksJSON(t, fromDate, toDate)
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"user": "octocat", "plugin_calendar": true, "plugin_calendar_limit": 1}),
	)
	out, err := calendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*calendar.Result)
	if len(r.Years) != 1 || r.Years[0].Year != 2026 {
		t.Fatalf("want a single 2026 year, got %+v", r.Years)
	}

	// The chunk boundary for 2026 (from Jan 1, 13-week windows) lands on
	// Sunday 2026-03-29. That week and the one before it must each survive
	// as a whole 7-cell column.
	assertWholeWeek(t, r.Years[0].Weeks, "2026-03-29")
	assertWholeWeek(t, r.Years[0].Weeks, "2026-03-22")

	// No interior week is fractured: only the year-start and now-clamped
	// weeks may be partial.
	partials := 0
	for _, w := range r.Years[0].Weeks {
		if len(w.ContributionDays) != 7 {
			partials++
		}
	}
	if partials > 2 {
		t.Errorf("found %d partial weeks; want at most 2 (year start + now)", partials)
	}
}

// sundayWeeksJSON emulates GitHub's contributionCalendar for [fromDate,
// toDate] (inclusive, YYYY-MM-DD): full Sunday-Saturday weeks with each
// day clipped to the queried range.
func sundayWeeksJSON(t *testing.T, fromDate, toDate string) string {
	t.Helper()
	from, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		t.Fatalf("parse from %q: %v", fromDate, err)
	}
	to, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		t.Fatalf("parse to %q: %v", toDate, err)
	}
	weekStart := from.AddDate(0, 0, -int(from.Weekday())) // back to Sunday
	var weeks []string
	for w := weekStart; !w.After(to); w = w.AddDate(0, 0, 7) {
		var days []string
		for i := 0; i < 7; i++ {
			day := w.AddDate(0, 0, i)
			if day.Before(from) || day.After(to) {
				continue
			}
			days = append(days, fmt.Sprintf(
				`{"date":"%s","contributionCount":1,"weekday":%d,"color":"#9be9a8"}`,
				day.Format("2006-01-02"), i))
		}
		if len(days) == 0 {
			continue
		}
		weeks = append(weeks, fmt.Sprintf(`{"firstDay":"%s","contributionDays":[%s]}`,
			w.Format("2006-01-02"), strings.Join(days, ",")))
	}
	return fmt.Sprintf(`{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[%s]}}}}}`,
		strings.Join(weeks, ","))
}

// assertWholeWeek finds the CalendarWeek containing wantSunday (a Sunday
// date) and asserts it carries all 7 days of that week in order.
func assertWholeWeek(t *testing.T, weeks []calendar.CalendarWeek, wantSunday string) {
	t.Helper()
	sunday, err := time.Parse("2006-01-02", wantSunday)
	if err != nil {
		t.Fatalf("parse %q: %v", wantSunday, err)
	}
	for _, w := range weeks {
		if len(w.ContributionDays) == 0 || w.ContributionDays[0].Date != wantSunday {
			continue
		}
		if len(w.ContributionDays) != 7 {
			t.Fatalf("week of %s has %d cells, want a whole 7-cell column: %+v",
				wantSunday, len(w.ContributionDays), w.ContributionDays)
		}
		for i, d := range w.ContributionDays {
			want := sunday.AddDate(0, 0, i).Format("2006-01-02")
			if d.Date != want {
				t.Fatalf("week of %s cell %d = %s, want %s", wantSunday, i, d.Date, want)
			}
		}
		return
	}
	t.Fatalf("no CalendarWeek starting %s found in %+v", wantSunday, weeks)
}

func TestRun_FetchedCalendarOverridesComputedCalendar(t *testing.T) {
	// Pin the clock early in the year so the single fetched year fits in
	// one 13-week window; a fixed OnBody response would otherwise be
	// replayed (and double-counted) across multiple chunks.
	restore := calendar.SetNowForTest(func() time.Time {
		return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	})
	defer restore()

	data := plugins.NewData()
	data.User = &plugins.User{
		Login:     "octocat",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	data.Computed.ContributionCalendar = makeCal([]int{2020})
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("UserIsocalendar", 200, `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[{"firstDay":"2026-01-01","contributionDays":[{"date":"2026-01-01","contributionCount":7,"weekday":4,"color":"#40c463"}]}]}}}}}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"user": "octocat", "plugin_calendar": true, "plugin_calendar_limit": 1}),
	)
	out, err := calendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*calendar.Result)
	if len(r.Years) != 1 || r.Years[0].Year != 2026 || r.Years[0].Total != 7 {
		t.Fatalf("fetched calendar should override computed calendar; got %+v", r.Years)
	}
}

func TestRun_FetchErrorFallsBackToComputedCalendar(t *testing.T) {
	data := plugins.NewData()
	data.User = &plugins.User{
		Login:     "octocat",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	data.Computed.ContributionCalendar = makeCal([]int{2025})
	mux := mocks.NewGraphQLMux(t)
	mux.OnBody("UserIsocalendar", 200, `{"errors":[{"message":"calendar blip"}]}`)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"user": "octocat", "plugin_calendar": true, "plugin_calendar_limit": 1}),
	)
	out, err := calendar.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*calendar.Result)
	if len(r.Years) != 1 || r.Years[0].Year != 2025 {
		t.Fatalf("computed calendar fallback should render; got %+v", r.Years)
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "yearly fetch") {
		t.Fatalf("expected yearly fetch error to be recorded; got %v", errs)
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

// TestPartial_NativeSVG pins the #409 Phase B6 conversion: the calendar
// partial emits native SVG (a WrapSection nested `<svg>`, no foreignObject
// HTML wrapper) and self-reports a non-zero pixel height.
func TestPartial_NativeSVG(t *testing.T) {
	t.Parallel()
	r := &calendar.Result{
		Years: []calendar.YearCalendar{
			{Year: 2026, Weeks: []calendar.CalendarWeek{
				{ContributionDays: []calendar.ContributionCell{{Color: "#9be9a8"}, {Color: "#216e39"}}},
			}},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(calendar.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, h, err := calendar.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if h <= 0 {
		t.Errorf("height = %d, want > 0 (self-reported)", h)
	}
	for _, marker := range []string{
		`<g data-section="calendar">`,
		`<svg class="calendar"`,
		`Contributions calendar`,
		`fill="#216e39"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
	for _, html := range []string{`<div`, `<h2`, `class="row"`, `class="field"`} {
		if strings.Contains(got, html) {
			t.Errorf("native SVG output should not contain HTML %q in:\n%s", html, got)
		}
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
