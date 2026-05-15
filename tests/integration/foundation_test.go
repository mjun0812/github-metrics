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
				"company": "@github"
			}
		}
	}`

	userRepositories250 = `{
		"data": {
			"user": {
				"repositories": {
					"totalCount": 250,
					"pageInfo": {"hasNextPage": true, "endCursor": "Y3Vyc29yOnYyOpHOABcZJg=="},
					"nodes": [
						{"databaseId": 1, "id": "R_kgDOA", "name": "alpha", "nameWithOwner": "octocat/alpha", "isFork": false, "stargazerCount": 100, "forkCount": 10, "watchers": {"totalCount": 5}},
						{"databaseId": 2, "id": "R_kgDOB", "name": "beta",  "nameWithOwner": "octocat/beta",  "isFork": false, "stargazerCount":  50, "forkCount":  3, "watchers": {"totalCount": 2}}
					]
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

	orgRepositories12 = `{
		"data": {
			"organization": {
				"repositories": {
					"totalCount": 12,
					"pageInfo": {"hasNextPage": false, "endCursor": null},
					"nodes": [
						{"databaseId": 100, "id": "R_kgDOX", "name": "site", "nameWithOwner": "github/site", "isFork": false, "stargazerCount": 7, "forkCount": 1, "watchers": {"totalCount": 4}}
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
		// exercised without starting chromium. Tests that need real
		// chromedp behavior live under the chromedp build tag.
		Render: &render.FakeRenderer{},
	}, fixture
}

// TestEngine_ComputeUser is US5 AS1 + AS2 combined: with mocked
// GraphQL returning octocat and 250 repositories, Compute populates
// Data.User.Login and Data.Computed.Repositories.Count.
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

	if res.Data.User == nil {
		t.Fatalf("Data.User is nil")
	}
	if res.Data.User.Login != "octocat" {
		t.Errorf("login = %q", res.Data.User.Login)
	}
	if got := res.Data.Computed.Repositories.Count; got != 250 {
		t.Errorf("Repositories.Count = %d, want 250", got)
	}
	if got := res.Data.Computed.Repositories.Stargazers; got != 150 {
		t.Errorf("Stargazers = %d, want 150", got)
	}
	if got := res.Data.Computed.Repositories.Forks; got != 13 {
		t.Errorf("Forks = %d, want 13", got)
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
	if res.Data.User == nil || res.Data.User.Login != "github" {
		t.Errorf("Data.User = %+v", res.Data.User)
	}
	if got := res.Data.Computed.Repositories.Count; got != 12 {
		t.Errorf("Repositories.Count = %d, want 12", got)
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
