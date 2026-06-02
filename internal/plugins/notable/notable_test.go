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
	gql.OnBody("UserNotable", 200, body)
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

// TestRun_DefaultsToOrganizationContributions verifies the issue #447
// fix: the plugin lists contributions to other org/user repositories,
// defaulting to organization-owned repos only (plugin_notable_from:
// organization). A user-owned repo in the same response is filtered out
// and the chip label is the owner segment ("@org"), not the repo name.
func TestRun_DefaultsToOrganizationContributions(t *testing.T) {
	t.Parallel()
	r, gql := runWithGraphQL(t, map[string]any{
		"user":           "octocat",
		"plugin_notable": true,
	}, notableGraphQLContributionsBody)
	if got := gql.Calls("UserNotable"); got != 1 {
		t.Fatalf("UserNotable calls = %d, want 1", got)
	}
	// Default from=organization keeps the two org-owned repos and drops
	// the user-owned one; default repositories=no collapses them to one
	// chip per owner.
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1 (org owner): %+v", len(r.List), r.List)
	}
	entry := r.List[0]
	if entry.Name != "huggingface" {
		t.Errorf("Name = %q, want huggingface (owner segment)", entry.Name)
	}
	if !entry.Organization {
		t.Errorf("Organization = false, want true")
	}
	if entry.Login != "huggingface" {
		t.Errorf("Login = %q, want huggingface", entry.Login)
	}
}

// TestRun_FromUserFiltersToUserOwnedRepos confirms plugin_notable_from
// can be flipped to "user" to keep only user-account-owned repos.
func TestRun_FromUserFiltersToUserOwnedRepos(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                "octocat",
		"plugin_notable":      true,
		"plugin_notable_from": "user",
	}, notableGraphQLContributionsBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1 (user owner): %+v", len(r.List), r.List)
	}
	if r.List[0].Name != "torvalds" {
		t.Errorf("Name = %q, want torvalds", r.List[0].Name)
	}
	if r.List[0].Organization {
		t.Errorf("Organization = true, want false for user-owned repo")
	}
}

// TestRun_FromAllKeepsEveryOwner confirms from=all bypasses owner-type
// filtering, yielding one chip per distinct owner.
func TestRun_FromAllKeepsEveryOwner(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                "octocat",
		"plugin_notable":      true,
		"plugin_notable_from": "all",
	}, notableGraphQLContributionsBody)
	if len(r.List) != 2 {
		t.Fatalf("List len = %d, want 2 (huggingface + torvalds): %+v", len(r.List), r.List)
	}
}

// TestRun_RepositoriesShowsFullHandle confirms plugin_notable_repositories
// keeps each repo distinct, labelling chips by the full owner/repo handle.
func TestRun_RepositoriesShowsFullHandle(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_repositories": true,
	}, notableGraphQLContributionsBody)
	if len(r.List) != 2 {
		t.Fatalf("List len = %d, want 2 distinct org handles: %+v", len(r.List), r.List)
	}
	names := map[string]bool{}
	for _, c := range r.List {
		names[c.Name] = true
	}
	for _, want := range []string{"huggingface/accelerate", "huggingface/transformers"} {
		if !names[want] {
			t.Errorf("missing handle chip %q in %v", want, names)
		}
	}
}

func TestRun_IndepthCollectsExtendedStats(t *testing.T) {
	t.Parallel()
	r, gql := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_indepth":      true,
		"plugin_notable_repositories": true,
		"plugin_notable_from":         "all",
	}, notableGraphQLIndepthBody)
	if got := gql.Calls("UserNotable"); got != 1 {
		t.Fatalf("UserNotable calls = %d, want 1", got)
	}
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1: %+v", len(r.List), r.List)
	}
	entry := r.List[0]
	if !entry.Indepth {
		t.Errorf("entry.Indepth = false")
	}
	if entry.Login != "huggingface" {
		t.Errorf("Login = %q, want huggingface", entry.Login)
	}
	// repositories=yes keeps the full "owner/repo" handle as the label.
	if entry.Name != "huggingface/accelerate" {
		t.Errorf("Name = %q, want huggingface/accelerate", entry.Name)
	}
	if entry.AvatarURL != "https://avatars.githubusercontent.com/u/25720743?v=4" {
		t.Errorf("AvatarURL = %q", entry.AvatarURL)
	}
	if entry.Commits != 42 || entry.Issues != 3 || entry.Pulls != 2 {
		t.Errorf("extended stats = commits:%d issues:%d pulls:%d", entry.Commits, entry.Issues, entry.Pulls)
	}
}

