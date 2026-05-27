package notable_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/notable"
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

func runWith(t *testing.T, inputs map[string]any) *notable.Result {
	t.Helper()
	data := plugins.NewData()
	pc := &plugins.PluginContext{Data: data, Inputs: inputs}
	out, err := notable.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*notable.Result)
}

func runWithGraphQL(t *testing.T, inputs map[string]any, body string) (*notable.Result, *mocks.GraphQLMux) {
	t.Helper()
	gql := mocks.NewGraphQLMux(t)
	gql.OnBody("ViewerNotable", 200, body)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(gql),
		mocks.WithInputs(inputs),
	)
	out, err := notable.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*notable.Result), gql
}

func TestRun_SkippedInM4(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !r.Skipped {
		t.Errorf("notable should be Skipped in M4")
	}
}

func TestRun_SkippedWithFilterTrue(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_notable_filter": true})
	if !r.Skipped {
		t.Errorf("filter=true still Skipped")
	}
}

func TestRun_SkippedWithIndepthTrue(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_notable_indepth": true})
	if !r.Skipped {
		t.Errorf("indepth=true still Skipped")
	}
}

func TestRun_EmptyList(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if len(r.List) != 0 {
		t.Errorf("List should be empty in M4; got %+v", r.List)
	}
}

// Spec 013: with GraphQL client unavailable (the M4 test path), Run
// returns Skipped + "GraphQL client unavailable". The original
// "follow-up" message was M4 baseline language; 013 replaces it with
// the precise gating reason.
func TestRun_SkippedReasonExplainsDeferral(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !strings.Contains(r.SkippedReason, "GraphQL") {
		t.Errorf("SkippedReason should mention GraphQL; got %q", r.SkippedReason)
	}
}

func TestRun_IndepthCollectsExtendedStats(t *testing.T) {
	t.Parallel()
	r, gql := runWithGraphQL(t, map[string]any{
		"user":                   "octocat",
		"plugin_notable":         true,
		"plugin_notable_indepth": true,
	}, notableGraphQLIndepthBody)
	if got := gql.Calls("ViewerNotable"); got != 1 {
		t.Fatalf("ViewerNotable calls = %d, want 1", got)
	}
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1: %+v", len(r.List), r.List)
	}
	entry := r.List[0]
	if !entry.Indepth {
		t.Errorf("entry.Indepth = false")
	}
	if entry.Login != "octocat" {
		t.Errorf("Login = %q, want octocat", entry.Login)
	}
	// Indepth groups by full repository handle ("@owner/repo").
	if entry.Name != "octocat/hello-world" {
		t.Errorf("Name = %q, want octocat/hello-world", entry.Name)
	}
	if entry.AvatarURL != "https://avatars.githubusercontent.com/u/583231?v=4" {
		t.Errorf("AvatarURL = %q", entry.AvatarURL)
	}
	if entry.Commits != 42 || entry.Issues != 3 || entry.Pulls != 2 {
		t.Errorf("extended stats = commits:%d issues:%d pulls:%d", entry.Commits, entry.Issues, entry.Pulls)
	}
	if !entry.Maintainer || entry.Percentage != 1 {
		t.Errorf("maintainer/percentage = %v/%v, want true/1", entry.Maintainer, entry.Percentage)
	}
}

func TestRun_BasicChipLabelIsRepoName(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":           "octocat",
		"plugin_notable": true,
	}, notableGraphQLIndepthBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1", len(r.List))
	}
	entry := r.List[0]
	if entry.Indepth {
		t.Errorf("entry.Indepth = true, want false in basic mode")
	}
	// Issue #422: basic mode chips label by repository name (the part
	// after "owner/") so the five chips on a same-owner user page are
	// distinguishable. The owner login is still preserved on the
	// underlying Login field for the avatar lookup.
	if entry.Name != "hello-world" {
		t.Errorf("Name = %q, want hello-world", entry.Name)
	}
	if entry.Login != "octocat" {
		t.Errorf("Login = %q, want octocat", entry.Login)
	}
	// Indepth-only counters stay zeroed in basic mode.
	if entry.Commits != 0 || entry.Issues != 0 || entry.Pulls != 0 {
		t.Errorf("basic-mode counters non-zero: commits:%d issues:%d pulls:%d", entry.Commits, entry.Issues, entry.Pulls)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &notable.Result{Skipped: true, List: []notable.NotableContrib{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "notable.json")
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

func TestRun_IndepthGoldenShape(t *testing.T) {
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                   "octocat",
		"plugin_notable":         true,
		"plugin_notable_indepth": true,
	}, notableGraphQLIndepthBody)
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "notable_indepth.json")
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

