package base

import (
	"context"
	"time"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

func fetchLifetimeCommits(ctx context.Context, gql *githubapi.GraphQL, login string, createdAt time.Time) (int, error) {
	if gql == nil || login == "" || createdAt.IsZero() {
		return 0, nil
	}
	now := time.Now().UTC()
	startYear := createdAt.UTC().Year()
	total := 0
	for year := startYear; year <= now.Year(); year++ {
		from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		if year == startYear && createdAt.After(from) {
			from = createdAt.UTC()
		}
		to := time.Date(year, time.December, 31, 23, 59, 59, int(time.Millisecond-time.Nanosecond), time.UTC)
		if year == now.Year() {
			to = now
		}
		resp, err := gql.UserCommitContributions(ctx, login, from, to)
		if err != nil {
			return 0, err
		}
		if resp == nil || resp.User == nil || resp.User.ContributionsCollection == nil {
			continue
		}
		total += resp.User.ContributionsCollection.TotalCommitContributions
	}
	return total, nil
}

func populateLifetimeCommits(ctx context.Context, pc *plugins.PluginContext, login string) {
	if pc == nil || pc.Data == nil || pc.Data.User == nil {
		return
	}
	total, err := fetchLifetimeCommits(ctx, pc.GraphQL, login, pc.Data.User.CreatedAt)
	if err != nil {
		return
	}
	if total > 0 {
		pc.Data.User.Commits = total
	}
}
