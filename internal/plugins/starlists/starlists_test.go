package starlists_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files in tests/golden/...")

type fakeNavigator struct {
	lists []starlists.Starlist
	repos map[string][]string
	err   error
	rErr  error
}

func (f *fakeNavigator) FetchLists(_ context.Context, _ string) ([]starlists.Starlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]starlists.Starlist, len(f.lists))
	copy(out, f.lists)
	return out, nil
}

func (f *fakeNavigator) FetchRepos(_ context.Context, listURL string) ([]string, error) {
	if f.rErr != nil {
		return nil, f.rErr
	}
	r, ok := f.repos[listURL]
	if !ok {
		return nil, errors.New("fakeNavigator: no repos for " + listURL)
	}
	return r, nil
}

func newPC(_ *testing.T, nav starlists.Navigator, inputs map[string]any) *plugins.PluginContext {
	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":             "octocat",
			"plugin_starlists": true,
		},
		Data: data,
	}
	if nav != nil {
		pc.Inputs[starlists.NavigatorKey] = nav
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

// TestRun_Skipped_GraphQLUnavailable — when neither a fake Navigator
// nor a real GraphQL client are wired in, the plugin records a
// *RetryableError on Data.Errors and returns Skipped=true. This is
// the test harness's "no API dep" path; production always has GraphQL
// available because base/core plugins fail loudly without it.
func TestRun_Skipped_GraphQLUnavailable(t *testing.T) {
	t.Parallel()
	pc := newPC(t, nil, nil)
	out, err := starlists.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*starlists.Result)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "graphql client not available" {
		t.Errorf("SkippedReason = %q", r.SkippedReason)
	}
	snapshot := pc.Data.SnapshotErrors()
	if len(snapshot) == 0 {
		t.Fatalf("expected *RetryableError on Data.Errors")
	}
	var re *xerrors.RetryableError
	if !errors.As(snapshot[0], &re) {
		t.Errorf("Data.Errors[0] type = %T, want *xerrors.RetryableError; err=%v", snapshot[0], snapshot[0])
	}
}

// TestRun_Skipped_PuppeteerDisabled skips even with a Navigator present.
func TestRun_Skipped_PuppeteerDisabled(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{lists: []starlists.Starlist{{Name: "foo"}}}
	pc := newPC(t, nav, map[string]any{
		"extras.metrics.run.puppeteer.scrapping": false,
	})
	out, _ := starlists.Plugin.Run(context.Background(), pc)
	r := out.(*starlists.Result)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "puppeteer scrapping disabled via extras" {
		t.Errorf("SkippedReason = %q", r.SkippedReason)
	}
}

// TestRun_Normal_FakeNavigator — 3 lists, default limit applies.
func TestRun_Normal_FakeNavigator(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{lists: []starlists.Starlist{
		{Name: "AI", Description: "ML/AI tools", Count: 12, URL: "/stars/octocat/lists/ai"},
		{Name: "Bandwidth", Description: "Networking", Count: 5, URL: "/stars/octocat/lists/bw"},
		{Name: "Compilers", Description: "Code generation", Count: 8, URL: "/stars/octocat/lists/comp"},
	}}
	pc := newPC(t, nav, nil)
	out, err := starlists.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*starlists.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %s", r.SkippedReason)
	}
	if len(r.List) != 3 {
		t.Errorf("List len = %d, want 3", len(r.List))
	}
	if r.List[0].Name != "AI" {
		t.Errorf("List[0] = %q, want AI (sorted)", r.List[0].Name)
	}
}

