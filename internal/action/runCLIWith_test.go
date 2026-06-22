package action

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunCLIWith_HappyPath_Dryrun exercises the full runCLIWith pipeline
// with injected fake deps (no network, no chromium). With dryrun=true the
// output file is written to the temp OutputDir but the committer is never
// called (no PUT /contents).
func TestRunCLIWith_HappyPath_Dryrun(t *testing.T) {
	// t.Setenv is used, so no t.Parallel().
	outDir := t.TempDir()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	rest := newFakeREST()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "github-metrics.svg",
		Dryrun:   true,
		Plugins:  map[string]string{},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    nil, // CLI mode uses os.Stderr for banner; stdout unused here
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith: %v", err)
	}

	// Output file should be written at targetOutputPath(inv) = outDir/github-metrics.svg
	outPath := filepath.Join(outDir, "github-metrics.svg")
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file at %s; err=%v", outPath, err)
	}

	// Committer MUST NOT run under dryrun.
	if len(rest.putBodies) > 0 {
		t.Errorf("committer ran under dryrun; PUT captures = %v", rest.putBodies)
	}
}

// TestRunCLIWith_FilenameStdout exercises the "--filename -" path through
// runCLIWith. When OutputFilename resolves to "-" the output writer is
// os.Stdout rather than a file — confirmed by asserting no regular file is
// created in OutputDir.
func TestRunCLIWith_FilenameStdout(t *testing.T) {
	// Uses t.Setenv indirectly via rest — run sequentially.
	outDir := t.TempDir()

	rest := newFakeREST()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "-",
		Dryrun:   true,
		Plugins:  map[string]string{},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		Stdout:    nil,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runCLIWith: %v", err)
	}

	// No file should have been created in outDir (stdout was the target).
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".svg") {
			t.Errorf("unexpected SVG file in outDir: %s", e.Name())
		}
	}
}

// TestRunCLIWith_OutputAction_UnsupportedFailFast verifies that an
// unsupported output_action (e.g., "gist") causes an early ConfigError
// without contacting the GitHub API (no PUT bodies).
func TestRunCLIWith_OutputAction_UnsupportedFailFast(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	rest := newFakeREST()
	cf := &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Dryrun:   true,
		Plugins:  map[string]string{"output_action": "gist"},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err == nil {
		t.Fatal("expected error for output_action=gist")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' in error; got %v", err)
	}
	if len(rest.putBodies) > 0 {
		t.Errorf("PUT seen despite fail-fast: %v", rest.putBodies)
	}
}

// TestRunCLIWith_MissingUser_Errors ensures that a missing --user flag (and
// no GITHUB_ACTOR fallback in CLI mode) causes newInvocation to return an error.
func TestRunCLIWith_MissingUser_Errors(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	rest := newFakeREST()
	cf := &CLIFlags{
		// User deliberately omitted.
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Dryrun:   true,
		Plugins:  map[string]string{},
	}

	err := runCLIWith(context.Background(), cf, runOptions{
		Mode:      ModeCLI,
		Env:       []string{},
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err == nil {
		t.Fatal("expected error when --user is missing in CLI mode")
	}
}
