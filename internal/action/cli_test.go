package action

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFlags_AllRecognized(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{
		"--user", "octocat",
		"--template", "classic",
		"--token-env", "GH_TOKEN",
		"--plugin", "plugin_languages=true",
		"--plugin", "plugin_languages_limit=5",
		"--output", "png",
		"--filename", "-",
		"--dryrun",
		"--preset", "p.yaml",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cf.User != "octocat" || cf.Template != "classic" || cf.TokenEnv != "GH_TOKEN" {
		t.Errorf("scalar flags: %+v", cf)
	}
	if cf.Output != "png" || cf.Filename != "-" || !cf.Dryrun {
		t.Errorf("output/filename/dryrun: %+v", cf)
	}
	if cf.Plugins["plugin_languages"] != "true" || cf.Plugins["plugin_languages_limit"] != "5" {
		t.Errorf("plugin map = %v", cf.Plugins)
	}
	if cf.Preset != "p.yaml" {
		t.Errorf("preset = %q", cf.Preset)
	}
	// `--filename -` must implicitly enable combined mode because
	// per-plugin mode cannot fan out multiple SVGs to a single stream.
	if !cf.Combined {
		t.Errorf("--filename - should imply Combined=true; got false")
	}
}

// TestParseFlags_PerPluginFlags covers the per-plugin surface added in
// PR #606: --combined, --output-dir, and --plugins (including the
// whitespace-trim path for comma-separated values).
func TestParseFlags_PerPluginFlags(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{
		"--user", "octocat",
		"--combined",
		"--output-dir", "./out",
		"--plugins", "header, languages,stars",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !cf.Combined {
		t.Errorf("--combined: want true, got false")
	}
	if cf.OutputDir != "./out" {
		t.Errorf("--output-dir: got %q", cf.OutputDir)
	}
	want := []string{"header", "languages", "stars"}
	if len(cf.PluginList) != len(want) {
		t.Fatalf("--plugins length: want %d, got %d (%v)", len(want), len(cf.PluginList), cf.PluginList)
	}
	for i, name := range want {
		if cf.PluginList[i] != name {
			t.Errorf("--plugins[%d]: want %q, got %q", i, name, cf.PluginList[i])
		}
	}
}

func TestParseFlags_DefaultsApplied(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{"--user", "x"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cf.Template != "classic" || cf.Output != "svg" {
		t.Errorf("defaults not applied: template=%q output=%q", cf.Template, cf.Output)
	}
}

func TestParseFlags_PluginMustBeKV(t *testing.T) {
	t.Parallel()
	_, err := ParseFlags([]string{"--plugin", "bogus"})
	if err == nil {
		t.Errorf("expected error for malformed --plugin")
	}
}

func TestLoadYAMLConfig_NestedFlattening(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inputs.yaml")
	body := `user: octocat
template: classic
output: svg
plugins:
  languages: true
  languages_limit: 5
config:
  timezone: Asia/Tokyo
  padding: 5%
committer:
  message: hello
output_action: commit
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	doc, err := LoadYAMLConfig(path)
	if err != nil {
		t.Fatalf("LoadYAMLConfig: %v", err)
	}
	want := map[string]any{
		"user":                   "octocat",
		"template":               "classic",
		"config_output":          "svg",
		"plugin_languages":       true,
		"plugin_languages_limit": 5,
		"config_timezone":        "Asia/Tokyo",
		"config_padding":         "5%",
		"committer_message":      "hello",
		"output_action":          "commit",
	}
	for k, v := range want {
		got, ok := doc[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if !equalAny(got, v) {
			t.Errorf("%s: got %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}
}

func TestLoadYAMLConfig_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := LoadYAMLConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestLoadYAMLConfig_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("this is: not: yaml: ["), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadYAMLConfig(path); err == nil {
		t.Errorf("expected error for malformed YAML")
	}
}

func TestToInvocation_PriorityCLIBeatsConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(cfg, []byte("user: from-config\ntemplate: classic\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cf := &CLIFlags{
		Config:   cfg,
		User:     "from-cli",
		Template: "classic",
		Output:   "svg",
		Plugins:  map[string]string{},
	}
	inputs, err := cf.ToInvocation(map[string]string{})
	if err != nil {
		t.Fatalf("ToInvocation: %v", err)
	}
	if inputs["user"] != "from-cli" {
		t.Errorf("CLI flag must beat YAML config; got %v", inputs["user"])
	}
}

func TestToInvocation_PresetEmitsConfigPresets(t *testing.T) {
	t.Parallel()
	cf := &CLIFlags{User: "x", Template: "classic", Output: "svg", Preset: "p.yaml", Plugins: map[string]string{}}
	inputs, err := cf.ToInvocation(map[string]string{})
	if err != nil {
		t.Fatalf("ToInvocation: %v", err)
	}
	if inputs["config_presets"] != "p.yaml" {
		t.Errorf("preset path should land in config_presets; got %v", inputs["config_presets"])
	}
}

func TestToInvocation_PluginsMerged(t *testing.T) {
	t.Parallel()
	cf := &CLIFlags{
		User: "x", Template: "classic", Output: "svg",
		Plugins: map[string]string{"plugin_languages": "true", "plugin_languages_limit": "5"},
	}
	inputs, _ := cf.ToInvocation(map[string]string{})
	if inputs["plugin_languages"] != "true" || inputs["plugin_languages_limit"] != "5" {
		t.Errorf("plugins not merged: %v", inputs)
	}
}

func TestResolveToken_FlagEnvPrecedence(t *testing.T) {
	t.Parallel()
	env := map[string]string{"FOO": "from-env"}
	get := func(k string) string { return env[k] }
	tok, err := ResolveToken(&CLIFlags{TokenEnv: "FOO", Token: "ignored"}, get, "")
	if err != nil || tok != "from-env" {
		t.Errorf("tok=%q err=%v want from-env", tok, err)
	}
}

func TestResolveToken_TokenFlagOnly(t *testing.T) {
	t.Parallel()
	tok, err := ResolveToken(&CLIFlags{Token: "ghp_abc"}, nil, "")
	if err != nil || tok != "ghp_abc" {
		t.Errorf("tok=%q err=%v", tok, err)
	}
}

func TestResolveToken_InputTokenFallback(t *testing.T) {
	t.Parallel()
	tok, err := ResolveToken(&CLIFlags{}, nil, "ghp_from_env")
	if err != nil || tok != "ghp_from_env" {
		t.Errorf("tok=%q err=%v", tok, err)
	}
}

func TestResolveToken_EnvEmptyIsError(t *testing.T) {
	t.Parallel()
	_, err := ResolveToken(&CLIFlags{TokenEnv: "MISSING"}, func(string) string { return "" }, "")
	if err == nil {
		t.Errorf("expected error for empty --token-env target")
	}
}

func TestResolveToken_NoneIsError(t *testing.T) {
	t.Parallel()
	_, err := ResolveToken(&CLIFlags{}, nil, "")
	if err == nil {
		t.Errorf("expected error when no token provided")
	}
}

func TestResolveOutputWriter_Stdout(t *testing.T) {
	t.Parallel()
	w, closeFn, err := ResolveOutputWriter("-", "svg")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer func() { _ = closeFn() }()
	if w != os.Stdout {
		t.Errorf("expected os.Stdout for '-'")
	}
}

func TestResolveOutputWriter_FilePath_MkdirP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "nested", "out.svg")
	w, closeFn, err := ResolveOutputWriter(target, "svg")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer func() { _ = closeFn() }()
	if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := closeFn(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Errorf("close: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("output file missing: %v", err)
	}
}

// equalAny is a forgiving equality for map values that may differ in
// integer width (yaml.v3 → int vs literal int).
func equalAny(a, b any) bool {
	switch av := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return av == bv
		}
		if bv, ok := b.(int64); ok {
			return int64(av) == bv
		}
	case int64:
		if bv, ok := b.(int); ok {
			return av == int64(bv)
		}
	}
	return a == b
}
