package contributors_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/contributors"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
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

func run(t *testing.T, account plugins.AccountKind) *contributors.Result {
	t.Helper()
	data := plugins.NewData()
	data.Account = account
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{}}
	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*contributors.Result)
}

func TestRun_UserAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountUser)
	if !r.Skipped {
		t.Errorf("user account should be Skipped in M4; got %+v", r)
	}
}

func TestRun_OrganizationAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountOrganization)
	if !r.Skipped {
		t.Errorf("organization account should be Skipped in M4; got %+v", r)
	}
}

func TestRun_RepositoryAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountRepository)
	if !r.Skipped {
		t.Errorf("repository account without RepoRef should be Skipped; got %+v", r)
	}
}

func TestRun_SkippedReasonNonEmpty(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountUser)
	if r.SkippedReason == "" {
		t.Errorf("SkippedReason should be non-empty for trace logs")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &contributors.Result{
		Skipped:  true,
		List:     []contributors.Contributor{},
		Sections: []string{},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "contributors.json")
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
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}

func TestRun_RepositoryContributionsStats(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusOK, `[
		{
			"total": 2,
			"weeks": [{"a": 10, "d": 3, "c": 1}, {"a": 5, "d": 2, "c": 1}],
			"author": {"login": "octocat", "avatar_url": "https://avatars.example/octocat.png"}
		},
		{
			"total": 5,
			"weeks": [{"a": 20, "d": 8, "c": 5}],
			"author": {"login": "hubot", "avatar_url": "https://avatars.example/hubot.png"}
		}
	]`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  2,
		DefaultBranch: "main",
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{
			"plugin_contributors_contributions": true,
			"plugin_contributors_sections":      []string{"contributors"},
			"plugin_contributors_ignored":       []string{"hubot"},
		}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if r.Skipped {
		t.Fatalf("repo result should not be skipped: %+v", r)
	}
	if !r.Contributions {
		t.Fatalf("Contributions should mirror plugin_contributors_contributions")
	}
	if r.Base != "main" || r.Head != "master" {
		t.Fatalf("unexpected base/head: %q/%q", r.Base, r.Head)
	}
	if len(r.List) != 1 {
		t.Fatalf("ignored contributor should be filtered; got %+v", r.List)
	}
	got := r.List[0]
	if got.Login != "octocat" || got.AvatarURL == "" || got.Commits != 2 || got.Additions != 15 || got.Deletions != 5 {
		t.Fatalf("unexpected contributor stats: %+v", got)
	}
}

func TestRun_RepositoryStatsFailureKeepsMinimalStub(t *testing.T) {
	// Override the 202 retry backoff so the bounded poll loop runs
	// instantly. Cannot run in parallel because SetSleepFn mutates a
	// package-level hook.
	var slept int
	restore := contributors.SetSleepFn(func(_ context.Context, _ time.Duration) { slept++ })
	defer restore()

	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusAccepted, `{"message":"Accepted"}`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  1,
		DefaultBranch: "main",
		Activity:      plugins.RepoActivity{RecentCommits: 3},
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"plugin_contributors_contributions": true}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if len(r.List) != 1 || r.List[0].Login != "octocat" || r.List[0].Commits != 3 {
		t.Fatalf("minimal stub should remain available on stats warm-up; got %+v", r.List)
	}
	// 202 Accepted should flag the result so the partial knows to
	// render "stats pending" instead of misleading ++0 --0.
	if !r.StatsPending {
		t.Fatalf("StatsPending should be true when /stats/contributors returns 202; got %+v", r)
	}
	// The bounded poll must have re-requested the endpoint and slept
	// between attempts before giving up.
	if got := rest.Calls("/repos/octocat/hello-world/stats/contributors"); got != contributors.StatsPendingMaxAttempts {
		t.Fatalf("expected %d attempts on persistent 202; got %d", contributors.StatsPendingMaxAttempts, got)
	}
	if slept != contributors.StatsPendingMaxAttempts-1 {
		t.Fatalf("expected %d backoff sleeps; got %d", contributors.StatsPendingMaxAttempts-1, slept)
	}
}