// TestRun_Languages — _languages=true joins per-list repos with
// base.Computed.RepositoryList to compute language totals.
func TestRun_Languages(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{
		lists: []starlists.Starlist{
			{Name: "Backend", Count: 2, URL: "/stars/octocat/lists/backend"},
		},
		repos: map[string][]string{
			"/stars/octocat/lists/backend": {"octocat/go-svc", "octocat/rust-svc"},
		},
	}
	pc := newPC(t, nav, map[string]any{
		"plugin_starlists_languages": true,
	})
	pc.Data.Computed.RepositoryList = []plugins.Repository{
		{
			NameWithOwner: "octocat/go-svc",
			Languages: []plugins.LanguageStat{
				{Name: "Go", Color: "#00ADD8", Size: 5000},
			},
		},
		{
			NameWithOwner: "octocat/rust-svc",
			Languages: []plugins.LanguageStat{
				{Name: "Rust", Color: "#dea584", Size: 3000},
			},
		},
	}
	out, _ := starlists.Plugin.Run(context.Background(), pc)
	r := out.(*starlists.Result)
	if len(r.List) != 1 {
		t.Fatalf("List len = %d, want 1", len(r.List))
	}
	langs := r.List[0].Languages
	if len(langs) != 2 {
		t.Fatalf("Languages len = %d, want 2; %+v", len(langs), langs)
	}
	if langs[0].Name != "Go" {
		t.Errorf("Languages[0] = %q, want Go (larger size)", langs[0].Name)
	}
}

// TestRun_LimitRepositories — the per-list repositories slice is
// populated from the navigator and capped by
// plugin_starlists_limit_repositories. The input is fed as a STRING
// ("1"), the shape Action mode delivers. repositories_skip_private
// drops private repos before the cap applies.
func TestRun_LimitRepositories(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{lists: []starlists.Starlist{{
		Name:  "AI",
		Count: 3,
		URL:   "/stars/octocat/lists/ai",
		Repositories: []starlists.Repository{
			{Name: "octocat/pub-a", Description: "first"},
			{Name: "octocat/priv", Description: "secret", IsPrivate: true},
			{Name: "octocat/pub-b", Description: "second"},
		},
	}}}

	// Cap = 1, private repos kept: expect the first repo only.
	pc := newPC(t, nav, map[string]any{
		"plugin_starlists_limit_repositories": "1",
	})
	out, err := starlists.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := out.(*starlists.Result).List
	if len(got) != 1 {
		t.Fatalf("List len = %d, want 1", len(got))
	}
	if len(got[0].Repositories) != 1 {
		t.Fatalf("Repositories len = %d, want 1 (capped by input)", len(got[0].Repositories))
	}
	if got[0].Repositories[0].Name != "octocat/pub-a" {
		t.Errorf("Repositories[0].Name = %q, want octocat/pub-a", got[0].Repositories[0].Name)
	}
	if got[0].Repositories[0].Description != "first" {
		t.Errorf("Repositories[0].Description = %q, want first", got[0].Repositories[0].Description)
	}

	// repositories_skip_private drops the private repo, so cap=2 yields
	// the two public repos (private one excluded, not just truncated).
	pc2 := newPC(t, nav, map[string]any{
		"plugin_starlists_limit_repositories": "2",
		"repositories_skip_private":           true,
	})
	out2, err := starlists.Plugin.Run(context.Background(), pc2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	repos := out2.(*starlists.Result).List[0].Repositories
	if len(repos) != 2 {
		t.Fatalf("skip_private Repositories len = %d, want 2", len(repos))
	}
	for _, r := range repos {
		if r.IsPrivate {
			t.Errorf("private repo %q leaked past repositories_skip_private", r.Name)
		}
	}
}

// TestRun_TimeoutWrapped — FetchLists error wraps as *RetryableError.
func TestRun_TimeoutWrapped(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{err: errors.New("chromedp: timeout")}
	pc := newPC(t, nav, nil)
	_, err := starlists.Plugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *xerrors.RetryableError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "starlists") {
		t.Errorf("err = %v, want starlists-prefixed", err)
	}
}

