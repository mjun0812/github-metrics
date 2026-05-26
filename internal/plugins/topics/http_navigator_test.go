package topics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins/topics"
)

// TestHTTPNavigator_Fixture drives the production httpNavigator against
// a local httptest server hosting the committed stars/topics fixture.
// This exercises the HTTP + goquery extraction path end-to-end without
// touching github.com.
func TestHTTPNavigator_Fixture(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(fixturePath(t, "tests/fixtures/plugins/topics/stars_topics.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	nav := topics.NewHTTPNavigator(srv.Client(), "github-metrics-test")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	list, err := nav.Fetch(ctx, srv.URL+"/stars/octocat/topics")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(list) != 5 {
		t.Fatalf("len(list) = %d, want 5; %+v", len(list), list)
	}
	want := []string{"Go", "Rust", "SVG", "Python", "LLM"}
	for i, w := range want {
		if list[i].Name != w {
			t.Errorf("list[%d].Name = %q, want %q", i, list[i].Name, w)
		}
	}
	if !strings.HasPrefix(list[0].Icon, "https://github.githubassets.com/topics/go.png") {
		t.Errorf("list[0].Icon = %q, want github.githubassets.com URL", list[0].Icon)
	}
	if list[0].URL != "/topics/go" {
		t.Errorf("list[0].URL = %q, want /topics/go", list[0].URL)
	}
	if list[0].StarredAt == "" {
		t.Errorf("list[0].StarredAt should be inherited from the <li data-starred-at>")
	}
}

// TestHTTPNavigator_BadStatus surfaces an HTTP error when the server
// responds with a non-2xx status. The error is wrapped with the URL so
// debug messages are useful.
func TestHTTPNavigator_BadStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	nav := topics.NewHTTPNavigator(srv.Client(), "test")
	_, err := nav.Fetch(context.Background(), srv.URL+"/stars/octocat/topics")
	if err == nil {
		t.Fatalf("expected error from 503 response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want 503 status in message", err)
	}
}

// TestHTTPNavigator_DropsNonDetailLinks ignores "/topics/" (index page)
// and "/topics/<slug>/extras" style links that aren't topic detail
// pages.
func TestHTTPNavigator_DropsNonDetailLinks(t *testing.T) {
	t.Parallel()
	body := `<!doctype html><html><body>
<a href="/topics/"><p>Index</p></a>
<a href="/topics/go"><p>Go</p></a>
<a href="/topics/go"><p>Go again</p></a>
<a href="/topics/python?tab=stars"><p>Python</p></a>
<a href="/topics/rust/popular"><p>Rust</p></a>
<a href="/foo/bar"><p>Not a topic</p></a>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	nav := topics.NewHTTPNavigator(srv.Client(), "test")
	list, err := nav.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got := make([]string, 0, len(list))
	for _, x := range list {
		got = append(got, x.Name)
	}
	want := []string{"Go", "Python", "Rust"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// fixturePath walks up to find a file under the repo's tests/fixtures.
func fixturePath(t *testing.T, relPath string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, relPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("fixture %s not found", relPath)
	return ""
}
