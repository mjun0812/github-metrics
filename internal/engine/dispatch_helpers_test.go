package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

func TestStringSliceInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputs map[string]any
		want   []string
	}{
		{"missing key", map[string]any{"other": "x"}, nil},
		{"[]string", map[string]any{"k": []string{"a", "b"}}, []string{"a", "b"}},
		{"single string", map[string]any{"k": "hello"}, []string{"hello"}},
		{"empty string", map[string]any{"k": ""}, nil},
		{"[]any drops non-strings", map[string]any{"k": []any{"p", 42, "r"}}, []string{"p", "r"}},
		{"unsupported type", map[string]any{"k": 12345}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stringSliceInput(tc.inputs, "k"); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("stringSliceInput = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAsBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"true bool", true, true},
		{"false bool", false, false},
		{"string true", "true", true},
		{"string 1", "1", true},
		{"string yes", "yes", false},
		{"int", 42, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := asBool(map[string]any{"k": tc.value}, "k"); got != tc.want {
				t.Errorf("asBool(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestOptimizeEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputs map[string]any
		pass   string
		want   bool
	}{
		{"bool form", map[string]any{"svg.optimize.css": true}, "css", true},
		{"bool form false", map[string]any{"svg.optimize.css": false}, "css", false},
		{"slice form", map[string]any{"optimize": []string{"css", "xml"}}, "xml", true},
		{"slice form other pass", map[string]any{"optimize": []string{"css", "xml"}}, "svg", false},
		{"comma-separated string", map[string]any{"optimize": "css, xml"}, "xml", true},
		{"case-insensitive", map[string]any{"optimize": []string{"CSS"}}, "css", true},
		{"absent", map[string]any{}, "css", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := optimizeEnabled(tc.inputs, tc.pass); got != tc.want {
				t.Errorf("optimizeEnabled(%v, %q) = %v, want %v", tc.inputs, tc.pass, got, tc.want)
			}
		})
	}
}

func TestBuildPipelineStages_WithFetcher(t *testing.T) {
	t.Parallel()

	// Use FakeRenderer as image fetcher stand-in — it satisfies render.ImageFetcher
	// because FakeRenderer.Fetch is not required for this path; we only need a non-nil value.
	fetcher := &fakeImageFetcher{}
	stages := buildPipelineStages(context.Background(), map[string]any{}, fetcher)
	// octicon + image-inline
	names := stageNames(stages)
	if !containsStage(names, "octicon") {
		t.Errorf("octicon stage missing: %v", names)
	}
	if !containsStage(names, "image-inline") {
		t.Errorf("image-inline stage missing: %v", names)
	}
}

// ---------------------------------------------------------------------------
// repoToMap (tested via Marshal with a populated Repo field)
// ---------------------------------------------------------------------------

func TestMarshal_WithRepo_WithPrimaryLanguage(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetRepo(&plugins.Repo{
		Owner:                "org",
		Name:                 "repo",
		PrimaryLanguage:      "Go",
		PrimaryLanguageColor: "#00ADD8",
	})

	body, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	repo := got["repo"].(map[string]any)

	lang, ok := repo["primary_language"].(map[string]any)
	if !ok {
		t.Fatalf("primary_language missing; repo = %v", repo)
	}
	if lang["name"] != "Go" {
		t.Errorf("primary_language.name = %v, want Go", lang["name"])
	}
	if lang["color"] != "#00ADD8" {
		t.Errorf("primary_language.color = %v, want #00ADD8", lang["color"])
	}
}

func TestMarshal_WithRepo_NoPrimaryLanguage(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetRepo(&plugins.Repo{
		Owner: "org",
		Name:  "repo",
		// PrimaryLanguage intentionally empty
	})

	body, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// The primary_language key must be absent when PrimaryLanguage == "".
	if strings.Contains(string(body), "primary_language") {
		t.Errorf("primary_language should be absent when empty; got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// collectPluginErrors
// ---------------------------------------------------------------------------

func TestCollectPluginErrors_Nil(t *testing.T) {
	t.Parallel()

	if errs := collectPluginErrors(nil); errs != nil {
		t.Errorf("nil data: want nil, got %v", errs)
	}
}

func TestCollectPluginErrors_NoErrors(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetPlugin("somePlugin", "a plain string value")

	if errs := collectPluginErrors(data); len(errs) != 0 {
		t.Errorf("no errors: want 0, got %d: %v", len(errs), errs)
	}
}

func TestCollectPluginErrors_PluginValueIsError(t *testing.T) {
	// Not parallel: modifies the plugin registry via RegisterForTest.
	// RegisterForTest restores the previous state via t.Cleanup.

	// Register a minimal stub plugin so plugins.Each visits it.
	stub := &stubPlugin{name: "failing-plugin"}
	plugins.RegisterForTest(t, stub)

	data := plugins.NewData()
	// Storing an error value in the Plugins map is how the runner records failures.
	sentinel := &sentinelError{msg: "plugin boom"}
	data.SetPlugin("failing-plugin", sentinel)

	errs := collectPluginErrors(data)
	if len(errs) == 0 {
		t.Fatal("expected at least 1 error, got 0")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "plugin boom") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sentinel error not surfaced: %v", errs)
	}
}

func TestCollectPluginErrors_SnapshotErrors(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.AppendError(&sentinelError{msg: "snapshot err"})

	errs := collectPluginErrors(data)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "snapshot err") {
		t.Errorf("snapshot error not surfaced: %v", errs[0])
	}
}

func TestCollectPluginErrors_Both(t *testing.T) {
	// Not parallel: modifies the plugin registry via RegisterForTest.

	stub := &stubPlugin{name: "bad-plugin"}
	plugins.RegisterForTest(t, stub)

	data := plugins.NewData()
	data.SetPlugin("bad-plugin", &sentinelError{msg: "plugin level"})
	data.AppendError(&sentinelError{msg: "snapshot level"})

	errs := collectPluginErrors(data)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

// ---------------------------------------------------------------------------
// dispatchOutput additional branches
// ---------------------------------------------------------------------------

// TestDispatch_JSONFormat verifies that Format="json" returns valid JSON
// and the application/json MIME type via the Marshal path.
func TestDispatch_JSONFormat(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	deps := Deps{Logger: logger}

	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.User = &plugins.User{Login: "testuser"}

	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "json"},
		deps,
		nil, // no template needed for json
		data,
		nil, // pcPartial unused for json
		&Result{},
	)
	if err != nil {
		t.Fatalf("dispatchOutput(json): %v", err)
	}
	if mime != "application/json" {
		t.Errorf("MIME = %q, want application/json", mime)
	}
	if !json.Valid(out) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}

// TestDispatch_EmptyFormat_NoTemplate verifies that when Format="" and
// there is no template, the default format is "json".
func TestDispatch_EmptyFormat_NoTemplate(t *testing.T) {
	t.Parallel()

	deps := Deps{Logger: slog.Default()}
	data := plugins.NewData()

	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: ""},
		deps,
		nil, // nil template triggers the default-json path
		data,
		nil,
		&Result{},
	)
	if err != nil {
		t.Fatalf("dispatchOutput(empty format, nil tmpl): %v", err)
	}
	if mime != "application/json" {
		t.Errorf("MIME = %q, want application/json (default format)", mime)
	}
	if !json.Valid(out) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}

// TestDispatch_EmptyFormat_WithTemplate verifies that when Format="" and
// a template is present, the template's first supported format is used.
func TestDispatch_EmptyFormat_WithTemplate(t *testing.T) {
	t.Parallel()

	deps := Deps{Logger: slog.Default(), Render: &render.FakeRenderer{}}
	data := plugins.NewData()

	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: ""},
		deps,
		stubTemplate{}, // stubTemplate.Metadata().Formats[0] == "svg"
		data,
		nil,
		&Result{},
	)
	if err != nil {
		t.Fatalf("dispatchOutput(empty format, stub tmpl): %v", err)
	}
	if mime != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml (template default)", mime)
	}
	_ = out
}

