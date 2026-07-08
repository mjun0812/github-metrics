// Package integration_test covers User Story 5 (P3) of the project
// foundation spec: end-to-end engine.Compute against a mocked GraphQL
// backend. The test deliberately does NOT exercise rendering — the M1
// engine returns once the plugin pipeline finishes populating Data,
// because the actual SVG/JSON marshaller is a M2 task.
package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"

	// Side-effect imports keep the P3 plugins (topics, starlists)
	// registered in the global plugin registry. Without this, the JSON
	// golden would differ depending on whether topics/starlists happen
	// to be registered elsewhere in the test binary.
	_ "github.com/mjun0812/github-metrics/internal/plugins/starlists"
	_ "github.com/mjun0812/github-metrics/internal/plugins/topics"

	// #602: register the opt-in header plugin so the
	// TestComputePNG_E2E (and any other integration test that runs
	// engine.Compute without pulling cmd/metrics-cli's plugin manifest)
	// can render the identity card when plugin_header=yes is set.
	// Without this side-effect import, the partial dispatcher silently
	// skips "plugin.header" because no Lookup is registered.
	_ "github.com/mjun0812/github-metrics/internal/plugins/header"

	// #625: register the base plugin so the activity+community and
	// repositories static partials have a Result to read when the
	// integration suite drives engine.Compute with plugin_base=yes.
	_ "github.com/mjun0812/github-metrics/internal/plugins/base"
)

// graphQLFixture is a tiny RoundTripper that inspects the GraphQL
// request body, identifies the operation name, and returns the
// pre-registered payload.
type graphQLFixture struct {
	responses map[string]string
	calls     atomic.Int32
}

func newGraphQLFixture() *graphQLFixture { return &graphQLFixture{responses: map[string]string{}} }

func (g *graphQLFixture) On(op, body string) { g.responses[op] = body }

func (g *graphQLFixture) RoundTrip(req *http.Request) (*http.Response, error) {
	g.calls.Add(1)
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()

	var payload struct {
		OpName string `json:"operationName"`
		Query  string `json:"query"`
	}
	_ = json.Unmarshal(body, &payload)

	resp, ok := g.responses[payload.OpName]
	if !ok {
		resp = `{"errors":[{"message":"no fixture for operation ` + payload.OpName + `"}]}`
		return jsonResponse(req, http.StatusBadRequest, resp), nil
	}
	return jsonResponse(req, http.StatusOK, resp), nil
}

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

