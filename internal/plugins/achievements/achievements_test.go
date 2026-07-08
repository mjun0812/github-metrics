package achievements_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func run(t *testing.T, data *plugins.Data, inputs map[string]any) *achievements.Result {
	t.Helper()
	pc := &plugins.PluginContext{Inputs: inputs, Data: data}
	out, err := achievements.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*achievements.Result)
}

// octocatData mimics a populated Data with values that put each
// achievement on a known tier. Thresholds mirror upstream users.mjs.
func octocatData() *plugins.Data {
	d := plugins.NewData()
	d.User = &plugins.User{
		Login:                "octocat",
		CreatedAt:            time.Now().AddDate(-7, 0, 0), // member: A (3..5..10 → 7 = A)
		Followers:            6000,                         // influencer: S
		Following:            300,                          // follower: B (200..500)
		Sponsoring:           4,                            // sponsor: B (3..5)
		Starred:              700,                          // stargazer: A (500..1000)
		Organizations:        3,                            // worker: A (2..4)
		Gists:                25,                           // gister: B (20..50)
		DiscussionsStarted:   100,                          // chatter: 100+150=250 → B (200..500)
		DiscussionsComments:  150,
		DiscussionAnswers:    30,  // helper: B (20..50)
		PullRequestsOpened:   600, // contributor: A (500..1000)
		PullRequestsReviewed: 220, // reviewer: B (200..500)
	}
	d.Computed = plugins.Computed{
		TotalCommits: 6000,
		Repositories: plugins.ComputedRepositories{
			Count:       80,  // developer: B (50..100)
			Packages:    7,   // packager: B (5..10)
			Deployments: 250, // deployer: B (200..500)
		},
		RepositoryList: []plugins.Repository{
			{NameWithOwner: "octocat/alpha", Stars: 1200, Forks: 120, IsFork: false, Languages: []plugins.LanguageStat{{Name: "Go"}, {Name: "Ruby"}}},
			{NameWithOwner: "octocat/beta", Stars: 300, Forks: 30, IsFork: true, Languages: []plugins.LanguageStat{{Name: "Python"}}},
			{NameWithOwner: "octocat/gamma", Stars: 80, Forks: 10, IsFork: true, Languages: []plugins.LanguageStat{{Name: "JavaScript"}, {Name: "TypeScript"}, {Name: "Go"}}},
			{NameWithOwner: "octocat/delta", Stars: 40, Forks: 5, IsFork: true, Languages: []plugins.LanguageStat{{Name: "Rust"}}},
			{NameWithOwner: "octocat/epsilon", Stars: 20, Forks: 1, IsFork: true, Languages: []plugins.LanguageStat{{Name: "C"}}},
			// 5 IsFork → forker: C (5..10)
			// max stars 1200 → maintainer: B (1000..5000)
			// max forks 120 → inspirer: B (100..500)
			// distinct languages: Go, Ruby, Python, JavaScript, TypeScript, Rust, C = 7 → polyglot: B (4..8)
		},
	}
	return d
}

// TestRun_Normal_ThresholdC — default threshold "C" lists 18 achievements
// (the seed data was tuned so every entry lands at C or better).
func TestRun_Normal_ThresholdC(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if r.Display != "detailed" {
		t.Errorf("Display = %q, want detailed", r.Display)
	}
	if len(r.Ranks) != 18 {
		t.Errorf("Ranks len = %d, want 18; got %+v", len(r.Ranks), r.Ranks)
	}
	for _, a := range r.List {
		if a.Rank == "X" {
			t.Errorf("X-rank should not appear with threshold C: %+v", a)
		}
	}
}

// TestRun_AchievementCoverage — every adopted id surfaces in Ranks with
// a non-empty rank token.
func TestRun_AchievementCoverage(t *testing.T) {
	t.Parallel()
	want := []string{
		"developer", "forker", "contributor", "reviewer", "packager",
		"gister", "worker", "stargazer", "follower", "influencer",
		"maintainer", "inspirer", "polyglot", "member", "sponsor",
		"deployer", "chatter", "helper",
	}
	r := run(t, octocatData(), nil)
	for _, id := range want {
		if _, ok := r.Ranks[id]; !ok {
			t.Errorf("missing rank entry for %q", id)
		}
	}
}

// TestRun_InfluencerWired — Followers > 0 must unlock Influencer
// (regression for the old `followers: 0` hardcoded bug).
func TestRun_InfluencerWired(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	found := false
	for _, a := range r.List {
		if a.ID == "influencer" {
			found = true
			if a.Value != 6000 {
				t.Errorf("influencer.Value = %d, want 6000", a.Value)
			}
			if a.Rank != "S" {
				t.Errorf("influencer.Rank = %q, want S (>= 1000)", a.Rank)
			}
		}
	}
	if !found {
		t.Errorf("influencer not in list")
	}
}

// TestRun_WorkerSemantics — Worker is now organizations count, not commits.
func TestRun_WorkerSemantics(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	for _, a := range r.List {
		if a.ID == "worker" {
			if a.Value != 3 {
				t.Errorf("worker.Value = %d, want 3 (Organizations)", a.Value)
			}
			return
		}
	}
	t.Errorf("worker not in list")
}

