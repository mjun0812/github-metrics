package notable_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
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

// TestRun_LimitReadsStringInput pins the #661 fix: Action mode delivers
// plugin_notable_limit as a string (INPUT_* env), which the previous
// bare v.(int) assertion silently ignored.
func TestRun_LimitReadsStringInput(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_repositories": true,
		"plugin_notable_limit":        "1",
	}, notableGraphQLContributionsBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1 (string limit honored): %+v", len(r.List), r.List)
	}
}

// TestRun_TypesAcceptsJSONArray pins the #664 fix: INPUTS-JSON arrays
// decode to []any, which the previous []string-only switch dropped —
// silently collapsing the requested types back to [COMMIT].
func TestRun_TypesAcceptsJSONArray(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	var gotTypes []any
	gql.OnFunc("UserNotable", func(vars map[string]any) (int, string) {
		if v, ok := vars["types"].([]any); ok {
			gotTypes = v
		}
		return 200, notableGraphQLContributionsBody
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(gql),
		mocks.WithInputs(map[string]any{
			"user":                 "octocat",
			"plugin_notable":       true,
			"plugin_notable_types": []any{"commit", "pull_request"},
		}),
	)
	if _, err := notable.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(gotTypes) != 2 {
		t.Fatalf("query types = %v, want [COMMIT PULL_REQUEST]", gotTypes)
	}
	want := map[string]bool{"COMMIT": false, "PULL_REQUEST": false}
	for _, v := range gotTypes {
		if s, ok := v.(string); ok {
			want[s] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("type %s missing from query variables %v", k, gotTypes)
		}
	}
}

// TestRun_SkippedAcceptsJSONArray pins the #664 fix for
// plugin_notable_skipped: a JSON-array value ([]any) must populate the
// skip filter like the comma-separated string form.
func TestRun_SkippedAcceptsJSONArray(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_repositories": true,
		"plugin_notable_skipped":      []any{"huggingface/accelerate"},
	}, notableGraphQLContributionsBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1 (accelerate skipped): %+v", len(r.List), r.List)
	}
	if r.List[0].Name != "huggingface/transformers" {
		t.Errorf("Name = %q, want huggingface/transformers", r.List[0].Name)
	}
}

// TestRun_SkipPrivateDropsPrivateContributions asserts the cross-plugin
// repositories_skip_private input (#656) drops isPrivate nodes from the
// contributions list.
func TestRun_SkipPrivateDropsPrivateContributions(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_repositories": true,
		"repositories_skip_private":   "yes",
	}, notableGraphQLMixedVisibilityBody)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1 (public only): %+v", len(r.List), r.List)
	}
	if r.List[0].Name != "huggingface/accelerate" {
		t.Errorf("Name = %q, want huggingface/accelerate", r.List[0].Name)
	}
}