// TestRun_BasicChipLabelIsOwner verifies issue #447: basic-mode chips
// label by the owner segment ("@owner"), not the repository name.
func TestRun_BasicChipLabelIsOwner(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                "octocat",
		"plugin_notable":      true,
		"plugin_notable_from": "all",
	}, notableGraphQLIndepthBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1", len(r.List))
	}
	entry := r.List[0]
	if entry.Indepth {
		t.Errorf("entry.Indepth = true, want false in basic mode")
	}
	if entry.Name != "huggingface" {
		t.Errorf("Name = %q, want huggingface (owner segment)", entry.Name)
	}
	if entry.Login != "huggingface" {
		t.Errorf("Login = %q, want huggingface", entry.Login)
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
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_indepth":      true,
		"plugin_notable_repositories": true,
		"plugin_notable_from":         "all",
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
	// Four organization contributions mirroring the issue #447 reference
	// card (mjun0812 data): each chip is labelled by the owner segment
	// and carries no star count.
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{
			{Name: "huggingface", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=4", Organization: true, Login: "huggingface", Repo: "huggingface/accelerate", Title: "huggingface/accelerate", Type: "owner", StargazerCount: 8000},
			{Name: "qdoga", AvatarURL: "https://avatars.githubusercontent.com/u/2?v=4", Organization: true, Login: "qdoga", Repo: "qdoga/ps-index", Title: "qdoga/ps-index", Type: "owner", StargazerCount: 40},
			{Name: "oxc-project", AvatarURL: "https://avatars.githubusercontent.com/u/3?v=4", Organization: true, Login: "oxc-project", Repo: "oxc-project/oxc", Title: "oxc-project/oxc", Type: "owner", StargazerCount: 14000},
			{Name: "azooKey", AvatarURL: "https://avatars.githubusercontent.com/u/4?v=4", Organization: true, Login: "azooKey", Repo: "azooKey/azooKey-Desktop", Title: "azooKey/azooKey-Desktop", Type: "owner", StargazerCount: 600},
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
	// Issue #447 acceptance: owner chips appear, no star badge is
	// rendered, and the gauge cluster does NOT render in basic mode.
	for _, marker := range []string{
		`@huggingface`,
		`@qdoga`,
		`@oxc-project`,
		`@azooKey`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
	if strings.Contains(got, `class="stars"`) {
		t.Errorf("basic-mode output should not render star badges:\n%s", got)
	}
	if strings.Contains(got, `class="gauge"`) {
		t.Errorf("basic-mode output should not render gauge SVGs:\n%s", got)
	}
}

func TestPartial_BasicTruncatesLongHandle(t *testing.T) {
	t.Parallel()
	// A 60-char owner/handle should be ellipsized so the chip does not
	// overflow the 480 px card width (layout guard).
	longName := "a-very-long-and-deliberately-overflowing-owner-or-handle-xyz0"
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           longName,
			AvatarURL:      "https://example.invalid/avatar.png",
			Organization:   true,
			Login:          longName,
			Repo:           longName + "/repo",
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
	if strings.Contains(got, "@"+longName+"<") {
		t.Errorf("untruncated full name still present in chip label; got:\n%s", got)
	}
}

func TestPartial_IndepthGolden(t *testing.T) {
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           "huggingface/accelerate",
			AvatarURL:      "https://avatars.githubusercontent.com/u/25720743?v=4",
			Organization:   true,
			Login:          "huggingface",
			Repo:           "huggingface/accelerate",
			Title:          "huggingface/accelerate",
			Type:           "owner",
			Indepth:        true,
			Description:    "Accelerate library",
			StargazerCount: 8000,
			ForkCount:      900,
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
		`@huggingface/accelerate`,
		`class="gauge"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, got)
		}
	}
}

// notableGraphQLContributionsBody mixes two organization-owned repos
// (one owner) with one user-owned repo so the from filter can be
// exercised.
const notableGraphQLContributionsBody = `{
  "data": {
    "user": {
      "repositoriesContributedTo": {
        "totalCount": 3,
        "pageInfo": { "hasNextPage": false, "endCursor": null },
        "nodes": [
          {
            "nameWithOwner": "huggingface/accelerate",
            "description": "Accelerate library",
            "url": "https://github.com/huggingface/accelerate",
            "isInOrganization": true,
            "owner": {
              "__typename": "Organization",
              "login": "huggingface",
              "avatarUrl": "https://avatars.githubusercontent.com/u/25720743?v=4"
            },
            "stargazerCount": 8000,
            "forkCount": 900,
            "isFork": false,
            "isPrivate": false,
            "defaultBranchRef": { "target": { "__typename": "Commit", "history": { "totalCount": 1000 } } },
            "issues": { "totalCount": 100 },
            "pullRequests": { "totalCount": 200 }
          },
          {
            "nameWithOwner": "huggingface/transformers",
            "description": "Transformers library",
            "url": "https://github.com/huggingface/transformers",
            "isInOrganization": true,
            "owner": {
              "__typename": "Organization",
              "login": "huggingface",
              "avatarUrl": "https://avatars.githubusercontent.com/u/25720743?v=4"
            },
            "stargazerCount": 120000,
            "forkCount": 24000,
            "isFork": false,
            "isPrivate": false,
            "defaultBranchRef": { "target": { "__typename": "Commit", "history": { "totalCount": 16000 } } },
            "issues": { "totalCount": 900 },
            "pullRequests": { "totalCount": 1500 }
          },
          {
            "nameWithOwner": "torvalds/linux",
            "description": "Linux kernel",
            "url": "https://github.com/torvalds/linux",
            "isInOrganization": false,
            "owner": {
              "__typename": "User",
              "login": "torvalds",
              "avatarUrl": "https://avatars.githubusercontent.com/u/1024025?v=4"
            },
            "stargazerCount": 170000,
            "forkCount": 53000,
            "isFork": false,
            "isPrivate": false,
            "defaultBranchRef": { "target": { "__typename": "Commit", "history": { "totalCount": 1200000 } } },
            "issues": { "totalCount": 0 },
            "pullRequests": { "totalCount": 0 }
          }
        ]
      }
    }
  }
}`

// notableGraphQLIndepthBody returns a single organization contribution.
const notableGraphQLIndepthBody = `{
  "data": {
    "user": {
      "repositoriesContributedTo": {
        "totalCount": 1,
        "pageInfo": { "hasNextPage": false, "endCursor": null },
        "nodes": [
          {
            "nameWithOwner": "huggingface/accelerate",
            "description": "Accelerate library",
            "url": "https://github.com/huggingface/accelerate",
            "isInOrganization": true,
            "owner": {
              "__typename": "Organization",
              "login": "huggingface",
              "avatarUrl": "https://avatars.githubusercontent.com/u/25720743?v=4"
            },
            "stargazerCount": 8000,
            "forkCount": 900,
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
