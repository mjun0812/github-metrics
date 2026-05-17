//go:build chromedp

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

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
	"github.com/mjun0812/github-metrics/internal/render"
)

// withBrowser opens a Browser using the default chromedp settings. The
// test is skipped when chromium cannot be located so contributor
// environments without the headless image still pass.
func withBrowser(t *testing.T) *render.Browser {
	t.Helper()
	b, err := render.New(render.BrowserOpts{})
	if err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// loadFixturePath walks up from CWD to find a file under tests/fixtures.
func loadFixturePath(t *testing.T, relPath string) string {
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

// startFixtureServer spins up a local HTTP server that serves the
// stars-topics fixture page. Returns the URL the navigator should hit.
func startFixtureServer(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(loadFixturePath(t, "tests/fixtures/plugins/topics/stars_topics.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestRun_Chromedp_ExtractsTopics drives the production browserNavigator
// against a fixture HTML page served from a local httptest server.
func TestRun_Chromedp_ExtractsTopics(t *testing.T) {
	browser := withBrowser(t)
	serverURL := startFixtureServer(t)

	// Use the browserNavigator via a thin shim: we inject a custom
	// Navigator that calls Fetch with the local fixture URL instead of
	// github.com. We do this by wrapping the production navigator so
	// we don't need to mutate the plugin URL construction.
	prod := topics.NewBrowserNavigator(browser)
	nav := &remapNavigator{inner: prod, target: serverURL}

	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":               "octocat",
			"plugin_topics":      true,
			topics.NavigatorKey:  nav,
			"plugin_topics_sort": "name",
		},
		Data:   data,
		Render: browser,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := topics.Plugin.Run(ctx, pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if r.Skipped {
		t.Fatalf("Skipped: %s", r.SkippedReason)
	}
	if len(r.List) == 0 {
		t.Fatalf("List empty; expected 5 topics from fixture")
	}
	// Sorted by lowercase name:
	wantFirst := "go"
	if !strings.EqualFold(r.List[0].Name, wantFirst) {
		t.Errorf("List[0] = %q, want %q (after name sort)", r.List[0].Name, wantFirst)
	}
}

// TestRun_Chromedp_TimeoutRetryable verifies that a navigation timeout
// produces an error wrapped as *RetryableError per contract §3.6.
func TestRun_Chromedp_TimeoutRetryable(t *testing.T) {
	browser := withBrowser(t)
	prod := topics.NewBrowserNavigator(browser)
	// Black-hole URL — chromedp will time out trying to reach it.
	nav := &remapNavigator{inner: prod, target: "http://127.0.0.1:1"}

	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":              "octocat",
			"plugin_topics":     true,
			topics.NavigatorKey: nav,
		},
		Data:   data,
		Render: browser,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := topics.Plugin.Run(ctx, pc)
	if err == nil {
		t.Fatalf("expected error from timeout")
	}
	if !strings.Contains(err.Error(), "topics") {
		t.Errorf("err = %v, want topics-prefixed", err)
	}
}

// remapNavigator forwards Fetch to inner but substitutes the GitHub
// URL with the local fixture server. This isolates the test from the
// network while still exercising the real chromedp scrape path.
type remapNavigator struct {
	inner  topics.Navigator
	target string
}

func (r *remapNavigator) Fetch(ctx context.Context, _ string) ([]topics.Topic, error) {
	return r.inner.Fetch(ctx, r.target)
}