// fixtures captures the canned GraphQL responses we use across every
// test case. Keeping them in one place documents the assumed user
// payload shape.
const (
	userOctocat = `{
		"data": {
			"user": {
				"databaseId": 12345,
				"id": "MDQ6VXNlcjEyMzQ1",
				"login": "octocat",
				"name": "The Octocat",
				"location": "San Francisco",
				"createdAt": "2008-01-14T04:33:35Z",
				"avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4",
				"websiteUrl": "https://github.com",
				"twitterUsername": null,
				"email": "",
				"bio": "Mascot",
				"company": "@github",
				"followers": {"totalCount": 1555},
				"following": {"totalCount": 617},
				"watching": {"totalCount": 16},
				"sponsorshipsAsMaintainer": {"totalCount": 4},
				"organizations": {"totalCount": 2},
				"sponsorshipsAsSponsor": {"totalCount": 0},
				"starredRepositories": {"totalCount": 310},
				"issueComments": {"totalCount": 333},
				"contributionsCollection": {
					"totalCommitContributions": 7293,
					"totalPullRequestReviewContributions": 68,
					"totalPullRequestContributions": 290,
					"totalIssueContributions": 443,
					"contributionCalendar": {"totalContributions": 8000, "weeks": []}
				}
			}
		}
	}`

	// M4 fixture: hasNextPage:false terminates the new base-plugin
	// batch-halving paging loop after the first page. totalCount stays
	// 250 so the assertion on Computed.Repositories.Count still holds.
	userRepositories250 = `{
		"data": {
			"user": {
				"repositories": {
					"totalCount": 250,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": [
						{"databaseId": 1, "id": "R_kgDOA", "name": "alpha", "nameWithOwner": "octocat/alpha", "url": "https://github.com/octocat/alpha", "isPrivate": false, "isFork": false, "stargazerCount": 100, "forkCount": 10, "watchers": {"totalCount": 5}},
						{"databaseId": 2, "id": "R_kgDOB", "name": "beta",  "nameWithOwner": "octocat/beta",  "url": "https://github.com/octocat/beta",  "isPrivate": false, "isFork": false, "stargazerCount":  50, "forkCount":  3, "watchers": {"totalCount": 2}}
					]
				}
			}
		}
	}`

	userCommitContributionsZero = `{
		"data": {
			"user": {
				"contributionsCollection": {
					"totalCommitContributions": 0
				}
			}
		}
	}`

	orgGithub = `{
		"data": {
			"organization": {
				"databaseId": 9919,
				"id": "MDEyOk9yZ2FuaXphdGlvbjk5MTk=",
				"login": "github",
				"name": "GitHub",
				"location": null,
				"createdAt": "2008-04-10T00:00:00Z",
				"avatarUrl": "https://avatars.githubusercontent.com/u/9919?v=4",
				"websiteUrl": "https://github.com",
				"email": null,
				"description": "How people build software."
			}
		}
	}`

	// M4: base.runOrganization now also fetches members. The minimal
	// payload below keeps the existing integration tests green without
	// changing their assertions.
	orgMembersEmpty = `{
		"data": {
			"organization": {
				"membersWithRole": {
					"totalCount": 0,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": []
				}
			}
		}
	}`

	orgRepositories12 = `{
		"data": {
			"organization": {
				"repositories": {
					"totalCount": 12,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": [
						{"databaseId": 100, "id": "R_kgDOX", "name": "site", "nameWithOwner": "github/site", "url": "https://github.com/github/site", "isPrivate": false, "isFork": false, "stargazerCount": 7, "forkCount": 1, "watchers": {"totalCount": 4}}
					]
				}
			}
		}
	}`
)

func newEngineDeps(t testing.TB, gqlBody map[string]string) (engine.Deps, *graphQLFixture) {
	t.Helper()
	fixture := newGraphQLFixture()
	for op, body := range gqlBody {
		fixture.On(op, body)
	}
	if _, ok := gqlBody["UserCommitContributions"]; !ok {
		fixture.On("UserCommitContributions", userCommitContributionsZero)
	}

	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: fixture, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return engine.Deps{
		Settings: &config.Settings{Repositories: 100},
		GraphQL:  gql,
		// Inject a FakeRenderer so the M3 dispatch path can be
		// exercised without starting a real browser.
		Render: &render.FakeRenderer{},
	}, fixture
}

// newEngineDepsWithREST extends newEngineDeps with a REST client backed
// by a mocks.RESTMux. restSetup is called with the mux before the REST
// client is constructed, allowing callers to register path handlers.
// Plugins that gate behavior on OAuth scopes (sponsors, traffic)
// need the "/" path registered with an X-OAuth-Scopes header.
//
// The activity plugin has no enable gate and always runs whenever a
// REST client is wired in (see internal/plugins/activity/activity.go),
// so we register an empty `/users/{login}/events` feed by default. The
// `login` argument is consulted to keep the path stable across logins.
// Callers may override the handler via restSetup if they need real
// events data.
func newEngineDepsWithREST(t *testing.T, login string, gqlBody map[string]string, restSetup func(*mocks.RESTMux)) (engine.Deps, *graphQLFixture) {
	t.Helper()
	deps, fixture := newEngineDeps(t, gqlBody)
	restMux := mocks.NewRESTMux(t)
	if login != "" {
		restMux.OnBody("/users/"+login+"/events", 200, "[]")
	}
	if restSetup != nil {
		restSetup(restMux)
	}
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: restMux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	deps.REST = rest
	return deps, fixture
}

