package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/repositories"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

func octocatRepos() []plugins.Repository {
	return []plugins.Repository{
		{NameWithOwner: "octocat/alpha", URL: "https://github.com/octocat/alpha", Stars: 100, Forks: 30},
		{NameWithOwner: "octocat/beta", URL: "https://github.com/octocat/beta", Stars: 80, Forks: 50},
		{NameWithOwner: "octocat/gamma", URL: "https://github.com/octocat/gamma", Stars: 200, Forks: 10},
		{NameWithOwner: "octocat/delta", URL: "https://github.com/octocat/delta", Stars: 5, Forks: 2, IsFork: true},
		{NameWithOwner: "octocat/epsilon", URL: "https://github.com/octocat/epsilon", Stars: 50, Forks: 5},
	}
}

func run(t *testing.T, repos []plugins.Repository, in map[string]any) *repositories.Result {
	t.Helper()
	data := plugins.NewData()
	data.Computed.RepositoryList = repos
	pc := &plugins.PluginContext{Inputs: in, Data: data}
	out, err := repositories.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*repositories.Result)
}

// TestRun_FeaturedDescByStars asserts default sort is by Stars desc.
func TestRun_FeaturedDescByStars(t *testing.T) {
	t.Parallel()
	r := run(t, octocatRepos(), nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if len(r.Featured) == 0 {
		t.Fatal("Featured empty")
	}
	// gamma 200 > alpha 100 > beta 80 > epsilon 50; fork delta dropped
	want := []string{"octocat/gamma", "octocat/alpha", "octocat/beta", "octocat/epsilon"}
	got := nameList(r.Featured)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Featured names = %v, want %v", got, want)
	}
}

// TestRun_OrderForks switches order to "forks" and verifies sort.
func TestRun_OrderForks(t *testing.T) {
	t.Parallel()
	r := run(t, octocatRepos(), map[string]any{
		"plugin_repositories_order": "forks",
	})
	want := []string{"octocat/beta", "octocat/alpha", "octocat/gamma", "octocat/epsilon"}
	got := nameList(r.Featured)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Featured (by forks) = %v, want %v", got, want)
	}
}

// TestRun_IncludeForks asserts _forks=true keeps the fork repo.
func TestRun_IncludeForks(t *testing.T) {
	t.Parallel()
	r := run(t, octocatRepos(), map[string]any{
		"plugin_repositories_forks": true,
	})
	for _, e := range r.Featured {
		if e.NameWithOwner == "octocat/delta" {
			return
		}
	}
	t.Errorf("expected octocat/delta to be included; got %v", nameList(r.Featured))
}

// TestRun_RandomSeedDeterministic asserts the same seed gives the same
// fisher-yates ordering across runs.
func TestRun_RandomSeedDeterministic(t *testing.T) {
	t.Parallel()
	in := map[string]any{
		"plugin_repositories_random":      true,
		"plugin_repositories_random_seed": 42,
	}
	r1 := run(t, octocatRepos(), in)
	r2 := run(t, octocatRepos(), in)
	if !reflect.DeepEqual(nameList(r1.Random), nameList(r2.Random)) {
		t.Errorf("random ordering not deterministic\n1=%v\n2=%v",
			nameList(r1.Random), nameList(r2.Random))
	}
}

// TestRun_Skipped — empty input yields Skipped=true.
func TestRun_Skipped(t *testing.T) {
	t.Parallel()
	r := run(t, nil, nil)
	if !r.Skipped {
		t.Errorf("expected Skipped=true; got %+v", r)
	}
}

// Golden tests.
func TestPartial_Repositories_Golden(t *testing.T) {
	r := &repositories.Result{
		Featured: []plugins.Repository{
			{NameWithOwner: "octocat/gamma", URL: "https://github.com/octocat/gamma", Stars: 200, Forks: 10},
			{NameWithOwner: "octocat/alpha", URL: "https://github.com/octocat/alpha", Stars: 100, Forks: 30},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(repositories.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := repositories.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "repositories.svg")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	if !strings.Contains(got, `class="repository"`) {
		t.Errorf("partial missing repository marker")
	}
}

func TestRun_GoldenShape_Repositories(t *testing.T) {
	r := &repositories.Result{
		Featured: []plugins.Repository{
			{NameWithOwner: "octocat/gamma", URL: "https://github.com/octocat/gamma", Stars: 200, Forks: 10},
		},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "repositories.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), string(got))
	}
}

func nameList(rs []plugins.Repository) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.NameWithOwner
	}
	return out
}

