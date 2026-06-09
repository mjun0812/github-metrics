package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	// Side-effect imports register the M4 plugin partials with the
	// classic template's lookup table. Without these blank imports the
	// dispatcher in classic.Run would silently skip every plugin slug.
	_ "github.com/mjun0812/github-metrics/internal/plugins/achievements"
	_ "github.com/mjun0812/github-metrics/internal/plugins/activity"
	_ "github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/repositories"
)

// p1UserOctocat mirrors userOctocat from foundation_test.go (kept
// local to avoid coupling the two suites' fixtures).
const p1UserOctocat = `{
	"data": {
		"user": {
			"databaseId": 12345,
			"id": "MDQ6VXNlcjEyMzQ1",
			"login": "octocat",
			"name": "The Octocat",
			"location": "San Francisco",
			"createdAt": "2008-01-14T04:33:35Z",
			"avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4"
		}
	}
}`

// p1UserRepositories provides 3 repositories with language byte
// breakdowns the languages plugin can aggregate. hasNextPage:false
// terminates the base paging loop after one call.
const p1UserRepositories = `{
	"data": {
		"user": {
			"repositories": {
				"totalCount": 3,
				"pageInfo": {"hasNextPage": false, "endCursor": null},
				"nodes": [
					{
						"databaseId": 1, "id": "R_a", "name": "alpha",
						"nameWithOwner": "octocat/alpha",
						"url": "https://github.com/octocat/alpha",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 150, "forkCount": 20,
						"watchers": {"totalCount": 5},
						"primaryLanguage": {"name": "Go", "color": "#00ADD8"},
						"languages": {
							"totalCount": 2, "totalSize": 8000,
							"edges": [
								{"size": 6000, "node": {"name": "Go", "color": "#00ADD8"}},
								{"size": 2000, "node": {"name": "JavaScript", "color": "#f1e05a"}}
							]
						}
					},
					{
						"databaseId": 2, "id": "R_b", "name": "beta",
						"nameWithOwner": "octocat/beta",
						"url": "https://github.com/octocat/beta",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 60, "forkCount": 5,
						"watchers": {"totalCount": 3},
						"primaryLanguage": {"name": "TypeScript", "color": "#3178c6"},
						"languages": {
							"totalCount": 1, "totalSize": 4500,
							"edges": [
								{"size": 4500, "node": {"name": "TypeScript", "color": "#3178c6"}}
							]
						}
					},
					{
						"databaseId": 3, "id": "R_c", "name": "gamma",
						"nameWithOwner": "octocat/gamma",
						"url": "https://github.com/octocat/gamma",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 30, "forkCount": 2,
						"watchers": {"totalCount": 2},
						"primaryLanguage": {"name": "Go", "color": "#00ADD8"},
						"languages": {
							"totalCount": 1, "totalSize": 4000,
							"edges": [
								{"size": 4000, "node": {"name": "Go", "color": "#00ADD8"}}
							]
						}
					}
				]
			}
		}
	}
}`

// p1UserIndepth populates ContributionCalendar (for isocalendar) and
// totals (for achievements). The defaultBranchRef commit totals push
// commits to the C rank (>100).
const p1UserIndepth = `{
	"data": {
		"user": {
			"contributionsCollection": {
				"contributionCalendar": {
					"totalContributions": 365,
					"weeks": [
						{
							"firstDay": "2026-W18",
							"contributionDays": [
								{"date": "2026-05-04", "contributionCount": 1, "weekday": 0, "color": "#aaa"},
								{"date": "2026-05-05", "contributionCount": 2, "weekday": 1, "color": "#aaa"},
								{"date": "2026-05-06", "contributionCount": 0, "weekday": 2, "color": "#fff"},
								{"date": "2026-05-07", "contributionCount": 3, "weekday": 3, "color": "#aaa"},
								{"date": "2026-05-08", "contributionCount": 4, "weekday": 4, "color": "#aaa"},
								{"date": "2026-05-09", "contributionCount": 2, "weekday": 5, "color": "#aaa"},
								{"date": "2026-05-10", "contributionCount": 1, "weekday": 6, "color": "#aaa"}
							]
						}
					]
				}
			},
			"repositories": {
				"totalCount": 3,
				"pageInfo": {"hasNextPage": false, "endCursor": null},
				"nodes": [
					{
						"defaultBranchRef": {
							"name": "main",
							"target": {
								"__typename": "Commit",
								"id": "C_a",
								"history": {"totalCount": 120}
							}
						},
						"issues": {"totalCount": 12},
						"pullRequests": {"totalCount": 80}
					},
					{
						"defaultBranchRef": {
							"name": "main",
							"target": {
								"__typename": "Commit",
								"id": "C_b",
								"history": {"totalCount": 30}
							}
						},
						"issues": {"totalCount": 3},
						"pullRequests": {"totalCount": 25}
					},
					{
						"defaultBranchRef": {
							"name": "main",
							"target": {
								"__typename": "Commit",
								"id": "C_c",
								"history": {"totalCount": 10}
							}
						},
						"issues": {"totalCount": 0},
						"pullRequests": {"totalCount": 5}
					}
				]
			}
		}
	}
}`