func TestRun_RepositoryStatsPendingFallsBackToContributorList(t *testing.T) {
	restore := contributors.SetSleepFn(func(_ context.Context, _ time.Duration) {})
	defer restore()

	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusAccepted, `{"message":"Accepted"}`)
	rest.OnBody("/repos/octocat/hello-world/contributors", http.StatusOK, `[
		{"login":"alice","avatar_url":"https://avatars.example/alice.png","contributions":8},
		{"login":"bob","avatar_url":"https://avatars.example/bob.png","contributions":3}
	]`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  2,
		DefaultBranch: "main",
		Activity:      plugins.RepoActivity{RecentCommits: 1},
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"plugin_contributors_contributions": true}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if !r.StatsPending {
		t.Fatalf("StatsPending should remain true for stats 202; got %+v", r)
	}
	if got := rest.Calls("/repos/octocat/hello-world/contributors"); got != 1 {
		t.Fatalf("contributors fallback calls = %d, want 1", got)
	}
	if len(r.List) != 2 || r.List[0].Login != "alice" || r.List[0].Commits != 8 {
		t.Fatalf("fallback contributor list not used/sorted: %+v", r.List)
	}
}

// TestRun_RepositoryStatsRetriesPendingThenSucceeds pins #471: GitHub
// returns 202 Accepted (empty body) while it warms the
// /stats/contributors cache. We must poll, not give up immediately, so a
// cold cache still yields the real per-contributor commits/additions/
// deletions instead of the "stats pending" placeholder.
func TestRun_RepositoryStatsRetriesPendingThenSucceeds(t *testing.T) {
	restore := contributors.SetSleepFn(func(_ context.Context, _ time.Duration) {})
	defer restore()

	rest := mocks.NewRESTMux(t)
	path := "/repos/octocat/hello-world/stats/contributors"
	calls := 0
	rest.OnFunc(path, func(_ *http.Request) (int, string, http.Header) {
		calls++
		if calls < 3 {
			// First two requests: GitHub is still computing the cache.
			return http.StatusAccepted, `{"message":"Accepted"}`, nil
		}
		return http.StatusOK, `[
			{
				"total": 7,
				"weeks": [{"a": 30, "d": 11, "c": 4}, {"a": 5, "d": 2, "c": 3}],
				"author": {"login": "octocat", "avatar_url": "https://avatars.example/octocat.png"}
			}
		]`, nil
	})
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  1,
		DefaultBranch: "main",
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"plugin_contributors_contributions": true}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if r.StatsPending {
		t.Fatalf("StatsPending must clear once 202 resolves to 200; got %+v", r)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts (2x202 + 1x200); got %d", calls)
	}
	if len(r.List) != 1 {
		t.Fatalf("expected one contributor after retry; got %+v", r.List)
	}
	got := r.List[0]
	if got.Login != "octocat" || got.Commits != 7 || got.Additions != 35 || got.Deletions != 13 {
		t.Fatalf("unexpected contributor stats after retry: %+v", got)
	}
}

func TestRun_RepositoryStatsErrorDoesNotFlagPending(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusInternalServerError, `{"message":"oops"}`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  1,
		DefaultBranch: "main",
		Activity:      plugins.RepoActivity{RecentCommits: 7},
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"plugin_contributors_contributions": true}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if r.StatsPending {
		t.Fatalf("non-202 failures must not flag StatsPending; got %+v", r)
	}
}

// TestRun_RepositoryStatsFailureFallsBackToContributorList mirrors the
// 202-path fallback but for the 500 path: when /stats/contributors fails
// (statsStatusFailed), Run must fall back to /repos/{owner}/{repo}/
// contributors, populate Result.List (sorted, ignored-filtered) and leave
// StatsPending false (a hard failure is not a warm-up).
func TestRun_RepositoryStatsFailureFallsBackToContributorList(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusInternalServerError, `{"message":"oops"}`)
	rest.OnBody("/repos/octocat/hello-world/contributors", http.StatusOK, `[
		{"login":"alice","avatar_url":"https://avatars.example/alice.png","contributions":8},
		{"login":"hubot","avatar_url":"https://avatars.example/hubot.png","contributions":12},
		{"login":"bob","avatar_url":"https://avatars.example/bob.png","contributions":3}
	]`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		OwnerAvatar:   "https://avatars.example/owner.png",
		Name:          "hello-world",
		Contributors:  3,
		DefaultBranch: "main",
		Activity:      plugins.RepoActivity{RecentCommits: 1},
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{
			"plugin_contributors_contributions": true,
			"plugin_contributors_ignored":       []string{"hubot"},
		}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if r.StatsPending {
		t.Fatalf("a hard 500 failure must not flag StatsPending; got %+v", r)
	}
	if got := rest.Calls("/repos/octocat/hello-world/contributors"); got != 1 {
		t.Fatalf("contributors fallback calls = %d, want 1", got)
	}
	// ignored "hubot" filtered out; remaining sorted by commits desc.
	if len(r.List) != 2 || r.List[0].Login != "alice" || r.List[0].Commits != 8 || r.List[1].Login != "bob" {
		t.Fatalf("fallback contributor list not used/filtered/sorted: %+v", r.List)
	}
}

