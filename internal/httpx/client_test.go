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
