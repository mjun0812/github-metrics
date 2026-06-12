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
	"strconv"
	"strings"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// rateLimitBackoffCap is the maximum wait we will honor from a Retry-After
// or x-ratelimit-reset header. When the indicated wait exceeds this cap,
// checkRetry returns false (no more retries) and the ErrorHandler wraps the
// final response as a *RateLimitedError so callers can use errors.As.
const rateLimitBackoffCap = 2 * time.Minute

// errRateLimitBeyondCap is the sentinel error returned by checkRetry when a
// rate-limit response indicates a wait beyond rateLimitBackoffCap. It causes
// retryablehttp to skip the "success" short-circuit and invoke ErrorHandler
// with the final response, so rateLimitErrorHandler can wrap it as
// *RateLimitedError. It is not surfaced to callers.
var errRateLimitBeyondCap = fmt.Errorf("httpx: rate-limit wait exceeds cap")

// RateLimitKind describes how GitHub signalled the rate limit.
type RateLimitKind int

const (
	// RateLimitSecondary indicates a secondary/abuse limit: the server
	// responded with a Retry-After header (GitHub docs: "secondary rate limits").
	RateLimitSecondary RateLimitKind = iota
	// RateLimitPrimary indicates primary quota exhaustion: the server set
	// x-ratelimit-remaining to "0" and x-ratelimit-reset to a reset epoch.
	RateLimitPrimary
)

// RateLimitedError is returned (or wrapped) when all retry attempts are
// exhausted due to a GitHub rate-limit response, or when the requested
// wait exceeds rateLimitBackoffCap.
//
// Callers that want to distinguish "rate limited" from "forbidden (no
// access)" can use errors.As:
//
//	var rle *httpx.RateLimitedError
//	if errors.As(err, &rle) { /* rate limited */ }
type RateLimitedError struct {
	// Kind is whether this was a secondary (Retry-After) or primary
	// (x-ratelimit-remaining=0) limit.
	Kind RateLimitKind
	// RetryAt is when GitHub says the limit resets. It may be zero if the
	// server did not supply a parseable reset time.
	RetryAt time.Time
}

func (e *RateLimitedError) Error() string {
	var kind string
	switch e.Kind {
	case RateLimitSecondary:
		kind = "secondary"
	default:
		kind = "primary"
	}
	if e.RetryAt.IsZero() {
		return fmt.Sprintf("httpx: GitHub %s rate limit exceeded", kind)
	}
	return fmt.Sprintf("httpx: GitHub %s rate limit exceeded, retry after %s", kind, e.RetryAt.UTC().Format(time.RFC3339))
}

// ClassifyRateLimit inspects a non-2xx *http.Response (typically 403 or 429)
// and returns a *RateLimitedError if the response carries rate-limit headers,
// or nil if the response is a plain permission error / unrelated failure.
//
// #531 should call this after receiving a 403 from a REST endpoint:
//
//	var rle *httpx.RateLimitedError
//	if rle = httpx.ClassifyRateLimit(resp); rle != nil {
//	    // rate limited — surface rle to caller
//	}
func ClassifyRateLimit(resp *http.Response) *RateLimitedError {
	if resp == nil {
		return nil
	}
	// Secondary limit: Retry-After header present.
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		retryAt := parseRetryAfter(ra)
		return &RateLimitedError{Kind: RateLimitSecondary, RetryAt: retryAt}
	}
	// Primary limit: x-ratelimit-remaining == "0".
	if resp.Header.Get("X-Ratelimit-Remaining") == "0" {
		var retryAt time.Time
		if reset := resp.Header.Get("X-Ratelimit-Reset"); reset != "" {
			if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil && epoch > 0 {
				retryAt = time.Unix(epoch, 0).UTC()
			}
		}
		return &RateLimitedError{Kind: RateLimitPrimary, RetryAt: retryAt}
	}
	return nil
}