// === 012 starred fetch tests =========================================
//
// Contract: specs/012-rest-data-fetch/contracts/plugin-repositories-starred.md

// runWithStarred is like run() but injects a *githubapi.REST built from
// MockTransport so the live fetchStarred path is exercised. Returns
// both the Result and the Data so the caller can inspect Errors.
func runWithStarred(t *testing.T, repos []plugins.Repository, login, body string, status int) (*repositories.Result, *plugins.Data) {
	t.Helper()
	mux := githubapi.NewMockTransport()
	resp := githubapi.MockResponse{Status: status, Body: []byte(body)}
	mux.Set("GET", "/users/"+login+"/starred", resp)
	mux.Set("GET", "/users/"+login+"/starred*", resp)
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0, DisableRetries: true},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	data := plugins.NewData()
	data.Computed.RepositoryList = repos
	data.User = &plugins.User{Login: login}
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"plugin_repositories":         true,
			"plugin_repositories_starred": true,
			"plugin_repositories_limit":   4,
		},
		Data: data,
		REST: rest,
	}
	out, err := repositories.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*repositories.Result), data
}

const starredFixtureFourRepos = `[
	{"full_name":"sindresorhus/awesome","description":"Awesome lists","html_url":"https://github.com/sindresorhus/awesome","private":false,"fork":false,"stargazers_count":250000,"forks_count":24000,"watchers_count":250000,"language":"Markdown"},
	{"full_name":"golang/go","description":"The Go programming language","html_url":"https://github.com/golang/go","private":false,"fork":false,"stargazers_count":118000,"forks_count":17500,"watchers_count":118000,"language":"Go"},
	{"full_name":"rust-lang/rust","description":"Empowering everyone","html_url":"https://github.com/rust-lang/rust","private":false,"fork":false,"stargazers_count":92000,"forks_count":12000,"watchers_count":92000,"language":"Rust"},
	{"full_name":"torvalds/linux","description":"Linux kernel source tree","html_url":"https://github.com/torvalds/linux","private":false,"fork":false,"stargazers_count":170000,"forks_count":54000,"watchers_count":170000,"language":"C"}
]`

// TC-001: Happy path — 4 starred repos returned, distinct from Featured.
func TestRun_Starred_HappyPath(t *testing.T) {
	t.Parallel()
	r, data := runWithStarred(t, octocatRepos(), "octocat", starredFixtureFourRepos, 200)
	if len(r.Starred) != 4 {
		t.Fatalf("Starred len = %d, want 4", len(r.Starred))
	}
	wantFirst := "sindresorhus/awesome"
	if r.Starred[0].NameWithOwner != wantFirst {
		t.Errorf("Starred[0] = %q, want %q", r.Starred[0].NameWithOwner, wantFirst)
	}
	// SC-001: at least one Starred entry must not match any Featured entry.
	featured := map[string]bool{}
	for _, f := range r.Featured {
		featured[f.NameWithOwner] = true
	}
	overlap := 0
	for _, s := range r.Starred {
		if featured[s.NameWithOwner] {
			overlap++
		}
	}
	if overlap == len(r.Starred) {
		t.Errorf("all Starred entries overlap with Featured — fetch likely returned the placeholder")
	}
	if errs := data.SnapshotErrors(); len(errs) != 0 {
		t.Errorf("unexpected Data.Errors: %v", errs)
	}
}

