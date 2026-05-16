// Package isocalendar owns the M4 "isocalendar" plugin. It reshapes
// the contributionsCollection.contributionCalendar payload (already
// fetched by base.runIndepth when isocalendar's input is enabled) into
// a week×day matrix and computes streaks.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p1-mvp.md §5
// Data model: specs/004-m4-github-plugins/data-model.md E-016
package isocalendar

import (
	"context"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "isocalendar"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &isocalendarPlugin{}

func init() {
	plugins.Register(Plugin)
}

type isocalendarPlugin struct{}

func (p *isocalendarPlugin) Name() string                     { return Name }
func (p *isocalendarPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["isocalendar"].
type Result struct {
	Skipped       bool      `json:"skipped,omitempty"`
	SkippedReason string    `json:"-"`
	Weeks         []ISOWeek `json:"weeks"`
	Streak        Streak    `json:"streak"`
	Average       float64   `json:"average"`
	Sum           int       `json:"sum"`
	Duration      string    `json:"duration"`
}

// IsSkipped lets the classic dispatcher detect the skipped path
// uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// ISOWeek holds 7 daily contribution counts for one ISO week.
type ISOWeek struct {
	FirstDay string `json:"firstDay"`
	Days     [7]int `json:"days"`
}

// Streak tracks contribution-day streaks (max ever, currently active).
type Streak struct {
	Max     int `json:"max"`
	Current int `json:"current"`
}

func (p *isocalendarPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.Data.Account == plugins.AccountOrganization {
		return &Result{
			Skipped:       true,
			SkippedReason: "not applicable to organizations",
			Weeks:         []ISOWeek{},
		}, nil
	}
	duration := parseDuration(pc.Inputs)
	weeksWanted := 26
	if duration == "full-year" {
		weeksWanted = 53
	}

	cal := pc.Data.Computed.ContributionCalendar
	if cal == nil || len(cal.Weeks) == 0 {
		return &Result{
			Skipped:       true,
			SkippedReason: "no contribution calendar",
			Weeks:         []ISOWeek{},
			Duration:      duration,
		}, nil
	}

	weeks := truncateWeeks(cal.Weeks, weeksWanted)
	iso := make([]ISOWeek, 0, len(weeks))
	flat := make([]int, 0, len(weeks)*7)
	sum := 0
	for _, w := range weeks {
		var row [7]int
		for _, d := range w.Days {
			idx := d.Weekday
			if idx < 0 {
				idx = 0
			}
			if idx > 6 {
				idx = 6
			}
			row[idx] = d.ContributionCount
			flat = append(flat, d.ContributionCount)
			sum += d.ContributionCount
		}
		iso = append(iso, ISOWeek{FirstDay: w.FirstDay, Days: row})
	}

	streak := computeStreak(flat)
	avg := 0.0
	if len(flat) > 0 {
		avg = float64(sum) / float64(len(flat))
	}
	return &Result{
		Weeks:    iso,
		Streak:   streak,
		Sum:      sum,
		Average:  avg,
		Duration: duration,
	}, nil
}

// truncateWeeks keeps the most-recent `n` weeks from the right-hand
// side of the GitHub-supplied calendar slice. Upstream returns the full
// 52-week window for the user; we slice to half-year (26) or full-year
// (53) depending on the configured duration.
func truncateWeeks(in []plugins.ContributionWeek, n int) []plugins.ContributionWeek {
	if n <= 0 || n >= len(in) {
		return in
	}
	return in[len(in)-n:]
}

// computeStreak walks the flattened day sequence newest-first to find
// the current streak (consecutive non-zero days starting at the tail)
// and oldest-first to find the max streak.
func computeStreak(days []int) Streak {
	var s Streak
	cur := 0
	for _, c := range days {
		if c > 0 {
			cur++
			if cur > s.Max {
				s.Max = cur
			}
		} else {
			cur = 0
		}
	}
	// Current streak counts from the tail until the first zero.
	tail := 0
	for i := len(days) - 1; i >= 0; i-- {
		if days[i] > 0 {
			tail++
			continue
		}
		break
	}
	s.Current = tail
	return s
}

func parseDuration(in map[string]any) string {
	if in == nil {
		return "half-year"
	}
	if v, ok := in["plugin_isocalendar_duration"]; ok {
		if s, ok := v.(string); ok {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "full-year" {
				return "full-year"
			}
		}
	}
	return "half-year"
}