// TestDispatch_SVG_NoTemplate verifies that requesting "svg" without a
// template returns an InputError.
func TestDispatch_SVG_NoTemplate(t *testing.T) {
	t.Parallel()

	deps := Deps{Logger: slog.Default()}

	_, _, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg"},
		deps,
		nil, // no template
		plugins.NewData(),
		nil,
		&Result{},
	)
	if err == nil {
		t.Fatal("expected error when requesting svg without a template")
	}
}

// ---------------------------------------------------------------------------
// Helpers and stubs
// ---------------------------------------------------------------------------

// sentinelError is a minimal error for test assertions.
type sentinelError struct{ msg string }

func (e *sentinelError) Error() string { return e.msg }

// stubPlugin is a minimal plugins.Plugin used to register a slot in the
// global registry so plugins.Each visits it during collectPluginErrors tests.
type stubPlugin struct{ name string }

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) Run(_ context.Context, _ *plugins.PluginContext) (any, error) { return nil, nil }

// fakeImageFetcher is a minimal render.ImageFetcher stub for pipeline stage tests.
type fakeImageFetcher struct{}

func (f *fakeImageFetcher) ImgB64(_ context.Context, _ string) (string, error) {
	return "", nil
}

// Verify fakeImageFetcher satisfies the ImageFetcher interface at compile time.
var _ render.ImageFetcher = (*fakeImageFetcher)(nil)

func stageNames(stages []render.PipelineStage) []string {
	names := make([]string, len(stages))
	for i, s := range stages {
		names[i] = s.Name
	}
	return names
}

func containsStage(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// jsonName / lowerFirst / normalizeReflect coverage
// ---------------------------------------------------------------------------
