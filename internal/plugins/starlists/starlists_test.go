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
	got, err := starlists.Partial(context.Background(), pc)
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
	// Markers asserted against the upstream-parity DOM shape (post 011).
	// Each starlist is a <div class="starlist"> with a header <h2>, a
	// <div class="count"> repo count, and an optional <div class="description">.
	for _, marker := range []string{`<div class="starlist">`, `class="count"`, `<h2 class="field">`} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
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
	got, err := starlists.Partial(context.Background(), pc)
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
