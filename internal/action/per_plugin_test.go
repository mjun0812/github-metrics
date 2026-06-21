package action

import (
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

// ---------- helpers ----------

// failingPlugin is a minimal Plugin stub whose Run always returns an error.
type failingPlugin struct {
	name string
}

func (f *failingPlugin) Name() string { return f.name }

func (f *failingPlugin) Metadata() *config.PluginMetadata {
	return &config.PluginMetadata{}
}

func (f *failingPlugin) Requires() []plugins.DataKey { return nil }

func (f *failingPlugin) Run(_ context.Context, _ *plugins.PluginContext) (any, error) {
	return nil, errors.New("stub plugin intentional failure")
}
