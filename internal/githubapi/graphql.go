package githubapi

import (
	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// GraphQL is the typed wrapper around the github-metrics GraphQL
// operations. The M1 build ships the constructor and the MOCKED_TOKEN
// guard wiring so other packages (plugins, engine) can hold a *GraphQL
// reference in their structs; the generated query methods themselves
// land with task T036 (genqlient setup) in a follow-up PR.
type GraphQL struct {
	client    *httpx.Client
	baseURL   string
	tokenKind TokenKind
}

// DefaultGraphQLURL is the production GraphQL endpoint.
const DefaultGraphQLURL = "https://api.github.com/graphql"

// NewGraphQL constructs the GraphQL client. Like NewREST it classifies
// the token, wraps the transport in the MOCKED_TOKEN panic guard, and
// installs the standard User-Agent. Query helpers are added in T036.
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
	return &GraphQL{
		client:    httpx.New(opts),
		baseURL:   base,
		tokenKind: kind,
	}, nil
}

// BaseURL returns the GraphQL endpoint URL.
func (g *GraphQL) BaseURL() string { return g.baseURL }

// TokenKind reports how the credential classified.
func (g *GraphQL) TokenKind() TokenKind { return g.tokenKind }

// HTTPClient exposes the underlying retry-enabled http.Client; once
// genqlient lands this is what is passed to graphql.NewClient.
func (g *GraphQL) HTTPClient() interface{} { return g.client.HTTPClient() }
