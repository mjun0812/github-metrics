package base

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"golang.org/x/sync/errgroup"
)

var (
	nowMu   sync.RWMutex
	nowFunc = func() time.Time { return time.Now().UTC() }
)

// SetNowForTest overrides the lifetime commit clock and returns a restore function.
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

func fetchLifetimeCommits(ctx context.Context, gql *githubapi.GraphQL, login string, createdAt time.Time) (int, error) {
	if gql == nil || login == "" || createdAt.IsZero() {
		return 0, nil
	}
	now := currentNow().UTC()
	startYear := createdAt.UTC().Year()
	var mu sync.Mutex
	total := 0
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for year := startYear; year <= now.Year(); year++ {
		year := year
		g.Go(func() error {
			from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			if year == startYear && createdAt.After(from) {
				from = createdAt.UTC()
			}
			to := time.Date(year, time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
			if year == now.Year() {
				to = now
			}
			resp, err := gql.UserCommitContributions(gctx, login, from, to)
			if err != nil {
				return err
			}
			if resp == nil || resp.User == nil || resp.User.ContributionsCollection == nil {
				return nil
			}
			mu.Lock()
			total += resp.User.ContributionsCollection.TotalCommitContributions
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return 0, err
	}
	return total, nil
}

func populateLifetimeCommits(ctx context.Context, pc *plugins.PluginContext, login string) {
	if pc == nil || pc.Data == nil || pc.Data.User == nil {
		return
	}
	if !lifetimeCommitsEnabled(pc.Inputs) {
		return
	}
	total, err := fetchLifetimeCommits(ctx, pc.GraphQL, login, pc.Data.User.CreatedAt)
	if err != nil {
		pc.Data.AppendError(fmt.Errorf("base: lifetime commits: %w", err))
		return
	}
	if total > 0 {
		pc.Data.User.Commits = total
	}
}

func lifetimeCommitsEnabled(inputs map[string]any) bool {
	raw, ok := inputs["base"]
	if !ok {
		return true
	}
	var value string
	switch v := raw.(type) {
	case string:
		value = v
	case []string:
		value = strings.Join(v, ",")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		value = strings.Join(parts, ",")
	default:
		return false
	}
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "activity") {
			return true
		}
	}
	return false
}
