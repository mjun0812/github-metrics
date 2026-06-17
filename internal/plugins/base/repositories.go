package base

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// licensePreferenceTopN caps the License-preference slice surfaced to
// downstream partials. Upstream `base.repositories.ejs` renders 3
// entries; we expose 5 in the data model so future renderers can show
// a longer breakdown without re-aggregating.
const licensePreferenceTopN = 5

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
//
// 429 Phase 2: releases / packages / diskUsage are summed across all
// owned repos, mirroring upstream `base.repositories.ejs`'s "N
// releases", "N packages", "<disk> used" labels. licenseCounts buckets
// repos by `licenseInfo.name` so populateRepositories can convert the
// raw map into a top-N LicensePreference slice (Count desc).
type pagingState struct {
	batch  int
	cursor *string
	acc    []plugins.Repository
	// totals collected alongside the per-node list so callers that only
	// care about counts (M1 compatibility) keep working.
	count         int
	stargazers    int
	forks         int
	watchers      int
	releases      int
	packages      int
	diskUsage     int
	deployments   int
	licenseCounts map[string]int
	// licensedRepos counts repositories that reported a non-nil
	// licenseInfo.name. It is the denominator for the LicensePreference
	// percentage so the figures sum to 100% across the licensed subset
	// rather than against the full repository total (which would mix
	// repos-with-license + repos-without).
	licensedRepos int
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
			if node.Releases != nil {
				state.releases += node.Releases.TotalCount
			}
			if node.Packages != nil {
				state.packages += node.Packages.TotalCount
			}
			if node.DiskUsage != nil {
				state.diskUsage += *node.DiskUsage
			}
			if node.Deployments != nil {
				state.deployments += node.Deployments.TotalCount
			}
			if node.LicenseInfo != nil && node.LicenseInfo.Name != "" {
				if state.licenseCounts == nil {
					state.licenseCounts = map[string]int{}
				}
				state.licenseCounts[node.LicenseInfo.Name]++
				state.licensedRepos++
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
		if node.Releases != nil {
			state.releases += node.Releases.TotalCount
		}
		if node.Packages != nil {
			state.packages += node.Packages.TotalCount
		}
		if node.DiskUsage != nil {
			state.diskUsage += *node.DiskUsage
		}
		if node.Deployments != nil {
			state.deployments += node.Deployments.TotalCount
		}
		if node.LicenseInfo != nil && node.LicenseInfo.Name != "" {
			if state.licenseCounts == nil {
				state.licenseCounts = map[string]int{}
			}
			state.licenseCounts[node.LicenseInfo.Name]++
			state.licensedRepos++
		}
	}
	pi := conn.PageInfo
	return pi.HasNextPage, pi.EndCursor, nil
}

// populateRepositories drives the paging loop and writes the resulting
// totals + per-node accumulator into pc.Data.Computed. Preserves the M1
// behavior of populating count/stargazers/forks/watchers so callers
// reading just totals keep working.
//
// 429 Phase 2: also writes Releases / Packages / DiskUsage /
// LicensePreference aggregated across all owned repos so the upstream
// base.repositories.ejs labels render with real numbers.
func populateRepositories(ctx context.Context, pc *plugins.PluginContext, login string, first int, isUser bool) error {
	state, err := fetchRepositories(ctx, pc, login, isUser, first)
	if err != nil {
		return err
	}
	pc.Data.Computed.Repositories.Count = state.count
	pc.Data.Computed.Repositories.Stargazers = state.stargazers
	pc.Data.Computed.Repositories.Forks = state.forks
	pc.Data.Computed.Repositories.Watchers = state.watchers
	pc.Data.Computed.Repositories.Releases = state.releases
	pc.Data.Computed.Repositories.Packages = state.packages
	pc.Data.Computed.Repositories.DiskUsage = state.diskUsage
	pc.Data.Computed.Repositories.Deployments = state.deployments
	pc.Data.Computed.Repositories.LicensePreference = topLicenseShares(
		state.licenseCounts, state.licensedRepos, licensePreferenceTopN,
	)
	pc.Data.Computed.RepositoryList = state.acc
	return nil
}

// topLicenseShares converts the raw license-name → count map produced
// by fetchRepositories into a top-N slice sorted by Count descending,
// breaking ties alphabetically so the ordering is deterministic. The
// returned Percent is computed against licensedRepos (the number of
// repos that reported a non-nil licenseInfo); when licensedRepos is 0
// the function returns nil so the partial hides the row entirely.
//
// limit <= 0 falls back to the package-level licensePreferenceTopN
// constant so unit tests can pin a smaller cap without touching the
// constant.
func topLicenseShares(counts map[string]int, licensedRepos, limit int) []plugins.LicenseShare {
	if len(counts) == 0 || licensedRepos <= 0 {
		return nil
	}
	if limit <= 0 {
		limit = licensePreferenceTopN
	}
	shares := make([]plugins.LicenseShare, 0, len(counts))
	for name, count := range counts {
		shares = append(shares, plugins.LicenseShare{
			Name:    name,
			Count:   count,
			Percent: float64(count) * 100 / float64(licensedRepos),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Count != shares[j].Count {
			return shares[i].Count > shares[j].Count
		}
		return shares[i].Name < shares[j].Name
	})
	if len(shares) > limit {
		shares = shares[:limit]
	}
	return shares
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
		CreatedAt:     n.CreatedAt,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.Issues != nil {
		r.Issues = n.Issues.TotalCount
	}
	if n.PullRequests != nil {
		r.PullRequests = n.PullRequests.TotalCount
	}
	if n.LicenseInfo != nil {
		r.License = &plugins.RepositoryLicense{
			Name:     n.LicenseInfo.Name,
			SpdxID:   derefString(n.LicenseInfo.SpdxId),
			Nickname: derefString(n.LicenseInfo.Nickname),
		}
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
		CreatedAt:     n.CreatedAt,
	}
	if n.Watchers != nil {
		r.Watchers = n.Watchers.TotalCount
	}
	if n.Issues != nil {
		r.Issues = n.Issues.TotalCount
	}
	if n.PullRequests != nil {
		r.PullRequests = n.PullRequests.TotalCount
	}
	if n.LicenseInfo != nil {
		r.License = &plugins.RepositoryLicense{
			Name:     n.LicenseInfo.Name,
			SpdxID:   derefString(n.LicenseInfo.SpdxId),
			Nickname: derefString(n.LicenseInfo.Nickname),
		}
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
