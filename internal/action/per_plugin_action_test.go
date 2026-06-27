package action

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	// Side-effect imports register additional plugins for per-plugin tests.
	_ "github.com/mjun0812/github-metrics/internal/plugins/header"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
)

// TestRunCLIWith_PerPlugin_DefaultMode verifies that per-plugin mode
// writes one SVG per enabled plugin to the output directory.
func TestRunCLIWith_PerPlugin_DefaultMode(t *testing.T) {
	outDir := t.TempDir()

	rest := newFakeREST()
	// CLIFlags with no --filename and no --combined → per-plugin mode.
	// We enable header and stars plugins.
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "", // not set → per-plugin mode
		Dryrun:   true,
		Plugins: map[string]string{
			"plugin_header": "true",
			"plugin_stars":  "true",
		},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith per-plugin: %v", err)
	}

	// Expect <outDir>/header.svg and <outDir>/stars.svg
	for _, slug := range []string{"header", "stars"} {
		path := filepath.Join(outDir, slug+".svg")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected per-plugin file %s; err=%v", path, err)
		}
	}
}

// TestRunCLIWith_PerPlugin_WithAllowlist verifies that --plugins restricts
// output to only the listed plugins.
func TestRunCLIWith_PerPlugin_WithAllowlist(t *testing.T) {
	outDir := t.TempDir()

	rest := newFakeREST()
	cf := &CLIFlags{
		User:            "octocat",
		Template:        "classic",
		Token:           "ghp_mock_pat_valid",
		Output:          "svg",
		Filename:        "",
		Dryrun:          true,
		PluginAllowlist: "header", // only header, even though stars is enabled
		Plugins: map[string]string{
			"plugin_header": "true",
			"plugin_stars":  "true",
		},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith per-plugin with allowlist: %v", err)
	}

	// Only header.svg should exist.
	if _, err := os.Stat(filepath.Join(outDir, "header.svg")); err != nil {
		t.Errorf("expected header.svg; err=%v", err)
	}
	// stars.svg must NOT exist.
	if _, err := os.Stat(filepath.Join(outDir, "stars.svg")); err == nil {
		t.Error("stars.svg should not exist (filtered by allowlist)")
	}
}

// TestRunCLIWith_Combined_ExplicitFlag verifies that --combined produces
// a single combined SVG file.
func TestRunCLIWith_Combined_ExplicitFlag(t *testing.T) {
	outDir := t.TempDir()

	rest := newFakeREST()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "metrics.svg",
		Combined: true,
		Dryrun:   true,
		Plugins:  map[string]string{},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith combined: %v", err)
	}

	// Combined mode → single file metrics.svg
	path := filepath.Join(outDir, "metrics.svg")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected combined file %s; err=%v", path, err)
	}
}

// TestRunCLIWith_FilenameImpliesCombined is the REGRESSION test for #616.
// When --filename foo.svg is explicitly set, the result must be a single
// combined SVG at <outDir>/foo.svg, NOT per-plugin fan-out.
func TestRunCLIWith_FilenameImpliesCombined(t *testing.T) {
	outDir := t.TempDir()

	rest := newFakeREST()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "foo.svg", // explicit filename → combined mode
		Dryrun:   true,
		Plugins:  map[string]string{},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith filename regression: %v", err)
	}

	// Single file at outDir/foo.svg.
	path := filepath.Join(outDir, "foo.svg")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected combined file at %s; err=%v", path, err)
	}
}
