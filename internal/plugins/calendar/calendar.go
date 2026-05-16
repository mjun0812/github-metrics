// Package calendar owns the M4 "calendar" plugin. It collapses the
// per-day contribution data already populated by base.runIndepth
// (Computed.ContributionCalendar) into a per-year × per-month
// histogram.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §1
// Data model: specs/004-m4-github-plugins/data-model.md E-020
package calendar

import (
	"context"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
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

// YearCalendar carries one calendar year's contribution histogram.
type YearCalendar struct {
	Year   int     `json:"year"`
	Total  int     `json:"total"`
	Months [12]int `json:"months"`
}

// Run aggregates ContributionCalendar.Weeks into per-year × per-month
// totals. Limit caps the number of most-recent years returned (0 =
// unlimited).
func (p *calendarPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	cal := pc.Data.Computed.ContributionCalendar
	if cal == nil || len(cal.Weeks) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no contribution calendar",
			Years:         []YearCalendar{},
		}, nil
	}
	limit := readInt(pc.Inputs, "plugin_calendar_limit")

	byYear := map[int]*YearCalendar{}
	yearsOrder := []int{}
	for _, w := range cal.Weeks {
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
		}
	}

	// Years sorted ascending by year (oldest first), then truncated by
	// limit (keep the most-recent N when limit > 0).
	sortInts(yearsOrder)
	years := make([]YearCalendar, 0, len(yearsOrder))
	for _, y := range yearsOrder {
		years = append(years, *byYear[y])
	}
	if limit > 0 && len(years) > limit {
		years = years[len(years)-limit:]
	}
	return &Result{Years: years, Limit: limit}, nil
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

func readInt(in map[string]any, key string) int {
	v, ok := in[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}
