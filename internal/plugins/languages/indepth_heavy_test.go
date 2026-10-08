//go:build heavy

package languages_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// makeRepo creates a minimal local git repository in dir containing the
// supplied (path, content) pairs. Returns the directory path so the
// caller can hand it to the fakeCloner.
func makeRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if merr := os.MkdirAll(filepath.Dir(full), 0o755); merr != nil {
			t.Fatalf("mkdir parent for %s: %v", name, merr)
		}
		if werr := os.WriteFile(full, []byte(content), 0o644); werr != nil {
			t.Fatalf("write %s: %v", name, werr)
		}
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for name := range files {
		if _, aerr := wt.Add(name); aerr != nil {
			t.Fatalf("Add %s: %v", name, aerr)
		}
	}
	_, err = wt.Commit("seed", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return dir
}

// fakeCloner implements the cloner interface by copying a prepared
// source directory into the destination. Tests use this to avoid
// hitting a real git server.
type fakeCloner struct {
	sources       map[string]string // URL -> source dir
	failURLs      map[string]error
	delay         time.Duration
	cloneCallback func(url string)
}

func (f *fakeCloner) Clone(ctx context.Context, dst, url string) (string, error) {
	if f.cloneCallback != nil {
		f.cloneCallback(url)
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if err, ok := f.failURLs[url]; ok {
		return "", err
	}
	src, ok := f.sources[url]
	if !ok {
		return "", errors.New("fakeCloner: unknown URL " + url)
	}
	// Recursively copy src → dst.
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	return dst, nil
}

// newIndepthPC builds a minimal PluginContext with the indepth plugin
// enabled and a fakeCloner injected via the cloner-key input.
func newIndepthPC(t *testing.T, cln languages.IndepthCloner, repos []plugins.Repository, inputs map[string]any) *plugins.PluginContext {
	t.Helper()
	data := plugins.NewData()
	data.Computed.RepositoryList = repos
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":                     "octocat",
			"plugin_languages":         true,
			"plugin_languages_indepth": true,
			languages.IndepthClonerKey: cln,
		},
		Data: data,
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

// TestIndepth_Normal — 2 repos, each cloned + analyzed, totals merged.
func TestIndepth_Normal(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	srcA := makeRepo(t, filepath.Join(base, "src-a"), map[string]string{
		"main.go": strings.Repeat("// hello world\n", 50),
	})
	srcB := makeRepo(t, filepath.Join(base, "src-b"), map[string]string{
		"app.js": strings.Repeat("// console log\n", 30),
	})
	cln := &fakeCloner{sources: map[string]string{
		"https://github.com/octocat/alpha.git": srcA,
		"https://github.com/octocat/beta.git":  srcB,
	}}
	repos := []plugins.Repository{
		{NameWithOwner: "octocat/alpha"},
		{NameWithOwner: "octocat/beta"},
	}
	pc := newIndepthPC(t, cln, repos, nil)
	out, err := languages.IndepthPlugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*languages.IndepthResult)
	if r.Skipped {
		t.Fatalf("Skipped: %s", r.SkippedReason)
	}
	if len(r.Analyzed) != 2 {
		t.Errorf("Analyzed = %v, want 2 repos", r.Analyzed)
	}
	if r.Total.Bytes["Go"] == 0 {
		t.Errorf("Go bytes = 0, want > 0 (total=%v)", r.Total.Bytes)
	}
	if r.Total.Bytes["JavaScript"] == 0 {
		t.Errorf("JavaScript bytes = 0, want > 0 (total=%v)", r.Total.Bytes)
	}
}

// TestIndepth_ExtrasMissing — extras flags off → Skipped=true.
func TestIndepth_ExtrasMissing(t *testing.T) {
	t.Parallel()
	cln := &fakeCloner{sources: map[string]string{}}
	pc := newIndepthPC(t, cln, nil, map[string]any{
		"extras.metrics.run.git": false,
	})
	out, _ := languages.IndepthPlugin.Run(context.Background(), pc)
	r := out.(*languages.IndepthResult)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "indepth extras not satisfied" {
		t.Errorf("SkippedReason = %q", r.SkippedReason)
	}
}

// TestIndepth_RepoTimeout — slow clone is killed by per-repo timeout
// but other repos continue.
func TestIndepth_RepoTimeout(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	srcOK := makeRepo(t, filepath.Join(base, "ok"), map[string]string{
		"ok.go": "package ok\n",
	})
	cln := &fakeCloner{
		sources: map[string]string{
			"https://github.com/octocat/slow.git": srcOK,
			"https://github.com/octocat/fast.git": srcOK,
		},
		// Per-call delay distinguished by URL via callback.
		cloneCallback: func(url string) {
			if strings.Contains(url, "slow") {
				time.Sleep(200 * time.Millisecond)
			}
		},
	}
	repos := []plugins.Repository{
		{NameWithOwner: "octocat/slow"},
		{NameWithOwner: "octocat/fast"},
	}
	pc := newIndepthPC(t, cln, repos, map[string]any{
		"plugin_languages_analysis_timeout_repositories": "50ms",
	})
	out, _ := languages.IndepthPlugin.Run(context.Background(), pc)
	r := out.(*languages.IndepthResult)
	// fast repo should be analyzed; slow should be in Errors.
	if !contains(r.Analyzed, "octocat/fast") {
		t.Errorf("Analyzed = %v, want contains octocat/fast", r.Analyzed)
	}
	if !anyContains(r.Errors, "octocat/slow") {
		t.Errorf("Errors = %v, want contains octocat/slow", r.Errors)
	}
}

