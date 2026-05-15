package githubapi

import (
	"context"
	"net/http"

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

// UserRepositories returns the first `first` owner-affiliated
// repositories for the given login. M1 fetches a single page (the full
// upstream paging loop with cursor-driven traversal lands with the
// M4 plugin work that actually consumes more than the totalCount).
func (g *GraphQL) UserRepositories(ctx context.Context, login string, first int) (*UserRepositoriesResponse, error) {
	return UserRepositories(ctx, g.client, login, first)
}

// OrganizationRepositories is the organization-side equivalent of
// [UserRepositories].
func (g *GraphQL) OrganizationRepositories(ctx context.Context, login string, first int) (*OrganizationRepositoriesResponse, error) {
	return OrganizationRepositories(ctx, g.client, login, first)
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
