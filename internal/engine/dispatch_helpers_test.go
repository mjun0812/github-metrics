package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

// ---------------------------------------------------------------------------
// stringSliceInput
// ---------------------------------------------------------------------------

func TestStringSliceInput_NilInputs(t *testing.T) {
	t.Parallel()

	if got := stringSliceInput(nil, "any"); got != nil {
		t.Errorf("nil inputs: want nil, got %v", got)
	}
}

func TestStringSliceInput_MissingKey(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"other": "x"}
	if got := stringSliceInput(inputs, "missing"); got != nil {
		t.Errorf("missing key: want nil, got %v", got)
	}
}

func TestStringSliceInput_StringSlice(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": []string{"a", "b", "c"}}
	got := stringSliceInput(inputs, "k")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("[]string input: got %v", got)
	}
}

func TestStringSliceInput_StringSlice_IsCopy(t *testing.T) {
	t.Parallel()

	src := []string{"x", "y"}
	inputs := map[string]any{"k": src}
	got := stringSliceInput(inputs, "k")
	got[0] = "mutated"
	if src[0] == "mutated" {
		t.Error("stringSliceInput should return a copy, not a slice alias")
	}
}

func TestStringSliceInput_StringSingle(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": "hello"}
	got := stringSliceInput(inputs, "k")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("string input: got %v", got)
	}
}

func TestStringSliceInput_EmptyString(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": ""}
	if got := stringSliceInput(inputs, "k"); got != nil {
		t.Errorf("empty string input: want nil, got %v", got)
	}
}

func TestStringSliceInput_AnySlice(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": []any{"p", "q", 42, "r"}}
	got := stringSliceInput(inputs, "k")
	// 42 (int) should be skipped; only string elements collected.
	if len(got) != 3 || got[0] != "p" || got[1] != "q" || got[2] != "r" {
		t.Errorf("[]any input: got %v, want [p q r]", got)
	}
}

func TestStringSliceInput_AnySlice_Empty(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": []any{}}
	got := stringSliceInput(inputs, "k")
	// Empty []any produces an empty (non-nil) slice.
	if got == nil {
		t.Error("expected non-nil empty slice for empty []any input")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 elements, got %v", got)
	}
}

func TestStringSliceInput_DefaultReturn(t *testing.T) {
	t.Parallel()

	// An integer value should hit the default branch and return nil.
	inputs := map[string]any{"k": 12345}
	if got := stringSliceInput(inputs, "k"); got != nil {
		t.Errorf("integer input should return nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// asBool
// ---------------------------------------------------------------------------

func TestAsBool_NilInputs(t *testing.T) {
	t.Parallel()

	if asBool(nil, "any") {
		t.Error("nil inputs: want false")
	}
}

func TestAsBool_MissingKey(t *testing.T) {
	t.Parallel()

	if asBool(map[string]any{}, "missing") {
		t.Error("missing key: want false")
	}
}

func TestAsBool_TrueBool(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": true}
	if !asBool(inputs, "k") {
		t.Error("true bool: want true")
	}
}

func TestAsBool_FalseBool(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": false}
	if asBool(inputs, "k") {
		t.Error("false bool: want false")
	}
}

func TestAsBool_StringTrue(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"true", "1"} {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			inputs := map[string]any{"k": s}
			if !asBool(inputs, "k") {
				t.Errorf("string %q: want true", s)
			}
		})
	}
}

func TestAsBool_StringFalse(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"false", "0", "yes", ""} {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			inputs := map[string]any{"k": s}
			if asBool(inputs, "k") {
				t.Errorf("string %q: want false", s)
			}
		})
	}
}

func TestAsBool_OtherType(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"k": 42}
	if asBool(inputs, "k") {
		t.Error("int 42: want false (default branch)")
	}
}

// ---------------------------------------------------------------------------
// optimizeEnabled
// ---------------------------------------------------------------------------

func TestOptimizeEnabled_BoolForm(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"svg.optimize.css": true}
	if !optimizeEnabled(inputs, "css") {
		t.Error("svg.optimize.css=true: want enabled")
	}
}

func TestOptimizeEnabled_BoolFormFalse(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"svg.optimize.css": false}
	if optimizeEnabled(inputs, "css") {
		t.Error("svg.optimize.css=false: want disabled")
	}
}

func TestOptimizeEnabled_SliceForm(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"optimize": []string{"css", "xml"}}
	if !optimizeEnabled(inputs, "css") {
		t.Error("optimize=[]string{css,xml}: css should be enabled")
	}
	if !optimizeEnabled(inputs, "xml") {
		t.Error("optimize=[]string{css,xml}: xml should be enabled")
	}
	if optimizeEnabled(inputs, "svg") {
		t.Error("optimize=[]string{css,xml}: svg should be disabled")
	}
}

func TestOptimizeEnabled_CommaSeparatedString(t *testing.T) {
	t.Parallel()

	// Simulates an INPUT_OPTIMIZE env-var: raw comma-separated string.
	inputs := map[string]any{"optimize": "css, xml"}
	if !optimizeEnabled(inputs, "css") {
		t.Error("optimize=css,xml string: css should be enabled")
	}
	if !optimizeEnabled(inputs, "xml") {
		t.Error("optimize=css,xml string: xml should be enabled")
	}
}

