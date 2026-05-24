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
	t.Parallel()
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
	got, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{"contributor-contributions", "2 commits", "++15 --5"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
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
	got, err := contributors.Partial(context.Background(), &templates.PartialContext{Data: d})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, notWant := range []string{"contributor-contributions", "2 commits", "++15 --5"} {
		if strings.Contains(got, notWant) {
			t.Fatalf("did not expect %q in %q", notWant, got)
		}
	}
	if !strings.Contains(got, "octocat") {
		t.Fatalf("default display should keep contributor row: %q", got)
	}
}
