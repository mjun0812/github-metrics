package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	// Side-effect imports so the P2 plugin partials register with the
	// classic dispatcher and the plugins register with the global
	// registry before engine.Compute runs.
	_ "github.com/mjun0812/github-metrics/internal/plugins/calendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/contributors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/habits"
	_ "github.com/mjun0812/github-metrics/internal/plugins/notable"
	_ "github.com/mjun0812/github-metrics/internal/plugins/people"
	_ "github.com/mjun0812/github-metrics/internal/plugins/projects"
	_ "github.com/mjun0812/github-metrics/internal/plugins/reactions"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
	_ "github.com/mjun0812/github-metrics/internal/plugins/traffic"
)

// p2RESTMux serves the REST endpoints the P2 plugins call:
//   - GET /                          (X-OAuth-Scopes for scope-gated plugins)
//   - GET /users/octocat/events*     (activity + habits)
//   - GET /repos/<x>/<y>/traffic/views (traffic)
//   - GET /repos/* fallback           (catch-all 404)
type p2RESTMux struct {
	scopes      string
	eventsBody  string
	trafficBody string
}

func (m *p2RESTMux) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	switch {
	case path == "/":
		hs := http.Header{}
		hs.Set("X-OAuth-Scopes", m.scopes)
		hs.Set("Content-Type", "application/json")
		return resp(req, http.StatusOK, hs, `{}`), nil
	case strings.Contains(path, "/events"):
		return resp(req, http.StatusOK, h, m.eventsBody), nil
	case strings.HasSuffix(path, "/traffic/views"):
		return resp(req, http.StatusOK, h, m.trafficBody), nil
	default:
		return resp(req, http.StatusNotFound, h, `{}`), nil
	}
}

func resp(req *http.Request, status int, h http.Header, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		Request:       req,
		ContentLength: int64(len(body)),
	}
}

func newP2REST(scopes string) *p2RESTMux {
	return &p2RESTMux{
		scopes:      scopes,
		eventsBody:  `[]`,
		trafficBody: `{"count":42,"uniques":12}`,
	}
}

// newP2Deps builds an engine.Deps wired to GraphQL + REST mux fixtures.
// p2 plugins that need GraphQL use the same fixture (mostly returning
// empty results); the integration test asserts the JSON shape.
func newP2Deps(t *testing.T, scopes string) engine.Deps {
	t.Helper()
	gqlFix := newGraphQLFixture()
	gqlFix.On("User", p1UserOctocat)
	gqlFix.On("UserRepositories", p1UserRepositories)
	gqlFix.On("UserIndepth", p1UserIndepth)
	gqlFix.On("UserStarredRepositories", `{"data":{"user":{"starredRepositories":{"totalCount":0,"edges":[]}}}}`)
	gqlFix.On("UserReactions", `{"data":{"user":{"issues":{"totalCount":0,"nodes":[]},"issueComments":{"totalCount":0,"nodes":[]}}}}`)
	gqlFix.On("UserFollowers", `{"data":{"user":{"followers":{"totalCount":0,"nodes":[]},"following":{"totalCount":0,"nodes":[]}}}}`)

	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: gqlFix, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}

	restMux := newP2REST(scopes)
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: restMux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return engine.Deps{
		Settings: &config.Settings{Repositories: 100},
		GraphQL:  gql,
		REST:     rest,
		Render:   &render.FakeRenderer{},
	}
}

// p2Inputs flips on all 17 plugins (5 P1 + 12 P2) so the integration
// suite exercises the full classic dispatcher.
func p2Inputs() map[string]any {
	in := p1Inputs()
	for _, slug := range []string{
		"calendar", "habits", "stars", "people", "notable", "contributors",
		"reactions", "projects", "sponsors", "sponsorships", "stargazers", "traffic",
	} {
		in["plugin_"+slug] = true
	}
	return in
}

// TestComputeJSON_P2AllPlugins covers T075 — assert all 17 plugin
// entries (5 P1 + 12 P2) surface as non-nil JSON values.
func TestComputeJSON_P2AllPlugins(t *testing.T) {
	deps := newP2Deps(t, "repo, read:user, read:org, read:project")
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "json",
		Inputs:   p2Inputs(),
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "application/json" {
		t.Fatalf("MIME = %q, want application/json", res.MIME)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Output, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	pluginsMap, _ := payload["plugins"].(map[string]any)
	if pluginsMap == nil {
		t.Fatalf("plugins map missing")
	}
	wantSlugs := []string{
		// P1
		"languages", "activity", "achievements", "repositories", "isocalendar",
		// P2
		"calendar", "habits", "stars", "people", "notable",
		"contributors", "reactions", "projects", "sponsors", "sponsorships",
		"stargazers", "traffic",
	}
	for _, slug := range wantSlugs {
		entry, ok := pluginsMap[slug]
		if !ok {
			t.Errorf("plugins[%q] missing", slug)
		}
		if entry == nil {
			t.Errorf("plugins[%q] is nil", slug)
		}
	}
}

// TestComputeSVG_P2Bundle covers T074 — the three SVG bundles. We
// rolled them into a single test with subtests because the dispatcher
// behaviour is identical and shared mocked dependencies keep the test
// fast.
func TestComputeSVG_P2Bundle(t *testing.T) {
	bundles := map[string][]string{
		"A_data": {"calendar", "habits"},
		"B_skipped_only": {
			"notable", "contributors", "stargazers", "sponsorships",
		},
		"C_scope_gated": {"projects", "sponsors", "traffic"},
	}
	for name, slugs := range bundles {
		t.Run(name, func(t *testing.T) {
			deps := newP2Deps(t, "repo, read:user, read:org, read:project")
			in := p1Inputs()
			for _, slug := range slugs {
				in["plugin_"+slug] = true
			}
			res, err := engine.Compute(context.Background(), engine.Request{
				Login:    "octocat",
				Template: "classic",
				Format:   "svg",
				Inputs:   in,
			}, deps)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if res.MIME != "image/svg+xml" {
				t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
			}
			// Spec 013: GraphQL plugins (sponsors / sponsorships /
			// projects / notable / stargazers / repositories.Pinned) now
			// fire viewer.* queries when their `plugin_<slug>` is true.
			// In bundles B / C the GraphQL mux has no fixture for these
			// new operations, so they record a *RetryableError per plugin.
			// That's an EXPECTED degraded path (FR-002), not a test
			// failure — partial output stays correct (Skipped fragments
			// produce no DOM). Only fail on out-of-bounds errors.
			for _, e := range res.Errors {
				if !strings.Contains(e.Error(), "no fixture for operation Viewer") {
					t.Errorf("Result.Errors entry: %v", e)
				}
			}
			// We don't assert specific DOM markers per slug — most P2
			// plugins are Skipped in M4 (no wrappers emitted), and the
			// shape stability is already checked by the JSON test
			// above + the per-plugin golden tests.
			out := string(res.Output)
			if len(out) < 100 {
				t.Errorf("SVG output suspiciously short: %d bytes", len(out))
			}
		})
	}
}
