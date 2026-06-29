package action

import (
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