// p1UserIsocalendar answers the isocalendar plugin's windowed
// contributionsCollection(from,to) query (#467). The fixture serves the
// same single week for every 4-week chunk the plugin requests (7 chunks
// for the default half-year window), which is fine for the DOM-marker
// assertions below.
const p1UserIsocalendar = `{
	"data": {
		"user": {
			"contributionsCollection": {
				"contributionCalendar": {
					"weeks": [
						{
							"firstDay": "2026-W18",
							"contributionDays": [
								{"date": "2026-05-04", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
								{"date": "2026-05-05", "contributionCount": 2, "weekday": 1, "color": "#40c463"},
								{"date": "2026-05-06", "contributionCount": 0, "weekday": 2, "color": "#ebedf0"},
								{"date": "2026-05-07", "contributionCount": 3, "weekday": 3, "color": "#30a14e"},
								{"date": "2026-05-08", "contributionCount": 4, "weekday": 4, "color": "#216e39"},
								{"date": "2026-05-09", "contributionCount": 2, "weekday": 5, "color": "#40c463"},
								{"date": "2026-05-10", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
							]
						}
					]
				}
			}
		}
	}
}`

// restEventsMux serves /users/{login}/events with a fixed payload.
type restEventsMux struct{ body string }

func (m *restEventsMux) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/events") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(m.body)),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Request:    req,
	}, nil
}

// p1Inputs flips on all five P1 plugins via their truthy gate.
func p1Inputs() map[string]any {
	return map[string]any{
		"plugin_languages":    true,
		"plugin_activity":     true,
		"plugin_achievements": true,
		"plugin_repositories": true,
		"plugin_isocalendar":  true,
	}
}

func newP1Deps(t *testing.T) engine.Deps {
	t.Helper()
	gqlFixture := newGraphQLFixture()
	gqlFixture.On("User", p1UserOctocat)
	gqlFixture.On("UserRepositories", p1UserRepositories)
	gqlFixture.On("UserIndepth", p1UserIndepth)
	gqlFixture.On("UserIsocalendar", p1UserIsocalendar)

	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: gqlFixture, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}

	// Timestamps are generated relative to "now" so the events always
	// fall inside the activity plugin's default 14-day recency window
	// (internal/plugins/activity: cutoff = now - days). Hardcoded dates
	// silently aged out of the window and made this test fail by the
	// calendar — the activity <section> only renders when ≥1 event
	// survives the cutoff.
	now := time.Now().UTC()
	eventsBody := fmt.Sprintf(
		`[
		{"type":"PushEvent","repo":{"name":"octocat/alpha"},"created_at":%q,"public":true},
		{"type":"PullRequestEvent","repo":{"name":"octocat/beta"},"created_at":%q,"public":true}
	]`,
		now.Add(-24*time.Hour).Format(time.RFC3339),
		now.Add(-25*time.Hour).Format(time.RFC3339),
	)
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: &restEventsMux{body: eventsBody}, MaxRetries: 0},
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

// TestComputeSVG_P1AllPlugins drives engine.Compute through the classic
// template with all five P1 plugins enabled and asserts the necessary
// DOM markers are present per contracts/partial-classic-m4.md §5.
func TestComputeSVG_P1AllPlugins(t *testing.T) {
	deps := newP1Deps(t)
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   p1Inputs(),
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}
	for _, e := range res.Errors {
		t.Errorf("Result.Errors entry: %v", e)
	}
	out := string(res.Output)
	wantMarkers := []string{
		// Wrapper per plugin (contract §3).
		`data-plugin="languages"`,
		`data-plugin="activity"`,
		`data-plugin="achievements"`,
		`data-plugin="repositories"`,
		`data-plugin="isocalendar"`,
		// Per-plugin DOM markers (contract §5).
		// 011 (Tier 2/3 sweep): activity-event was retired when the
		// activity partial dropped its bare <text> elements in favour
		// of upstream-parity HTML rows wrapped in <section class="activity">.
		// languages-progress / language-bar remain inside the <svg class="bar">
		// wrapper. achievement / repository markers unchanged (still emitted
		// by their respective partials).
		`class="languages-progress"`,
		`class="language-bar"`,
		`class="activity"`,
		// achievement entries land in `<div class="achievement <rank> largeable-width-half">`
		// so the literal `class="achievement"` (closing quote) no longer
		// matches. Anchor on the multi-class prefix instead.
		`class="achievement `,
		`class="repository"`,
		`class="isocalendar-grid"`,
		`<filter id="brightness1">`,
	}
	for _, m := range wantMarkers {
		if !strings.Contains(out, m) {
			t.Errorf("SVG missing marker %q\n---\n%s\n---", m, snippet(out))
		}
	}
}

// TestComputeJSON_P1AllPlugins asserts the JSON output exposes all 5
// P1 plugin entries with non-null payloads and the expected shape.
func TestComputeJSON_P1AllPlugins(t *testing.T) {
	deps := newP1Deps(t)
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "json",
		Inputs:   p1Inputs(),
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
	// M2 JSON layout flattens "plugins" at the top level (not under
	// "data"); see internal/engine/json.go.
	pluginsMap, _ := payload["plugins"].(map[string]any)
	if pluginsMap == nil {
		t.Fatalf("plugins map missing in JSON output:\n%s", string(res.Output))
	}
	for _, slug := range []string{
		"languages", "activity", "achievements", "repositories", "isocalendar",
	} {
		entry, ok := pluginsMap[slug]
		if !ok || entry == nil {
			t.Errorf("data.plugins[%q] missing or nil", slug)
		}
	}
	// Spot-check languages aggregation: Go should be Mostly given the
	// fixture (6000 + 4000 bytes > TypeScript 4500).
	if langs, ok := pluginsMap["languages"].(map[string]any); ok {
		if mostly, ok := langs["mostly"].(map[string]any); ok {
			if name, _ := mostly["name"].(string); name != "Go" {
				t.Errorf("languages.mostly.name = %q, want Go", name)
			}
		} else {
			t.Errorf("languages.mostly not an object: %v", langs["mostly"])
		}
	}
}

// snippet returns a bounded preview of s so failure messages stay
// readable in CI logs.
func snippet(s string) string {
	const max = 800
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
