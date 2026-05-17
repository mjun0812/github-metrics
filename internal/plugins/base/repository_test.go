package base_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
)

// repositoryHelloWorldFixture is the canned `Repository` GraphQL
// response matching internal/githubapi/queries/repository.graphql.
const repositoryHelloWorldFixture = `{
  "data": {
    "repository": {
      "databaseId": 1296269,
      "name": "hello-world",
      "nameWithOwner": "octocat/hello-world",
      "description": "My first repository on GitHub.",
      "stargazerCount": 80,
      "forkCount": 9,
      "isArchived": false,
      "primaryLanguage": { "name": "Go", "color": "#00ADD8" },
      "licenseInfo": { "name": "MIT License", "spdxId": "MIT" },
      "defaultBranchRef": { "name": "master" },
      "owner": { "__typename": "User", "login": "octocat", "avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4" },
      "issues": { "totalCount": 5 },
      "pullRequests": { "totalCount": 2 }
    }
  }
}`

const repositoryOrgOwnerFixture = `{
  "data": {
    "repository": {
      "databaseId": 9876,
      "name": "kubernetes",
      "nameWithOwner": "kubernetes/kubernetes",
      "description": "Production-grade container orchestration",
      "stargazerCount": 100000,
      "forkCount": 30000,
      "isArchived": false,
      "primaryLanguage": { "name": "Go", "color": "#00ADD8" },
      "licenseInfo": { "name": "Apache License 2.0", "spdxId": "Apache-2.0" },
      "defaultBranchRef": { "name": "master" },
      "owner": { "__typename": "Organization", "login": "kubernetes", "avatarUrl": "https://avatars.githubusercontent.com/u/13629408?v=4" },
      "issues": { "totalCount": 2500 },
      "pullRequests": { "totalCount": 1200 }
    }
  }
}`

const repositoryNotFoundFixture = `{ "data": { "repository": null } }`

// restRouter is a small RoundTripper used by the FetchRepo tests to
// canned-respond per request path. Unknown paths return 404.
type restRouter struct {
	mu       sync.Mutex
	handlers map[string]restEntry
}

type restEntry struct {
	status int
	body   string
	header http.Header
}

func (r *restRouter) on(path string, e restEntry) {
	if r.handlers == nil {
		r.handlers = map[string]restEntry{}
	}
	r.handlers[path] = e
}

func (r *restRouter) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.handlers[req.URL.Path]
	if !ok {
		return mkRESTResponse(req, http.StatusNotFound, nil, `{"message":"Not Found"}`), nil
	}
	return mkRESTResponse(req, e.status, e.header, e.body), nil
}

func mkRESTResponse(req *http.Request, status int, h http.Header, body string) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func newRESTClient(t *testing.T, router *restRouter) *githubapi.REST {
	t.Helper()
	c, err := githubapi.NewREST(config.NewToken("MOCKED_TOKEN"), "http://mock.localhost",
		httpx.Options{Transport: router, DisableRetries: true})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return c
}

// TestFetchRepo_HappyPath_UserOwner — happy path: GraphQL returns a
// User-owned repository, REST helpers return contributor + commit
// counts, plugins.Repo is fully populated.
func TestFetchRepo_HappyPath_UserOwner(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryHelloWorldFixture})

	rest := &restRouter{}
	rest.on("/repos/octocat/hello-world/contributors", restEntry{
		status: http.StatusOK,
		body:   `[{"login":"octocat"}]`,
		header: http.Header{"Link": []string{`<https://api.github.com/repos/octocat/hello-world/contributors?per_page=1&anon=true&page=12>; rel="last"`}},
	})
	rest.on("/repos/octocat/hello-world/commits", restEntry{
		status: http.StatusOK,
		body:   `[{"sha":"a"},{"sha":"b"},{"sha":"c"}]`,
	})

	r, err := base.FetchRepo(
		context.Background(), "octocat", "hello-world",
		newRESTClient(t, rest), newGraphQL(t, mux),
	)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Owner != "octocat" || r.Name != "hello-world" {
		t.Errorf("identity: %+v", r)
	}
	if r.Stargazers != 80 || r.Forks != 9 {
		t.Errorf("counts: stars=%d forks=%d", r.Stargazers, r.Forks)
	}
	if r.PrimaryLanguage != "Go" || r.PrimaryLanguageColor != "#00ADD8" {
		t.Errorf("language: %q/%q", r.PrimaryLanguage, r.PrimaryLanguageColor)
	}
	if r.LicenseName != "MIT License" {
		t.Errorf("license: %q", r.LicenseName)
	}
	if r.DefaultBranch != "master" {
		t.Errorf("default_branch: %q", r.DefaultBranch)
	}
	if r.Activity.OpenIssues != 5 || r.Activity.OpenPullRequests != 2 {
		t.Errorf("activity: open=%d/%d", r.Activity.OpenIssues, r.Activity.OpenPullRequests)
	}
	if r.Activity.RecentCommits != 3 {
		t.Errorf("recent commits: %d, want 3 (REST list length)", r.Activity.RecentCommits)
	}
	if r.Contributors != 12 {
		t.Errorf("contributors: %d, want 12 (Link header rel=last page=12)", r.Contributors)
	}
}

