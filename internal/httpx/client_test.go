package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/httpx"
)

func newClient(t *testing.T, transport http.RoundTripper) *httpx.Client {
	t.Helper()
	return httpx.New(httpx.Options{
		Transport:  transport,
		MaxRetries: 4,
		MinBackoff: time.Millisecond, // keep tests fast
		MaxBackoff: 5 * time.Millisecond,
	})
}

func TestClient_GetRetries5xxThenSucceeds(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			http.Error(w, "boom", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	c := newClient(t, nil)
	body, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 calls (2 fail + 1 success), got %d", got)
	}
}

func TestClient_GetDoesNotRetry4xx(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	c := newClient(t, nil)
	_, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 call (no retry on 404), got %d", got)
	}
}

func TestClient_Retries429(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	c := newClient(t, nil)
	_, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestClient_SendsDefaultUserAgent(t *testing.T) {
	t.Parallel()

	var seenUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpx.New(httpx.Options{}) // bare defaults
	_, _, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := seenUA.Load().(string)
	if got == "" {
		t.Fatalf("no User-Agent observed")
	}
	if !strings.Contains(got, "github-metrics") {
		t.Fatalf("User-Agent = %q, want substring 'github-metrics'", got)
	}
}

func TestClient_CallerOverridesUserAgent(t *testing.T) {
	t.Parallel()

	var seenUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(t, nil)
	h := http.Header{}
	h.Set("User-Agent", "custom/1.2")
	_, _, err := c.Get(context.Background(), srv.URL, h)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := seenUA.Load().(string)
	if got != "custom/1.2" {
		t.Fatalf("UA = %q, want %q", got, "custom/1.2")
	}
}

func TestClient_ContextCancelStopsRetries(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newClient(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, _, err := c.Get(ctx, srv.URL, nil)
	if err == nil {
		t.Fatalf("expected error from cancelled context")
	}
}

func TestClient_PostJSON(t *testing.T) {
	t.Parallel()

	type echo struct {
		Echoed map[string]string `json:"echoed"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "wrong content type", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"echoed":%s}`, body)
	}))
	defer srv.Close()

	c := newClient(t, nil)
	body, _, err := c.PostJSON(context.Background(), srv.URL, map[string]string{"msg": "hi"}, nil)
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	var got echo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Echoed["msg"] != "hi" {
		t.Fatalf("echoed = %v", got.Echoed)
	}
}

func TestClient_PostForm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		_, _ = fmt.Fprint(w, r.Form.Get("hello"))
	}))
	defer srv.Close()

	c := newClient(t, nil)
	body, _, err := c.PostForm(context.Background(), srv.URL, url.Values{"hello": {"world"}})
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	if string(body) != "world" {
		t.Fatalf("body = %q", body)
	}
}

func TestClient_Binary(t *testing.T) {
	t.Parallel()

	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=utf-8")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := newClient(t, nil)
	body, mime, err := c.Binary(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Binary: %v", err)
	}
	if mime != "image/png" {
		t.Fatalf("mime = %q", mime)
	}
	if string(body) != string(payload) {
		t.Fatalf("body bytes mismatch")
	}
}

func TestClient_ImgB64(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	defer srv.Close()

	c := newClient(t, nil)
	got, err := c.ImgB64(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ImgB64: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("prefix missing: %q", got)
	}
}

func TestClient_HTTPClientExposesUnderlying(t *testing.T) {
	t.Parallel()

	c := httpx.New(httpx.Options{})
	if c.HTTPClient() == nil {
		t.Fatalf("HTTPClient() returned nil")
	}
}

// --- Rate-limit retry tests ---

// newFastClient uses millisecond-scale backoff so rate-limit tests finish
// quickly without relying on real clock sleeps from Retry-After headers.
func newFastClient(t *testing.T, transport http.RoundTripper) *httpx.Client {
	t.Helper()
	return httpx.New(httpx.Options{
		Transport:  transport,
		MaxRetries: 3,
		MinBackoff: time.Millisecond,
		MaxBackoff: 5 * time.Millisecond,
	})
}

func TestClient_403WithRetryAfterIsRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			// First call: secondary rate limit with a very short Retry-After.
			w.Header().Set("Retry-After", "0")
			http.Error(w, "secondary rate limited", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	c := newFastClient(t, nil)
	_, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls (1 rate-limited + 1 success), got %d", got)
	}
}

func TestClient_403WithRatelimitRemainingZeroIsRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			// First call: primary rate exhaustion.
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", "1") // epoch 1 = in the past
			http.Error(w, "rate limit exhausted", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	c := newFastClient(t, nil)
	_, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestClient_403WithoutRateHeadersIsNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// Plain permission error: no rate-limit headers.
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newFastClient(t, nil)
	_, resp, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 call (no retry on permission 403), got %d", got)
	}
}

// --- ClassifyRateLimit tests ---

func TestClassifyRateLimit_RetryAfterHeader(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"Retry-After": []string{"60"},
		},
	}
	rle := httpx.ClassifyRateLimit(resp)
	if rle == nil {
		t.Fatal("expected *RateLimitedError, got nil")
	}
	if rle.Kind != httpx.RateLimitSecondary {
		t.Fatalf("Kind = %v, want RateLimitSecondary", rle.Kind)
	}
	if rle.RetryAt.IsZero() {
		t.Fatal("RetryAt should not be zero when Retry-After: 60 is present")
	}
}

func TestClassifyRateLimit_PrimaryExhaustion(t *testing.T) {
	t.Parallel()

	reset := fmt.Sprintf("%d", time.Now().Add(5*time.Minute).Unix())
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"X-Ratelimit-Remaining": []string{"0"},
			"X-Ratelimit-Reset":     []string{reset},
		},
	}
	rle := httpx.ClassifyRateLimit(resp)
	if rle == nil {
		t.Fatal("expected *RateLimitedError, got nil")
	}
	if rle.Kind != httpx.RateLimitPrimary {
		t.Fatalf("Kind = %v, want RateLimitPrimary", rle.Kind)
	}
	if rle.RetryAt.IsZero() {
		t.Fatal("RetryAt should not be zero when x-ratelimit-reset is present")
	}
}

func TestClassifyRateLimit_PlainForbidden(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{},
	}
	if rle := httpx.ClassifyRateLimit(resp); rle != nil {
		t.Fatalf("expected nil for plain 403, got %v", rle)
	}
}

func TestClassifyRateLimit_Nil(t *testing.T) {
	t.Parallel()

	if rle := httpx.ClassifyRateLimit(nil); rle != nil {
		t.Fatalf("expected nil for nil response, got %v", rle)
	}
}

func TestRateLimitedError_ErrorString(t *testing.T) {
	t.Parallel()

	rle := &httpx.RateLimitedError{
		Kind:    httpx.RateLimitSecondary,
		RetryAt: time.Time{},
	}
	if rle.Error() == "" {
		t.Fatal("Error() should return non-empty string")
	}

	rle2 := &httpx.RateLimitedError{
		Kind:    httpx.RateLimitPrimary,
		RetryAt: time.Now().Add(time.Minute),
	}
	msg := rle2.Error()
	if !strings.Contains(msg, "primary") {
		t.Fatalf("Error() = %q, want to contain 'primary'", msg)
	}
}
