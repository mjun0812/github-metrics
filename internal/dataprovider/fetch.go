package dataprovider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// recentContributionDays is the trailing-day window the header / repository
// mini calendar renders (upstream core/index.mjs slice(-14)).
const recentContributionDays = 14

// calendarWindowDays bounds each contributionsCollection(from,to) calendar
// slice used to reconstruct the trailing year. GitHub applies a
// per-request resource limit to the contributionsCollection subtree, so
// the full-year calendar is fetched as consecutive windows of at most
// this width and the returned weeks are concatenated. 13 weeks
// (~3 months) leaves generous headroom under the observed node limit,
// which is undocumented and may shrink further. It is a whole number of
// weeks so — combined with a Sunday-aligned start — every window boundary
// lands on a week edge and no calendar week is split across two windows
// (which would render as a broken mid-year column).
const calendarWindowDays = 13 * 7

// defaultRepoBatch matches the upstream `plugin_repositories_batch`
// default and is used as the initial paging size when no per-Provider
// override is configured.
const defaultRepoBatch = 100

// licensePreferenceTopN caps the License-preference slice surfaced in
// the repository summary. Mirrors the cap the legacy base plugin used.
const licensePreferenceTopN = 5

// repoResult bundles the per-node repository accumulator together with
// the aggregated totals so Repositories and RepositorySummary can share
// a single memoized paging walk.
type repoResult struct {
	repos   []plugins.Repository
	summary *plugins.ComputedRepositories
}

// fetchProfile discovers whether p.login refers to a user or an
// organization account. The current implementation issues the user
// query first; on a `not found` style failure it falls back to the
// organization query. This mirrors the base plugin's branching
// (organization branch only ran when AccountKind == Organization) while
// removing the engine-supplied hint dependency.
func (p *Provider) fetchProfile(ctx context.Context) (*plugins.Profile, error) {
	if p.gql == nil {
		return nil, fmt.Errorf("dataprovider: nil GraphQL client")
	}
	if p.login == "" {
		return nil, fmt.Errorf("dataprovider: empty login")
	}

	userResp, userErr := p.gql.User(ctx, p.login)
	if userErr == nil && userResp != nil && userResp.User != nil {
		user := userFromGraphQL(userResp.User)
		if err := p.hydrateContributions(ctx, user); err != nil {
			return nil, fmt.Errorf("dataprovider: user(%q) contributions: %w", p.login, err)
		}
		return &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: user,
		}, nil
	}

	// If the user lookup failed with a transient error, surface that
	// directly so callers see the real cause instead of a misleading
	// "organization not found".
	if userErr != nil && isTransient(userErr) {
		return nil, fmt.Errorf("dataprovider: user(%q): %w", p.login, userErr)
	}

	orgResp, orgErr := p.gql.Organization(ctx, p.login)
	if orgErr == nil && orgResp != nil && orgResp.Organization != nil {
		return &plugins.Profile{
			Kind:         plugins.ProfileKindOrganization,
			Organization: organizationFromGraphQL(orgResp.Organization),
		}, nil
	}
	if userErr != nil {
		return nil, fmt.Errorf("dataprovider: user(%q): %w", p.login, userErr)
	}
	if orgErr != nil {
		return nil, fmt.Errorf("dataprovider: organization(%q): %w", p.login, orgErr)
	}
	return nil, fmt.Errorf("dataprovider: %q: account not found", p.login)
}

