package repositories_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
