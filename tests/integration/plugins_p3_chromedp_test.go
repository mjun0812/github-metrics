//go:build chromedp

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
)

// fixtureBytes reads a file under tests/fixtures from the repo root.
func fixtureBytes(t *testing.T, rel string) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		if b, err := os.ReadFile(candidate); err == nil {
			return b
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("fixture %s not found", rel)
	return nil
}

// TestComputeSVG_P3Chromedp drives engine.Compute through the classic
// template with topics + starlists enabled and asserts the necessary
// DOM markers. The chromedp scrape targets a local httptest server
// serving the committed HTML fixtures.
func TestComputeSVG_P3Chromedp(t *testing.T) {
	browser, err := render.New(render.BrowserOpts{})
	if err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	browserForRemap = browser
	t.Cleanup(func() { browserForRemap = nil })

	topicsBody := fixtureBytes(t, "tests/fixtures/plugins/topics/stars_topics.html")
	listsBody := fixtureBytes(t, "tests/fixtures/plugins/starlists/stars_lists.html")
	listDetail := fixtureBytes(t, "tests/fixtures/plugins/starlists/list_backend.html")
	mux := http.NewServeMux()
	mux.HandleFunc("/topics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(topicsBody)
	})
	mux.HandleFunc("/lists", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(listsBody)
	})
	mux.HandleFunc("/list-detail", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(listDetail)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Reuse the P1 GraphQL + REST fixtures so base + the other plugins
	// still have something to chew on. We don't enable any of the P1
	// plugin gates here — only topics + starlists.
	gqlFixture := newGraphQLFixture()
	gqlFixture.On("User", p1UserOctocat)
	gqlFixture.On("UserRepositories", p1UserRepositories)
	gqlFixture.On("UserIndepth", p1UserIndepth)
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: gqlFixture, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: &restEventsMux{body: `[]`}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}

	// Inject local-fixture-server-aware Navigators so chromedp does
	// not hit github.com. Wired through the plugin Inputs map via the
	// dedicated test keys.
	deps := engine.Deps{
		Settings: &config.Settings{Repositories: 100},
		GraphQL:  gql,
		REST:     rest,
		Render:   browser,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := engine.Compute(ctx, engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   p3ChromedpInputs(srv.URL),
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}

	out := string(res.Output)
	wantMarkers := []string{
		`data-plugin="topics"`,
		`data-plugin="starlists"`,
		`<g class="topic"`,
		`<text class="topic-name">`,
		`<g class="starlist"`,
		`<text class="starlist-name">`,
	}
	for _, m := range wantMarkers {
		if !strings.Contains(out, m) {
			t.Errorf("SVG missing marker %q\n---\n%s\n---", m, snippet(out))
		}
	}
}

// p3ChromedpInputs builds the inputs needed for the P3 chromedp
// integration: topics + starlists enabled, both pointed at the local
// fixture server via the test Navigator-key seam.
func p3ChromedpInputs(serverURL string) map[string]any {
	return map[string]any{
		"plugin_topics":      true,
		"plugin_starlists":   true,
		"plugin_topics_sort": "name",
		// Inject test Navigators that read from the local fixture
		// server instead of github.com.
		topics.NavigatorKey:    newRemappedTopicsNavigator(browserForRemap, serverURL+"/topics"),
		starlists.NavigatorKey: newRemappedStarlistsNavigator(browserForRemap, serverURL+"/lists", serverURL+"/list-detail"),
	}
}

// browserForRemap holds the test's *render.Browser so the navigator
// remap helpers can build production browserNavigators against it. The
// integration test sets this in init-of-test (TestComputeSVG_P3Chromedp)
// before invoking p3ChromedpInputs.
var browserForRemap *render.Browser

type remappedTopicsNavigator struct {
	inner topics.Navigator
	url   string
}

func newRemappedTopicsNavigator(b *render.Browser, url string) *remappedTopicsNavigator {
	return &remappedTopicsNavigator{
		inner: topics.NewBrowserNavigator(b),
		url:   url,
	}
}

func (r *remappedTopicsNavigator) Fetch(ctx context.Context, _ string) ([]topics.Topic, error) {
	return r.inner.Fetch(ctx, r.url)
}

type remappedStarlistsNavigator struct {
	inner     starlists.Navigator
	listsURL  string
	detailURL string
}

func newRemappedStarlistsNavigator(b *render.Browser, listsURL, detailURL string) *remappedStarlistsNavigator {
	return &remappedStarlistsNavigator{
		inner:     starlists.NewBrowserNavigator(b),
		listsURL:  listsURL,
		detailURL: detailURL,
	}
}

func (r *remappedStarlistsNavigator) FetchLists(ctx context.Context, _ string) ([]starlists.Starlist, error) {
	return r.inner.FetchLists(ctx, r.listsURL)
}

func (r *remappedStarlistsNavigator) FetchRepos(ctx context.Context, _ string) ([]string, error) {
	return r.inner.FetchRepos(ctx, r.detailURL)
}