// userFromGraphQL converts the genqlient-generated user payload into
// the plugins.User shape downstream code consumes. The activity
// aggregates (Commits / PullRequestsReviewed / PullRequestsOpened /
// IssuesOpened), ContributedTo and RecentContributions are filled
// separately by hydrateContributions from the split contributionsCollection
// queries.
func userFromGraphQL(u *githubapi.UserUser) *plugins.User {
	if u == nil {
		return nil
	}
	return &plugins.User{
		Login:                    u.Login,
		Name:                     derefString(u.Name),
		AvatarURL:                u.AvatarUrl,
		CreatedAt:                u.CreatedAt,
		Followers:                connTotalFollowers(u),
		Following:                connTotalFollowing(u),
		Watching:                 connTotalWatching(u),
		SponsorshipsAsMaintainer: connTotalSponsorMaintainer(u),
		IssueComments:            connTotalIssueComments(u),
		Organizations:            connTotalOrganizations(u),
		Sponsoring:               connTotalSponsorshipsAsSponsor(u),
		Starred:                  connTotalStarred(u),
		Gists:                    connTotalGists(u),
		DiscussionsStarted:       connTotalDiscussionsStarted(u),
		DiscussionsComments:      connTotalDiscussionsComments(u),
		DiscussionAnswers:        connTotalDiscussionAnswers(u),
	}
}

// hydrateContributions fills the activity aggregates (commits / reviews /
// PRs / issues), the "contributed to" counter and the trailing
// mini-calendar days on user. GitHub rejects a contributionsCollection
// selection that combines two or more aggregate fields
// (RESOURCE_LIMITS_EXCEEDED nulls the whole user), so each aggregate is
// fetched as its own single-field query and the calendar is
// reconstructed from windowed slices. The requests are issued
// sequentially: concurrent GraphQL requests on a single token trip
// GitHub's secondary rate limit (observed deterministically on Actions
// runners), and GitHub's guidance is to keep GraphQL calls serial.
//
// The four aggregates and the calendar are load-bearing: any failure
// aborts the profile fetch so the caller surfaces a real error rather
// than silently zeroed counters. repositoriesContributedTo is best
// effort — that single field also trips the resource limit on
// high-activity accounts, so a failure only logs a warning and leaves
// ContributedTo at zero (the header hides the counter row) instead of
// blanking the whole card.
func (p *Provider) hydrateContributions(ctx context.Context, user *plugins.User) error {
	if user == nil {
		return nil
	}
	commits, err := p.gql.UserContributionCommits(ctx, p.login)
	if err != nil {
		return err
	}
	if commits != nil && commits.User != nil && commits.User.ContributionsCollection != nil {
		user.Commits = commits.User.ContributionsCollection.TotalCommitContributions
	}
	reviews, err := p.gql.UserContributionPullRequestReviews(ctx, p.login)
	if err != nil {
		return err
	}
	if reviews != nil && reviews.User != nil && reviews.User.ContributionsCollection != nil {
		user.PullRequestsReviewed = reviews.User.ContributionsCollection.TotalPullRequestReviewContributions
	}
	prs, err := p.gql.UserContributionPullRequests(ctx, p.login)
	if err != nil {
		return err
	}
	if prs != nil && prs.User != nil && prs.User.ContributionsCollection != nil {
		user.PullRequestsOpened = prs.User.ContributionsCollection.TotalPullRequestContributions
	}
	issues, err := p.gql.UserContributionIssues(ctx, p.login)
	if err != nil {
		return err
	}
	if issues != nil && issues.User != nil && issues.User.ContributionsCollection != nil {
		user.IssuesOpened = issues.User.ContributionsCollection.TotalIssueContributions
	}
	cal, err := p.CommitCalendar(ctx)
	if err != nil {
		return err
	}
	if cal != nil {
		user.RecentContributions = recentDaysFromWeeks(cal.Weeks, recentContributionDays)
	}
	contributedTo, err := p.gql.UserRepositoriesContributedTo(ctx, p.login)
	if err != nil {
		p.logger.Warn("dataprovider: repositoriesContributedTo unavailable; hiding contributed-to counter",
			"login", p.login, "err", err)
		return nil
	}
	if contributedTo != nil && contributedTo.User != nil && contributedTo.User.RepositoriesContributedTo != nil {
		user.ContributedTo = contributedTo.User.RepositoriesContributedTo.TotalCount
	}
	return nil
}

