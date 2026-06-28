package action

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInputs_FromInputUpper(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"INPUT_USER":     "octocat",
		"INPUT_TEMPLATE": "classic",
		"INPUT_TOKEN":    "ghp_redacted",
		"UNRELATED":      "ignored",
	}
	got, err := ParseInputs(env)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"user":     "octocat",
		"template": "classic",
		"token":    "ghp_redacted",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := got["unrelated"]; ok {
		t.Errorf("non-INPUT_ key leaked into output: %v", got)
	}
}

func TestParseInputs_InputsJSON_OverridesInputUpper(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"INPUT_USER":  "old-value",
		"INPUT_TOKEN": "ghp_via_upper",
		"INPUTS":      `{"user":"new-value","template":"classic"}`,
	}
	got, err := ParseInputs(env)
	if err != nil {
		t.Fatal(err)
	}
	if got["user"] != "new-value" {
		t.Errorf("INPUTS JSON should override INPUT_USER; got %v", got["user"])
	}
	if got["template"] != "classic" {
		t.Errorf("INPUTS JSON template missing; got %v", got["template"])
	}
	if got["token"] != "ghp_via_upper" {
		t.Errorf("INPUT_TOKEN should survive when not in INPUTS; got %v", got["token"])
	}
}

func TestParseInputs_MalformedJSON_Errors(t *testing.T) {
	t.Parallel()
	env := map[string]string{"INPUTS": "not-json"}
	if _, err := ParseInputs(env); err == nil {
		t.Errorf("expected error for malformed INPUTS JSON")
	}
}

func TestParseInputs_EmptyInputsString_Skipped(t *testing.T) {
	t.Parallel()
	env := map[string]string{"INPUTS": "", "INPUT_USER": "x"}
	got, err := ParseInputs(env)
	if err != nil {
		t.Fatal(err)
	}
	if got["user"] != "x" {
		t.Errorf("INPUT_USER should be retained when INPUTS is empty; got %v", got["user"])
	}
}

func TestParseInputs_KeyCasing(t *testing.T) {
	t.Parallel()
	env := map[string]string{"INPUT_PLUGIN_LANGUAGES_LIMIT": "5"}
	got, err := ParseInputs(env)
	if err != nil {
		t.Fatal(err)
	}
	if got["plugin_languages_limit"] != "5" {
		t.Errorf("INPUT_<UPPER> not lowercased; got %v", got)
	}
}

func TestWildcardFilename(t *testing.T) {
	t.Parallel()
	cases := []struct {
		template, format, want string
		wantErr                bool
	}{
		{"github-metrics.*", "svg", "github-metrics.svg", false},
		{"github-metrics.*", "png", "github-metrics.png", false},
		{"github-metrics.*", "jpeg", "github-metrics.jpeg", false},
		{"github-metrics.*", "jpg", "github-metrics.jpeg", false}, // jpg → jpeg
		{"github-metrics.*", "json", "github-metrics.json", false},
		{"github-metrics.*", "", "github-metrics.svg", false}, // empty → svg
		{"custom.svg", "json", "custom.svg", false},           // no wildcard, pass-through
		{"a.*.b.*.c", "svg", "", true},                        // multiple wildcards → error
	}
	for _, tc := range cases {
		t.Run(tc.template+"_"+tc.format, func(t *testing.T) {
			got, err := WildcardFilename(tc.template, tc.format)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadPreset_AndMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "preset.yaml")
	body := []byte(`
q:
  plugin_languages: true
  plugin_languages_limit: 5
  template: classic
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	preset, err := LoadPreset(path)
	if err != nil {
		t.Fatalf("LoadPreset: %v", err)
	}
	if preset.Q["plugin_languages_limit"] != 5 {
		t.Errorf("preset.Q[plugin_languages_limit] = %v, want 5", preset.Q["plugin_languages_limit"])
	}

	// MergeInto: existing keys not overwritten, new keys added.
	inputs := map[string]any{
		"template": "user-supplied", // already set
	}
	preset.MergeInto(inputs)
	if inputs["template"] != "user-supplied" {
		t.Errorf("preset should not overwrite existing key; got %v", inputs["template"])
	}
	if inputs["plugin_languages"] != true {
		t.Errorf("preset should add new key; got %v", inputs["plugin_languages"])
	}
}

func TestLoadPreset_MissingQMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(path, []byte("other: 1\n"), 0o600)
	if _, err := LoadPreset(path); err == nil {
		t.Errorf("expected error when q: map is missing")
	}
}

func TestLoadPreset_FileNotFound(t *testing.T) {
	t.Parallel()
	if _, err := LoadPreset("/nonexistent/preset.yaml"); err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestPresetBundle_MergeInto_NilReceiver(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat"}
	var p *PresetBundle
	p.MergeInto(inputs)
	if inputs["user"] != "octocat" {
		t.Errorf("nil MergeInto mutated inputs: %v", inputs)
	}
}

// ---------------------------------------------------------------------
// WarnLegacyChromeInputs (#649)
// ---------------------------------------------------------------------

func TestWarnLegacyChromeInputs_DoesNotMutate(t *testing.T) {
	t.Parallel()
	// The warning is purely diagnostic — the inputs map must be
	// untouched so downstream sees the raw legacy keys as unknown.
	in := map[string]any{
		"base":                     "header,repositories",
		"plugin_base_activity":     "yes",
		"plugin_base_repositories": "yes",
		"chrome_metadata":          true,
	}
	WarnLegacyChromeInputs(in)
	if len(in) != 4 {
		t.Errorf("WarnLegacyChromeInputs should not add/remove keys; got %v", in)
	}
	for _, k := range []string{"base", "plugin_base_activity", "plugin_base_repositories", "chrome_metadata"} {
		if _, ok := in[k]; !ok {
			t.Errorf("key %q should be preserved; got %v", k, in)
		}
	}
}

func TestWarnLegacyChromeInputs_NilMap(t *testing.T) {
	t.Parallel()
	// Must not panic.
	WarnLegacyChromeInputs(nil)
}
