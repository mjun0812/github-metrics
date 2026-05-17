package githubapi

import (
	"context"
	"net/http"
	"time"

	"github.com/Khan/genqlient/graphql"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

//go:generate go run ../../tools/gen-graphql

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

	return &GraphQL{
		client:    graphql.NewClient(base, authedClient),
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
// fetch path. See specs/006-m7-repository-template/contracts/base-repository-query.md.
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

// UserIndepth issues the "indepth" GraphQL query that augments the base
// payload with per-repository commit/issue/PR totals and the user's
// contribution calendar. Triggered only when at least one indepth-
// dependent plugin is enabled (see specs/004-m4-github-plugins/
// contracts/plugin-base-extension.md §2.1).
func (g *GraphQL) UserIndepth(ctx context.Context, login string, from, to *time.Time, reposFirst int, reposAfter *string) (*UserIndepthResponse, error) {
	return UserIndepth(ctx, g.client, login, from, to, reposFirst, reposAfter)
}

// UserStarredRepositories fetches the user's most-recently starred
// repositories (most-recent first). Consumed by the "stars" plugin.
func (g *GraphQL) UserStarredRepositories(ctx context.Context, login string, first int) (*UserStarredRepositoriesResponse, error) {
	return UserStarredRepositories(ctx, g.client, login, first)
}

// UserReactions aggregates reaction totalCount across the user's
// issues and issue comments. Consumed by the "reactions" plugin.
func (g *GraphQL) UserReactions(ctx context.Context, login string, issuesFirst, commentsFirst int) (*UserReactionsResponse, error) {
	return UserReactions(ctx, g.client, login, issuesFirst, commentsFirst)
}

// UserFollowers fetches the user's followers + following pages.
// Consumed by the "people" plugin.
func (g *GraphQL) UserFollowers(ctx context.Context, login string, first int) (*UserFollowersResponse, error) {
	return UserFollowers(ctx, g.client, login, first)
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