// organizationFromGraphQL converts the org payload into plugins.Organization.
// Members are NOT fetched here — the consumer (organization people plugin)
// pages through that connection separately. We populate the static header
// fields only.
func organizationFromGraphQL(o *githubapi.OrganizationOrganization) *plugins.Organization {
	if o == nil {
		return nil
	}
	return &plugins.Organization{
		Login:       o.Login,
		Name:        derefString(o.Name),
		Description: derefString(o.Description),
		AvatarURL:   o.AvatarUrl,
	}
}

// fetchRepoResult drives the batch-halving repository paging loop and
// folds the per-node accumulator + aggregated totals into a repoResult.
// Mirrors the base plugin's prior populateRepositories: on full
// completion it returns the accumulator with a nil error. On a
// non-transient failure, or after batch=1 exhausts its retry budget of
// maxConsecutiveAttempts consecutive transient failures, it returns the
// partial result gathered so far together with a non-nil error.
//
// The transient batch-halving phase (100→50→25→…→1) does not draw on
// that budget: each halving step is a distinct, self-terminating
// recovery attempt, so the retry counter is spent only once batch has
// bottomed out at 1. This gives batch=1 the full budget to ride out a
// transient burst (e.g. a run of 503s) instead of being cut off after a
// single failure by attempts consumed while halving.
func (p *Provider) fetchRepoResult(ctx context.Context) (*repoResult, error) {
	prof, err := p.Profile(ctx)
	if err != nil {
		return nil, err
	}
	isUser := prof.Kind == plugins.ProfileKindUser
	state := &repoPagingState{batch: defaultRepoBatch}

	const maxConsecutiveAttempts = 6
	batchOneAttempts := 0
	for {
		hasNext, endCursor, ferr := p.fetchOneRepoPage(ctx, isUser, state)
		if ferr == nil {
			batchOneAttempts = 0
			if !hasNext || endCursor == nil || *endCursor == "" {
				return state.result(), nil
			}
			cursor := *endCursor
			state.cursor = &cursor
			continue
		}
		if !isTransient(ferr) {
			return state.result(), fmt.Errorf("dataprovider: repositories(%q): %w", p.login, ferr)
		}
		if state.batch > 1 {
			state.batch /= 2
			if state.batch < 1 {
				state.batch = 1
			}
			continue
		}
		batchOneAttempts++
		if batchOneAttempts >= maxConsecutiveAttempts {
			return state.result(), fmt.Errorf("dataprovider: repositories(%q): batch=1 failed after %d retries: %w", p.login, batchOneAttempts, ferr)
		}
	}
}

// synthesizeRepoResult builds a one-element repoResult from the single
// Repo fetch for repository-template mode, mirroring the legacy
// base.runRepository synthesis (upstream template.mjs:14-17). User-
// centric plugins (languages / activity / stargazers / ...) then operate
// in repo scope without special-casing.
//
// repositories_skip_private is intentionally NOT honoured here: the user
// explicitly named the target repo via `--repo owner/name`, so honouring
// the cross-plugin private filter would silently erase the single repo
// they asked for. The filter only governs the implicit account-wide
// paging walk (fetchOneRepoPage).
func (p *Provider) synthesizeRepoResult(ctx context.Context) (*repoResult, error) {
	r, err := p.fetchRepo(ctx)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &repoResult{repos: nil, summary: &plugins.ComputedRepositories{}}, nil
	}
	syntheticRepo := plugins.Repository{
		NameWithOwner: r.Owner + "/" + r.Name,
		Description:   r.Description,
		Stars:         r.Stargazers,
		Forks:         r.Forks,
		Watchers:      r.Watchers,
		Languages:     append([]plugins.LanguageStat(nil), r.Languages...),
	}
	if r.PrimaryLanguage != "" {
		lang := plugins.LanguageStat{
			Name:  r.PrimaryLanguage,
			Color: r.PrimaryLanguageColor,
		}
		syntheticRepo.Language = &lang
		if len(syntheticRepo.Languages) == 0 {
			syntheticRepo.Languages = []plugins.LanguageStat{{
				Name:  r.PrimaryLanguage,
				Color: r.PrimaryLanguageColor,
				Size:  1,
			}}
		}
	}
	return &repoResult{
		repos: []plugins.Repository{syntheticRepo},
		summary: &plugins.ComputedRepositories{
			Count:      1,
			Stargazers: r.Stargazers,
			Forks:      r.Forks,
			Watchers:   r.Watchers,
		},
	}, nil
}