// TestRun_StargazerSemantics — Stargazer is now starred-by-me count.
func TestRun_StargazerSemantics(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	for _, a := range r.List {
		if a.ID == "stargazer" {
			if a.Value != 700 {
				t.Errorf("stargazer.Value = %d, want 700 (User.Starred)", a.Value)
			}
			return
		}
	}
	t.Errorf("stargazer not in list")
}

// TestRun_PolyglotSemantics — Polyglot is distinct language count.
func TestRun_PolyglotSemantics(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	for _, a := range r.List {
		if a.ID == "polyglot" {
			if a.Value != 7 {
				t.Errorf("polyglot.Value = %d, want 7 distinct languages", a.Value)
			}
			return
		}
	}
	t.Errorf("polyglot not in list")
}

// TestRun_MemberSemantics — Member is floor((now - CreatedAt) / year).
func TestRun_MemberSemantics(t *testing.T) {
	t.Parallel()
	d := octocatData()
	d.User.CreatedAt = time.Now().AddDate(-10, -6, 0) // 10.5 years
	r := run(t, d, nil)
	for _, a := range r.List {
		if a.ID == "member" {
			if a.Value != 10 {
				t.Errorf("member.Value = %d, want 10 (years floor)", a.Value)
			}
			if a.Rank != "S" {
				t.Errorf("member.Rank = %q, want S (>= 10)", a.Rank)
			}
			return
		}
	}
	t.Errorf("member not in list")
}

// TestRun_NoEngineerTitle — the Go-only Engineer achievement is gone.
func TestRun_NoEngineerTitle(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), nil)
	for _, a := range r.List {
		if a.Title == "Engineer" || a.ID == "engineer" {
			t.Errorf("Engineer should not exist; got %+v", a)
		}
	}
}

// TestRun_DisplayCompact — display=compact normalises to compact.
func TestRun_DisplayCompact(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), map[string]any{
		"plugin_achievements_display": " compact ",
	})
	if r.Display != "compact" {
		t.Errorf("Display = %q, want compact", r.Display)
	}
}

// TestRun_ThresholdS — only S-rank entries survive.
func TestRun_ThresholdS(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), map[string]any{
		"plugin_achievements_threshold": "S",
	})
	for _, a := range r.List {
		if a.Rank != "S" {
			t.Errorf("expected S-only, got %+v", a)
		}
	}
}

// TestRun_OnlyFilter — limits to developer + stargazer only.
func TestRun_OnlyFilter(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), map[string]any{
		"plugin_achievements_only": "developer,stargazer",
	})
	ids := map[string]bool{}
	for _, a := range r.List {
		ids[a.ID] = true
	}
	if !ids["developer"] || !ids["stargazer"] {
		t.Errorf("expected developer+stargazer; got %+v", ids)
	}
	for id := range ids {
		if id != "developer" && id != "stargazer" {
			t.Errorf("unexpected achievement id %q", id)
		}
	}
}

// TestRun_IgnoredFilter — drops the "stargazer" achievement explicitly.
func TestRun_IgnoredFilter(t *testing.T) {
	t.Parallel()
	r := run(t, octocatData(), map[string]any{
		"plugin_achievements_ignored": "stargazer",
	})
	for _, a := range r.List {
		if a.ID == "stargazer" {
			t.Errorf("stargazer should be ignored: %+v", a)
		}
	}
}

// TestRun_BaseUnavailable — empty data → Skipped.
func TestRun_BaseUnavailable(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.NewData(), nil)
	if !r.Skipped {
		t.Errorf("expected Skipped=true; got %+v", r)
	}
}

// TestRun_PluralizationY — repositor-y/-ies switches with value.
func TestRun_PluralizationY(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.User = &plugins.User{Login: "u"}
	d.Computed.Repositories.Count = 1
	d.Computed.RepositoryList = []plugins.Repository{{NameWithOwner: "u/a"}}
	r := run(t, d, map[string]any{"plugin_achievements_only": "developer"})
	if len(r.List) != 1 || !strings.Contains(r.List[0].Description, "repository") {
		t.Errorf("want 'repository' (singular), got %q", r.List[0].Description)
	}

	d.Computed.Repositories.Count = 5
	r = run(t, d, map[string]any{"plugin_achievements_only": "developer"})
	if len(r.List) != 1 || !strings.Contains(r.List[0].Description, "repositories") {
		t.Errorf("want 'repositories' (plural), got %q", r.List[0].Description)
	}
}