// TestEngine_ComputeUser is US5 AS1 + AS2 combined: with mocked
// GraphQL returning octocat and 250 repositories, Compute wires a
// Provider that resolves Provider.User and Provider.RepositorySummary.
//
// #605: after base deletion the engine no longer eagerly populates
// pc.Data.User / pc.Data.Computed.* — the canonical values live behind
// the per-request dataprovider, exposed via Result.Provider.
func TestEngine_ComputeUser(t *testing.T) {
	t.Parallel()

	deps, fixture := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if res.Provider == nil {
		t.Fatal("Result.Provider is nil")
	}
	user, err := res.Provider.User(context.Background())
	if err != nil || user == nil {
		t.Fatalf("Provider.User: %v (user=%v)", err, user)
	}
	if user.Login != "octocat" {
		t.Errorf("login = %q, want octocat", user.Login)
	}
	summary, err := res.Provider.RepositorySummary(context.Background())
	if err != nil || summary == nil {
		t.Fatalf("Provider.RepositorySummary: %v (summary=%v)", err, summary)
	}
	if summary.Count != 250 {
		t.Errorf("Repositories.Count = %d, want 250", summary.Count)
	}
	if summary.Stargazers != 150 {
		t.Errorf("Stargazers = %d, want 150", summary.Stargazers)
	}
	if summary.Forks != 13 {
		t.Errorf("Forks = %d, want 13", summary.Forks)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Result.Errors = %v", res.Errors)
	}
	if got := fixture.calls.Load(); got < 2 {
		t.Errorf("expected at least 2 GraphQL calls, got %d", got)
	}
}

// TestEngine_ComputeOrganization is US5 AS3: organization dispatch
// uses the GraphQL Organization queries.
func TestEngine_ComputeOrganization(t *testing.T) {
	t.Parallel()

	deps, _ := newEngineDeps(t, map[string]string{
		"Organization":             orgGithub,
		"OrganizationMembers":      orgMembersEmpty,
		"OrganizationRepositories": orgRepositories12,
	})

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "github",
		Template: "noop",
		Account:  plugins.AccountOrganization,
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.Data.Account != plugins.AccountOrganization {
		t.Errorf("Account = %q", res.Data.Account)
	}
	if res.Provider == nil {
		t.Fatal("Result.Provider is nil")
	}
	org, err := res.Provider.Organization(context.Background())
	if err != nil || org == nil {
		t.Fatalf("Provider.Organization: %v (org=%v)", err, org)
	}
	if org.Login != "github" {
		t.Errorf("Organization.Login = %q, want github", org.Login)
	}
	summary, err := res.Provider.RepositorySummary(context.Background())
	if err != nil || summary == nil {
		t.Fatalf("Provider.RepositorySummary: %v", err)
	}
	if summary.Count != 12 {
		t.Errorf("Repositories.Count = %d, want 12", summary.Count)
	}
}

// TestEngine_RejectsEmptyLogin guards a common misuse case.
func TestEngine_RejectsEmptyLogin(t *testing.T) {
	t.Parallel()

	deps, _ := newEngineDeps(t, map[string]string{})
	_, err := engine.Compute(context.Background(), engine.Request{Login: ""}, deps)
	if err == nil {
		t.Fatalf("expected error for empty login")
	}
}

// TestEngine_NoopTemplateSkipsLookup verifies the "noop" sentinel
// skips templates.MustGet so tests can run without registering one.
func TestEngine_NoopTemplateSkipsLookup(t *testing.T) {
	t.Parallel()

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	if _, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "noop",
	}, deps); err != nil {
		t.Fatalf("noop template should not require registration: %v", err)
	}
}

// TestEngine_UnknownTemplateErrors confirms that any non-"noop"
// template name must be registered.
func TestEngine_UnknownTemplateErrors(t *testing.T) {
	t.Parallel()

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "no-such-template",
	}, deps)
	if err == nil {
		t.Fatalf("expected error for unregistered template")
	}
}