// parseRetryAfter parses a Retry-After header value and returns the
// absolute time at which the client may retry. Returns zero time on parse
// failure.
func parseRetryAfter(header string) time.Time {
	if header == "" {
		return time.Time{}
	}
	// Retry-After: <seconds>
	if secs, err := strconv.ParseInt(header, 10, 64); err == nil && secs >= 0 {
		return time.Now().Add(time.Duration(secs) * time.Second)
	}
	// Retry-After: <HTTP-date>
	if t, err := time.Parse(time.RFC1123, header); err == nil {
		return t
	}
	return time.Time{}
}

// rateLimitWait returns the wait duration indicated by rate-limit response
// headers and whether the response is at all rate-limited. It is the shared
// source of truth used by both checkRetry and rateLimitAwareBackoff so the
// retry decision and the sleep duration are always consistent.
//
// ok is true when resp carries rate-limit headers (Retry-After or
// X-Ratelimit-Remaining=0); wait is the computed duration to sleep (may be
// zero if the reset is already in the past or the header is unparseable).
func rateLimitWait(resp *http.Response) (wait time.Duration, ok bool) {
	if resp == nil {
		return 0, false
	}
	// Secondary limit: Retry-After header.
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		retryAt := parseRetryAfter(ra)
		if retryAt.IsZero() {
			// Unparseable header: treat as rate-limited but unknown wait.
			// checkRetry will retry with default backoff; backoff returns 0
			// which retryablehttp clamps to RetryWaitMin.
			return 0, true
		}
		return time.Until(retryAt), true
	}
	// Primary limit: x-ratelimit-remaining == "0".
	if resp.Header.Get("X-Ratelimit-Remaining") == "0" {
		if reset := resp.Header.Get("X-Ratelimit-Reset"); reset != "" {
			if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil && epoch > 0 {
				return time.Until(time.Unix(epoch, 0)), true
			}
		}
		return 0, true
	}
	return 0, false
}

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
	// request. Default 4 (so total = 5 attempts). Ignored when
	// DisableRetries is true.
	MaxRetries int
	// MinBackoff is the floor for the exponential backoff. Default 1s.
	MinBackoff time.Duration
	// MaxBackoff is the ceiling for the exponential backoff.
	// Default 30s.
	MaxBackoff time.Duration
	// DisableRetries turns off the inner retryablehttp loop entirely.
	// Callers that own retry classification at a higher layer (M6
	// action package's RetryPolicy) MUST set this true to avoid the
	// double-retry tower (1+MaxRetries)·(1+action.Retries) attempts,
	// which is especially harmful for non-idempotent POST /pulls and
	// POST /git/refs (would create duplicate PRs / refs on a 5xx after
	// a successful server-side write).
	DisableRetries bool
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
	if opts.DisableRetries {
		maxRetries = 0
	} else if maxRetries <= 0 {
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
	rc.Backoff = rateLimitAwareBackoff
	rc.ErrorHandler = rateLimitErrorHandler
	if opts.Transport != nil {
		rc.HTTPClient.Transport = opts.Transport
	}

	return &Client{
		inner:     rc,
		userAgent: opts.UserAgent,
		logger:    opts.Logger,
	}
}

// rateLimitErrorHandler is the retryablehttp.ErrorHandler set on every Client.
// It is called once after all retry attempts are exhausted (or after checkRetry
// returns false) with the final response still open.
//
// When the final response carries rate-limit headers, rateLimitErrorHandler
// drains and closes the body, then returns a wrapped *RateLimitedError so
// callers can use errors.As. Otherwise it reproduces the library's default
// behavior: drain+close body and return the same "giving up after N attempt(s)"
// message retryablehttp would have produced.
func rateLimitErrorHandler(resp *http.Response, err error, numTries int) (*http.Response, error) {
	// Always drain and close the body to avoid connection leaks, regardless of path.
	if resp != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}

	if rle := ClassifyRateLimit(resp); rle != nil {
		// Determine method and redacted URL from the wrapped error text when
		// resp itself does not carry them (retryablehttp does not expose the
		// request here). We reconstruct a compatible message shape.
		var prefix string
		if resp != nil && resp.Request != nil {
			u := resp.Request.URL
			if u != nil {
				ru := *u
				if _, has := ru.User.Password(); has {
					ru.User = url.UserPassword(ru.User.Username(), "xxxxx")
				}
				prefix = resp.Request.Method + " " + ru.String() + " "
			}
		}
		return nil, fmt.Errorf("%sgiving up after %d attempt(s): %w", prefix, numTries, rle)
	}

	// Default behavior: mirror retryablehttp's own tail (client.go:836-842).
	if err == nil {
		return nil, fmt.Errorf("giving up after %d attempt(s)", numTries)
	}
	return nil, fmt.Errorf("giving up after %d attempt(s): %w", numTries, err)
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
	defer func() { _ = resp.Body.Close() }()
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