// Golden tests.
func TestPartial_Achievements_Golden(t *testing.T) {
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			{ID: "developer", Rank: "S", Title: "Developer", Description: "Published 120 public repositories", Icon: "repo", Value: 120},
			{ID: "stargazer", Rank: "A", Title: "Stargazer", Description: "Starred 700 repositories", Icon: "star", Value: 700},
			{ID: "worker", Rank: "A", Title: "Worker", Description: "Member of 3 organizations", Icon: "organization", Value: 3},
		},
		Ranks: map[string]string{
			"developer": "S", "stargazer": "A", "worker": "A",
		},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := achievements.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "achievements.svg")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", err)
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
			{ID: "developer", Rank: "S", Title: "Developer", Description: "Published 120 public repositories", Icon: "repo", Value: 120},
			{ID: "stargazer", Rank: "A", Title: "Stargazer", Description: "Starred 700 repositories", Icon: "star", Value: 700},
			{ID: "worker", Rank: "A", Title: "Worker", Description: "Member of 3 organizations", Icon: "organization", Value: 3},
		},
		Ranks: map[string]string{
			"developer": "S", "stargazer": "A", "worker": "A",
		},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := achievements.Partial(context.Background(), pc)
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
	if strings.Contains(got, `class="text"`) {
		t.Errorf("compact output should not render descriptions:\n%s", got)
	}
}

// TestPartial_IconResolution — confirms that the per-achievement icon
// pulled from iconsByID lands in the rendered output (not the trophy
// fallback) and that the rank's hex pair has replaced the
// `#primary` / `#secondary` placeholders.
func TestPartial_IconResolution(t *testing.T) {
	t.Parallel()
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			// Polyglot's icon has a unique path opener — use it as
			// the discriminating substring so we don't pin the full
			// ~500-char icon string.
			{ID: "polyglot", Rank: "S", Title: "Polyglot", Description: "x", Icon: "code-square", Value: 16},
		},
		Ranks: map[string]string{"polyglot": "S"},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := achievements.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// (a) The icon must be the upstream polyglot SVG, not the trophy
	// placeholder.
	if !strings.Contains(got, `d="M17.135 7.988`) {
		t.Errorf("missing polyglot icon path in:\n%s", got)
	}
	// (b) The rank S color pair must replace the placeholders — no
	// raw "#primary" / "#secondary" tokens should survive.
	if strings.Contains(got, "#primary") || strings.Contains(got, "#secondary") {
		t.Errorf("unresolved placeholder in:\n%s", got)
	}
	if !strings.Contains(got, "#EB355E") || !strings.Contains(got, "#731237") {
		t.Errorf("missing S-rank color pair (#EB355E / #731237) in:\n%s", got)
	}
}

// TestPartial_RankPrefixLabels — the prefix span carries the
// "Master/Super/Great" label (rank S/A/B), and is omitted for
// rank C entries.
func TestPartial_RankPrefixLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rank       string
		wantPrefix string // empty → expect no <span class="prefix">
	}{
		{"S", "Master"},
		{"A", "Super"},
		{"B", "Great"},
		{"C", ""},
	}
	for _, tc := range cases {
		r := &achievements.Result{
			Display: "detailed",
			List: []achievements.Achievement{
				{ID: "developer", Rank: tc.rank, Title: "Developer", Description: "x", Icon: "repo", Value: 1},
			},
			Ranks: map[string]string{"developer": tc.rank},
		}
		data := plugins.NewData()
		data.SetPlugin(achievements.Name, r)
		pc := &templates.PartialContext{Data: data}
		got, _, err := achievements.Partial(context.Background(), pc)
		if err != nil {
			t.Fatalf("rank %s Partial: %v", tc.rank, err)
		}
		if tc.wantPrefix == "" {
			if strings.Contains(got, `<span class="prefix">`) {
				t.Errorf("rank %s should omit prefix span; got:\n%s", tc.rank, got)
			}
		} else {
			marker := `<span class="prefix">` + tc.wantPrefix + `</span>`
			if !strings.Contains(got, marker) {
				t.Errorf("rank %s missing %q in:\n%s", tc.rank, marker, got)
			}
		}
	}
}

// TestPartial_UnknownIDFallsBackToTrophy — an id not registered in
// iconsByID falls back to the trophy octicon path so the slot is
// never empty.
func TestPartial_UnknownIDFallsBackToTrophy(t *testing.T) {
	t.Parallel()
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			{ID: "unknown-id", Rank: "C", Title: "Unknown", Description: "x", Icon: "trophy", Value: 1},
		},
		Ranks: map[string]string{"unknown-id": "C"},
	}
	data := plugins.NewData()
	data.SetPlugin(achievements.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := achievements.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// The trophy octicon has a distinctive opening "M3.217 6.962" segment.
	if !strings.Contains(got, "M3.217 6.962") {
		t.Errorf("expected trophy fallback path in:\n%s", got)
	}
}

func TestRun_GoldenShape_Achievements(t *testing.T) {
	r := &achievements.Result{
		Display: "detailed",
		List: []achievements.Achievement{
			{ID: "developer", Rank: "S", Title: "Developer", Description: "Published 120 public repositories", Icon: "repo", Value: 120},
		},
		Ranks: map[string]string{
			"developer": "S", "forker": "X", "contributor": "A", "reviewer": "B",
			"packager": "B", "gister": "B", "worker": "A", "stargazer": "A",
			"follower": "B", "influencer": "S", "maintainer": "B", "inspirer": "B",
			"polyglot": "B", "member": "A", "sponsor": "B", "deployer": "B",
			"chatter": "B", "helper": "B",
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