// TestRun_ContributorFallbackFollowsPagination pins the #743 fix: the
// /repos/{owner}/{repo}/contributors fallback must follow the RFC5988
// Link rel="next" header so contributors beyond the first page are not
// silently dropped.
func TestRun_ContributorFallbackFollowsPagination(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusInternalServerError, `{"message":"oops"}`)
	rest.OnFunc("/repos/octocat/hello-world/contributors", func(req *http.Request) (int, string, http.Header) {
		if req.URL.Query().Get("page") == "2" {
			return http.StatusOK, `[{"login":"bob","avatar_url":"b","contributions":3}]`, nil
		}
		h := http.Header{"Link": []string{
			`<https://api.github.com/repos/octocat/hello-world/contributors?per_page=100&anon=true&page=2>; rel="next"`,
		}}
		return http.StatusOK, `[{"login":"alice","avatar_url":"a","contributions":8}]`, h
	})
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{Owner: "octocat", Name: "hello-world", Contributors: 2, DefaultBranch: "main"})
	pc := mocks.NewPluginContext(t, mocks.WithREST(rest), mocks.WithData(data), mocks.WithInputs(map[string]any{}))

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if got := rest.Calls("/repos/octocat/hello-world/contributors"); got != 2 {
		t.Fatalf("contributors calls = %d, want 2 (page 1 + page 2)", got)
	}
	if len(r.List) != 2 || r.List[0].Login != "alice" || r.List[1].Login != "bob" {
		t.Fatalf("second-page contributor not merged: %+v", r.List)
	}
}

// TestRun_ContributorFallbackCapsPages pins the #743 defensive cap: a
// misbehaving endpoint that always advertises a next page must not spin
// forever — the loop stops at contributorsMaxPages requests.
func TestRun_ContributorFallbackCapsPages(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusInternalServerError, `{"message":"oops"}`)
	rest.OnFunc("/repos/octocat/hello-world/contributors", func(_ *http.Request) (int, string, http.Header) {
		h := http.Header{"Link": []string{
			`<https://api.github.com/repos/octocat/hello-world/contributors?per_page=100&anon=true&page=999>; rel="next"`,
		}}
		return http.StatusOK, `[{"login":"alice","avatar_url":"a","contributions":1}]`, h
	})
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{Owner: "octocat", Name: "hello-world", Contributors: 1, DefaultBranch: "main"})
	pc := mocks.NewPluginContext(t, mocks.WithREST(rest), mocks.WithData(data), mocks.WithInputs(map[string]any{}))

	if _, err := contributors.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := rest.Calls("/repos/octocat/hello-world/contributors"); got != contributors.ContributorsMaxPages {
		t.Fatalf("contributors calls = %d, want %d (page cap)", got, contributors.ContributorsMaxPages)
	}
}

