package achievements_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/achievements"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files in tests/golden/...")

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

func run(t *testing.T, computed plugins.Computed, inputs map[string]any) *achievements.Result {
	t.Helper()
	data := plugins.NewData()
	data.Computed = computed
	pc := &plugins.PluginContext{Inputs: inputs, Data: data}
	out, err := achievements.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*achievements.Result)
}

func octocatComputed() plugins.Computed {
	return plugins.Computed{
		TotalCommits:      6000, // S
		TotalIssues:       150,  // B
		TotalPullRequests: 600,  // A
		Repositories: plugins.ComputedRepositories{
			Count:        80,  // A
			Stargazers:   200, // B
			Issues:       150,
			PullRequests: 600,
		},
		RepositoryList: []plugins.Repository{{NameWithOwner: "octocat/alpha"}},
	}
}

// TestRun_Normal_ThresholdC — default threshold "C" should list 5+
// entries (commits S, repos A, stars B, issues B, PRs A; followers X
// dropped by threshold).
func TestRun_Normal_ThresholdC(t *testing.T) {
	t.Parallel()
	r := run(t, octocatComputed(), nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if r.Display != "detailed" {
		t.Errorf("Display = %q, want detailed", r.Display)
	}
	if len(r.List) < 5 {
		t.Errorf("List len = %d, want >= 5; %+v", len(r.List), r.List)
	}
	for _, a := range r.List {
		if a.Rank == "X" {
			t.Errorf("X-rank should not appear with threshold C: %+v", a)
		}
	}
}

func TestRun_DisplayCompact(t *testing.T) {
	t.Parallel()
	r := run(t, octocatComputed(), map[string]any{
		"plugin_achievements_display": " compact ",
	})
	if r.Display != "compact" {
		t.Errorf("Display = %q, want compact", r.Display)
	}
}

// TestRun_ThresholdS — only S-rank entries (commits) survive.
func TestRun_ThresholdS(t *testing.T) {
	t.Parallel()
	r := run(t, octocatComputed(), map[string]any{
		"plugin_achievements_threshold": "S",
	})
	for _, a := range r.List {
		if a.Rank != "S" {
			t.Errorf("expected S-only, got %+v", a)
		}
	}
}

// TestRun_OnlyFilter — limits to commits, stars only.
func TestRun_OnlyFilter(t *testing.T) {
	t.Parallel()
	r := run(t, octocatComputed(), map[string]any{
		"plugin_achievements_only": "commits,stars",
	})
	ids := map[string]bool{}
	for _, a := range r.List {
		ids[a.ID] = true
	}
	if !ids["commits"] || !ids["stars"] {
		t.Errorf("expected commits+stars; got %+v", ids)
	}
	for id := range ids {
		if id != "commits" && id != "stars" {
			t.Errorf("unexpected achievement id %q", id)
		}
	}
}

// TestRun_IgnoredFilter — drops the "stars" achievement explicitly.
func TestRun_IgnoredFilter(t *testing.T) {
	t.Parallel()
	r := run(t, octocatComputed(), map[string]any{
		"plugin_achievements_ignored": "stars",
	})
	for _, a := range r.List {
		if a.ID == "stars" {
			t.Errorf("stars should be ignored: %+v", a)
		}
	}
}

// TestRun_BaseUnavailable — empty computed → Skipped.
func TestRun_BaseUnavailable(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.Computed{}, nil)
	if !r.Skipped {
		t.Errorf("expected Skipped=true; got %+v", r)
	}
}

// Golden tests.
func TestPartial_Achievements_Golden(t *testing.T) {
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			{ID: "commits", Rank: "S", Title: "Worker", Description: "Total commits", Icon: "git-commit", Value: 6000},
			{ID: "pull-requests", Rank: "A", Title: "Engineer", Description: "Pull requests", Icon: "git-pull-request", Value: 600},
			{ID: "repositories", Rank: "A", Title: "Member", Description: "Public repositories", Icon: "repo", Value: 80},
		},
		Ranks: map[string]string{
			"commits":       "S",
			"pull-requests": "A",
			"repositories":  "A",
			"stars":         "B",
			"issues":        "B",
			"followers":     "X",
		},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := achievements.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "achievements.svg")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
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
	// Tier 3 (011) rewrite: each entry is now
	// `<div class="achievement <rank> largeable-width-half" data-rank=...>`
	// so the literal `class="achievement"` (closing quote) and the old
	// `data-achievement=` attribute no longer match. Anchor on the
	// multi-class prefix + the new `data-rank=` attribute instead.
	for _, marker := range []string{
		`class="achievement `,
		`data-rank="`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
}

func TestPartial_AchievementsCompact_Golden(t *testing.T) {
	r := &achievements.Result{
		Display: "compact",
		List: []achievements.Achievement{
			{ID: "commits", Rank: "S", Title: "Worker", Description: "Total commits", Icon: "git-commit", Value: 6000},
			{ID: "pull-requests", Rank: "A", Title: "Engineer", Description: "Pull requests", Icon: "git-pull-request", Value: 600},
			{ID: "repositories", Rank: "A", Title: "Member", Description: "Public repositories", Icon: "repo", Value: 80},
		},
		Ranks: map[string]string{
			"commits":       "S",
			"pull-requests": "A",
			"repositories":  "A",
			"stars":         "B",
			"issues":        "B",
			"followers":     "X",
		},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := achievements.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "achievements_compact.svg")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", werr)
		}
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
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
	for _, marker := range []string{
		`class="achievements compact largeable-flex-wrap"`,
		`class="value-wrapper"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `class="text"`) || strings.Contains(got, "Total commits") {
		t.Errorf("compact output should not render descriptions:\n%s", got)
	}
}

func TestRun_GoldenShape_Achievements(t *testing.T) {
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			{ID: "commits", Rank: "S", Title: "Worker", Description: "Total commits", Icon: "git-commit", Value: 6000},
		},
		Ranks: map[string]string{
			"commits": "S", "repositories": "A", "stars": "B", "followers": "X",
			"issues": "B", "pull-requests": "A",
		},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "achievements.json")
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