// TC-002: Mapping correctness — private + fork + language fields land on Repository.
func TestRun_Starred_MappingCorrectness(t *testing.T) {
	t.Parallel()
	body := `[
		{"full_name":"acme/secret","description":"private one","html_url":"https://github.com/acme/secret","private":true,"fork":false,"stargazers_count":3,"forks_count":1,"watchers_count":3,"language":"Go"},
		{"full_name":"forky/clone","description":"forked","html_url":"https://github.com/forky/clone","private":false,"fork":true,"stargazers_count":10,"forks_count":0,"watchers_count":10,"language":""}
	]`
	r, _ := runWithStarred(t, octocatRepos(), "octocat", body, 200)
	if len(r.Starred) != 2 {
		t.Fatalf("Starred len = %d, want 2", len(r.Starred))
	}
	if r.Starred[0].Visibility != "private" {
		t.Errorf("Starred[0].Visibility = %q, want private", r.Starred[0].Visibility)
	}
	if r.Starred[0].Language == nil || r.Starred[0].Language.Name != "Go" {
		t.Errorf("Starred[0].Language = %+v, want {Name:Go}", r.Starred[0].Language)
	}
	if !r.Starred[1].IsFork {
		t.Errorf("Starred[1].IsFork = false, want true")
	}
	if r.Starred[1].Language != nil {
		t.Errorf("Starred[1].Language = %+v, want nil for empty language string", r.Starred[1].Language)
	}
}

// TC-003: Network failure (5xx) — Starred is nil, RetryableError recorded.
func TestRun_Starred_NetworkFailure(t *testing.T) {
	t.Parallel()
	r, data := runWithStarred(t, octocatRepos(), "octocat", `{"message":"boom"}`, 502)
	if r.Starred != nil {
		t.Errorf("Starred = %v, want nil on 5xx", r.Starred)
	}
	errs := data.SnapshotErrors()
	if len(errs) != 1 {
		t.Fatalf("Data.Errors len = %d, want 1: %v", len(errs), errs)
	}
	var rerr *xerrors.RetryableError
	if !errors.As(errs[0], &rerr) {
		t.Errorf("error not *RetryableError: %T %v", errs[0], errs[0])
	}
	// Featured must remain populated even when Starred fails (SC-003).
	if len(r.Featured) == 0 {
		t.Errorf("Featured emptied as side-effect of Starred failure")
	}
}

// TC-004: Empty user login → no HTTP call, Starred is empty slice.
func TestRun_Starred_EmptyLogin(t *testing.T) {
	t.Parallel()
	mux := githubapi.NewMockTransport()
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0, DisableRetries: true},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	data := plugins.NewData()
	data.Computed.RepositoryList = octocatRepos()
	data.User = &plugins.User{Login: ""} // empty login
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"plugin_repositories":         true,
			"plugin_repositories_starred": true,
		},
		Data: data,
		REST: rest,
	}
	out, err := repositories.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*repositories.Result)
	if r.Starred == nil || len(r.Starred) != 0 {
		t.Errorf("Starred = %v, want empty slice []", r.Starred)
	}
	if calls := mux.Calls(); len(calls) != 0 {
		t.Errorf("unexpected HTTP calls: %+v", calls)
	}
}

// TC-005: Starred not enabled → Starred is nil (M4 baseline preserved).
func TestRun_Starred_NotEnabled(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.Computed.RepositoryList = octocatRepos()
	data.User = &plugins.User{Login: "octocat"}
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"plugin_repositories": true,
			// plugin_repositories_starred intentionally omitted
		},
		Data: data,
	}
	out, err := repositories.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*repositories.Result)
	if r.Starred != nil {
		t.Errorf("Starred = %v, want nil when plugin_repositories_starred unset", r.Starred)
	}
}

// TC-006: REST nil + starred enabled → fall back to placeholder (FR-006).
//
// This preserves existing M4 tests that construct a PluginContext
// without a REST client; they still expect Starred to be populated as
// a Featured copy.
func TestRun_Starred_RESTNilFallback(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.Computed.RepositoryList = octocatRepos()
	data.User = &plugins.User{Login: "octocat"}
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"plugin_repositories":         true,
			"plugin_repositories_starred": true,
		},
		Data: data,
		// REST: nil — placeholder fallback path
	}
	out, err := repositories.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*repositories.Result)
	if len(r.Starred) == 0 {
		t.Errorf("Starred empty under REST-nil fallback; expected Featured copy")
	}
	// Featured-copy means the names overlap exactly with Featured.
	if !reflect.DeepEqual(nameList(r.Starred), nameList(r.Featured)) {
		t.Errorf("Starred should equal Featured under fallback\nStarred=%v\nFeatured=%v",
			nameList(r.Starred), nameList(r.Featured))
	}
}