// TestFetchRepo_OrganizationOwner exercises the genqlient Organization
// typename branch of the RepositoryOwner interface.
func TestFetchRepo_OrganizationOwner(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryOrgOwnerFixture})

	rest := &restRouter{}
	r, err := base.FetchRepo(
		context.Background(), "kubernetes", "kubernetes",
		newRESTClient(t, rest), newGraphQL(t, mux),
	)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Owner != "kubernetes" {
		t.Errorf("Owner = %q, want kubernetes (from Organization owner branch)", r.Owner)
	}
	if r.OwnerAvatar == "" {
		t.Error("OwnerAvatar should populate from Organization avatarUrl")
	}
	if r.LicenseName != "Apache License 2.0" {
		t.Errorf("License: %q", r.LicenseName)
	}
}

// TestFetchRepo_NotFound surfaces a 404 (nil repository in the
// response) as a typed *xerrors.InputError on the "repo" input so the
// M6 RetryPolicy classifies it as non-retryable (fail-fast).
func TestFetchRepo_NotFound(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryNotFoundFixture})

	_, err := base.FetchRepo(
		context.Background(), "octocat", "nonexistent",
		newRESTClient(t, &restRouter{}), newGraphQL(t, mux),
	)
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *xerrors.InputError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message should mention 'not found'; got %v", err)
	}
}

// TestFetchRepo_NilGraphQLClient guards against programming errors —
// callers passing nil for the GraphQL client should fail clearly,
// not panic later inside the response decoder.
func TestFetchRepo_NilGraphQLClient(t *testing.T) {
	t.Parallel()
	_, err := base.FetchRepo(context.Background(), "octocat", "hello-world", nil, nil)
	if err == nil {
		t.Fatal("expected error for nil GraphQL client")
	}
	if !strings.Contains(err.Error(), "GraphQL") {
		t.Errorf("error should mention GraphQL; got %v", err)
	}
}

// TestFetchRepo_EmptyArgs guards the login/repo non-empty contract.
func TestFetchRepo_EmptyArgs(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	for _, tc := range []struct{ login, repo string }{
		{"", "hello-world"},
		{"octocat", ""},
		{"", ""},
	} {
		_, err := base.FetchRepo(context.Background(), tc.login, tc.repo,
			newRESTClient(t, &restRouter{}), newGraphQL(t, mux))
		if err == nil {
			t.Errorf("FetchRepo(%q, %q) should error", tc.login, tc.repo)
		}
	}
}

// TestFetchRepo_RESTContributors5xx_BestEffort — when the contributors
// REST call returns 5xx, FetchRepo MUST keep the GraphQL fields and
// surface Contributors=0 with a slog.Warn. The whole call still
// succeeds (best-effort REST contract).
func TestFetchRepo_RESTContributors5xx_BestEffort(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryHelloWorldFixture})

	rest := &restRouter{}
	rest.on("/repos/octocat/hello-world/contributors", restEntry{
		status: http.StatusInternalServerError,
		body:   `{"message":"server error"}`,
	})
	rest.on("/repos/octocat/hello-world/commits", restEntry{
		status: http.StatusOK,
		body:   `[{"sha":"a"}]`,
	})

	r, err := base.FetchRepo(
		context.Background(), "octocat", "hello-world",
		newRESTClient(t, rest), newGraphQL(t, mux),
	)
	if err != nil {
		t.Fatalf("FetchRepo should not error on best-effort REST 5xx: %v", err)
	}
	if r.Contributors != 0 {
		t.Errorf("Contributors = %d, want 0 (REST best-effort failure)", r.Contributors)
	}
	if r.Activity.RecentCommits != 1 {
		t.Errorf("RecentCommits should still populate from the successful commits call; got %d", r.Activity.RecentCommits)
	}
	// GraphQL-sourced fields must remain intact.
	if r.Stargazers != 80 || r.Name != "hello-world" {
		t.Errorf("GraphQL fields must survive REST failure: %+v", r)
	}
}

