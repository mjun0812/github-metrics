// Package isocalendar owns the M4 "isocalendar" plugin. It reshapes
// the contributionsCollection.contributionCalendar payload (already
// fetched by base.runIndepth when isocalendar's input is enabled) into
// a week×day matrix and computes streaks.
package isocalendar

import (
	"context"
	"log/slog"
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
	Max           int       `json:"max"`
	Duration      string    `json:"duration"`
}

// IsSkipped lets the classic dispatcher detect the skipped path
// uniformly across plugins.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// ISOWeek holds 7 daily contribution counts for one ISO week.
//
// Days[i] is the raw contribution count for weekday i (0 = Sunday).
// DayColors[i] is the GitHub heatmap color for the same day. Both
// arrays are aligned and always 7 entries (zero-filled when the week
// is partial at the start/end of the duration window).
type ISOWeek struct {
	FirstDay  string    `json:"firstDay"`
	Days      [7]int    `json:"days"`
	DayColors [7]string `json:"dayColors"`
}

// Streak tracks contribution-day streaks (max ever, currently active).
type Streak struct {
	Max     int `json:"max"`
	Current int `json:"current"`
}

func (p *isocalendarPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{
			Skipped:       true,
			SkippedReason: reason,
			Weeks:         []ISOWeek{},
		}, nil
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

	// Primary path: fetch the upstream-parity window (half-year =
	// now-180d, full-year = now-1y, both snapped to a UTC Sunday) in
	// 4-week chunks so GitHub's per-range color normalization matches
	// upstream's gradient (#467).
	weeks, fetchErr := fetchWindowedWeeks(ctx, pc, duration)
	if fetchErr != nil && pc.Logger != nil {
		pc.Logger.Warn(
			"isocalendar: windowed calendar fetch failed; falling back to shared calendar",
			slog.String("error", fetchErr.Error()),
		)
	}
	if len(weeks) == 0 {
		// Degraded path (no GraphQL client / fetch failure): slice the
		// most-recent weeks off the shared indepth calendar like before.
		cal := pc.Data.Computed.ContributionCalendar
		if cal == nil || len(cal.Weeks) == 0 {
			return &Result{
				Skipped:       true,
				SkippedReason: "no contribution calendar",
				Weeks:         []ISOWeek{},
				Duration:      duration,
			}, nil
		}
		weeks = truncateWeeks(cal.Weeks, weeksWanted)
	}
	// Aggregation mirrors upstream isocalendar (index.mjs::statistics):
	// every ContributionDay in the window contributes its GitHub-reported
	// ContributionCount verbatim to sum/avg/max and to the streak passes.
	// That count already folds in private contributions when the user's
	// "Include private contributions on my profile" setting allows it, so
	// no public/private branching happens here — the half-year and
	// full-year variants differ only by window width, never by counting
	// rule. See #467 and the matching unit tests.
	iso := make([]ISOWeek, 0, len(weeks))
	flat := make([]int, 0, len(weeks)*7)
	sum := 0
	maxDay := 0
	for _, w := range weeks {
		var row [7]int
		var colors [7]string
		for _, d := range w.Days {
			idx := d.Weekday
			if idx < 0 {
				idx = 0
			}
			if idx > 6 {
				idx = 6
			}
			row[idx] = d.ContributionCount
			colors[idx] = d.Color
			if colors[idx] == "" {
				colors[idx] = colorForCount(d.ContributionCount)
			}
			flat = append(flat, d.ContributionCount)
			sum += d.ContributionCount
			if d.ContributionCount > maxDay {
				maxDay = d.ContributionCount
			}
		}
		// Fill any missing-day slots with the zero color so per-day
		// rendering can rely on DayColors[i] being populated.
		for i := 0; i < 7; i++ {
			if colors[i] == "" {
				colors[i] = colorForCount(0)
			}
		}
		iso = append(iso, ISOWeek{FirstDay: w.FirstDay, Days: row, DayColors: colors})
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
		Max:      maxDay,
		Average:  avg,
		Duration: duration,
	}, nil
}

// colorForCount maps a contribution-count int to GitHub's standard
// 5-level heatmap color palette (fallback when upstream's
// ContributionDay.Color isn't populated).
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
