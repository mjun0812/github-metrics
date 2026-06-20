package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Get issues an authenticated GET to path (which MUST start with "/")
// and returns the raw body + response. The default GitHub Accept /
// Authorization headers attached at construction time are sent on every
// request; callers can layer additional headers via the second
// argument.
//
// Used by plugins that need an arbitrary REST endpoint that hasn't
// earned a dedicated typed helper yet (activity events, habits commit
// diffs, traffic counters, ...). On non-2xx status the response is
// still returned alongside the body so callers can inspect status code
// + headers; a transport-level error returns (nil, nil, err).
func (r *REST) Get(ctx context.Context, path string, extra http.Header) ([]byte, *http.Response, error) {
	h := r.header.Clone()
	for k, vs := range extra {
		h[k] = append(h[k][:0:0], vs...)
	}
	return r.client.Get(ctx, r.baseURL+path, h)
}

// Put issues an authenticated PUT with the given JSON body. Mirrors
// Get's response surface (body + http.Response + transport error).
// Used by the M6 committer for `PUT /repos/.../contents/<filename>`
// (content commits) and `PUT /git/refs` (branch creation).
func (r *REST) Put(ctx context.Context, path string, body []byte, extra http.Header) ([]byte, *http.Response, error) {
	return r.doBody(ctx, http.MethodPut, path, body, extra)
}

// Post issues an authenticated POST with the given JSON body. Used by
// the M6 committer for `POST /repos/.../pulls` (PR creation, Phase 4).
func (r *REST) Post(ctx context.Context, path string, body []byte, extra http.Header) ([]byte, *http.Response, error) {
	return r.doBody(ctx, http.MethodPost, path, body, extra)
}

// doBody is the shared helper for non-GET requests. Builds the
// http.Request via the underlying *http.Client so we bypass the
// retryablehttp body re-read mechanic — committer-class operations
// are not idempotent and the M6 RetryPolicy (action package) handles
// retry classification at a higher level.
//
// The response body is drained and closed before returning so callers
// can inspect status / headers without taking ownership of the
// connection. The body bytes are returned for callers that need the
// JSON envelope (mirrors Get's contract).
func (r *REST) doBody(ctx context.Context, method, path string, body []byte, extra http.Header) ([]byte, *http.Response, error) {
	h := r.header.Clone()
	for k, vs := range extra {
		h[k] = append(h[k][:0:0], vs...)
	}
	if body != nil && h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json")
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, rdr)
	if err != nil {
		return nil, nil, err
	}
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := r.client.HTTPClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp, readErr
	}
	return respBody, resp, nil
}
