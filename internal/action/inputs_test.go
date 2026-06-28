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
// TranslateLegacyChromeInputs (#640)
// ---------------------------------------------------------------------

func TestTranslateLegacyChromeInputs_BaseCSV(t *testing.T) {
	t.Parallel()
	in := map[string]any{"base": "header,repositories"}
	TranslateLegacyChromeInputs(in)
	if v, ok := in["chrome_header"]; !ok || v != true {
		t.Errorf("chrome_header = %v (ok=%v); want true", v, ok)
	}
	if v, ok := in["chrome_repositories"]; !ok || v != true {
		t.Errorf("chrome_repositories = %v (ok=%v); want true", v, ok)
	}
	// Sections NOT in the CSV must remain absent.
	if _, ok := in["chrome_metadata"]; ok {
		t.Errorf("chrome_metadata should not be set; got %v", in["chrome_metadata"])
	}
}

func TestTranslateLegacyChromeInputs_BaseEmpty(t *testing.T) {
	t.Parallel()
	in := map[string]any{"base": ""}
	TranslateLegacyChromeInputs(in)
	// No sections to translate → no chrome_* keys set.
	for _, s := range []string{
		"chrome_header", "chrome_activity", "chrome_community",
		"chrome_repositories", "chrome_metadata", "chrome_introduction",
	} {
		if _, ok := in[s]; ok {
			t.Errorf("%s should not be set; got %v", s, in[s])
		}
	}
}

func TestTranslateLegacyChromeInputs_DoesNotOverwriteCanonical(t *testing.T) {
	t.Parallel()
	// Caller pinned chrome_header=false explicitly; base= should NOT
	// override that even though the CSV lists "header".
	in := map[string]any{
		"base":          "header,activity",
		"chrome_header": false,
	}
	TranslateLegacyChromeInputs(in)
	if v, ok := in["chrome_header"]; !ok || v != false {
		t.Errorf("chrome_header overwritten: got %v (ok=%v); want false", v, ok)
	}
	// activity wasn't pre-set, should now be true.
	if v, ok := in["chrome_activity"]; !ok || v != true {
		t.Errorf("chrome_activity = %v (ok=%v); want true", v, ok)
	}
}

func TestTranslateLegacyChromeInputs_PluginBaseActivity(t *testing.T) {
	t.Parallel()
	in := map[string]any{"plugin_base_activity": "yes"}
	TranslateLegacyChromeInputs(in)
	if v, ok := in["chrome_activity"]; !ok || v != true {
		t.Errorf("chrome_activity = %v (ok=%v); want true", v, ok)
	}
}

func TestTranslateLegacyChromeInputs_PluginBaseRepositories(t *testing.T) {
	t.Parallel()
	in := map[string]any{"plugin_base_repositories": true}
	TranslateLegacyChromeInputs(in)
	if v, ok := in["chrome_repositories"]; !ok || v != true {
		t.Errorf("chrome_repositories = %v (ok=%v); want true", v, ok)
	}
}

func TestTranslateLegacyChromeInputs_NoOp(t *testing.T) {
	t.Parallel()
	// Already on chrome_*; no legacy inputs to translate.
	in := map[string]any{"chrome_metadata": true}
	TranslateLegacyChromeInputs(in)
	if len(in) != 1 || in["chrome_metadata"] != true {
		t.Errorf("expected no mutations; got %v", in)
	}
}

func TestTranslateLegacyChromeInputs_NilMap(t *testing.T) {
	t.Parallel()
	// Must not panic.
	TranslateLegacyChromeInputs(nil)
}

func TestTranslateLegacyChromeInputs_SliceBase(t *testing.T) {
	t.Parallel()
	in := map[string]any{"base": []string{"header", "metadata"}}
	TranslateLegacyChromeInputs(in)
	if v, ok := in["chrome_header"]; !ok || v != true {
		t.Errorf("chrome_header = %v (ok=%v); want true", v, ok)
	}
	if v, ok := in["chrome_metadata"]; !ok || v != true {
		t.Errorf("chrome_metadata = %v (ok=%v); want true", v, ok)
	}
}
