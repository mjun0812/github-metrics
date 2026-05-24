package base

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Generated genqlient types are deeply-nested per-query distinct
// structs; aliasing here keeps the repository-conversion call sites
// readable.
type (
	userRepoNode = githubapi.UserRepositoriesUserRepositoriesRepositoryConnectionNodesRepository
	orgRepoNode  = githubapi.OrganizationRepositoriesOrganizationRepositoriesRepositoryConnectionNodesRepository
)

// pagingState bundles the inputs and accumulator of a single
// repository-paging loop. batch shrinks on transient failure (5xx /
// network timeout), cursor advances on success, acc grows monotonically.
type pagingState struct {
	batch  int
	cursor *string
	acc    []plugins.Repository
	// totals collected alongside the per-node list so callers that only
	// care about counts (M1 compatibility) keep working.
	count      int
	stargazers int
	forks      int
	watchers   int
}

// fetchRepositories walks the connection until exhausted, halving the
// batch size on 502 / network timeout / generic 5xx responses. It
// returns (acc, nil) on full completion; on persistent failure at
// batch=1 it records a *RetryableError on pc.Data.Errors and returns
// the partial accumulator with a nil error so the caller can still
// surface useful data (degraded path).
//
// initialBatch <= 0 falls back to 100 to match upstream defaults.
func fetchRepositories(ctx context.Context, pc *plugins.PluginContext, login string, isUser bool, initialBatch int) (*pagingState, error) {
	if initialBatch <= 0 {
		initialBatch = 100
	}
	state := &pagingState{batch: initialBatch}

	const maxConsecutiveAttempts = 6 // batch_start = 100 → 50 → 25 → 12 → 6 → 3 → 1
	attempts := 0
	for {
		hasNext, endCursor, err := fetchOnePage(ctx, pc, login, isUser, state)
		if err == nil {
			attempts = 0
			if !hasNext || endCursor == nil || *endCursor == "" {
				return state, nil
			}
			cursor := *endCursor
			state.cursor = &cursor
			continue
		}
		if !isTransient(err) {
			return state, fmt.Errorf("base: fetchRepositories(%q): %w", login, err)
		}
		attempts++
		if state.batch > 1 {
			state.batch /= 2
			if state.batch < 1 {
				state.batch = 1
			}
			continue
		}
		// batch == 1 and still failing: stop with degraded result.
		if attempts >= maxConsecutiveAttempts {
			pc.Data.AppendError(xerrors.NewRetryableError(
				fmt.Errorf("base: repositories paging: batch=1 failed after %d retries: %w", attempts, err),
			))
			return state, nil
		}
		// retry once more at batch=1 before declaring failure
	}
}

// fetchOnePage issues one GraphQL query at the current batch size /
// cursor and folds the result into state. Returns (hasNextPage,
// endCursor, err); on err the caller decides whether to halve batch or
// give up.
func fetchOnePage(ctx context.Context, pc *plugins.PluginContext, login string, isUser bool, state *pagingState) (bool, *string, error) {
	if isUser {
		resp, err := pc.GraphQL.UserRepositories(ctx, login, state.batch, state.cursor)
		if err != nil {
			return false, nil, err
		}
		if resp == nil || resp.User == nil || resp.User.Repositories == nil {
			return false, nil, nil
		}
		conn := resp.User.Repositories
		if state.count == 0 {
			state.count = conn.TotalCount
		}
		for _, node := range conn.Nodes {
			if node == nil {
				continue
			}
			state.acc = append(state.acc, repositoryFromUserNode(node))
			state.stargazers += node.StargazerCount
			state.forks += node.ForkCount
			if node.Watchers != nil {
				state.watchers += node.Watchers.TotalCount
			}
		}
		pi := conn.PageInfo
		return pi.HasNextPage, pi.EndCursor, nil
	}

	resp, err := pc.GraphQL.OrganizationRepositories(ctx, login, state.batch, state.cursor)
	if err != nil {
		return false, nil, err
	}
	if resp == nil || resp.Organization == nil || resp.Organization.Repositories == nil {
		return false, nil, nil
	}
	conn := resp.Organization.Repositories
	if state.count == 0 {
		state.count = conn.TotalCount
	}
	for _, node := range conn.Nodes {
		if node == nil {
			continue
		}
		state.acc = append(state.acc, repositoryFromOrgNode(node))
		state.stargazers += node.StargazerCount
		state.forks += node.ForkCount
		if node.Watchers != nil {
			state.watchers += node.Watchers.TotalCount
		}
	}
	pi := conn.PageInfo
	return pi.HasNextPage, pi.EndCursor, nil
}