// repoPagingState is dataprovider's equivalent of the legacy base
// pagingState. It carries the per-node accumulator and the totals
// aggregated alongside it (stargazers / forks / watchers / releases /
// packages / disk usage / deployments / issues / pull requests /
// license buckets).
type repoPagingState struct {
	batch  int
	cursor *string
	acc    []plugins.Repository

	count         int
	stargazers    int
	forks         int
	forked        int
	watchers      int
	releases      int
	packages      int
	diskUsage     int
	deployments   int
	issues        int
	pullRequests  int
	licenseCounts map[string]int
	// licensedRepos counts repositories that reported a non-nil
	// licenseInfo.name. It is the denominator for the LicensePreference
	// percentage so the figures sum to 100% across the licensed subset.
	licensedRepos int
}

// result snapshots the accumulator + totals into a repoResult.
func (s *repoPagingState) result() *repoResult {
	return &repoResult{
		repos: s.acc,
		summary: &plugins.ComputedRepositories{
			Count:             s.count,
			Stargazers:        s.stargazers,
			Forks:             s.forks,
			Forked:            s.forked,
			Watchers:          s.watchers,
			Releases:          s.releases,
			Packages:          s.packages,
			DiskUsage:         s.diskUsage,
			Deployments:       s.deployments,
			Issues:            s.issues,
			PullRequests:      s.pullRequests,
			LicensePreference: topLicenseShares(s.licenseCounts, s.licensedRepos, licensePreferenceTopN),
		},
	}
}