// TestFetchRepo_RESTCommits409_EmptyRepo — empty repos return 409 on
// the /commits endpoint; FetchRepo treats this as zero commits (not
// an error).
func TestFetchRepo_RESTCommits409_EmptyRepo(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryHelloWorldFixture})

	rest := &restRouter{}
	rest.on("/repos/octocat/hello-world/contributors", restEntry{
		status: http.StatusOK,
		body:   `[]`,
	})
	rest.on("/repos/octocat/hello-world/commits", restEntry{
		status: http.StatusConflict,
		body:   `{"message":"Git Repository is empty."}`,
	})

	r, err := base.FetchRepo(
		context.Background(), "octocat", "hello-world",
		newRESTClient(t, rest), newGraphQL(t, mux),
	)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Activity.RecentCommits != 0 {
		t.Errorf("empty-repo 409 should map to RecentCommits=0; got %d", r.Activity.RecentCommits)
	}
}

// TestFetchRepo_NoRESTClient — when the caller passes a nil REST
// client, the contributors + commits fields stay at zero but FetchRepo
// still succeeds with the GraphQL-only payload (used by tests that
// don't want to set up REST mocks).
func TestFetchRepo_NoRESTClient(t *testing.T) {
	t.Parallel()
	mux := newGraphQLMux()
	mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryHelloWorldFixture})

	r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", nil, newGraphQL(t, mux))
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Contributors != 0 || r.Activity.RecentCommits != 0 {
		t.Errorf("nil REST client should leave REST-derived fields zero; got %+v", r)
	}
	if r.Stargazers != 80 {
		t.Errorf("GraphQL fields should populate; got Stargazers=%d", r.Stargazers)
	}
}

// TestParseLinkLastPage table tests the contributors-count Link header
// parsing trick used by fetchContributorsCount. The function is
// package-private; we exercise it indirectly via the contributors
// REST call's Link-header path, but the canonical inputs are easier
// to assert through a tabular wrapper. This test reaches the parser
// via FetchRepo with varied Link headers.
func TestParseLinkLastPage_ViaFetchRepo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		linkHdr string
		wantNum int
	}{
		{"single page no header", "", 1}, // fallback to len(parsed JSON)
		{"explicit last page=7", `<https://api.github.com/repos/octocat/hello-world/contributors?per_page=1&anon=true&page=7>; rel="last"`, 7},
		{"prev+last variant", `<https://api.github.com/x/x?page=1>; rel="prev", <https://api.github.com/x/x?page=42>; rel="last"`, 42},
		{"malformed link (no rel)", `<https://api.github.com/x?page=99>`, 1},
		{"no page= in link", `<https://api.github.com/x>; rel="last"`, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux := newGraphQLMux()
			mux.OnSequence("Repository", gqlResp{Status: 200, Body: repositoryHelloWorldFixture})
			rest := &restRouter{}
			hdr := http.Header{}
			body := `[{"login":"octocat"}]`
			if tc.linkHdr != "" {
				hdr.Set("Link", tc.linkHdr)
			}
			rest.on("/repos/octocat/hello-world/contributors", restEntry{
				status: http.StatusOK,
				body:   body,
				header: hdr,
			})
			rest.on("/repos/octocat/hello-world/commits", restEntry{status: http.StatusOK, body: `[]`})

			r, err := base.FetchRepo(context.Background(), "octocat", "hello-world",
				newRESTClient(t, rest), newGraphQL(t, mux))
			if err != nil {
				t.Fatalf("FetchRepo: %v", err)
			}
			if r.Contributors != tc.wantNum {
				t.Errorf("Contributors = %d, want %d", r.Contributors, tc.wantNum)
			}
		})
	}
}