// populateRepositories drives the paging loop and writes the resulting
// totals + per-node accumulator into pc.Data.Computed. Preserves the M1
// behavior of populating count/stargazers/forks/watchers so callers
// reading just totals keep working.
func populateRepositories(ctx context.Context, pc *plugins.PluginContext, login string, first int, isUser bool) error {
	state, err := fetchRepositories(ctx, pc, login, isUser, first)
	if err != nil {
		return err
	}
	pc.Data.Computed.Repositories.Count = state.count
	pc.Data.Computed.Repositories.Stargazers = state.stargazers
	pc.Data.Computed.Repositories.Forks = state.forks
	pc.Data.Computed.Repositories.Watchers = state.watchers
	pc.Data.Computed.RepositoryList = state.acc
	return nil
}

func repositoryFromUserNode(n *userRepoNode) plugins.Repository {
	r := plugins.Repository{
		NameWithOwner: n.NameWithOwner,
		Description:   derefString(n.Description),
		URL:           n.Url,
		Visibility:    visibilityFromIsPrivate(n.IsPrivate),
		IsFork:        n.IsFork,
		Stars:         n.StargazerCount,
		Forks:         n.ForkCount,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.PrimaryLanguage != nil {
		r.Language = &plugins.LanguageStat{
			Name:  n.PrimaryLanguage.Name,
			Color: derefString(n.PrimaryLanguage.Color),
		}
	}
	if n.Languages != nil {
		for _, e := range n.Languages.Edges {
			if e == nil || e.Node == nil {
				continue
			}
			r.Languages = append(r.Languages, plugins.LanguageStat{
				Name:  e.Node.Name,
				Color: derefString(e.Node.Color),
				Size:  e.Size,
			})
		}
	}
	return r
}

func repositoryFromOrgNode(n *orgRepoNode) plugins.Repository {
	r := plugins.Repository{
		NameWithOwner: n.NameWithOwner,
		Description:   derefString(n.Description),
		URL:           n.Url,
		Visibility:    visibilityFromIsPrivate(n.IsPrivate),
		IsFork:        n.IsFork,
		Stars:         n.StargazerCount,
		Forks:         n.ForkCount,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.PrimaryLanguage != nil {
		r.Language = &plugins.LanguageStat{
			Name:  n.PrimaryLanguage.Name,
			Color: derefString(n.PrimaryLanguage.Color),
		}
	}
	if n.Languages != nil {
		for _, e := range n.Languages.Edges {
			if e == nil || e.Node == nil {
				continue
			}
			r.Languages = append(r.Languages, plugins.LanguageStat{
				Name:  e.Node.Name,
				Color: derefString(e.Node.Color),
				Size:  e.Size,
			})
		}
	}
	return r
}

func visibilityFromIsPrivate(isPrivate bool) string {
	if isPrivate {
		return "private"
	}
	return "public"
}

// isTransient reports whether err represents a 5xx response or a
// network-level timeout. The genqlient client surfaces HTTP errors as
// generic Go errors; we pattern-match the embedded text for 5xx because
// it is the most reliable signal across transport implementations.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return true
	}
	msg := err.Error()
	for _, marker := range []string{
		"500", "502", "503", "504",
		"Internal Server Error", "Bad Gateway",
		"Service Unavailable", "Gateway Timeout",
		"i/o timeout", "context deadline exceeded",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