func TestPartial_ContributionsDisplayMode(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(contributors.Name, &contributors.Result{
		Contributions: true,
		List: []contributors.Contributor{{
			Login:     "octocat",
			AvatarURL: "https://avatars.example/octocat.png",
			Commits:   2,
			Additions: 15,
			Deletions: 5,
		}},
		Sections: []string{"contributors"},
	})
	got, _, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// #409 Phase B3: chips render as native SVG — a <g data-login> group,
	// the login as a <text>, and the commit count on a <g
	// class="contributions"> badge with an inline commits octicon. Adds/
	// dels stay in a <g class="label-right"> badge as a Go-specific
	// extension while contributors_contributions is unadopted upstream.
	for _, want := range []string{
		`<g data-login="octocat"`,
		`class="contributions"`,
		`>2</text>`,
		"++15 --5",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
	if strings.Contains(got, "octocat2") {
		t.Fatalf("login must not fuse with commit count; got %q", got)
	}
}

// TestPartial_LoginWithDigitsUsesSeparateBadge pins #421 directly:
// the bug surfaced with login "mjun0812" because the rendered SVG
// dropped the whitespace between the login token and the commits chip.
// The upstream-equivalent layout (#540) places the commit count in a
// separate <div class="contributions"> badge, so digit-only logins
// can never fuse with the count even though the login is now raw text.
func TestPartial_LoginWithDigitsHasExplicitDelimiter(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(contributors.Name, &contributors.Result{
		Contributions: true,
		List: []contributors.Contributor{{
			Login:     "mjun0812",
			AvatarURL: "https://avatars.example/mjun0812.png",
			Commits:   67,
			Additions: 1234,
			Deletions: 56,
		}},
		Sections: []string{"contributors"},
	})
	got, _, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, `class="contributions"`) || !strings.Contains(got, `>67</text>`) {
		t.Fatalf("expected separate count badge; got %q", got)
	}
	if strings.Contains(got, "mjun081267") {
		t.Fatalf("regression: login and commit count fused into %q", "mjun081267")
	}
	for _, want := range []string{`class="contributions"`, `>67</text>`, "++1234 --56"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestPartial_StatsPendingOmitsDiffSpan(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(contributors.Name, &contributors.Result{
		Contributions: true,
		StatsPending:  true,
		List: []contributors.Contributor{{
			Login:   "octocat",
			Commits: 3,
		}},
		Sections: []string{"contributors"},
	})
	got, _, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// When /stats/contributors stays 202 (StatsPending), the add/del
	// diff span is omitted entirely. The earlier "stats pending" chip
	// was a misleading placeholder, not data the viewer can act on, so
	// tests/content/dom_contract_test.go forbids it for #471.
	if strings.Contains(got, "stats pending") {
		t.Fatalf("StatsPending must not emit a 'stats pending' chip; got %q", got)
	}
	// Neither the false "++0 --0" nor any add/del diff span may appear.
	if strings.Contains(got, "++") {
		t.Fatalf("StatsPending must omit the add/del diff span; got %q", got)
	}
	// The commit count still carries the contribution signal via the
	// <g class="contributions"> badge.
	if !strings.Contains(got, `class="contributions"`) || !strings.Contains(got, `>3</text>`) {
		t.Fatalf("commit count from minimal stub should still render; got %q", got)
	}
}

func TestPartial_DefaultDisplayHidesContributionNumbers(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.SetPlugin(contributors.Name, &contributors.Result{
		Contributions: false,
		List: []contributors.Contributor{{
			Login:     "octocat",
			Commits:   2,
			Additions: 15,
			Deletions: 5,
		}},
		Sections: []string{"contributors"},
	})
	got, _, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, notWant := range []string{"2 commits", "++15 --5", `class="contributions"`} {
		if strings.Contains(got, notWant) {
			t.Fatalf("did not expect %q in %q", notWant, got)
		}
	}
	if strings.Contains(got, `class="label-right"`) {
		t.Fatalf("default display should hide numeric chips; got %q", got)
	}
	if !strings.Contains(got, "octocat") {
		t.Fatalf("default display should keep contributor row: %q", got)
	}
}

// TestRun_RepositoryStatsFailed_AppendError verifies that when
// /stats/contributors returns a hard failure (5xx), Run (a) still
// renders a result (not Skipped), (b) falls back to /contributors,
// and (c) records exactly one AppendError entry mentioning the failed
// endpoint so operators can observe the degradation.
func TestRun_RepositoryStatsFailed_AppendError(t *testing.T) {
	t.Parallel()
	rest := mocks.NewRESTMux(t)
	rest.OnBody("/repos/octocat/hello-world/stats/contributors", http.StatusInternalServerError, `{"message":"oops"}`)
	rest.OnBody("/repos/octocat/hello-world/contributors", http.StatusOK, `[
		{"login":"alice","avatar_url":"https://avatars.example/alice.png","contributions":8}
	]`)
	data := plugins.NewData()
	data.Account = plugins.AccountRepository
	data.SetRepo(&plugins.Repo{
		Owner:         "octocat",
		Name:          "hello-world",
		Contributors:  1,
		DefaultBranch: "main",
		Activity:      plugins.RepoActivity{RecentCommits: 1},
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithREST(rest),
		mocks.WithData(data),
		mocks.WithInputs(map[string]any{"plugin_contributors_contributions": true}),
	)

	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*contributors.Result)
	if r.Skipped {
		t.Fatalf("/stats/contributors failure must not Skipped the whole result; got %+v", r)
	}
	if r.StatsPending {
		t.Fatalf("hard 5xx failure must not set StatsPending; got %+v", r)
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 {
		t.Fatalf("SnapshotErrors len = %d, want 1; errors: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "stats fetch failed") {
		t.Errorf("error message should mention stats fetch failed; got %q", errs[0].Error())
	}
}
