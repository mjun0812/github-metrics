package topics_test

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
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
// It records the last fetched URL so tests can assert the sort mapping.
type fakeNavigator struct {
	list   []topics.Topic
	err    error
	gotURL string
}

func (f *fakeNavigator) Fetch(_ context.Context, url string) ([]topics.Topic, error) {
	f.gotURL = url
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

// TestRun_DefaultNavigator_IsHTTP — when no Navigator is injected the
// plugin falls back to the stdlib-backed httpNavigator. The plugin is
// no longer "skipped" for missing chromedp because the SSR topics page
// is reachable via plain HTTPS.
func TestRun_DefaultNavigator_IsHTTP(t *testing.T) {
	t.Parallel()
	// Stand up a local server returning a minimal stars/topics page
	// (5 anchors). The navigator follows whatever URL the plugin
	// constructs, so we cannot redirect there — instead we point the
	// plugin at the local server via a NavigatorKey that wraps a
	// real httpNavigator with our test endpoint.
	body := `<!doctype html><html><body>
<a href="/topics/go"><p>Go</p><p>Go lang</p><img src="/go.png"></a>
<a href="/topics/rust"><p>Rust</p><p>Rust lang</p><img src="/rust.png"></a>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	httpNav := topics.NewHTTPNavigator(srv.Client(), "test-agent")
	pc := newPC(t, &fixedURLNav{inner: httpNav, target: srv.URL}, nil)
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %s", r.SkippedReason)
	}
	if len(r.List) != 2 {
		t.Fatalf("List len = %d, want 2; %+v", len(r.List), r.List)
	}
}

// fixedURLNav forwards Fetch to inner but always targets `target`,
// ignoring the URL the plugin constructed. Used in network tests so we
// don't have to mutate the plugin's URL builder.
type fixedURLNav struct {
	inner  topics.Navigator
	target string
}

func (f *fixedURLNav) Fetch(ctx context.Context, _ string) ([]topics.Topic, error) {
	return f.inner.Fetch(ctx, f.target)
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

// TestRun_Normal_FakeNavigator — happy path: 3 topics, limit not hit.
// Asserts the plugin preserves the page's own order (#672).
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
	// Page order is preserved (#672): sorting is delegated to the
	// stars/topics page's own sort parameter, so no client-side
	// re-ordering happens.
	want := []string{"rust", "Go", "python"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v (page order preserved)", names, want)
	}
}

// TestRun_SortMapsToPageParameter pins the #672 fix: the declared
// plugin_topics_sort values map to the stars/topics page's server-side
// sort parameter, and the default is the declared "stars" (previously
// every declared value silently fell back to alphabetical).
func TestRun_SortMapsToPageParameter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sort  any // absent when nil
		param string
	}{
		{nil, "sort=stars"},           // declared default
		{"stars", "sort=stars"},       // Most stars
		{"activity", "sort=updated"},  // Recently active
		{"starred", "sort=created"},   // Recently starred
		{"bogus-value", "sort=stars"}, // unknown → declared default
	}
	for _, c := range cases {
		nav := &fakeNavigator{}
		inputs := map[string]any{}
		if c.sort != nil {
			inputs["plugin_topics_sort"] = c.sort
		}
		pc := newPC(t, nav, inputs)
		if _, err := topics.Plugin.Run(context.Background(), pc); err != nil {
			t.Fatalf("Run(sort=%v): %v", c.sort, err)
		}
		if !strings.Contains(nav.gotURL, c.param) || !strings.Contains(nav.gotURL, "direction=desc") {
			t.Errorf("sort=%v: fetched %q, want it to contain %q and direction=desc",
				c.sort, nav.gotURL, c.param)
		}
	}
}

// TestRun_DefaultModeIsStarred pins the #672 fix: absent
// plugin_topics_mode must behave as the metadata default `starred`
// (labels), not the previous internal default `icons`.
func TestRun_DefaultModeIsStarred(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{list: []topics.Topic{{Name: "go"}}}
	pc := newPC(t, nav, nil)
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if r.Mode != "starred" {
		t.Errorf("Mode = %q, want starred (metadata default)", r.Mode)
	}
}

// TestRun_TruncationAppendsMoreLabel pins the #672 fix: labels-mode
// truncation appends the upstream "and N more..." pseudo-label, while
// icons mode truncates silently (the icons renderer has nothing to
// draw for a text-only entry).
func TestRun_TruncationAppendsMoreLabel(t *testing.T) {
	t.Parallel()
	five := []topics.Topic{
		{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}, {Name: "e"},
	}
	// labels (default starred) mode: trailer appended.
	nav := &fakeNavigator{list: five}
	pc := newPC(t, nav, map[string]any{"plugin_topics_limit": "2"})
	out, err := topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*topics.Result)
	if len(r.List) != 3 {
		t.Fatalf("List len = %d, want 3 (2 kept + trailer): %+v", len(r.List), r.List)
	}
	if r.List[2].Name != "and 3 more..." {
		t.Errorf("trailer = %q, want 'and 3 more...'", r.List[2].Name)
	}
	// icons mode: plain truncation, no trailer.
	nav = &fakeNavigator{list: five}
	pc = newPC(t, nav, map[string]any{"plugin_topics_limit": "2", "plugin_topics_mode": "icons"})
	out, err = topics.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run(icons): %v", err)
	}
	r = out.(*topics.Result)
	if len(r.List) != 2 {
		t.Errorf("icons List len = %d, want 2 (no trailer): %+v", len(r.List), r.List)
	}
}

// TestPartial_MasteredRendersIcons pins the #672 fix: `mastered` is the
// metadata alias for icons mode, so its body must be <img> entries, not
// text labels (previously only literal "icons" hit the image branch).
func TestPartial_MasteredRendersIcons(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(topics.Name, &topics.Result{
		Mode: "mastered",
		List: []topics.Topic{{Name: "go", Icon: "/img/go.png"}},
	})
	got, _, err := topics.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "Mastered technologies and topics") {
		t.Errorf("missing mastered heading in:\n%s", got)
	}
	if !strings.Contains(got, "<img src=") {
		t.Errorf("mastered mode should render icons (<img>); got:\n%s", got)
	}
	if strings.Contains(got, `class="label"`) {
		t.Errorf("mastered mode must not render text labels; got:\n%s", got)
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
	got, _, err := topics.Partial(context.Background(), pc)
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