// TestPartial_Starlists_Golden — partial DOM shape matches golden.
func TestPartial_Starlists_Golden(t *testing.T) {
	r := &starlists.Result{
		List: []starlists.Starlist{
			{Name: "AI", Description: "ML tools", Count: 12},
			{Name: "Compilers", Description: "Codegen", Count: 8},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(starlists.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := starlists.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := goldenPath(t, "classic", "m4", "starlists.svg")
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
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	// Markers asserted against the native-SVG shape (#409 Phase B2). Each
	// starlist is a <g class="starlist"> with a header <text>, a
	// <g class="count"> repo count, and an optional description paragraph.
	for _, marker := range []string{`class="starlist"`, `class="count"`, `>AI</text>`} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

// TestPartial_Starlists_EmptyListRendersHeader guards against the
// regression (issue #474) where a non-Skipped Result with zero star
// lists produced a completely empty card. Upstream still renders the
// "0 Star lists" section header at count zero, so the partial must emit
// non-empty output containing that header even with an empty List.
func TestPartial_Starlists_EmptyListRendersHeader(t *testing.T) {
	t.Parallel()
	for _, list := range [][]starlists.Starlist{
		nil, // List == nil
		{},  // List == empty (non-nil) slice
	} {
		r := &starlists.Result{List: list}
		data := plugins.NewData()
		data.SetPlugin(starlists.Name, r)
		pc := &templates.PartialContext{Data: data}
		got, _, err := starlists.Partial(context.Background(), pc)
		if err != nil {
			t.Fatalf("Partial: %v", err)
		}
		if got == "" {
			t.Fatalf("Partial returned empty output for empty List; want rendered header")
		}
		// Header must reflect the zero count with plural "lists".
		if !strings.Contains(got, "0 Star lists") {
			t.Errorf("partial missing %q for empty List in:\n%s", "0 Star lists", got)
		}
		// The section wrapper is present but carries no <div class="starlist">
		// because the for-loop does not run.
		if !strings.Contains(got, `<section data-section="starlists">`) {
			t.Errorf("partial missing section wrapper in:\n%s", got)
		}
		if strings.Contains(got, `class="starlist"`) {
			t.Errorf("partial unexpectedly rendered a starlist entry for empty List:\n%s", got)
		}
	}
}

// TestPartial_Starlists_Repositories asserts the per-repo card block:
// the `<div class="repositories">` wrapper plus name + description are
// rendered when repos exist, and the wrapper is absent when the slice
// is empty.
func TestPartial_Starlists_Repositories(t *testing.T) {
	t.Parallel()
	r := &starlists.Result{
		List: []starlists.Starlist{
			{
				Name:  "AI",
				Count: 2,
				Repositories: []starlists.Repository{
					{Name: "octocat/repo-a", Description: "handy tool"},
				},
			},
			{Name: "Empty", Count: 0}, // no repositories → no wrapper
		},
	}
	data := plugins.NewData()
	data.SetPlugin(starlists.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := starlists.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, marker := range []string{
		`class="repositories"`,
		`class="repository"`,
		`>octocat/repo-a</text>`,
		`>handy tool</text>`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
	// Exactly one wrapper — the empty starlist must not emit its own.
	if n := strings.Count(got, `class="repositories"`); n != 1 {
		t.Errorf("repositories wrapper count = %d, want 1 (empty list omits it)", n)
	}
}

// TestPartial_Starlists_NoDuplicateMaskID guards against the regression
// where multiple starlists with language bars all emitted the same
// `<mask id="languages-bar">`, which made every list inherit the first
// list's clip rectangle (invalid SVG, broken render).
func TestPartial_Starlists_NoDuplicateMaskID(t *testing.T) {
	t.Parallel()
	r := &starlists.Result{
		List: []starlists.Starlist{
			{
				Name:  "AI",
				Count: 2,
				Languages: []plugins.LanguageStat{
					{Name: "Go", Color: "#00ADD8", Size: 5000},
				},
			},
			{
				Name:  "Backend",
				Count: 2,
				Languages: []plugins.LanguageStat{
					{Name: "Rust", Color: "#dea584", Size: 3000},
				},
			},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(starlists.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := starlists.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	idRe := regexp.MustCompile(`<mask id="([^"]+)"`)
	matches := idRe.FindAllStringSubmatch(got, -1)
	seen := map[string]int{}
	for _, m := range matches {
		seen[m[1]]++
	}
	if len(seen) != 2 {
		t.Errorf("expected 2 distinct mask ids (one per starlist), got %d: %v\n%s", len(seen), seen, got)
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("mask id %q appears %d times in single SVG (duplicate ids are invalid):\n%s", id, n, got)
		}
		if !strings.HasPrefix(id, "starlists-bar-") {
			t.Errorf("mask id %q should be prefixed with starlists-bar- to avoid colliding with languages plugin", id)
		}
	}
}

func goldenPath(t *testing.T, parts ...string) string {
	t.Helper()
	root := repoRoot(t)
	return filepath.Join(append([]string{root, "tests", "golden"}, parts...)...)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", cwd)
	return ""
}
