//go:build chromedp

package starlists_test

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
	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
	"github.com/mjun0812/github-metrics/internal/render"
)

func withBrowser(t *testing.T) *render.Browser {
	t.Helper()
	b, err := render.New(render.BrowserOpts{})
	if err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

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

// startFixtureServer returns the URL of an httptest server that
// serves the two starlists fixture pages. /lists → stars_lists.html,
// /list-detail → list_backend.html.
func startFixtureServer(t *testing.T) string {
	t.Helper()
	lists, err := os.ReadFile(loadFixturePath(t, "tests/fixtures/plugins/starlists/stars_lists.html"))
	if err != nil {
		t.Fatalf("read lists fixture: %v", err)
	}
	detail, err := os.ReadFile(loadFixturePath(t, "tests/fixtures/plugins/starlists/list_backend.html"))
	if err != nil {
		t.Fatalf("read detail fixture: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/lists", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(lists)
	})
	mux.HandleFunc("/list-detail", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(detail)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// remapNavigator routes the production browserNavigator at the local
// fixture server rather than github.com.
type remapNavigator struct {
	inner   starlists.Navigator
	listURL string
	repoURL string
}

func (r *remapNavigator) FetchLists(ctx context.Context, _ string) ([]starlists.Starlist, error) {
	return r.inner.FetchLists(ctx, r.listURL)
}

func (r *remapNavigator) FetchRepos(ctx context.Context, _ string) ([]string, error) {
	return r.inner.FetchRepos(ctx, r.repoURL)
}

// TestRun_Chromedp_ExtractsLists drives the production navigator
// against the fixture HTTP server.
func TestRun_Chromedp_ExtractsLists(t *testing.T) {
	browser := withBrowser(t)
	serverURL := startFixtureServer(t)
	nav := &remapNavigator{
		inner:   starlists.NewBrowserNavigator(browser),
		listURL: serverURL + "/lists",
		repoURL: serverURL + "/list-detail",
	}
	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":                 "octocat",
			"plugin_starlists":     true,
			starlists.NavigatorKey: nav,
		},
		Data:   data,
		Render: browser,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := starlists.Plugin.Run(ctx, pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*starlists.Result)
	if r.Skipped {
		t.Fatalf("Skipped: %s", r.SkippedReason)
	}
	if len(r.List) != 2 {
		t.Fatalf("List len = %d, want 2", len(r.List))
	}
	if r.List[0].Name != "Backend" {
		t.Errorf("List[0] = %q, want Backend (sorted)", r.List[0].Name)
	}
}

// TestRun_Chromedp_Languages_ExtractsRepos verifies that with
// _languages=true the navigator drills into each list's detail page
// and the resulting Starlist.Languages reflect the joined data.
func TestRun_Chromedp_Languages_ExtractsRepos(t *testing.T) {
	browser := withBrowser(t)
	serverURL := startFixtureServer(t)
	nav := &remapNavigator{
		inner:   starlists.NewBrowserNavigator(browser),
		listURL: serverURL + "/lists",
		repoURL: serverURL + "/list-detail",
	}
	data := plugins.NewData()
	data.Computed.RepositoryList = []plugins.Repository{
		{
			NameWithOwner: "octocat/go-svc",
			Languages:     []plugins.LanguageStat{{Name: "Go", Color: "#00ADD8", Size: 4000}},
		},
		{
			NameWithOwner: "octocat/rust-svc",
			Languages:     []plugins.LanguageStat{{Name: "Rust", Color: "#dea584", Size: 2000}},
		},
	}
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":                       "octocat",
			"plugin_starlists":           true,
			"plugin_starlists_languages": true,
			starlists.NavigatorKey:       nav,
		},
		Data:   data,
		Render: browser,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := starlists.Plugin.Run(ctx, pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*starlists.Result)
	if !r.Languages {
		t.Errorf("Result.Languages = false, want true")
	}
	// The "Backend" list points at /list-detail which yields go-svc + rust-svc.
	var backend *starlists.Starlist
	for i := range r.List {
		if r.List[i].Name == "Backend" {
			backend = &r.List[i]
		}
	}
	if backend == nil {
		t.Fatalf("Backend list missing; got %+v", r.List)
	}
	if len(backend.Languages) != 2 {
		t.Errorf("Backend.Languages len = %d, want 2; %+v", len(backend.Languages), backend.Languages)
	}
}

// TestRun_Chromedp_Timeout — navigation to an unreachable host wraps
// the error as *RetryableError.
func TestRun_Chromedp_Timeout(t *testing.T) {
	browser := withBrowser(t)
	nav := &remapNavigator{
		inner:   starlists.NewBrowserNavigator(browser),
		listURL: "http://127.0.0.1:1/lists",
		repoURL: "http://127.0.0.1:1/list-detail",
	}
	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":                 "octocat",
			"plugin_starlists":     true,
			starlists.NavigatorKey: nav,
		},
		Data:   data,
		Render: browser,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := starlists.Plugin.Run(ctx, pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "starlists") {
		t.Errorf("err = %v, want starlists-prefixed", err)
	}
}