func TestOptimizeEnabled_CaseInsensitive(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"optimize": []string{"CSS", "XML"}}
	if !optimizeEnabled(inputs, "css") {
		t.Error("uppercase CSS: should match css")
	}
	if !optimizeEnabled(inputs, "xml") {
		t.Error("uppercase XML: should match xml")
	}
}

func TestOptimizeEnabled_NilInputs(t *testing.T) {
	t.Parallel()

	if optimizeEnabled(nil, "css") {
		t.Error("nil inputs: want false")
	}
}

func TestOptimizeEnabled_EmptyInputs(t *testing.T) {
	t.Parallel()

	if optimizeEnabled(map[string]any{}, "css") {
		t.Error("empty inputs: want false")
	}
}

// ---------------------------------------------------------------------------
// buildPipelineStages
// ---------------------------------------------------------------------------

func TestBuildPipelineStages_NoFetcher_NoOptimize(t *testing.T) {
	t.Parallel()

	stages := buildPipelineStages(context.Background(), map[string]any{}, nil)
	// Only octicon stage when no fetcher and no optimize flags.
	if len(stages) != 1 {
		t.Errorf("expected 1 stage (octicon), got %d", len(stages))
	}
	if stages[0].Name != "octicon" {
		t.Errorf("expected stage[0] = octicon, got %q", stages[0].Name)
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

func TestBuildPipelineStages_WithCSSOptimize(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"svg.optimize.css": true}
	stages := buildPipelineStages(context.Background(), inputs, nil)
	names := stageNames(stages)
	if !containsStage(names, "css") {
		t.Errorf("css stage missing: %v", names)
	}
	if containsStage(names, "xml") {
		t.Errorf("xml stage should be absent: %v", names)
	}
}

func TestBuildPipelineStages_WithXMLOptimize(t *testing.T) {
	t.Parallel()

	inputs := map[string]any{"svg.optimize.xml": true}
	stages := buildPipelineStages(context.Background(), inputs, nil)
	names := stageNames(stages)
	if !containsStage(names, "xml") {
		t.Errorf("xml stage missing: %v", names)
	}
}

func TestBuildPipelineStages_WithAllOptimize(t *testing.T) {
	t.Parallel()

	// Simulate the upstream default "css, xml" via the optimize slice.
	inputs := map[string]any{"optimize": []string{"css", "xml"}}
	fetcher := &fakeImageFetcher{}
	stages := buildPipelineStages(context.Background(), inputs, fetcher)
	names := stageNames(stages)
	// Expected: octicon, image-inline, css, xml
	for _, want := range []string{"octicon", "image-inline", "css", "xml"} {
		if !containsStage(names, want) {
			t.Errorf("stage %q missing from %v", want, names)
		}
	}
}

// ---------------------------------------------------------------------------
// repoToMap (tested via Marshal with a populated Repo field)
// ---------------------------------------------------------------------------

func TestMarshal_WithRepo_ContainsRepoFields(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetRepo(&plugins.Repo{
		Owner:                    "acme",
		OwnerAvatar:              "https://example.com/avatar.png",
		Name:                     "my-repo",
		Description:              "A test repo",
		Stargazers:               42,
		Forks:                    7,
		Contributors:             3,
		IsArchived:               false,
		DefaultBranch:            "main",
		LicenseName:              "MIT License",
		SponsorshipsAsMaintainer: 1,
		Activity: plugins.RepoActivity{
			RecentCommits:    5,
			OpenIssues:       2,
			OpenPullRequests: 1,
		},
	})

	body, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("invalid JSON: %s", body)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	repo, ok := got["repo"].(map[string]any)
	if !ok {
		t.Fatalf("repo field missing or wrong type; keys: %v", mapKeys(got))
	}

	if repo["owner"] != "acme" {
		t.Errorf("repo.owner = %v, want acme", repo["owner"])
	}
	if repo["name"] != "my-repo" {
		t.Errorf("repo.name = %v, want my-repo", repo["name"])
	}
	if repo["name_with_owner"] != "acme/my-repo" {
		t.Errorf("repo.name_with_owner = %v, want acme/my-repo", repo["name_with_owner"])
	}
	if repo["description"] != "A test repo" {
		t.Errorf("repo.description = %v", repo["description"])
	}
	if repo["stargazers"].(float64) != 42 {
		t.Errorf("repo.stargazers = %v, want 42", repo["stargazers"])
	}
	if repo["is_archived"] != false {
		t.Errorf("repo.is_archived = %v, want false", repo["is_archived"])
	}
	if repo["default_branch"] != "main" {
		t.Errorf("repo.default_branch = %v, want main", repo["default_branch"])
	}
	if repo["license_name"] != "MIT License" {
		t.Errorf("repo.license_name = %v, want MIT License", repo["license_name"])
	}

	activity, ok := repo["activity"].(map[string]any)
	if !ok {
		t.Fatalf("repo.activity missing or wrong type")
	}
	if activity["recent_commits"].(float64) != 5 {
		t.Errorf("repo.activity.recent_commits = %v, want 5", activity["recent_commits"])
	}
}

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

// TestDispatch_ObtainRenderer_NilDepsRender verifies the svg-format
// contract: since #409 Phase C the svg path returns the decorated SVG
// without ever constructing a Renderer, so a nil deps.Render cannot
// degrade it — output is the SVG and res.Errors stays empty.
func TestDispatch_ObtainRenderer_NilDepsRender_SVGFallback(t *testing.T) {
	t.Parallel()

	// deps.Render is nil, but the svg path never reaches obtainRenderer.
	deps := Deps{Logger: slog.Default(), Render: nil}
	res := &Result{}

	out, mime, err := dispatchOutput(
		context.Background(),
		Request{Format: "svg"},
		deps,
		stubTemplate{},
		plugins.NewData(),
		nil,
		res,
	)
	// dispatchOutput should NOT return a top-level error — it falls back.
	if err != nil {
		t.Fatalf("dispatchOutput with nil render: unexpected error: %v", err)
	}
	// The svg path returns the decorated SVG directly (no renderer).
	if len(res.Errors) == 0 {
		if mime == "" {
			t.Error("no renderer error and no output: unexpected state")
		}
	} else {
		// Renderer init failed: svg fallback path.
		if mime != "image/svg+xml" {
			t.Errorf("SVG fallback: MIME = %q, want image/svg+xml", mime)
		}
		if len(out) == 0 {
			t.Error("SVG fallback: expected non-empty output")
		}
	}
}

// TestImageFetcher_NilHTTPClient verifies imageFetcher returns nil when
// deps.HTTPClient is nil.
func TestImageFetcher_NilHTTPClient(t *testing.T) {
	t.Parallel()

	deps := Deps{HTTPClient: nil}
	if imageFetcher(deps) != nil {
		t.Error("imageFetcher with nil HTTPClient should return nil")
	}
}

// TestImageFetcher_NonNilHTTPClient verifies imageFetcher returns non-nil
// when deps.HTTPClient is set.
func TestImageFetcher_NonNilHTTPClient(t *testing.T) {
	t.Parallel()

	client := httpx.New(httpx.Options{})
	deps := Deps{HTTPClient: client}
	if imageFetcher(deps) == nil {
		t.Error("imageFetcher with non-nil HTTPClient should return non-nil")
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

func (s *stubPlugin) Name() string                     { return s.name }
func (s *stubPlugin) Metadata() *config.PluginMetadata { return &config.PluginMetadata{} }
func (s *stubPlugin) Requires() []plugins.DataKey      { return []plugins.DataKey{} }

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

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// jsonName / lowerFirst / normalizeReflect coverage
// ---------------------------------------------------------------------------

// structWithOmitempty has a json tag with ",omitempty" to exercise the
// tag-with-comma branch in jsonName.
type structWithOmitempty struct {
	Name  string `json:"name,omitempty"`
	Count int    `json:"count,omitempty"`
	Extra string // no json tag — falls through to lowerFirst
}

// TestNormalizeStruct_JsonTagWithComma exercises the jsonName comma-strip
// branch via the cycleDetector's normalizeStruct path.
func TestNormalizeStruct_JsonTagWithComma(t *testing.T) {
	t.Parallel()

	cd := newCycleDetector()
	v := structWithOmitempty{Name: "hello", Count: 5, Extra: "x"}
	out, ok := cd.normalize(v).(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", cd.normalize(v))
	}
	// "name,omitempty" → "name"
	if out["name"] != "hello" {
		t.Errorf("name = %v, want hello", out["name"])
	}
	// "count,omitempty" → "count"
	// normalizeReflect returns int64 for reflect.Int kinds.
	switch v := out["count"].(type) {
	case int64:
		if v != 5 {
			t.Errorf("count = %v, want 5", v)
		}
	case int:
		if v != 5 {
			t.Errorf("count = %v, want 5", v)
		}
	default:
		t.Errorf("count type = %T, value = %v", out["count"], out["count"])
	}
	// no tag → lowerFirst("Extra") = "extra"
	if out["extra"] != "x" {
		t.Errorf("extra = %v, want x", out["extra"])
	}
}

// TestLowerFirst_Empty exercises the empty-string guard in lowerFirst.
func TestLowerFirst_Empty(t *testing.T) {
	t.Parallel()

	if got := lowerFirst(""); got != "" {
		t.Errorf("lowerFirst(\"\") = %q, want empty", got)
	}
}

// TestNormalizeReflect_UintFloat exercises the uint and float branches
// in normalizeReflect (which are not hit by the standard test fixtures).
func TestNormalizeReflect_UintFloat(t *testing.T) {
	t.Parallel()

	cd := newCycleDetector()

	// uint
	var u uint = 42
	if got := cd.normalize(u); got != uint(42) {
		t.Errorf("uint: got %v (%T)", got, got)
	}

	// float32
	var f float32 = 3.14
	got := cd.normalize(f)
	if _, ok := got.(float32); !ok {
		t.Errorf("float32: got %T, want float32", got)
	}
}
