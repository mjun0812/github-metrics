package languages_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
)

// repos is a small octocat-shaped fixture: 3 repos covering Go,
// JavaScript, TypeScript with deliberately uneven byte totals so sort
// order, Other aggregation, and the alias collapse can all be asserted.
func octocatRepos() []plugins.Repository {
	return []plugins.Repository{
		{
			NameWithOwner: "octocat/alpha",
			Languages: []plugins.LanguageStat{
				{Name: "Go", Color: "#00ADD8", Size: 6000},
				{Name: "JavaScript", Color: "#f1e05a", Size: 2000},
			},
		},
		{
			NameWithOwner: "octocat/beta",
			Languages: []plugins.LanguageStat{
				{Name: "TypeScript", Color: "#3178c6", Size: 4500},
				{Name: "JavaScript", Color: "#f1e05a", Size: 1000},
				{Name: "Markdown", Color: "#083fa1", Size: 800},
			},
		},
		{
			NameWithOwner: "octocat/gamma",
			Languages: []plugins.LanguageStat{
				{Name: "Go", Color: "#00ADD8", Size: 4000},
				{Name: "Shell", Color: "#89e051", Size: 300},
			},
		},
	}
}

func runWith(t *testing.T, repos []plugins.Repository, inputs map[string]any) *languages.Result {
	t.Helper()
	data := plugins.NewData()
	data.Computed.RepositoryList = repos
	pc := &plugins.PluginContext{
		Inputs: inputs,
		Data:   data,
	}
	out, err := languages.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := out.(*languages.Result)
	if !ok {
		t.Fatalf("Run returned %T, want *languages.Result", out)
	}
	return r
}

// TestRun_Normal covers the octocat 正常系: top 3 favorites in
// descending byte order, Mostly = Favorites[0], non-zero Value.
func TestRun_Normal(t *testing.T) {
	t.Parallel()
	r := runWith(t, octocatRepos(), nil)
	if r.Skipped {
		t.Fatalf("expected Skipped=false, got Skipped=%v reason=%q", r.Skipped, r.SkippedReason)
	}
	// Bytes: Go 10000, TS 4500, JS 3000, MD 800, Shell 300 → Go top.
	if got := r.Mostly.Name; got != "Go" {
		t.Errorf("Mostly.Name = %q, want Go", got)
	}
	if len(r.Favorites) == 0 || r.Favorites[0].Name != "Go" {
		t.Errorf("Favorites[0] not Go: %+v", r.Favorites)
	}
	// Favorites[0..2] = Go, TypeScript, JavaScript
	want := []string{"Go", "TypeScript", "JavaScript"}
	for i, name := range want {
		if i >= len(r.Favorites) {
			t.Fatalf("Favorites short: %+v", r.Favorites)
		}
		if r.Favorites[i].Name != name {
			t.Errorf("Favorites[%d] = %q, want %q", i, r.Favorites[i].Name, name)
		}
	}
	if r.Favorites[0].Value <= r.Favorites[1].Value {
		t.Errorf("Favorites not byte-descending: %+v", r.Favorites)
	}
}

// TestRun_LimitOtherAggregation forces limit=3 and asserts the 4th+
// languages collapse into "Other" with the correct cumulative size.
func TestRun_LimitOtherAggregation(t *testing.T) {
	t.Parallel()
	r := runWith(t, octocatRepos(), map[string]any{
		"plugin_languages_limit": 3,
	})
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if len(r.Favorites) != 3 {
		t.Fatalf("Favorites len = %d, want 3", len(r.Favorites))
	}
	// Markdown (800) + Shell (300) should land in Other.
	if r.Other.Size != 1100 {
		t.Errorf("Other.Size = %d, want 1100", r.Other.Size)
	}
	if r.Other.Name != "Other" {
		t.Errorf("Other.Name = %q, want Other", r.Other.Name)
	}
}

// TestRun_Alias merges TypeScript into JavaScript via alias and asserts
// the combined Size + Count match expectations.
func TestRun_Alias(t *testing.T) {
	t.Parallel()
	r := runWith(t, octocatRepos(), map[string]any{
		"plugin_languages_aliases": "TypeScript:JavaScript",
	})
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	var js *plugins.LanguageStat
	for i := range r.Favorites {
		if r.Favorites[i].Name == "JavaScript" {
			js = &r.Favorites[i]
			break
		}
	}
	if js == nil {
		t.Fatalf("JavaScript missing after alias: %+v", r.Favorites)
	}
	// JS direct 3000 + TS-aliased 4500 = 7500
	if js.Size != 7500 {
		t.Errorf("JavaScript.Size = %d, want 7500", js.Size)
	}
	// Count: 2 repos contributed JS directly, beta also produced TS so
	// JavaScript appears in 3 distinct (alpha, beta-direct, beta-aliased)
	// — but the per-repo seen set collapses beta's TS+JS into a single
	// JavaScript counted once. Expected: alpha + beta = 2.
	if js.Count != 2 {
		t.Errorf("JavaScript.Count = %d, want 2", js.Count)
	}
	for _, f := range r.Favorites {
		if f.Name == "TypeScript" {
			t.Errorf("TypeScript should have been collapsed: %+v", r.Favorites)
		}
	}
}

// TestRun_EmptyRepositories asserts the no-repository path skips.
func TestRun_EmptyRepositories(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil, nil)
	if !r.Skipped {
		t.Errorf("Skipped = false, want true; result=%+v", r)
	}
	if r.SkippedReason == "" {
		t.Errorf("SkippedReason empty, want a non-empty hint")
	}
}

// TestRun_Ignored excludes "Markdown" via _ignored and asserts it
// disappears from Favorites and Other.
func TestRun_Ignored(t *testing.T) {
	t.Parallel()
	r := runWith(t, octocatRepos(), map[string]any{
		"plugin_languages_ignored": "Markdown",
		"plugin_languages_limit":   3,
	})
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	for _, f := range r.Favorites {
		if f.Name == "Markdown" {
			t.Errorf("Markdown should have been filtered out")
		}
	}
	// Now Other only holds Shell (300).
	if r.Other.Size != 300 {
		t.Errorf("Other.Size = %d, want 300 (Shell only)", r.Other.Size)
	}
}
