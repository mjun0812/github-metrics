// Package httpx wraps net/http with retry, structured logging, and
// the User-Agent string the project uses for every outbound request.
// It is the foundation for github-metrics' GitHub REST and GraphQL
// clients (internal/githubapi) and for any other external fetcher
// (twemoji, octicons, gemoji) that lands in later phases.
package httpx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// DefaultUserAgent is used when Options.UserAgent is empty. Callers
// (notably the github-metrics action binary) typically override it with
// a more specific string at start-up.
const DefaultUserAgent = "github-metrics/0 (https://github.com/mjun0812/github-metrics)"

// Client is the thin wrapper around retryablehttp that the rest of the
// project consumes. Construct one per process (or per binary mode);
// safe for concurrent use.
type Client struct {
	inner     *retryablehttp.Client
	userAgent string
	logger    *slog.Logger
}

// Options controls Client construction. Zero values are safe; see field
// docs for the defaults each one falls back to.
type Options struct {
	// UserAgent overrides DefaultUserAgent.
	UserAgent string
	// Logger receives retry/backoff messages. nil falls back to a
	// no-op slog handler so production binaries can route logs
	// explicitly.
	Logger *slog.Logger
	// Transport is the underlying RoundTripper. nil uses
	// retryablehttp's default (which wraps http.DefaultTransport).
	// internal/githubapi swaps this for mock transports during tests.
	Transport http.RoundTripper
	// MaxRetries caps the number of retry attempts after the initial
	// request. Default 4 (so total = 5 attempts).
	MaxRetries int
	// MinBackoff is the floor for the exponential backoff. Default 1s.
	MinBackoff time.Duration
	// MaxBackoff is the ceiling for the exponential backoff.
	// Default 30s.
	MaxBackoff time.Duration
}

// New constructs a Client using opts; zero-value Options is a valid
// production default for a tool that talks to small handful of HTTP
// endpoints (mainly api.github.com plus a few asset CDNs).
func New(opts Options) *Client {
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 4
	}
	minBackoff := opts.MinBackoff
	if minBackoff <= 0 {
		minBackoff = time.Second
	}
	maxBackoff := opts.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = maxRetries
	rc.RetryWaitMin = minBackoff
	rc.RetryWaitMax = maxBackoff
	rc.Logger = &slogAdapter{logger: opts.Logger}
	rc.CheckRetry = checkRetry
	rc.Backoff = retryablehttp.DefaultBackoff
	if opts.Transport != nil {
		rc.HTTPClient.Transport = opts.Transport
	}

	return &Client{
		inner:     rc,
		userAgent: opts.UserAgent,
		logger:    opts.Logger,
	}
}

// HTTPClient returns the underlying *http.Client. Exposed so callers
// that need a stdlib client (e.g. genqlient.NewClient) can hand it the
// retry-enabled instance without re-implementing transport plumbing.
func (c *Client) HTTPClient() *http.Client {
	return c.inner.StandardClient()
}

// Get issues a GET request. Header is merged onto User-Agent (caller
// wins for duplicate keys).
func (c *Client) Get(ctx context.Context, target string, header http.Header) ([]byte, *http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, target, nil, header)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}

// PostJSON marshals body as application/json and POSTs it.
func (c *Client) PostJSON(ctx context.Context, target string, body any, header http.Header) ([]byte, *http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("httpx: marshal body: %w", err)
	}
	h := cloneOrNew(header)
	h.Set("Content-Type", "application/json")
	req, err := c.newRequest(ctx, http.MethodPost, target, bytes.NewReader(buf), h)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}

// PostForm POSTs application/x-www-form-urlencoded data.
func (c *Client) PostForm(ctx context.Context, target string, values url.Values) ([]byte, *http.Response, error) {
	body := strings.NewReader(values.Encode())
	h := http.Header{}
	h.Set("Content-Type", "application/x-www-form-urlencoded")
	req, err := c.newRequest(ctx, http.MethodPost, target, body, h)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}

// Binary fetches an arbitrary blob, returning the body bytes and the
// reported MIME (Content-Type with parameters stripped).
func (c *Client) Binary(ctx context.Context, target string) ([]byte, string, error) {
	body, resp, err := c.Get(ctx, target, nil)
	if err != nil {
		return nil, "", err
	}
	mime := resp.Header.Get("Content-Type")
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	return body, mime, nil
}

// ImgB64 fetches an image and returns a `data:<mime>;base64,<payload>`
// string suitable for embedding directly in SVG/HTML.
func (c *Client) ImgB64(ctx context.Context, target string) (string, error) {
	body, mime, err := c.Binary(ctx, target)
	if err != nil {
		return "", err
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

// newRequest builds a retryablehttp.Request and applies the standard
// User-Agent. Caller-supplied header values take precedence so test
// helpers can override the UA without subclassing the client.
func (c *Client) newRequest(
	ctx context.Context,
	method, target string,
	body io.Reader,
	header http.Header,
) (*retryablehttp.Request, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("httpx: %s %s: %w", method, target, err)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	for k, vs := range header {
		req.Header[k] = append(req.Header[k][:0:0], vs...)
	}
	return req, nil
}

// do executes the request and reads the body fully so callers don't
// have to manage the underlying connection.
func (c *Client) do(req *retryablehttp.Request) ([]byte, *http.Response, error) {
	resp, err := c.inner.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("httpx: %s %s: %w", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, fmt.Errorf("httpx: read body: %w", err)
	}
	return body, resp, nil
}

func cloneOrNew(h http.Header) http.Header {
	if h == nil {
		return http.Header{}
	}
	return h.Clone()
}

// checkRetry implements the project's retry policy: 5xx and 429 are
// transient, 4xx (other than 429) is not, context cancellation/timeout
// stops retrying immediately. retryablehttp's DefaultRetryPolicy is
// close but treats 429 as definitive and ignores Retry-After — we
// explicitly opt into both behaviors.
func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return true, nil
	}
	if resp == nil {
		return true, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return true, nil
	}
	return false, nil
}

// slogAdapter bridges retryablehttp's Logger interface onto slog.
type slogAdapter struct {
	logger *slog.Logger
}

func (a *slogAdapter) Printf(format string, v ...any) {
	a.logger.Debug(fmt.Sprintf(format, v...))
}
