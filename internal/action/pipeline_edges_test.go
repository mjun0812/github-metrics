package action

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// TestRunWith_ParseInputsError covers malformed INPUTS JSON in Action mode.
func TestRunWith_ParseInputsError(t *testing.T) {
	t.Parallel()
	err := runWith(context.Background(), runOptions{
		Mode:      ModeAction,
		Env:       []string{"INPUTS=not-json"},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "parse inputs") {
		t.Fatalf("expected parse inputs error, got %v", err)
	}
}

// TestRunWith_PresetOverlayError covers Action preset load failures.
func TestRunWith_PresetOverlayError(t *testing.T) {
	t.Parallel()
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_CONFIG_PRESETS=/nonexistent/preset.yaml",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "load preset") {
		t.Fatalf("expected load preset error, got %v", err)
	}
}

// TestRunWith_NewInvocationError covers required login validation.
func TestRunWith_NewInvocationError(t *testing.T) {
	t.Parallel()
	err := runWith(context.Background(), runOptions{
		Mode:      ModeAction,
		Env:       []string{"INPUT_TOKEN=ghp_mock_pat_valid"},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("expected user input error, got %v", err)
	}
}

// TestRunWith_CommitterSuccess exercises the non-dryrun commit path and
// metrics_url output using only mocked REST/GraphQL deps.
func TestRunWith_CommitterSuccess(t *testing.T) {
	outDir := t.TempDir()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	rest := newFakeREST()
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TEMPLATE=classic",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_OUTPUT_ACTION=commit",
			"INPUT_DRYRUN=no",
		},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runWith: %v", err)
	}
	if len(rest.putBodies) != 1 {
		t.Fatalf("PUT captures = %d, want 1", len(rest.putBodies))
	}
	outBody, err := os.ReadFile(filepath.Join(outDir, "github_output"))
	if err != nil {
		t.Fatalf("ReadFile github_output: %v", err)
	}
	if !strings.Contains(string(outBody), "metrics_url=") {
		t.Fatalf("metrics_url output missing: %q", outBody)
	}
}

// TestRunWith_ComputeError wraps engine failures from invalid deps.
func TestRunWith_ComputeError(t *testing.T) {
	t.Parallel()
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_USE_MOCKED_DATA=yes",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: func(context.Context, *Invocation) (engine.Deps, error) {
			return engine.Deps{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "engine.Compute") {
		t.Fatalf("expected engine.Compute error, got %v", err)
	}
}

// TestRunWith_WriteOutputError covers output write failures after compute.
func TestRunWith_WriteOutputError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: blocker,
		BuildDeps: buildTestDeps(t, newFakeREST()),
	})
	if err == nil || !strings.Contains(err.Error(), "write output") {
		t.Fatalf("expected write output error, got %v", err)
	}
}

// TestRunWith_BuildDepsError verifies dependency construction failures are
// wrapped before token validation or compute.
func TestRunWith_BuildDepsError(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: func(context.Context, *Invocation) (engine.Deps, error) {
			return engine.Deps{}, errBoom
		},
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
}

// TestRunWith_QuotaInsufficientSkips exercises the Action-mode quota skip
// branch before banner, compute, or output writes.
func TestRunWith_QuotaInsufficientSkips(t *testing.T) {
	t.Parallel()
	rest := newFakeREST()
	rest.rateBody = `{"resources":{"core":{"remaining":0,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`
	err := runWith(context.Background(), runOptions{
		Mode: ModeAction,
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runWith: %v", err)
	}
	if len(rest.putBodies) != 0 {
		t.Fatalf("PUT should not run on quota skip: %v", rest.putBodies)
	}
}

// TestRunCLIWith_PresetOverlayError covers CLI preset load failures.
func TestRunCLIWith_PresetOverlayError(t *testing.T) {
	t.Parallel()
	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "github-metrics.svg",
		Dryrun:   true,
		Preset:   "/nonexistent/preset.yaml",
		Plugins:  map[string]string{},
	}, runOptions{OutputDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "load preset") {
		t.Fatalf("expected load preset error, got %v", err)
	}
}

// TestRunCLIWith_PresetOverlayAndTokenBypass covers successful preset merge
// plus the dryrun/use_mocked_data token bypass branch.
func TestRunCLIWith_PresetOverlayAndTokenBypass(t *testing.T) {
	t.Parallel()
	preset := filepath.Join(t.TempDir(), "preset.yaml")
	if err := os.WriteFile(preset, []byte("q:\n  plugin_languages: true\n"), 0o600); err != nil {
		t.Fatalf("write preset: %v", err)
	}
	errBoom := errors.New("boom")
	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Output:   "svg",
		Filename: "github-metrics.svg",
		Dryrun:   true,
		Preset:   preset,
		Plugins:  map[string]string{"use_mocked_data": "yes"},
	}, runOptions{
		OutputDir: t.TempDir(),
		BuildDeps: func(context.Context, *Invocation) (engine.Deps, error) {
			return engine.Deps{}, errBoom
		},
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
}

// TestRunCLIWith_RepositoryValidationError covers repository template
// fail-fast validation in CLI mode.
func TestRunCLIWith_RepositoryValidationError(t *testing.T) {
	t.Parallel()
	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Template: "repository",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "github-metrics.svg",
		Dryrun:   true,
		Plugins:  map[string]string{},
	}, runOptions{OutputDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "requires --repo") {
		t.Fatalf("expected repo validation error, got %v", err)
	}
}

// TestRunCLIWith_BuildDepsError verifies CLI dependency construction error
// wrapping.
func TestRunCLIWith_BuildDepsError(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")
	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: "github-metrics.svg",
		Dryrun:   true,
		Plugins:  map[string]string{},
	}, runOptions{
		OutputDir: t.TempDir(),
		BuildDeps: func(context.Context, *Invocation) (engine.Deps, error) {
			return engine.Deps{}, errBoom
		},
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
}

// TestRunCLIWith_NonDryrunCommitterInitError reaches the CLI committer
// branch. CLI mode has no GITHUB_REPOSITORY owner/name wiring, so commit
// mode fails after output writing when initializing the committer.
func TestRunCLIWith_NonDryrunCommitterInitError(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "github-metrics.svg")
	err := runCLIWith(context.Background(), &CLIFlags{
		User:     "octocat",
		Template: "classic",
		Token:    "ghp_mock_pat_valid",
		Output:   "svg",
		Filename: out,
		Dryrun:   false,
		Plugins:  map[string]string{},
	}, runOptions{
		OutputDir: t.TempDir(),
		BuildDeps: buildTestDeps(t, newFakeREST()),
	})
	if err == nil || !strings.Contains(err.Error(), "committer init") {
		t.Fatalf("expected committer init error, got %v", err)
	}
	if _, statErr := os.Stat(out); statErr != nil {
		t.Fatalf("output should be written before committer init: %v", statErr)
	}
}