func (p *Provider) fetchOneRepoPage(ctx context.Context, isUser bool, state *repoPagingState) (bool, *string, error) {
	if isUser {
		resp, err := p.gql.UserRepositories(ctx, p.login, state.batch, state.cursor)
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
			// repositories_skip_private: drop private nodes before
			// they reach the accumulator so both Computed.RepositoryList
			// AND every aggregate counter reflect the public subset.
			// Note: the totalCount above is the unfiltered server-side
			// total — we overwrite it with len(state.acc) at the end so
			// state.count matches the visible list. We do this only
			// when filtering is active so the unfiltered code path is
			// byte-identical to the previous behavior.
			if p.skipPrivate && node.IsPrivate {
				continue
			}
			state.acc = append(state.acc, repositoryFromUserNode(node))
			state.stargazers += node.StargazerCount
			state.forks += node.ForkCount
			if node.IsFork {
				state.forked++
			}
			if node.Watchers != nil {
				state.watchers += node.Watchers.TotalCount
			}
			if node.Issues != nil {
				state.issues += node.Issues.TotalCount
			}
			if node.PullRequests != nil {
				state.pullRequests += node.PullRequests.TotalCount
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
		if p.skipPrivate {
			state.count = len(state.acc)
		}
		pi := conn.PageInfo
		return pi.HasNextPage, pi.EndCursor, nil
	}
	resp, err := p.gql.OrganizationRepositories(ctx, p.login, state.batch, state.cursor)
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
		// repositories_skip_private: see the symmetric user-branch
		// comment above. Same rationale — drop private nodes before
		// the accumulator so both list and aggregates exclude them.
		if p.skipPrivate && node.IsPrivate {
			continue
		}
		state.acc = append(state.acc, repositoryFromOrgNode(node))
		state.stargazers += node.StargazerCount
		state.forks += node.ForkCount
		if node.IsFork {
			state.forked++
		}
		if node.Watchers != nil {
			state.watchers += node.Watchers.TotalCount
		}
		if node.Issues != nil {
			state.issues += node.Issues.TotalCount
		}
		if node.PullRequests != nil {
			state.pullRequests += node.PullRequests.TotalCount
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
	if p.skipPrivate {
		state.count = len(state.acc)
	}
	pi := conn.PageInfo
	return pi.HasNextPage, pi.EndCursor, nil
}

// fetchCommitCalendar reconstructs the trailing-year contribution
// calendar from windowed contributionsCollection(from,to) queries and
// returns it. nil with a nil error when the payload is absent
// (organization profile, fresh user account). A single whole-year
// selection is rejected by GitHub's per-request resource limit, so the
// year is split into consecutive windows of at most calendarWindowDays
// and the returned weeks are concatenated. TotalContributions is
// reconstructed as the sum of the per-day counts (no day appears in two
// windows because each window ends one millisecond before the next
// begins).
func (p *Provider) fetchCommitCalendar(ctx context.Context) (*plugins.ContributionCalendar, error) {
	weeks, err := p.fetchCalendarWeeks(ctx, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("dataprovider: calendar(%q): %w", p.login, err)
	}
	if len(weeks) == 0 {
		return nil, nil
	}
	total := 0
	for _, w := range weeks {
		for _, d := range w.Days {
			total += d.ContributionCount
		}
	}
	return &plugins.ContributionCalendar{
		TotalContributions: total,
		Weeks:              weeks,
	}, nil
}

// fetchCalendarWeeks pages the trailing year ending at now in
// calendarWindowDays-wide windows and concatenates the returned weeks.
func (p *Provider) fetchCalendarWeeks(ctx context.Context, now time.Time) ([]plugins.ContributionWeek, error) {
	var weeks []plugins.ContributionWeek
	for _, w := range calendarWindows(now) {
		resp, err := p.gql.UserIsocalendar(ctx, p.login, w[0], w[1])
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.User == nil || resp.User.ContributionsCollection == nil ||
			resp.User.ContributionsCollection.ContributionCalendar == nil {
			continue
		}
		weeks = append(weeks, weeksFromIsocalendar(resp.User.ContributionsCollection.ContributionCalendar.Weeks)...)
	}
	return weeks, nil
}

// calendarWindows splits the trailing year ending at now into
// consecutive [from, to] ranges of calendarWindowDays each. The start is
// snapped back to the previous Sunday at UTC midnight (mirroring GitHub's
// Sunday-first calendar weeks, as isocalendar does) and each window is a
// whole number of weeks, so every window boundary falls on a Sunday and
// no calendar week is split across two windows. Each range ends one
// millisecond before the next begins so GitHub never reports a boundary
// day in two adjacent windows.
func calendarWindows(now time.Time) [][2]time.Time {
	now = now.UTC()
	start := previousSundayUTC(now.AddDate(-1, 0, 0))
	var out [][2]time.Time
	for from := start; from.Before(now); {
		to := from.AddDate(0, 0, calendarWindowDays)
		if to.After(now) {
			to = now
		}
		out = append(out, [2]time.Time{from, to.Add(-time.Millisecond)})
		from = to
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

// isTransient mirrors base.isTransient so dataprovider can apply the
// same batch-halving heuristic without depending on the base package.
//
// Note: githubapi.ErrEmptyGraphQLResponse is deliberately excluded from
// the transient set. GitHub emits the `{"data": null}` shape from its
// secondary rate limit path (see #732); the block self-heals on the
// server side and additional retries (batch-halving or otherwise) only
// add load to the exact quota that just rejected us. Propagate the
// error instead so the plugin can surface a real signal.
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
