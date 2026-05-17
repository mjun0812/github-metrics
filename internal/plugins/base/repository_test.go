package base_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestFetchRepo_HappyPath_UserOwner — happy path: GraphQL returns a
// User-owned repository, REST helpers return contributor + commit
// counts, plugins.Repo is fully populated.
func TestFetchRepo_HappyPath_UserOwner(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_hello_world.json")

	rest := mocks.NewRESTMux(t)
	rest.OnHeader(
		"/repos/octocat/hello-world/contributors",
		http.StatusOK,
		`[{"login":"octocat"}]`,
		http.Header{
			"Link": []string{`<https://api.github.com/repos/octocat/hello-world/contributors?per_page=1&anon=true&page=12>; rel="last"`},
		},
	)
	rest.OnFile("/repos/octocat/hello-world/commits", "github/rest/commits_3.json")

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
	r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", pc.REST, pc.GraphQL)
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
		t.Errorf("recent commits: %d, want 3", r.Activity.RecentCommits)
	}
	if r.Contributors != 12 {
		t.Errorf("contributors: %d, want 12 (Link rel=last page=12)", r.Contributors)
	}
}

// TestFetchRepo_OrganizationOwner exercises the genqlient
// Organization typename branch of the RepositoryOwner interface.
func TestFetchRepo_OrganizationOwner(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_organization.json")

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql))
	r, err := base.FetchRepo(context.Background(), "kubernetes", "kubernetes", pc.REST, pc.GraphQL)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Owner != "kubernetes" {
		t.Errorf("Owner = %q, want kubernetes", r.Owner)
	}
	if r.OwnerAvatar == "" {
		t.Error("OwnerAvatar should populate from Organization avatarUrl")
	}
	if r.LicenseName != "Apache License 2.0" {
		t.Errorf("License: %q", r.LicenseName)
	}
}

// TestFetchRepo_NotFound surfaces a 404 (nil repository in the
// response) as a typed *xerrors.InputError on the "repo" input.
func TestFetchRepo_NotFound(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_not_found.json")

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql))
	_, err := base.FetchRepo(context.Background(), "octocat", "nonexistent", pc.REST, pc.GraphQL)
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *xerrors.InputError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found'; got %v", err)
	}
}

// TestFetchRepo_NilGraphQLClient guards against programming errors.
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
	gql := mocks.NewGraphQLMux(t)
	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql))
	for _, tc := range []struct{ login, repo string }{
		{"", "hello-world"},
		{"octocat", ""},
		{"", ""},
	} {
		_, err := base.FetchRepo(context.Background(), tc.login, tc.repo, pc.REST, pc.GraphQL)
		if err == nil {
			t.Errorf("FetchRepo(%q, %q) should error", tc.login, tc.repo)
		}
	}
}

// TestFetchRepo_RESTContributors5xx_BestEffort — when the
// contributors REST call returns 5xx, FetchRepo MUST keep the
// GraphQL fields and surface Contributors=0 with a slog.Warn. The
// whole call still succeeds (best-effort REST contract).
func TestFetchRepo_RESTContributors5xx_BestEffort(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_hello_world.json")

	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/contributors",
		http.StatusInternalServerError, `{"message":"server error"}`)
	rest.OnBody("/repos/octocat/hello-world/commits", http.StatusOK, `[{"sha":"a"}]`)

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
	r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", pc.REST, pc.GraphQL)
	if err != nil {
		t.Fatalf("FetchRepo should not error on best-effort REST 5xx: %v", err)
	}
	if r.Contributors != 0 {
		t.Errorf("Contributors = %d, want 0 (REST best-effort failure)", r.Contributors)
	}
	if r.Activity.RecentCommits != 1 {
		t.Errorf("RecentCommits should still populate from successful commits call; got %d", r.Activity.RecentCommits)
	}
	if r.Stargazers != 80 || r.Name != "hello-world" {
		t.Errorf("GraphQL fields must survive REST failure: %+v", r)
	}
}

// TestFetchRepo_RESTCommits409_EmptyRepo — empty repos return 409 on
// /commits; FetchRepo treats this as zero (not an error).
func TestFetchRepo_RESTCommits409_EmptyRepo(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_hello_world.json")

	rest := mocks.NewRESTMux(t)
	rest.OnFile("/repos/octocat/hello-world/contributors", "github/rest/contributors_empty.json")
	rest.OnBody("/repos/octocat/hello-world/commits", http.StatusConflict,
		`{"message":"Git Repository is empty."}`)

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
	r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", pc.REST, pc.GraphQL)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if r.Activity.RecentCommits != 0 {
		t.Errorf("empty-repo 409 should map to RecentCommits=0; got %d", r.Activity.RecentCommits)
	}
}

// TestFetchRepo_NoRESTClient — nil REST client → REST-derived fields
// stay zero but FetchRepo still succeeds with GraphQL-only data.
func TestFetchRepo_NoRESTClient(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFile("Repository", "github/graphql/repository_hello_world.json")

	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql))
	r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", nil, pc.GraphQL)
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

// TestParseLinkLastPage_ViaFetchRepo exercises the contributors
// Link-header parser through varied Link headers.
func TestParseLinkLastPage_ViaFetchRepo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		linkHdr string
		wantNum int
	}{
		{"single page no header", "", 1},
		{"explicit last page=7", `<https://api.github.com/repos/octocat/hello-world/contributors?per_page=1&anon=true&page=7>; rel="last"`, 7},
		{"prev+last variant", `<https://api.github.com/x/x?page=1>; rel="prev", <https://api.github.com/x/x?page=42>; rel="last"`, 42},
		{"malformed link (no rel)", `<https://api.github.com/x?page=99>`, 1},
		{"no page= in link", `<https://api.github.com/x>; rel="last"`, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gql := mocks.NewGraphQLMux(t)
			gql.OnFile("Repository", "github/graphql/repository_hello_world.json")
			rest := mocks.NewRESTMux(t)
			hdr := http.Header{}
			if tc.linkHdr != "" {
				hdr.Set("Link", tc.linkHdr)
			}
			rest.OnHeader("/repos/octocat/hello-world/contributors",
				http.StatusOK, `[{"login":"octocat"}]`, hdr)
			rest.OnFile("/repos/octocat/hello-world/commits", "github/rest/commits_empty.json")

			pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
			r, err := base.FetchRepo(context.Background(), "octocat", "hello-world", pc.REST, pc.GraphQL)
			if err != nil {
				t.Fatalf("FetchRepo: %v", err)
			}
			if r.Contributors != tc.wantNum {
				t.Errorf("Contributors = %d, want %d", r.Contributors, tc.wantNum)
			}
		})
	}
}
