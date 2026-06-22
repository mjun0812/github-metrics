package isocalendar

import (
	"context"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// chunkDays is the width of one contributionsCollection(from,to) query
// slice. Upstream isocalendar (index.mjs::statistics) walks the window
// in 4-week slices; besides bounding each response, this is what makes
// the heatmap colorful — GitHub normalizes ContributionDay.Color
// against the maximum of the *queried range*, so 4-week slices yield
// chunk-local gradients while a single whole-window query flattens
// most days to the lightest palette bucket (#467).
const chunkDays = 4 * 7

// windowStart mirrors upstream's start-day computation: half-year is
// now-180d, full-year is now-1y, then snapped back to the previous
// Sunday at 00:00:00.000 UTC so the window aligns with GitHub's
// Sunday-first calendar weeks.
func windowStart(now time.Time, duration string) time.Time {
	now = now.UTC()
	var start time.Time
	if duration == "full-year" {
		start = now.AddDate(-1, 0, 0)
	} else {
		start = now.Add(-180 * 24 * time.Hour)
	}
	start = start.AddDate(0, 0, -int(start.Weekday()))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
}

// chunkRanges splits [start, now] into consecutive 4-week query
// ranges. Each range ends 1ms before the next one starts so GitHub
// never reports the boundary day in two chunks (upstream rewinds the
// chunk end to 23:59:59.999 of the previous day for the same reason).
func chunkRanges(start, now time.Time) [][2]time.Time {
	var out [][2]time.Time
	for from := start; from.Before(now); {
		to := from.Add(chunkDays * 24 * time.Hour)
		if to.After(now) {
			to = now
		}
		out = append(out, [2]time.Time{from, to.Add(-time.Millisecond)})
		from = to
	}
	return out
}

// fetchWindowedWeeks queries the contribution calendar for the
// upstream-parity duration window in 4-week chunks and concatenates
// the returned weeks. It returns nil with no error when the
// PluginContext has no GraphQL client or no resolvable login (test
// harnesses / degraded runs); callers then fall back to slicing the
// shared indepth calendar.
func fetchWindowedWeeks(ctx context.Context, pc *plugins.PluginContext, duration string) ([]plugins.ContributionWeek, error) {
	// core.RunPlugins drives every registered plugin regardless of
	// inputs, so gate the network fetch on the plugin actually being
	// enabled — disabled runs stay on the shared-calendar path.
	if pc.GraphQL == nil || !pluginutil.TruthyInput(pc.Inputs, "plugin_isocalendar") {
		return nil, nil
	}
	login := resolveLogin(pc)
	if login == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	var weeks []plugins.ContributionWeek
	for _, r := range chunkRanges(windowStart(now, duration), now) {
		resp, err := pc.GraphQL.UserIsocalendar(ctx, login, r[0], r[1])
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.User == nil || resp.User.ContributionsCollection == nil ||
			resp.User.ContributionsCollection.ContributionCalendar == nil {
			continue
		}
		for _, w := range resp.User.ContributionsCollection.ContributionCalendar.Weeks {
			if w == nil {
				continue
			}
			days := make([]plugins.ContributionDay, 0, len(w.ContributionDays))
			for _, d := range w.ContributionDays {
				if d == nil {
					continue
				}
				days = append(days, plugins.ContributionDay{
					Date:              d.Date,
					ContributionCount: d.ContributionCount,
					Weekday:           d.Weekday,
					Color:             d.Color,
				})
			}
			weeks = append(weeks, plugins.ContributionWeek{FirstDay: w.FirstDay, Days: days})
		}
	}
	return weeks, nil
}

// resolveLogin returns the page user's login, preferring the base
// plugin's resolved User payload over the raw inputs.
func resolveLogin(pc *plugins.PluginContext) string {
	if pc.Data != nil && pc.Data.User != nil && pc.Data.User.Login != "" {
		return pc.Data.User.Login
	}
	if v, ok := pc.Inputs["user"].(string); ok {
		return v
	}
	return ""
}
