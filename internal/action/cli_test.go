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
		"--plugin", "plugin_languages=true",
		"--plugin", "plugin_languages_limit=5",
		"--output", "png",
		"--filename", "-",
		"--dryrun",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cf.User != "octocat" || cf.Template != "classic" {
		t.Errorf("scalar flags: %+v", cf)
	}
	if cf.Output != "png" || cf.Filename != "-" || !cf.Dryrun {
		t.Errorf("output/filename/dryrun: %+v", cf)
	}
	if cf.Plugins["plugin_languages"] != "true" || cf.Plugins["plugin_languages_limit"] != "5" {
		t.Errorf("plugin map = %v", cf.Plugins)
	}
}

// TestParseFlags_TokenFlagsRejected pins the v3.0 removal of --token /
// --token-env. The flag parser must reject both with the standard
// `flag provided but not defined` error.
func TestParseFlags_TokenFlagsRejected(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--token", "ghp_xxx"},
		{"--token-env", "GH_TOKEN"},
	} {
		if _, err := ParseFlags(args); err == nil {
			t.Errorf("ParseFlags(%v): expected error for removed flag", args)
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

func TestLoadYAMLConfig_NestedSectionMustBeMap(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad-section.yaml")
	if err := os.WriteFile(path, []byte("plugins: true\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadYAMLConfig(path); err == nil {
		t.Errorf("expected nested section type error")
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

func TestToInvocation_RepoOwnerPrefixStripped(t *testing.T) {
	t.Parallel()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "repository",
		Repo:     "octocat/hello-world",
		Output:   "svg",
		Plugins:  map[string]string{},
	}
	inputs, err := cf.ToInvocation(map[string]string{})
	if err != nil {
		t.Fatalf("ToInvocation: %v", err)
	}
	if inputs["repo"] != "hello-world" {
		t.Errorf("repo = %v, want hello-world", inputs["repo"])
	}
}

func TestToInvocation_ConfigErrorPropagates(t *testing.T) {
	t.Parallel()
	cf := &CLIFlags{Config: filepath.Join(t.TempDir(), "missing.yaml"), Plugins: map[string]string{}}
	if _, err := cf.ToInvocation(map[string]string{}); err == nil {
		t.Errorf("expected config load error")
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

func TestResolveOutputWriter_FilePathError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	if _, _, err := ResolveOutputWriter(filepath.Join(blocker, "out.svg"), "svg"); err == nil {
		t.Errorf("expected mkdir error under file path")
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
