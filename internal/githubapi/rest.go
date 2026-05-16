package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// DefaultRESTBaseURL is the production endpoint for the REST API. The
// constructor accepts overrides so tests can point the client at an
// httptest server.
const DefaultRESTBaseURL = "https://api.github.com"

// REST wraps the httpx.Client with GitHub REST helpers. It is safe for
// concurrent use; the underlying transport stays alive for the life of
// the process.
type REST struct {
	client    *httpx.Client
	baseURL   string
	tokenKind TokenKind
	header    http.Header

	// scopes cache populated lazily by (*REST).Scopes.
	scopesMu     sync.Mutex
	scopesCached bool
	scopes       []string
}

// NewREST constructs a REST client. The token kind is classified
// up-front so callers can branch on it; an empty / fine-grained / junk
// token returns the typed *InputError defined by [ValidateToken].
//
// When TokenKind == TokenMocked the transport is wrapped by
// [newMockedGuard] so requests to real GitHub hosts panic loudly
// (FR-017).
func NewREST(token config.Token, customBaseURL string, opts httpx.Options) (*REST, error) {
	if err := ValidateToken(token.Reveal()); err != nil {
		return nil, err
	}
	kind := ClassifyToken(token.Reveal())
	if kind == TokenMocked {
		opts.Transport = newMockedGuard(opts.Transport)
	}

	client := httpx.New(opts)

	base := customBaseURL
	if base == "" {
		base = DefaultRESTBaseURL
	}

	h := http.Header{}
	h.Set("Accept", "application/vnd.github+json")
	h.Set("X-GitHub-Api-Version", "2022-11-28")
	if kind == TokenClassic || kind == TokenMocked {
		h.Set("Authorization", "token "+token.Reveal())
	}

	return &REST{
		client:    client,
		baseURL:   base,
		tokenKind: kind,
		header:    h,
	}, nil
}

// TokenKind reports how the credential was classified.
func (r *REST) TokenKind() TokenKind { return r.tokenKind }

// HTTPClient exposes the underlying retry-enabled http.Client.
func (r *REST) HTTPClient() *http.Client { return r.client.HTTPClient() }

// BaseURL returns the configured base URL.
func (r *REST) BaseURL() string { return r.baseURL }

// RateLimitResponse mirrors the upstream /rate_limit payload (only the
// fields the project consumes).
type RateLimitResponse struct {
	Resources struct {
		Core    Quota `json:"core"`
		GraphQL Quota `json:"graphql"`
		Search  Quota `json:"search"`
	} `json:"resources"`
	Rate Quota `json:"rate"`
}

// Quota is the per-bucket rate counter; the JSON tags mirror upstream.
type Quota struct {
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// UnmarshalJSON accepts the integer epoch seconds that GitHub returns
// for "reset" and folds it into time.Time.
func (q *Quota) UnmarshalJSON(b []byte) error {
	type alias struct {
		Limit     int   `json:"limit"`
		Used      int   `json:"used"`
		Remaining int   `json:"remaining"`
		Reset     int64 `json:"reset"`
	}
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	q.Limit, q.Used, q.Remaining = a.Limit, a.Used, a.Remaining
	if a.Reset > 0 {
		q.Reset = time.Unix(a.Reset, 0).UTC()
	}
	return nil
}

// RateLimit queries /rate_limit and returns the decoded payload.
func (r *REST) RateLimit(ctx context.Context) (*RateLimitResponse, error) {
	body, resp, err := r.client.Get(ctx, r.baseURL+"/rate_limit", r.header.Clone())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rate_limit: status %d: %s", resp.StatusCode, string(body))
	}
	var out RateLimitResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("rate_limit: decode: %w", err)
	}
	return &out, nil
}

// HeadRoot inspects the root endpoint primarily to read X-OAuth-Scopes
// during token validation (T-108 in M6).
func (r *REST) HeadRoot(ctx context.Context) (*http.Response, error) {
	_, resp, err := r.client.Get(ctx, r.baseURL+"/", r.header.Clone())
	return resp, err
}
