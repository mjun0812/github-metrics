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
	login := resolveLogin(pc)
	if login == "" || pc.Data.User == nil || pc.Data.User.CreatedAt.IsZero() {
		return nil, nil
	}
	now := currentNow().UTC()
	startYear := pc.Data.User.CreatedAt.UTC().Year()
	if limit > 0 {
		startYear = now.Year() - limit + 1
		if createdYear := pc.Data.User.CreatedAt.UTC().Year(); startYear < createdYear {
			startYear = createdYear
		}
	}

	var weeks []plugins.ContributionWeek
	for year := startYear; year <= now.Year(); year++ {
		from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		if year == pc.Data.User.CreatedAt.UTC().Year() && pc.Data.User.CreatedAt.After(from) {
			from = pc.Data.User.CreatedAt.UTC()
		}
		to := time.Date(year, time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
		if year == now.Year() {
			to = now
		}
		resp, err := pc.GraphQL.UserIsocalendar(ctx, login, from, to)
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

func resolveLogin(pc *plugins.PluginContext) string {
	if pc.Data != nil && pc.Data.User != nil && pc.Data.User.Login != "" {
		return pc.Data.User.Login
	}
	if v, ok := pc.Inputs["user"].(string); ok {
		return v
	}
	return ""
}