// checkRetry implements the project's retry policy:
//   - 5xx: always retry (transient).
//   - 429 with Retry-After within the cap: retry honoring the header.
//   - 429 with Retry-After beyond the cap: NOT retried → ErrorHandler returns *RateLimitedError.
//   - 429 without rate-limit headers: retry with default backoff.
//   - 403 with rate-limit headers and wait within cap: retry.
//   - 403 with rate-limit headers and wait beyond cap: NOT retried → *RateLimitedError.
//   - 403 without rate-limit headers (permission error): NOT retried.
//   - Other 4xx: not retried.
//   - Context cancellation/timeout: stop immediately.
//
// The shared rateLimitWait helper ensures checkRetry and rateLimitAwareBackoff
// always agree on whether a given response's wait is within the cap.
func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		// retryablehttp's CheckRetry contract: returning (true, nil) means
		// "retry; do not surface this error". Returning the underlying err
		// here would short-circuit retries on every transient network
		// blip, defeating the purpose of the wrapper. nilerr flags this
		// pattern as a generic false positive.
		return true, nil //nolint:nilerr // intentional per retryablehttp.CheckRetry contract
	}
	if resp == nil {
		return true, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		wait, isRL := rateLimitWait(resp)
		if isRL {
			// Beyond the cap → stop retrying. Return a sentinel error so
			// retryablehttp hands the final response to ErrorHandler, which
			// will wrap it as *RateLimitedError for errors.As callers.
			if wait > rateLimitBackoffCap {
				return false, errRateLimitBeyondCap
			}
			// Within the cap (or unknown/past wait): retry.
			return true, nil
		}
		// 403 without rate-limit headers is a plain permission error: not retried.
		if resp.StatusCode == http.StatusForbidden {
			return false, nil
		}
		// 429 without rate-limit headers: retry with default backoff.
		return true, nil
	}
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return true, nil
	}
	return false, nil
}

// rateLimitAwareBackoff is the project's Backoff function. It extends
// retryablehttp.DefaultBackoff to honor Retry-After and x-ratelimit-reset on
// 403 and 429 responses (GitHub rate-limit signals).
//
// It uses the same rateLimitWait helper as checkRetry, so the sleep duration
// is always consistent with the retry decision: if checkRetry allowed a retry
// (wait ≤ rateLimitBackoffCap), this function returns that wait clamped to
// max. For all other responses (5xx, network errors, 429 without headers)
// it delegates to retryablehttp.DefaultBackoff, which also handles
// Retry-After on 429/503 responses.
func rateLimitAwareBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) {
		if wait, ok := rateLimitWait(resp); ok {
			// checkRetry already ensured wait ≤ rateLimitBackoffCap (it returns
			// false otherwise), so we just clamp to [0, max].
			if wait > max {
				return max
			}
			if wait > 0 {
				return wait
			}
			// wait == 0: reset already passed or unknown; use default backoff.
		}
	}
	// Fall back to retryablehttp.DefaultBackoff for 5xx/network errors and
	// 429/503 without our rate-limit headers (DefaultBackoff also honors
	// Retry-After on 429/503, preserving that existing behavior).
	return retryablehttp.DefaultBackoff(min, max, attemptNum, resp)
}

// slogAdapter bridges retryablehttp's Logger interface onto slog.
type slogAdapter struct {
	logger *slog.Logger
}

func (a *slogAdapter) Printf(format string, v ...any) {
	a.logger.Debug(fmt.Sprintf(format, v...))
}
