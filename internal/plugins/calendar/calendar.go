// Package calendar owns the M4 "calendar" plugin. It collapses the
// per-day contribution data already populated by base.runIndepth
// (Computed.ContributionCalendar) into a per-year × per-month
// histogram.
package calendar

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "calendar"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &calendarPlugin{}

func init() {
	plugins.Register(Plugin)
}

type calendarPlugin struct{}

func (p *calendarPlugin) Name() string                     { return Name }
func (p *calendarPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["calendar"].
type Result struct {
	Skipped       bool           `json:"skipped,omitempty"`
	SkippedReason string         `json:"-"`
	Years         []YearCalendar `json:"years"`
	Limit         int            `json:"limit"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// YearCalendar carries one calendar year's contribution histogram +
// per-week per-day cells for the upstream-equivalent heatmap render.
// Months[12] is preserved for backward compat with the v1 JSON shape
// (Principle II additive extension).
type YearCalendar struct {
	Year   int            `json:"year"`
	Total  int            `json:"total"`
	Months [12]int        `json:"months"`
	Weeks  []CalendarWeek `json:"weeks"`
}

// CalendarWeek mirrors upstream `week.contributionDays`. May have 1-7
// entries; a year-boundary week is short (1-6 days) and the partial
// offsets it so the cells align with the right weekday rows.
type CalendarWeek struct {
	ContributionDays []ContributionCell `json:"contributionDays"`
}

// ContributionCell mirrors one cell in the heatmap — `{date, color}`
// per upstream EJS line 21. Count is included as additional metadata
// (upstream's GraphQL exposes it; the EJS doesn't render it, but
// downstream consumers may use it).
type ContributionCell struct {
	Date  string `json:"date"`
	Color string `json:"color"`
	Count int    `json:"count"`
}

// Run aggregates ContributionCalendar.Weeks into per-year × per-month
// totals. Limit caps the number of most-recent years returned (0 =
// unlimited).
func (p *calendarPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			Years:         []YearCalendar{},
		}, nil
	}
	// plugin_calendar_limit defaults to 1 per assets/plugins/calendar/metadata.yml
	// (display the last year only). The key is absent unless the user sets it
	// explicitly, so apply the metadata default here instead of falling back to
	// the Go zero value (0 = "all years"), matching upstream's index.mjs.
	limit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_calendar_limit", 1)
	cal := pc.Data.Computed.ContributionCalendar
	weeks := []plugins.ContributionWeek{}
	if fetched, err := fetchYearlyWeeks(ctx, pc, limit); err != nil {
		pc.Data.AppendError(fmt.Errorf("calendar: yearly fetch: %w", err))
	} else if len(fetched) > 0 {
		weeks = fetched
	}
	if len(weeks) == 0 && cal != nil {
		weeks = cal.Weeks
	}
	if len(weeks) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no contribution calendar",
			Years:         []YearCalendar{},
		}, nil
	}

	byYear := map[int]*YearCalendar{}
	yearsOrder := []int{}
	for _, w := range weeks {
		// A week may span 2 years at year boundaries. Group its days by
		// year so a Dec/Jan boundary week appears in BOTH years' weeks
		// list — each year gets the days that fall within it. This is
		// equivalent to upstream's per-year column layout where a
		// partial first week is offset down per EJS line 22.
		dayByYear := map[int][]plugins.ContributionDay{}
		for _, d := range w.Days {
			year, month, ok := parseDate(d.Date)
			if !ok {
				continue
			}
			yc, exists := byYear[year]
			if !exists {
				yc = &YearCalendar{Year: year}
				byYear[year] = yc
				yearsOrder = append(yearsOrder, year)
			}
			yc.Total += d.ContributionCount
			if month >= 1 && month <= 12 {
				yc.Months[month-1] += d.ContributionCount
			}
			dayByYear[year] = append(dayByYear[year], d)
		}
		// Emit a single CalendarWeek per year (containing only the days
		// that fall within that year) so the upstream-equivalent partial
		// can render the boundary offset correctly.
		for year, days := range dayByYear {
			cells := make([]ContributionCell, 0, len(days))
			for _, d := range days {
				color := d.Color
				if color == "" {
					color = colorForCount(d.ContributionCount)
				}
				cells = append(cells, ContributionCell{
					Date:  d.Date,
					Color: color,
					Count: d.ContributionCount,
				})
			}
			byYear[year].Weeks = append(byYear[year].Weeks, CalendarWeek{ContributionDays: cells})
		}
	}

	// Years sorted descending by year (newest first), matching upstream
	// calendar.full. Apply limit before reversing semantics: keep the
	// most-recent N when limit > 0.
	sortInts(yearsOrder)
	if limit > 0 && len(yearsOrder) > limit {
		yearsOrder = yearsOrder[len(yearsOrder)-limit:]
	}
	years := make([]YearCalendar, 0, len(yearsOrder))
	for i := len(yearsOrder) - 1; i >= 0; i-- {
		y := yearsOrder[i]
		years = append(years, *byYear[y])
	}
	return &Result{Years: years, Limit: limit}, nil
}

// colorForCount maps a contribution-count int to GitHub's standard
// 5-level heatmap color palette. Used as a fallback when the upstream
// GraphQL ContributionDay.Color isn't populated.
func colorForCount(n int) string {
	switch {
	case n <= 0:
		return "#ebedf0"
	case n < 5:
		return "#9be9a8"
	case n < 10:
		return "#40c463"
	case n < 20:
		return "#30a14e"
	default:
		return "#216e39"
	}
}

// parseDate decodes a "YYYY-MM-DD" date into year + month. Returns
// (0, 0, false) on malformed input.
func parseDate(s string) (year, month int, ok bool) {
	if len(s) < 10 || s[4] != '-' || s[7] != '-' {
		return 0, 0, false
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, 0, false
	}
	m, err := strconv.Atoi(s[5:7])
	if err != nil {
		return 0, 0, false
	}
	return y, m, true
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
