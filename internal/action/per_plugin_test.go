package action

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"

	// Side-effect imports register the classic template and the plugins
	// required by the test scenarios.
	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/plugins/header"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// TestPerPluginOutput_ThreePlugins exercises per-plugin mode with three
// enabled plugins (header, languages, stars) and asserts that three
// separate SVG files are written to the output directory.
func TestPerPluginOutput_ThreePlugins(t *testing.T) {
	outDir := t.TempDir()
	rest := newFakeREST()

	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Token:    "ghp_mock_pat_valid",
		Template: "classic",
		Output:   "svg",
		Dryrun:   true,
		Plugins: map[string]string{
			"plugin_header":    "yes",
			"plugin_languages": "yes",
			"plugin_stars":     "yes",
		},
	}, runOptions{
		Mode:      ModeCLI,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
		Env:       []string{},
	})
	if err != nil {
		t.Fatalf("runCLIWith per-plugin: %v", err)
	}

	for _, plugin := range []string{"header", "languages", "stars"} {
		path := filepath.Join(outDir, plugin+".svg")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s; err=%v", path, err)
		}
	}
}

// TestPerPluginOutput_PluginLocalFailure injects a failing stub plugin
// and asserts that the other two plugins still produce output files.
func TestPerPluginOutput_PluginLocalFailure(t *testing.T) {
	outDir := t.TempDir()
	rest := newFakeREST()

	// Register a stub "stub" plugin that always errors.
	stub := &failingPlugin{name: "stub"}
	plugins.RegisterForTest(t, stub)

	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Token:    "ghp_mock_pat_valid",
		Template: "classic",
		Output:   "svg",
		Dryrun:   true,
		Plugins: map[string]string{
			"plugin_header": "yes",
			"plugin_stars":  "yes",
			"plugin_stub":   "yes",
		},
	}, runOptions{
		Mode:      ModeCLI,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
		Env:       []string{},
	})
	// Per-plugin mode: partial failures are non-fatal; runCLIWith returns nil.
	if err != nil {
		t.Fatalf("runCLIWith per-plugin (partial fail): %v", err)
	}

	// Successful plugins should have output.
	for _, plugin := range []string{"header", "stars"} {
		path := filepath.Join(outDir, plugin+".svg")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s for successful plugin; err=%v", path, err)
		}
	}

	// The failing plugin should NOT have an output file.
	stubPath := filepath.Join(outDir, "stub.svg")
	if _, err := os.Stat(stubPath); err == nil {
		t.Errorf("stub.svg should not exist because the plugin failed")
	}
}

// TestPerPluginOutput_Combined exercises --combined mode and asserts that
// a single SVG file is written at the configured output path.
func TestPerPluginOutput_Combined(t *testing.T) {
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "m.svg")
	rest := newFakeREST()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Token:    "ghp_mock_pat_valid",
		Template: "classic",
		Output:   "svg",
		Filename: outFile,
		Dryrun:   true,
		Combined: true,
		Plugins: map[string]string{
			"plugin_header": "yes",
			"plugin_stars":  "yes",
		},
	}, runOptions{
		Mode:      ModeCLI,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
		Env:       []string{},
	})
	if err != nil {
		t.Fatalf("runCLIWith combined: %v", err)
	}

	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("expected single combined SVG at %s; err=%v", outFile, err)
	}
}