// TestRun_SkipPrivateDefaultOffKeepsPrivateContributions pins the
// default: without the flag, private contributions the token can see
// stay in the list.
func TestRun_SkipPrivateDefaultOffKeepsPrivateContributions(t *testing.T) {
	t.Parallel()
	r, _ := runWithGraphQL(t, map[string]any{
		"user":                        "octocat",
		"plugin_notable":              true,
		"plugin_notable_repositories": true,
	}, notableGraphQLMixedVisibilityBody)
	if len(r.List) != 2 {
		t.Fatalf("List len = %d, want 2 (public + private): %+v", len(r.List), r.List)
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

// TestRun_FollowsPagination pins the #743 fix: the plugin must walk
// repositoriesContributedTo via pageInfo.hasNextPage / endCursor so
// contributions beyond the first 100-repo page still surface.
func TestRun_FollowsPagination(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFunc("UserNotable", func(vars map[string]any) (int, string) {
		if after, _ := vars["after"].(string); after == "CURSOR1" {
			return 200, notablePageBody("org2", false, "")
		}
		return 200, notablePageBody("org1", true, "CURSOR1")
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(gql),
		mocks.WithInputs(map[string]any{
			"user":                        "octocat",
			"plugin_notable":              true,
			"plugin_notable_from":         "all",
			"plugin_notable_repositories": true,
		}),
	)
	out, err := notable.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*notable.Result)
	if got := gql.Calls("UserNotable"); got != 2 {
		t.Fatalf("UserNotable calls = %d, want 2 (page 1 + page 2)", got)
	}
	names := map[string]bool{}
	for _, c := range r.List {
		names[c.Login] = true
	}
	if !names["org1"] || !names["org2"] {
		t.Fatalf("second-page contribution missing; got %+v", r.List)
	}
	// Natural exhaustion (hasNextPage=false) is not a degraded state:
	// no truncation error may be recorded.
	if errs := pc.Data.SnapshotErrors(); len(errs) != 0 {
		t.Errorf("SnapshotErrors = %v, want none for a fully-walked connection", errs)
	}
}

// TestRun_PaginationCapsPages pins the #743 defensive cap: a response
// that always advertises a next page must not loop forever — the walk
// stops at notableMaxPages requests.
func TestRun_PaginationCapsPages(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	gql.OnFunc("UserNotable", func(_ map[string]any) (int, string) {
		return 200, notablePageBody("org1", true, "CURSOR")
	})
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(gql),
		mocks.WithInputs(map[string]any{
			"user":           "octocat",
			"plugin_notable": true,
		}),
	)
	if _, err := notable.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := gql.Calls("UserNotable"); got != notable.NotableMaxPages {
		t.Fatalf("UserNotable calls = %d, want %d (page cap)", got, notable.NotableMaxPages)
	}
	// The cap-hit truncation must not be silent: one aggregated
	// AppendError surfaces the degraded state (traffic/activity
	// precedent).
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 {
		t.Fatalf("SnapshotErrors len = %d, want 1; errors: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "page cap reached") {
		t.Errorf("error message should mention page cap; got %q", errs[0].Error())
	}
}

// notablePageBody builds a single-node repositoriesContributedTo page
// owned by an organization, with the given pageInfo cursor state.
func notablePageBody(owner string, hasNext bool, endCursor string) string {
	cursor := "null"
	if endCursor != "" {
		cursor = `"` + endCursor + `"`
	}
	return `{"data":{"user":{"repositoriesContributedTo":{
		"totalCount":1,
		"pageInfo":{"hasNextPage":` + strconv.FormatBool(hasNext) + `,"endCursor":` + cursor + `},
		"nodes":[{
			"nameWithOwner":"` + owner + `/repo",
			"description":null,
			"url":"https://github.com/` + owner + `/repo",
			"isInOrganization":true,
			"owner":{"__typename":"Organization","login":"` + owner + `","avatarUrl":"a"},
			"stargazerCount":10,"forkCount":1,"isFork":false,"isPrivate":false,
			"defaultBranchRef":{"target":{"__typename":"Commit","history":{"totalCount":5}}},
			"issues":{"totalCount":1},"pullRequests":{"totalCount":1}
		}]
	}}}}`
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
	got, _, err := notable.Partial(context.Background(), pc)
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
	got, _, err := notable.Partial(context.Background(), pc)
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

// TestPartial_DefaultModeOmitsIndepthClass pins upstream parity (#557):
// upstream `partials/notable.ejs` emits a single chip class for both
// basic and indepth modes, with only the gauge SVGs differing. Default
// chips must therefore not carry the divergent `indepth` token.
func TestPartial_DefaultModeOmitsIndepthClass(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           "huggingface",
			AvatarURL:      "https://example.invalid/avatar.png",
			Organization:   true,
			Login:          "huggingface",
			Repo:           "huggingface/accelerate",
			Type:           "owner",
			StargazerCount: 12000,
			// Indepth left false on purpose: default-mode chip.
		}},
	})
	pc := &templates.PartialContext{Data: data}
	got, _, err := notable.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if strings.Contains(got, "indepth") {
		t.Errorf("default-mode chip must not carry the `indepth` class; got:\n%s", got)
	}
}

// TestPartial_IndepthModeOmitsIndepthClass pins upstream parity (#557):
// even in indepth mode the chip class must match upstream
// `partials/notable.ejs` exactly (`organization contribution <level>`).
// Earlier we attached an extra `indepth` token to scope a fixed-width
// CSS rule, but that fixed width forced the gauge stack to collide
// inside the box — so the marker is removed and the chip width becomes
// content-driven again, matching upstream.
func TestPartial_IndepthModeOmitsIndepthClass(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(notable.Name, &notable.Result{
		List: []notable.NotableContrib{{
			Name:           "huggingface/accelerate",
			AvatarURL:      "https://example.invalid/avatar.png",
			Organization:   true,
			Login:          "huggingface",
			Repo:           "huggingface/accelerate",
			Type:           "owner",
			StargazerCount: 12000,
			Indepth:        true,
			Commits:        42,
			Percentage:     0.5,
		}},
	})
	pc := &templates.PartialContext{Data: data}
	got, _, err := notable.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if strings.Contains(got, "indepth") {
		t.Errorf("indepth chip must not carry the `indepth` class; got:\n%s", got)
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
	got, _, err := notable.Partial(context.Background(), pc)
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

// notableGraphQLMixedVisibilityBody mixes one public and one private
// organization-owned repository so the repositories_skip_private filter
// can be exercised.
const notableGraphQLMixedVisibilityBody = `{
  "data": {
    "user": {
      "repositoriesContributedTo": {
        "totalCount": 2,
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
            "nameWithOwner": "acme/secret-tool",
            "description": "Internal tool",
            "url": "https://github.com/acme/secret-tool",
            "isInOrganization": true,
            "owner": {
              "__typename": "Organization",
              "login": "acme",
              "avatarUrl": "https://avatars.githubusercontent.com/u/1?v=4"
            },
            "stargazerCount": 2,
            "forkCount": 0,
            "isFork": false,
            "isPrivate": true,
            "defaultBranchRef": { "target": { "__typename": "Commit", "history": { "totalCount": 10 } } },
            "issues": { "totalCount": 1 },
            "pullRequests": { "totalCount": 2 }
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
