package calendar

import (
	"context"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

var (
	nowMu   sync.RWMutex
	nowFunc = func() time.Time { return time.Now().UTC() }
)

// SetNowForTest overrides the calendar clock and returns a restore function.
func SetNowForTest(fn func() time.Time) func() {
	nowMu.Lock()
	old := nowFunc
	nowFunc = fn
	nowMu.Unlock()
	return func() {
		nowMu.Lock()
		nowFunc = old
		nowMu.Unlock()
	}
}

func currentNow() time.Time {
	nowMu.RLock()
	fn := nowFunc
	nowMu.RUnlock()
	return fn()
}

func fetchYearlyWeeks(ctx context.Context, pc *plugins.PluginContext, limit int) ([]plugins.ContributionWeek, error) {
	if pc == nil || pc.Data == nil || pc.GraphQL == nil || !pluginutil.TruthyInput(pc.Inputs, "plugin_calendar") {
		return nil, nil
	}
	user, ok := resolveUser(ctx, pc)
	if !ok || user.CreatedAt.IsZero() {
		return nil, nil
	}
	login := user.Login
	if login == "" {
		return nil, nil
	}
	now := currentNow().UTC()
	createdYear := user.CreatedAt.UTC().Year()
	startYear := createdYear
	if limit > 0 {
		startYear = now.Year() - limit + 1
		if startYear < createdYear {
			startYear = createdYear
		}
	}

	var weeks []plugins.ContributionWeek
	for year := startYear; year <= now.Year(); year++ {
		from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		if year == createdYear && user.CreatedAt.After(from) {
			from = user.CreatedAt.UTC()
		}
		to := time.Date(year, time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
		if year == now.Year() {
			to = now
		}
		// GitHub applies a per-request resource limit to the
		// contributionsCollection subtree, so a whole calendar year is
		// fetched as consecutive windows and the weeks concatenated.
		for _, window := range calendarChunks(from, to) {
			resp, err := pc.GraphQL.UserIsocalendar(ctx, login, window[0], window[1])
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
	}
	return weeks, nil
}

// chunkWindowDays bounds each contributionsCollection(from,to) calendar
// slice. 13 weeks (~3 months) keeps every request well under GitHub's
// per-request node limit for the contributionsCollection subtree (which
// is undocumented and may shrink) while covering one calendar year in a
// handful of calls. It is a whole number of weeks so, combined with
// Sunday-aligned interior boundaries, no calendar week is ever split
// across two windows.
const chunkWindowDays = 13 * 7

// calendarChunks splits [from, to] into consecutive [start, end] ranges
// of about chunkWindowDays each. Interior boundaries are snapped back to
// a Sunday (GitHub's week start) so a calendar week is never returned as
// two partial weeks in adjacent windows — calendar.go emits one column
// per incoming week, so a split week would render as a broken mid-year
// column. The outer from/to are left as-is: they are the caller's year
// boundaries, where calendar.go already splits year-spanning weeks into
// per-year partial columns (a partial first/last column is expected
// there), so only the interior cuts need week alignment. Each range ends
// one millisecond before the next begins so no boundary day appears in
// two windows.
func calendarChunks(from, to time.Time) [][2]time.Time {
	var out [][2]time.Time
	for start := from; start.Before(to); {
		cut := previousSundayUTC(start.AddDate(0, 0, chunkWindowDays))
		if !cut.After(start) {
			// Defensive: guarantee forward progress if chunkWindowDays is
			// ever set below a week.
			cut = previousSundayUTC(start.AddDate(0, 0, chunkWindowDays+7))
		}
		if !cut.Before(to) {
			out = append(out, [2]time.Time{start, to})
			break
		}
		out = append(out, [2]time.Time{start, cut.Add(-time.Millisecond)})
		start = cut
	}
	return out
}

// previousSundayUTC snaps t back to the most recent Sunday at 00:00:00
// UTC so calendar windows align with GitHub's Sunday-first weeks.
func previousSundayUTC(t time.Time) time.Time {
	t = t.UTC()
	t = t.AddDate(0, 0, -int(t.Weekday()))
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// resolveContributionCalendar returns the shared contribution-calendar
// payload via Provider.CommitCalendar, falling back to
// pc.Data.Computed.ContributionCalendar for unit tests that build
// PluginContext by hand without wiring a Provider. Returns nil when
// neither source carries data so the plugin can branch on absence.
func resolveContributionCalendar(ctx context.Context, pc *plugins.PluginContext) *plugins.ContributionCalendar {
	if pc == nil {
		return nil
	}
	if pc.Provider != nil {
		if c, err := pc.Provider.CommitCalendar(ctx); err == nil && c != nil {
			return c
		}
	}
	if pc.Data != nil {
		return pc.Data.Computed.ContributionCalendar
	}
	return nil
}

// resolveUser fetches the User payload via the shared dataprovider,
// falling back to the legacy pc.Data.User for unit tests that build
// PluginContext by hand without wiring a Provider. Returns
// (nil, false) when both sources are empty — calendar treats that as
// "no payload to render" and skips its fetch.
func resolveUser(ctx context.Context, pc *plugins.PluginContext) (*plugins.User, bool) {
	if pc == nil {
		return nil, false
	}
	if pc.Provider != nil {
		if u, err := pc.Provider.User(ctx); err == nil && u != nil {
			return u, true
		}
	}
	if pc.Data != nil && pc.Data.User != nil {
		return pc.Data.User, true
	}
	return nil, false
}