// TestIndepth_CloneFailure — a clone-stage error lands in Errors but
// does not break the other repos.
func TestIndepth_CloneFailure(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	srcOK := makeRepo(t, filepath.Join(base, "ok"), map[string]string{
		"keep.go": "package keep\n",
	})
	cln := &fakeCloner{
		sources: map[string]string{
			"https://github.com/octocat/ok.git": srcOK,
		},
		failURLs: map[string]error{
			"https://github.com/octocat/bad.git": errors.New("disk full"),
		},
	}
	repos := []plugins.Repository{
		{NameWithOwner: "octocat/ok"},
		{NameWithOwner: "octocat/bad"},
	}
	pc := newIndepthPC(t, cln, repos, nil)
	out, _ := languages.IndepthPlugin.Run(context.Background(), pc)
	r := out.(*languages.IndepthResult)
	if !contains(r.Analyzed, "octocat/ok") {
		t.Errorf("Analyzed = %v, want contains octocat/ok", r.Analyzed)
	}
	if !anyContains(r.Errors, "disk full") {
		t.Errorf("Errors = %v, want contains 'disk full'", r.Errors)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func anyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// TestIndepth_PartialSizeAndPercentageShareAggregation — the details rows
// (order, size, percentage) must all come from the indepth totals, not a
// mix of the indepth bytes and the GraphQL-based standard aggregation.
func TestIndepth_PartialSizeAndPercentageShareAggregation(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	src := makeRepo(t, filepath.Join(base, "src"), map[string]string{
		"main.go": strings.Repeat("// hello world\n", 50),
		"app.js":  strings.Repeat("// console log\n", 30),
	})
	cln := &fakeCloner{sources: map[string]string{
		"https://github.com/octocat/alpha.git": src,
	}}
	repos := []plugins.Repository{{
		NameWithOwner: "octocat/alpha",
		Languages: []plugins.LanguageStat{
			{Name: "JavaScript", Color: "#f1e05a", Size: 90000},
			{Name: "Go", Color: "#00ADD8", Size: 10},
		},
	}}
	inputs := map[string]any{"plugin_languages_details": "bytes-size, percentage"}
	pc := newIndepthPC(t, cln, repos, inputs)
	ctx := context.Background()

	std, err := languages.Plugin.Run(ctx, pc)
	if err != nil {
		t.Fatalf("languages Run: %v", err)
	}
	ind, err := languages.IndepthPlugin.Run(ctx, pc)
	if err != nil {
		t.Fatalf("indepth Run: %v", err)
	}
	pc.Data.SetPlugin(languages.Name, std)
	pc.Data.SetPlugin(languages.IndepthName, ind)

	got, _, err := languages.Partial(ctx, &templates.PartialContext{Data: pc.Data, Inputs: pc.Inputs})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// Go 750 B (62.5%), JavaScript 450 B (37.5%) of 1200 B; Go ranks first.
	for _, want := range []string{"750 B  62.5%", "450 B  37.5%", "estimation from 1.2 kB of code"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
	if strings.Index(got, `data-language="Go"`) > strings.Index(got, `data-language="JavaScript"`) {
		t.Errorf("Go must be listed before JavaScript (indepth order)")
	}
}

// TestIndepth_CategoriesFilterRankedLanguages pins plugin_languages_categories
// (default "markup, programming"): data languages such as JSON stay in the
// headline total but are left out of the ranked rows, as upstream's
// analyzer only adds allowed categories to its stats.
func TestIndepth_CategoriesFilterRankedLanguages(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	src := makeRepo(t, filepath.Join(base, "src"), map[string]string{
		"main.go":   strings.Repeat("// hello world\n", 50),
		"data.json": "[" + strings.Repeat(`"0123456789",`, 200) + `""]`,
	})
	cln := &fakeCloner{sources: map[string]string{
		"https://github.com/octocat/alpha.git": src,
	}}
	repos := []plugins.Repository{{NameWithOwner: "octocat/alpha"}}

	for _, tc := range []struct {
		name       string
		categories string
		wantJSON   bool
	}{
		{name: "default", wantJSON: false},
		{name: "data allowed", categories: "data, programming", wantJSON: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := map[string]any{}
			if tc.categories != "" {
				inputs["plugin_languages_categories"] = tc.categories
			}
			pc := newIndepthPC(t, cln, repos, inputs)
			out, err := languages.IndepthPlugin.Run(context.Background(), pc)
			if err != nil {
				t.Fatalf("indepth Run: %v", err)
			}
			r := out.(*languages.IndepthResult)
			if r.Total.Bytes["JSON"] == 0 {
				t.Fatalf("Total must keep JSON bytes for the headline; got %v", r.Total.Bytes)
			}
			gotJSON := false
			for _, f := range r.Favorites {
				if f.Name == "JSON" {
					gotJSON = true
				}
			}
			if gotJSON != tc.wantJSON {
				t.Errorf("JSON in favorites = %v, want %v (favorites %+v)", gotJSON, tc.wantJSON, r.Favorites)
			}
		})
	}
}
