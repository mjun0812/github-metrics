package githubapi

import (
	"context"
	"net/http"
	"time"

	"github.com/Khan/genqlient/graphql"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

//go:generate go run ../tools/gen-graphql

// DefaultGraphQLURL is the production GraphQL endpoint.
const DefaultGraphQLURL = "https://api.github.com/graphql"

// GraphQL wraps the genqlient client with the project's standard
// transport (retryable HTTP + MOCKED_TOKEN panic guard) and exposes
// the typed query methods that plugins consume.
type GraphQL struct {
	client    graphql.Client
	baseURL   string
	tokenKind TokenKind
}

// NewGraphQL constructs the GraphQL client. The token is validated and
// classified up front; for TokenMocked the transport is wrapped in
// [newMockedGuard] so any request that escapes to a real GitHub host
// fails loudly (FR-017).
func NewGraphQL(token config.Token, customBaseURL string, opts httpx.Options) (*GraphQL, error) {
	if err := ValidateToken(token.Reveal()); err != nil {
		return nil, err
	}
	kind := ClassifyToken(token.Reveal())
	if kind == TokenMocked {
		opts.Transport = newMockedGuard(opts.Transport)
	}

	base := customBaseURL
	if base == "" {
		base = DefaultGraphQLURL
	}

	// Wrap the project's retryable http.Client with a header-injecting
	// RoundTripper so every outbound GraphQL request gets the standard
	// Authorization and Accept headers without callers having to know.
	inner := httpx.New(opts).HTTPClient()
	authedClient := &http.Client{
		Transport: &graphqlAuthTransport{
			inner: inner.Transport,
			token: token,
			kind:  kind,
		},
		Timeout: inner.Timeout,
	}

	// Wrap the stock genqlient client in emptyDataGuardClient so a
	// GitHub reply of `{"data": null}` (secondary rate limit / abuse
	// heuristic) surfaces as ErrEmptyGraphQLResponse instead of being
	// swallowed as a zero-valued success (#732).
	return &GraphQL{
		client:    newEmptyDataGuardClient(graphql.NewClient(base, authedClient)),
		baseURL:   base,
		tokenKind: kind,
	}, nil
}

// Client exposes the underlying genqlient client.
func (g *GraphQL) Client() graphql.Client { return g.client }

// BaseURL returns the configured GraphQL endpoint.
func (g *GraphQL) BaseURL() string { return g.baseURL }

// TokenKind reports how the credential classified.
func (g *GraphQL) TokenKind() TokenKind { return g.tokenKind }

// User fetches the typed [UserUser] payload by login.
func (g *GraphQL) User(ctx context.Context, login string) (*UserResponse, error) {
	return User(ctx, g.client, login)
}

// Organization fetches the typed [OrganizationOrganization] payload by login.
func (g *GraphQL) Organization(ctx context.Context, login string) (*OrganizationResponse, error) {
	return Organization(ctx, g.client, login)
}

// Repository fetches the typed [RepositoryRepository] payload for a
// single owner/name pair. Used by the M7 base plugin's repository
// fetch path.
func (g *GraphQL) Repository(ctx context.Context, login, repo string) (*RepositoryResponse, error) {
	return Repository(ctx, g.client, login, repo)
}

// UserRepositories returns up to `first` owner-affiliated repositories
// for the given login, starting after the `after` cursor (or from the
// beginning when after is nil). Callers thread the
// `pageInfo.endCursor` from one response into the next to walk the
// entire connection; the base plugin's repositories.go does so with a
// batch-halving retry strategy that mirrors upstream.
func (g *GraphQL) UserRepositories(ctx context.Context, login string, first int, after *string) (*UserRepositoriesResponse, error) {
	return UserRepositories(ctx, g.client, login, first, after)
}

// OrganizationRepositories is the organization-side equivalent of
// [UserRepositories]. The `after` cursor is threaded for paging the
// same way.
func (g *GraphQL) OrganizationRepositories(ctx context.Context, login string, first int, after *string) (*OrganizationRepositoriesResponse, error) {
	return OrganizationRepositories(ctx, g.client, login, first, after)
}

// OrganizationMembers fetches a page of an organization's members. The
// `after` cursor enables paging.
func (g *GraphQL) OrganizationMembers(ctx context.Context, login string, first int, after *string) (*OrganizationMembersResponse, error) {
	return OrganizationMembers(ctx, g.client, login, first, after)
}

// UserContributionCommits fetches the trailing-year commit-contribution
// total. Split into its own query operation because GitHub's per-request
// resource limit on the contributionsCollection subtree rejects two or
// more aggregate fields in a single request.
func (g *GraphQL) UserContributionCommits(ctx context.Context, login string) (*UserContributionCommitsResponse, error) {
	return UserContributionCommits(ctx, g.client, login)
}

// UserContributionPullRequestReviews fetches the trailing-year
// pull-request-review-contribution total. Isolated for the same reason
// as [GraphQL.UserContributionCommits].
func (g *GraphQL) UserContributionPullRequestReviews(ctx context.Context, login string) (*UserContributionPullRequestReviewsResponse, error) {
	return UserContributionPullRequestReviews(ctx, g.client, login)
}

// UserContributionPullRequests fetches the trailing-year
// pull-request-contribution total. Isolated for the same reason as
// [GraphQL.UserContributionCommits].
func (g *GraphQL) UserContributionPullRequests(ctx context.Context, login string) (*UserContributionPullRequestsResponse, error) {
	return UserContributionPullRequests(ctx, g.client, login)
}

// UserContributionIssues fetches the trailing-year issue-contribution
// total. Isolated for the same reason as
// [GraphQL.UserContributionCommits].
func (g *GraphQL) UserContributionIssues(ctx context.Context, login string) (*UserContributionIssuesResponse, error) {
	return UserContributionIssues(ctx, g.client, login)
}

// UserRepositoriesContributedTo fetches the "contributed to N
// repositories" counter. Isolated into its own request because on
// high-activity accounts this single field also trips GitHub's
// per-request resource limit; callers treat a failure as a hidden
// counter rather than a profile-wide error.
func (g *GraphQL) UserRepositoriesContributedTo(ctx context.Context, login string) (*UserRepositoriesContributedToResponse, error) {
	return UserRepositoriesContributedTo(ctx, g.client, login)
}

// UserIsocalendar fetches the contribution calendar for an explicit
// from/to window. The isocalendar plugin issues it in 4-week chunks
// (mirroring upstream) so GitHub normalizes each chunk's heatmap colors
// against the chunk-local maximum instead of the whole-year maximum
// (see #467).
func (g *GraphQL) UserIsocalendar(ctx context.Context, login string, from, to time.Time) (*UserIsocalendarResponse, error) {
	return UserIsocalendar(ctx, g.client, login, from, to)
}

// UserCommitContributions fetches the commit-contribution total for an
// explicit from/to window. The base plugin calls it once per account
// year to reproduce upstream's lifetime Activity commit total.
func (g *GraphQL) UserCommitContributions(ctx context.Context, login string, from, to time.Time) (*UserCommitContributionsResponse, error) {
	return UserCommitContributions(ctx, g.client, login, from, to)
}

// UserStarredRepositories fetches the user's most-recently starred
// repositories (most-recent first). Consumed by the "stars" plugin.
func (g *GraphQL) UserStarredRepositories(ctx context.Context, login string, first int) (*UserStarredRepositoriesResponse, error) {
	return UserStarredRepositories(ctx, g.client, login, first)
}

// UserReactions fetches the per-emoji reaction content (and totalCount)
// across the user's issues and issue comments. Consumed by the
// "reactions" plugin. The reactions connection is paginated with
// last: 100 because GitHub rejects a bare nodes selection on a
// connection without a first/last argument (see #472).
func (g *GraphQL) UserReactions(ctx context.Context, login string, issuesFirst, commentsFirst int) (*UserReactionsResponse, error) {
	return UserReactions(ctx, g.client, login, issuesFirst, commentsFirst)
}

// UserFollowers fetches the user's followers + following pages.
// Consumed by the "people" plugin. size bounds the fetched avatar
// resolution via avatarUrl(size:) so embedded base64 stays small.
func (g *GraphQL) UserFollowers(ctx context.Context, login string, first, size int) (*UserFollowersResponse, error) {
	return UserFollowers(ctx, g.client, login, first, size)
}

// ViewerSponsors fetches sponsorshipsAsMaintainer (active + past) and
// sponsorsListing.activeGoal. Consumed by the "sponsors" plugin (spec 013).
func (g *GraphQL) ViewerSponsors(ctx context.Context, activeFirst, pastFirst int) (*ViewerSponsorsResponse, error) {
	return ViewerSponsors(ctx, g.client, activeFirst, pastFirst)
}

// ViewerSponsorships fetches sponsorshipsAsSponsor (the maintainers
// the viewer is currently sponsoring). Consumed by the "sponsorships"
// plugin (spec 013).
func (g *GraphQL) ViewerSponsorships(ctx context.Context, first int) (*ViewerSponsorshipsResponse, error) {
	return ViewerSponsorships(ctx, g.client, first)
}

// UserNotable fetches the repositories a user contributed to on other
// users'/organizations' accounts, ordered by stargazers, for the
// "notable" plugin (issue #447). `types` selects the contribution kinds
// (commit / pull_request / issue) and `self` (upstream
// plugin_notable_self, default no) controls whether the user's own
// repositories are included.
func (g *GraphQL) UserNotable(ctx context.Context, login string, first int, after *string, types []RepositoryContributionType, self *bool) (*UserNotableResponse, error) {
	return UserNotable(ctx, g.client, login, first, after, types, self)
}

// ViewerStargazersRepos fetches the viewer's top repos and their
// stargazers (latest N) for the "stargazers" plugin (spec 013).
func (g *GraphQL) ViewerStargazersRepos(ctx context.Context, repoFirst, starFirst int) (*ViewerStargazersReposResponse, error) {
	return ViewerStargazersRepos(ctx, g.client, repoFirst, starFirst)
}

// ViewerPinnedItems fetches the viewer's pinned repositories for the
// "repositories" plugin's Pinned slot (spec 013).
func (g *GraphQL) ViewerPinnedItems(ctx context.Context, first int) (*ViewerPinnedItemsResponse, error) {
	return ViewerPinnedItems(ctx, g.client, first)
}

// UserLists fetches the user's starlists (user.lists) and their items
// in a single round-trip. Consumed by the "starlists" plugin (spec 014).
func (g *GraphQL) UserLists(ctx context.Context, login string, listsFirst, itemsFirst int) (*UserListsResponse, error) {
	return UserLists(ctx, g.client, login, listsFirst, itemsFirst)
}

// graphqlAuthTransport adds the Authorization and Accept headers that
// every GitHub GraphQL request needs. inner may be nil, in which case
// http.DefaultTransport is used.
type graphqlAuthTransport struct {
	inner http.RoundTripper
	token config.Token
	kind  TokenKind
}

// RoundTrip implements http.RoundTripper.
func (t *graphqlAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = http.Header{}
	}
	if req2.Header.Get("Accept") == "" {
		req2.Header.Set("Accept", "application/vnd.github+json")
	}
	// GitHub rejects requests carrying the Go default User-Agent
	// ("Go-http-client/1.1") as bot/abuse traffic with a 403 (no GraphQL
	// errors body, just {"data":null}). The REST client sets a real UA
	// via httpx, but genqlient's GraphQL transport otherwise sends none —
	// so every base/plugin GraphQL call went out unidentified and got
	// blocked. Set the project UA here to match the REST client.
	if req2.Header.Get("User-Agent") == "" {
		req2.Header.Set("User-Agent", httpx.DefaultUserAgent)
	}
	if t.kind == TokenClassic || t.kind == TokenMocked {
		if req2.Header.Get("Authorization") == "" {
			req2.Header.Set("Authorization", "bearer "+t.token.Reveal())
		}
	}
	inner := t.inner
	if inner == nil {
		inner = http.DefaultTransport
	}
	return inner.RoundTrip(req2)
}
