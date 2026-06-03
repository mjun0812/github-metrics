//go:build integration

// Cross-validation integration tests — verify that REST and GraphQL return
// consistent data for the endpoints where both APIs cover the same resource.
//
// Run with:
//
//	GITHUB_TOKEN=ghp_xxx GITHUB_USER=<login> \
//	  go test -tags integration -run TestCrossValidate ./internal/githubapi/...
//
// The tests skip automatically when the env vars are absent so they never
// block CI runs that don't provide credentials.
//
// Each test fetches the same "first page" (up to 100 items) from both REST
// and GraphQL and asserts that the resulting login / nameWithOwner sets are
// identical. Differences surface pagination bugs, off-by-one errors, or
// silent API-level discrepancies before they reach users.
package githubapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// crossValidateLimit is the page size used for follower / starred fetches.
const crossValidateLimit = 100

// repoPageSize is the initial batch size for UserRepositories. The query
// fetches many nested fields per node (languages, issues, pullRequests …)
// and the GitHub GraphQL complexity ceiling is easily hit. fetchGQLAllRepos
// halves this value on each retryable error, mirroring the base plugin's
// batch-halving strategy, until the batch is small enough to succeed.
const repoPageSize = 50

// newIntegrationClients reads GITHUB_TOKEN and GITHUB_USER from the
// environment, skips the test when either is missing, and returns ready-to-
// use REST and GraphQL clients alongside the login string.
func newIntegrationClients(t *testing.T) (*githubapi.REST, *githubapi.GraphQL, string) {
	t.Helper()
	token := os.Getenv("GITHUB_TOKEN")
	login := os.Getenv("GITHUB_USER")
	if token == "" || login == "" {
		t.Skip("skipping: GITHUB_TOKEN and GITHUB_USER must be set for cross-validation tests")
	}

	rest, err := githubapi.NewREST(config.NewToken(token), "", httpx.Options{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	gql, err := githubapi.NewGraphQL(config.NewToken(token), "", httpx.Options{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return rest, gql, login
}

// fetchRESTLogins calls a paginated REST endpoint that returns an array of
// objects with a "login" field and collects up to limit unique logins.
// path must NOT include per_page — it is appended here.
func fetchRESTLogins(t *testing.T, rest *githubapi.REST, ctx context.Context, pathWithoutPaging string, limit int) []string {
	t.Helper()
	path := fmt.Sprintf("%s?per_page=%d", pathWithoutPaging, limit)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		t.Fatalf("REST GET %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("REST GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	var nodes []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &nodes); err != nil {
		t.Fatalf("REST GET %s: decode: %v", path, err)
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Login != "" {
			out = append(out, strings.ToLower(n.Login))
		}
	}
	return out
}

// fetchRESTRepoNames calls a REST endpoint that returns an array of repository
// objects and collects up to limit nameWithOwner strings (REST field: "full_name").
func fetchRESTRepoNames(t *testing.T, rest *githubapi.REST, ctx context.Context, pathWithoutPaging string, limit int) []string {
	t.Helper()
	path := fmt.Sprintf("%s?per_page=%d", pathWithoutPaging, limit)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		t.Fatalf("REST GET %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("REST GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	var nodes []struct {
		FullName string `json:"full_name"`
	}
	if err := json.Unmarshal(body, &nodes); err != nil {
		t.Fatalf("REST GET %s: decode: %v", path, err)
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.FullName != "" {
			out = append(out, strings.ToLower(n.FullName))
		}
	}
	return out
}

// fetchGQLAllRepos paginates UserRepositories until exhausted and returns
// every nameWithOwner.
//
// The query fetches many nested fields per node (languages, issues,
// pullRequests …) so the GitHub GraphQL complexity ceiling is hit at large
// batch sizes. This function mirrors the base plugin's batch-halving
// strategy: when a page request fails it halves the batch size and retries
// the same cursor rather than skipping ahead.
func fetchGQLAllRepos(t *testing.T, gql *githubapi.GraphQL, ctx context.Context, login string) []string {
	t.Helper()
	var cursor *string
	var all []string
	batch := repoPageSize
	for {
		resp, err := gql.UserRepositories(ctx, login, batch, cursor)
		if err != nil {
			if batch > 2 {
				batch /= 2
				t.Logf("repositories: GraphQL complexity error, retrying with batch=%d: %v", batch, err)
				continue
			}
			t.Fatalf("GraphQL UserRepositories (batch=%d): %v", batch, err)
		}
		if resp == nil || resp.GetUser() == nil || resp.GetUser().GetRepositories() == nil {
			break
		}
		conn := resp.GetUser().GetRepositories()
		for _, n := range conn.GetNodes() {
			if n != nil {
				all = append(all, strings.ToLower(n.GetNameWithOwner()))
			}
		}
		pi := conn.GetPageInfo()
		if pi == nil || !pi.GetHasNextPage() {
			break
		}
		cursor = pi.GetEndCursor()
	}
	return all
}

// fetchRESTAllRepos paginates GET /user/repos (authenticated user) until
// the last partial page and returns every full_name.
//
// /users/{login}/repos is intentionally NOT used here because that endpoint
// returns only PUBLIC repositories regardless of authentication. /user/repos
// returns all repositories (public + private) for the authenticated caller,
// matching what the GraphQL UserRepositories query returns when the viewer
// owns the account.
func fetchRESTAllRepos(t *testing.T, rest *githubapi.REST, ctx context.Context, _ string) []string {
	t.Helper()
	var all []string
	page := 1
	for {
		path := fmt.Sprintf("/user/repos?per_page=%d&type=owner&page=%d", crossValidateLimit, page)
		body, resp, err := rest.Get(ctx, path, nil)
		if err != nil {
			t.Fatalf("REST GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("REST GET %s: status %d: %s", path, resp.StatusCode, string(body))
		}
		var nodes []struct {
			FullName string `json:"full_name"`
		}
		if err := json.Unmarshal(body, &nodes); err != nil {
			t.Fatalf("REST GET %s: decode: %v", path, err)
		}
		if len(nodes) == 0 {
			break
		}
		for _, n := range nodes {
			if n.FullName != "" {
				all = append(all, strings.ToLower(n.FullName))
			}
		}
		// REST uses the Link header for pagination; stop when we get a
		// partial page (< per_page items) — that means we've hit the last
		// page without needing to parse the Link header.
		if len(nodes) < crossValidateLimit {
			break
		}
		page++
	}
	return all
}

// assertSetsMatch fails the test when want and got (treated as unordered
// sets) differ. It reports both the missing and the extra elements so the
// diff is actionable without the caller needing to sort manually.
func assertSetsMatch(t *testing.T, label string, want, got []string) {
	t.Helper()
	wantSet := make(map[string]struct{}, len(want))
	for _, v := range want {
		wantSet[v] = struct{}{}
	}
	gotSet := make(map[string]struct{}, len(got))
	for _, v := range got {
		gotSet[v] = struct{}{}
	}
	var missing, extra []string
	for v := range wantSet {
		if _, ok := gotSet[v]; !ok {
			missing = append(missing, v)
		}
	}
	for v := range gotSet {
		if _, ok := wantSet[v]; !ok {
			extra = append(extra, v)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	t.Errorf("%s mismatch (REST=%d GraphQL=%d): missing from GraphQL=%v extra in GraphQL=%v",
		label, len(want), len(got), missing, extra)
}

// assertGQLSubsetOfREST fails the test when any GraphQL-returned item is not
// present in the REST result. REST may legally return extra items (e.g.
// organization accounts that the GraphQL User connection silently drops),
// but GraphQL must never return items that REST does not know about.
func assertGQLSubsetOfREST(t *testing.T, label string, rest, gql []string) {
	t.Helper()
	restSet := make(map[string]struct{}, len(rest))
	for _, v := range rest {
		restSet[v] = struct{}{}
	}
	var onlyInGQL []string
	for _, v := range gql {
		if _, ok := restSet[v]; !ok {
			onlyInGQL = append(onlyInGQL, v)
		}
	}
	sort.Strings(onlyInGQL)
	if len(onlyInGQL) > 0 {
		t.Errorf("%s: GraphQL returned items not present in REST (REST=%d GraphQL=%d): %v",
			label, len(rest), len(gql), onlyInGQL)
	}
	// REST-only items are expected to be organization accounts that the
	// GraphQL User connection omits — log them as informational.
	gqlSet := make(map[string]struct{}, len(gql))
	for _, v := range gql {
		gqlSet[v] = struct{}{}
	}
	var onlyInREST []string
	for _, v := range rest {
		if _, ok := gqlSet[v]; !ok {
			onlyInREST = append(onlyInREST, v)
		}
	}
	sort.Strings(onlyInREST)
	if len(onlyInREST) > 0 {
		t.Logf("%s: REST-only entries (likely org accounts omitted by GraphQL User connection): %v",
			label, onlyInREST)
	}
}

// TestCrossValidate_Followers verifies that the first page of followers
// returned by REST and GraphQL are identical.
//
// REST:    GET /users/{login}/followers?per_page=100
// GraphQL: user.followers(first: 100)
func TestCrossValidate_Followers(t *testing.T) {
	rest, gql, login := newIntegrationClients(t)
	ctx := context.Background()

	restLogins := fetchRESTLogins(t, rest, ctx,
		fmt.Sprintf("/users/%s/followers", login), crossValidateLimit)

	resp, err := gql.UserFollowers(ctx, login, crossValidateLimit, 48)
	if err != nil {
		t.Fatalf("GraphQL UserFollowers: %v", err)
	}
	var gqlLogins []string
	if resp != nil && resp.GetUser() != nil && resp.GetUser().GetFollowers() != nil {
		for _, n := range resp.GetUser().GetFollowers().GetNodes() {
			if n != nil {
				gqlLogins = append(gqlLogins, strings.ToLower(n.GetLogin()))
			}
		}
	}

	t.Logf("followers: REST=%d GraphQL=%d", len(restLogins), len(gqlLogins))
	assertSetsMatch(t, "followers", restLogins, gqlLogins)
}

// TestCrossValidate_Following verifies that the first page of following
// returned by REST and GraphQL are consistent.
//
// REST:    GET /users/{login}/following?per_page=100
// GraphQL: user.following(first: 100)  — same UserFollowers call as above
//
// Known difference: REST returns both User and Organization accounts;
// the GraphQL UserConnection only contains User nodes, so organizations
// the user follows appear in REST but not in GraphQL. The test therefore
// asserts the weaker condition: every GraphQL result must appear in REST
// (GraphQL ⊆ REST), and logs REST-only entries for visibility.
func TestCrossValidate_Following(t *testing.T) {
	rest, gql, login := newIntegrationClients(t)
	ctx := context.Background()

	restLogins := fetchRESTLogins(t, rest, ctx,
		fmt.Sprintf("/users/%s/following", login), crossValidateLimit)

	resp, err := gql.UserFollowers(ctx, login, crossValidateLimit, 48)
	if err != nil {
		t.Fatalf("GraphQL UserFollowers (following): %v", err)
	}
	var gqlLogins []string
	if resp != nil && resp.GetUser() != nil && resp.GetUser().GetFollowing() != nil {
		for _, n := range resp.GetUser().GetFollowing().GetNodes() {
			if n != nil {
				gqlLogins = append(gqlLogins, strings.ToLower(n.GetLogin()))
			}
		}
	}

	t.Logf("following: REST=%d GraphQL=%d", len(restLogins), len(gqlLogins))
	assertGQLSubsetOfREST(t, "following", restLogins, gqlLogins)
}

// TestCrossValidate_Repositories verifies that the complete set of
// owner-affiliated repositories is identical across REST and GraphQL.
// Both sides are fully paginated so no repos are missed.
//
// REST:    GET /users/{login}/repos?type=owner  (paginated)
// GraphQL: UserRepositories(login, 100, cursor) (paginated)
func TestCrossValidate_Repositories(t *testing.T) {
	rest, gql, login := newIntegrationClients(t)
	ctx := context.Background()

	restNames := fetchRESTAllRepos(t, rest, ctx, login)
	gqlNames := fetchGQLAllRepos(t, gql, ctx, login)

	t.Logf("repositories: REST=%d GraphQL=%d", len(restNames), len(gqlNames))
	assertSetsMatch(t, "repositories", restNames, gqlNames)
}

// TestCrossValidate_StarredRepositories verifies that the first page of
// starred repositories (newest-first) is identical across REST and GraphQL.
//
// REST:    GET /users/{login}/starred?per_page=100&sort=created&direction=desc
// GraphQL: user.starredRepositories(first: 100, orderBy: STARRED_AT DESC)
func TestCrossValidate_StarredRepositories(t *testing.T) {
	rest, gql, login := newIntegrationClients(t)
	ctx := context.Background()

	// REST fetch — include sort params so ordering matches the GraphQL side.
	restPath := fmt.Sprintf("/users/%s/starred?per_page=%d&sort=created&direction=desc", login, crossValidateLimit)
	restBody, restResp, err := rest.Get(ctx, restPath, nil)
	if err != nil {
		t.Fatalf("REST GET %s: %v", restPath, err)
	}
	if restResp.StatusCode != http.StatusOK {
		t.Fatalf("REST GET %s: status %d: %s", restPath, restResp.StatusCode, string(restBody))
	}
	var rawRepos []struct {
		FullName string `json:"full_name"`
	}
	if err := json.Unmarshal(restBody, &rawRepos); err != nil {
		t.Fatalf("REST GET %s: decode: %v", restPath, err)
	}
	restNames := make([]string, 0, len(rawRepos))
	for _, r := range rawRepos {
		if r.FullName != "" {
			restNames = append(restNames, strings.ToLower(r.FullName))
		}
	}

	// GraphQL fetch — orderBy: STARRED_AT DESC mirrors the REST sort above.
	gqlResp, err := gql.UserStarredRepositories(ctx, login, crossValidateLimit)
	if err != nil {
		t.Fatalf("GraphQL UserStarredRepositories: %v", err)
	}
	var gqlNames []string
	if gqlResp != nil && gqlResp.GetUser() != nil && gqlResp.GetUser().GetStarredRepositories() != nil {
		for _, edge := range gqlResp.GetUser().GetStarredRepositories().GetEdges() {
			if edge != nil && edge.GetNode() != nil {
				gqlNames = append(gqlNames, strings.ToLower(edge.GetNode().GetNameWithOwner()))
			}
		}
	}

	t.Logf("starred repositories: REST=%d GraphQL=%d", len(restNames), len(gqlNames))
	assertSetsMatch(t, "starred repositories", restNames, gqlNames)
}
