package dataprovider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// defaultRepoBatch matches the upstream `plugin_repositories_batch`
// default and is used as the initial paging size when no per-Provider
// override is configured.
const defaultRepoBatch = 100

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
		return &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: userFromGraphQL(userResp.User),
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
// the plugins.User shape downstream code consumes.
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
		ContributedTo:            connTotalContributedTo(u),
		RecentContributions:      recentContributionDays(u, 14),
		Commits:                  contributionCommits(u),
		PullRequestsReviewed:     contributionPullRequestReviews(u),
		PullRequestsOpened:       contributionPullRequests(u),
		IssuesOpened:             contributionIssues(u),
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

// fetchRepositories drives the batch-halving repository paging loop.
// Mirrors the base plugin's prior populateRepositories: returns the
// accumulator on full completion, or the partial result with a nil
// error on degraded paths (batch=1 still failing). Callers that need
// the side-effect of an error appended to Data.Errors must consult the
// returned error directly — dataprovider does not touch Data.
func (p *Provider) fetchRepositories(ctx context.Context) ([]plugins.Repository, error) {
	prof, err := p.Profile(ctx)
	if err != nil {
		return nil, err
	}
	isUser := prof.Kind == plugins.ProfileKindUser
	state := &repoPagingState{batch: defaultRepoBatch}

	const maxConsecutiveAttempts = 6
	attempts := 0
	for {
		hasNext, endCursor, ferr := p.fetchOneRepoPage(ctx, isUser, state)
		if ferr == nil {
			attempts = 0
			if !hasNext || endCursor == nil || *endCursor == "" {
				return state.acc, nil
			}
			cursor := *endCursor
			state.cursor = &cursor
			continue
		}
		if !isTransient(ferr) {
			return state.acc, fmt.Errorf("dataprovider: repositories(%q): %w", p.login, ferr)
		}
		attempts++
		if state.batch > 1 {
			state.batch /= 2
			if state.batch < 1 {
				state.batch = 1
			}
			continue
		}
		if attempts >= maxConsecutiveAttempts {
			return state.acc, fmt.Errorf("dataprovider: repositories(%q): batch=1 failed after %d retries: %w", p.login, attempts, ferr)
		}
	}
}

// repoPagingState is dataprovider's compact equivalent of the base
// pagingState. We only need the per-node accumulator here — totals
// (stargazers / forks / etc.) are still aggregated by the legacy base
// shim until it is removed in #605.
type repoPagingState struct {
	batch  int
	cursor *string
	acc    []plugins.Repository
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
		for _, node := range conn.Nodes {
			if node == nil {
				continue
			}
			state.acc = append(state.acc, repositoryFromUserNode(node))
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
	for _, node := range conn.Nodes {
		if node == nil {
			continue
		}
		state.acc = append(state.acc, repositoryFromOrgNode(node))
	}
	pi := conn.PageInfo
	return pi.HasNextPage, pi.EndCursor, nil
}

// fetchCommitCalendar issues the indepth GraphQL query and returns the
// aggregated contribution calendar. nil with a nil error when the
// payload is absent (organization profile, fresh user account).
func (p *Provider) fetchCommitCalendar(ctx context.Context) (*plugins.ContributionCalendar, error) {
	resp, err := p.gql.UserIndepth(ctx, p.login, nil, nil, defaultRepoBatch, nil)
	if err != nil {
		return nil, fmt.Errorf("dataprovider: indepth(%q): %w", p.login, err)
	}
	if resp == nil || resp.User == nil {
		return nil, nil
	}
	cc := resp.User.ContributionsCollection
	if cc == nil || cc.ContributionCalendar == nil {
		return nil, nil
	}
	cal := cc.ContributionCalendar
	return &plugins.ContributionCalendar{
		TotalContributions: cal.TotalContributions,
		Weeks:              weeksFromIndepth(cal.Weeks),
	}, nil
}

// isTransient mirrors base.isTransient so dataprovider can apply the
// same batch-halving heuristic without depending on the base package.
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