func TestPartial_BasicGolden(t *testing.T) {
	data := plugins.NewData()
	// Five entries owned by the same user, mirroring the issue #422
	// repro on mjun0812's account (chips collapsed into "@mjun0812"
	// before the fix). After the fix each chip carries a distinct
	// repository name and an inline star badge.
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{
			{Name: "flash-attention-prebuild-wheels", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Login: "mjun0812", Repo: "mjun0812/flash-attention-prebuild-wheels", Title: "mjun0812/flash-attention-prebuild-wheels", Type: "owner", StargazerCount: 1500},
			{Name: "github-metrics", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Login: "mjun0812", Repo: "mjun0812/github-metrics", Title: "mjun0812/github-metrics", Type: "owner", StargazerCount: 32},
			{Name: "dotfiles", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Login: "mjun0812", Repo: "mjun0812/dotfiles", Title: "mjun0812/dotfiles", Type: "owner", StargazerCount: 26},
			{Name: "claude-code-tools", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Login: "mjun0812", Repo: "mjun0812/claude-code-tools", Title: "mjun0812/claude-code-tools", Type: "owner", StargazerCount: 7},
			{Name: "vim-config", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Login: "mjun0812", Repo: "mjun0812/vim-config", Title: "mjun0812/vim-config", Type: "owner", StargazerCount: 3},
		},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := notable.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "notable.svg")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
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
	// Issue #422 acceptance: distinct repo names appear, the star
	// badge text is part of the chip body (not just a tiny gauge),
	// and the gauge cluster does NOT render in basic mode.
	for _, marker := range []string{
		`@flash-attention-prebuild-wheels`,
		`@github-metrics`,
		`@dotfiles`,
		`@claude-code-tools`,
		`@vim-config`,
		`<span class="stars">★ 1.5k</span>`,
		`<span class="stars">★ 32</span>`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `class="gauge"`) {
		t.Errorf("basic-mode output should not render gauge SVGs:\n%s", got)
	}
}

func TestPartial_BasicTruncatesLongRepoName(t *testing.T) {
	t.Parallel()
	// A 60-char repo name should be ellipsized so the chip does not
	// overflow the 480 px card width (issue #422 layout guard).
	longRepo := "a-very-long-and-deliberately-overflowing-repository-name-xyz"
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           longRepo,
			AvatarURL:      "https://example.invalid/avatar.png",
			Login:          "octocat",
			Repo:           "octocat/" + longRepo,
			Type:           "owner",
			StargazerCount: 4,
		}},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := notable.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in truncated chip label; got:\n%s", got)
	}
	if strings.Contains(got, "@"+longRepo+"<") {
		t.Errorf("untruncated full name still present in chip label; got:\n%s", got)
	}
}

func TestPartial_IndepthGolden(t *testing.T) {
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           "octocat/hello-world",
			AvatarURL:      "https://avatars.githubusercontent.com/u/583231?v=4",
			Organization:   false,
			Login:          "octocat",
			Repo:           "octocat/hello-world",
			Title:          "octocat/hello-world",
			Type:           "owner",
			Indepth:        true,
			Description:    "Demo repository",
			StargazerCount: 80,
			ForkCount:      9,
			Commits:        42,
			Issues:         3,
			Pulls:          2,
			Maintainer:     true,
			Percentage:     1,
		}},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := notable.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "notable_indepth.svg")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
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
	// Parity markers from upstream notable.ejs indepth output: the
	// organization-contributions row, the "@owner/repo" chip name, the
	// maintainer contribution-level class, and the gauge visualizations.
	for _, marker := range []string{
		`class="row organization contributions"`,
		`class="organization contribution s "`,
		`@octocat/hello-world`,
		`class="gauge"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
}

const notableGraphQLIndepthBody = `{
  "data": {
    "viewer": {
      "repositories": {
        "totalCount": 1,
        "nodes": [
          {
            "nameWithOwner": "octocat/hello-world",
            "description": "Demo repository",
            "url": "https://github.com/octocat/hello-world",
            "owner": {
              "__typename": "User",
              "login": "octocat",
              "avatarUrl": "https://avatars.githubusercontent.com/u/583231?v=4"
            },
            "stargazerCount": 80,
            "forkCount": 9,
            "isFork": false,
            "isPrivate": false,
            "defaultBranchRef": {
              "target": {
                "__typename": "Commit",
                "history": {
                  "totalCount": 42
                }
              }
            },
            "issues": {
              "totalCount": 3
            },
            "pullRequests": {
              "totalCount": 2
            }
          }
        ]
      }
    }
  }
}`
