package calendar

import (
	"context"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

func fetchYearlyWeeks(ctx context.Context, pc *plugins.PluginContext, limit int) ([]plugins.ContributionWeek, error) {
	if pc == nil || pc.Data == nil || pc.GraphQL == nil || !truthyInput(pc.Inputs, "plugin_calendar") {
		return nil, nil
	}
	login := resolveLogin(pc)
	if login == "" || pc.Data.User == nil || pc.Data.User.CreatedAt.IsZero() {
		return nil, nil
	}
	now := time.Now().UTC()
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
		to := time.Date(year, time.December, 31, 23, 59, 59, int(time.Millisecond-time.Nanosecond), time.UTC)
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

func truthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
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