// TestPerPluginOutput_AppendErrorFailure injects a plugin that records
// its failure via pc.Data.AppendError instead of returning a non-nil
// error from Run. The per-plugin pipeline must still treat this as a
// plugin-local failure and skip writing its SVG file, even though the
// failure does not flow through Data.SetPlugin(name, err).
func TestPerPluginOutput_AppendErrorFailure(t *testing.T) {
	outDir := t.TempDir()
	rest := newFakeREST()

	// Register a stub whose Run reports failure via AppendError.
	stub := &appendErrorPlugin{name: "stubappend"}
	plugins.RegisterForTest(t, stub)

	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Token:    "ghp_mock_pat_valid",
		Template: "classic",
		Output:   "svg",
		Dryrun:   true,
		Plugins: map[string]string{
			"plugin_header":     "yes",
			"plugin_stars":      "yes",
			"plugin_stubappend": "yes",
		},
	}, runOptions{
		Mode:      ModeCLI,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
		Env:       []string{},
	})
	if err != nil {
		t.Fatalf("runCLIWith per-plugin (append-error): %v", err)
	}

	for _, plugin := range []string{"header", "stars"} {
		path := filepath.Join(outDir, plugin+".svg")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s for successful plugin; err=%v", path, err)
		}
	}

	stubPath := filepath.Join(outDir, "stubappend.svg")
	if _, err := os.Stat(stubPath); err == nil {
		t.Errorf("stubappend.svg should not exist because the plugin used AppendError to signal failure")
	}
}

// TestPerPluginOutput_ActionMode exercises the Action-side dispatch
// (`runWith`, not `runCLIWith`) with INPUT_COMBINED=no and asserts that
// per-plugin SVGs land in the directory pointed to by INPUT_OUTPUT_DIR.
func TestPerPluginOutput_ActionMode(t *testing.T) {
	// Disabled t.Parallel: t.Setenv is documented as parallel-unsafe.
	rest := newFakeREST()
	outDir := t.TempDir()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	var stdout bytes.Buffer
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TEMPLATE=classic",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_DRYRUN=yes",
			"INPUT_OUTPUT_ACTION=none",
			"INPUT_USE_MOCKED_DATA=false",
			"INPUT_COMBINED=no",
			"INPUT_OUTPUT_DIR=" + outDir,
			"INPUT_PLUGIN_HEADER=yes",
			"INPUT_PLUGIN_STARS=yes",
		},
		Stdout:    &stdout,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runWith per-plugin (Action mode): %v", err)
	}

	for _, plugin := range []string{"header", "stars"} {
		path := filepath.Join(outDir, plugin+".svg")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected Action per-plugin file %s; err=%v", path, err)
		}
	}
}

// ---------- helpers ----------

// gateEnabled returns true when inputs["plugin_<name>"] is truthy. Real
// plugins inspect their gate in Run() and self-skip when off; the test
// stubs below honor the same contract so per-plugin mode's
// disableAllPluginGates fan-out actually limits execution to the
// targeted plugin.
func gateEnabled(pc *plugins.PluginContext, name string) bool {
	if pc == nil || pc.Inputs == nil {
		return false
	}
	v, ok := pc.Inputs["plugin_"+name]
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	switch s {
	case "yes", "true", "1", "on":
		return true
	}
	return false
}

// failingPlugin is a minimal Plugin stub whose Run returns an error
// when its gate is enabled and self-skips otherwise.
type failingPlugin struct {
	name string
}

func (f *failingPlugin) Name() string { return f.name }

func (f *failingPlugin) Metadata() *config.PluginMetadata {
	return &config.PluginMetadata{}
}

func (f *failingPlugin) Requires() []plugins.DataKey { return nil }

func (f *failingPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if !gateEnabled(pc, f.name) {
		return nil, nil
	}
	return nil, errors.New("stub plugin intentional failure")
}

// appendErrorPlugin is a Plugin stub that records its failure via
// pc.Data.AppendError instead of returning a non-nil error from Run.
// It self-skips when its gate is off so per-plugin fan-out targets it
// exactly once.
type appendErrorPlugin struct {
	name string
}

func (p *appendErrorPlugin) Name() string { return p.name }

func (p *appendErrorPlugin) Metadata() *config.PluginMetadata {
	return &config.PluginMetadata{}
}

func (p *appendErrorPlugin) Requires() []plugins.DataKey { return nil }

func (p *appendErrorPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if !gateEnabled(pc, p.name) {
		return nil, nil
	}
	pc.Data.AppendError(errors.New("stubappend plugin intentional append-error failure"))
	return nil, nil
}
