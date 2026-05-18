package topics_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files in tests/golden/...")

// fakeNavigator returns a canned topic list, optionally with an error.
type fakeNavigator struct {
	list []topics.Topic
	err  error
}

func (f *fakeNavigator) Fetch(_ context.Context, _ string) ([]topics.Topic, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]topics.Topic, len(f.list))
	copy(out, f.list)
	return out, nil
}

func newPC(_ *testing.T, nav topics.Navigator, inputs map[string]any) *plugins.PluginContext {
	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":          "octocat",
			"plugin_topics": true,
		},
		Data: data,
	}
	if nav != nil {
		pc.Inputs[topics.NavigatorKey] = nav
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

// TestRun_Skipped_ChromedpUnavailable — no navigator and no Render =>
// chromedp not available, returns Skipped=true and records a
// *RetryableError on Data.Errors per contract §3.4 step 2.
func TestRun_Skipped_ChromedpUnavailable(t *testing.T) {
	t.Parallel()
	pc := newPC(t, nil, nil)
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "chromedp not available" {
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

// TestRun_Skipped_PuppeteerDisabled — extras toggle short-circuits
// before any navigator interaction.
func TestRun_Skipped_PuppeteerDisabled(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{list: []topics.Topic{{Name: "go"}}}
	pc := newPC(t, nav, map[string]any{
		"extras.metrics.run.puppeteer.scrapping": false,
	})
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "puppeteer scrapping disabled via extras" {
		t.Errorf("SkippedReason = %q", r.SkippedReason)
	}
}

// TestRun_Normal_FakeNavigator — happy path: 3 topics, sort=name (default),
// limit not hit. Asserts the plugin returns them in alpha order.
func TestRun_Normal_FakeNavigator(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{list: []topics.Topic{
		{Name: "rust", URL: "/topics/rust", Icon: "/img/rust.png"},
		{Name: "Go", URL: "/topics/go", Icon: "/img/go.png"},
		{Name: "python", URL: "/topics/python", Icon: "/img/py.png"},
	}}
	pc := newPC(t, nav, nil)
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %s", r.SkippedReason)
	}
	names := make([]string, 0, len(r.List))
	for _, top := range r.List {
		names = append(names, top.Name)
	}
	want := []string{"Go", "python", "rust"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// TestRun_TimeoutWrapped — Navigator returning an error wraps it as
// *RetryableError so the engine surfaces it on Result.Errors.
func TestRun_TimeoutWrapped(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{err: errors.New("chromedp: navigate timeout")}
	pc := newPC(t, nav, nil)
	_, err := topics.Plugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *xerrors.RetryableError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "topics") {
		t.Errorf("err = %v, want topics-prefixed", err)
	}
}

// TestPartial_Topics_Golden checks the SVG fragment shape against the
// committed golden file.
func TestPartial_Topics_Golden(t *testing.T) {
	r := &topics.Result{
		List: []topics.Topic{
			{Name: "go", Icon: "https://github.githubassets.com/topics/go.png", URL: "/topics/go"},
			{Name: "rust", Icon: "https://github.githubassets.com/topics/rust.png", URL: "/topics/rust"},
		},
		Mode:  "icons",
		Limit: 15,
		Sort:  "name",
	}
	data := plugins.NewData()
	data.SetPlugin(topics.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := topics.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := goldenPath(t, "classic", "m4", "topics.svg")
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
	// 011 v2: upstream-equivalent emission. Topics partial now mirrors
	// upstream EJS — h2 header + per-mode rendering (img tags for icons,
	// div.label for labels). The legacy <g class="topic"> + <text> shape
	// is replaced.
	for _, marker := range []string{`<h2 class="field">`, `Starred topics`, `<div class="topics fill-width">`, `<img src="https://github.githubassets.com/topics/go.png"`} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
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
