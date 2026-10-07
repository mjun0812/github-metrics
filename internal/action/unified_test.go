package action

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// errCaptured aborts the pipeline once captureInvocation has recorded
// the resolved Invocation.
var errCaptured = errors.New("test: invocation captured")

// captureInvocation returns a BuildDeps that records inv into out
// then aborts the pipeline with errCaptured. Tests use it to assert
// the resolved Invocation shape without running the full engine
// compute path.

func captureInvocation(out **Invocation) func(context.Context, *Invocation) (engine.Deps, error) {
	return func(_ context.Context, inv *Invocation) (engine.Deps, error) {
		*out = inv
		return engine.Deps{}, errCaptured
	}
}

// TestUnified_HybridFlagBeatsEnv pins the post-#646 invariant that a
// CLI flag overrides the matching INPUT_<UPPER> env value on conflict.
// Without this guarantee, a workflow that sets `token` via a secret
// (INPUT_TOKEN) and then layers `--debug` / `--user override` via a
// run step would silently lose the env values.
func TestUnified_HybridFlagBeatsEnv(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{"--user", "fromflag", "--combined", "--dryrun"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env: []string{
			"INPUT_USER=fromenv",
			"INPUT_TOKEN=ghp_mock_pat_valid",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith hybrid: %v", err)
	}
	if inv == nil {
		t.Fatal("BuildDeps did not receive Invocation")
	}
	if inv.Login != "fromflag" {
		t.Errorf("Login = %q, want %q (CLI flag must beat INPUT_USER)",
			inv.Login, "fromflag")
	}
}

// TestUnified_EnvOnly verifies the GitHub-Actions-runner driven flow:
// INPUT_<UPPER> alone (no CLI args) populates the invocation.
func TestUnified_EnvOnly(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags(nil)
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env: []string{
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_COMBINED=yes",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith env-only: %v", err)
	}
	if inv.Login != "octocat" {
		t.Errorf("Login = %q, want octocat (from INPUT_USER)", inv.Login)
	}
	if inv.Token.Reveal() != "ghp_mock_pat_valid" {
		t.Errorf("Token = %q, want ghp_mock_pat_valid (from INPUT_TOKEN)", inv.Token.Reveal())
	}
}

// TestUnified_NoEnvSuppressesEnvLayer pins the --no-env opt-out: even
// when INPUT_USER=fromenv is present in the process env, the
// invocation MUST resolve user from the --user flag alone.
func TestUnified_NoEnvSuppressesEnvLayer(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{
		"--no-env",
		"--user", "fromflag",
		"--combined",
		"--dryrun",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !cf.NoEnv {
		t.Fatalf("ParseFlags did not record --no-env; cf.NoEnv=%v", cf.NoEnv)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		// Deliberately set INPUT_USER=fromenv to prove --no-env masks it.
		// GITHUB_TOKEN is consulted by newInvocation's token fallback —
		// NOT by ParseInputs — so it survives --no-env. That's the
		// documented behaviour: --no-env suppresses the INPUT_*/INPUTS
		// layer, not the GITHUB_TOKEN env fallback.
		Env: []string{
			"INPUT_USER=fromenv",
			"GITHUB_TOKEN=ghp_mock_pat_valid",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith --no-env: %v", err)
	}
	if inv.Login != "fromflag" {
		t.Errorf("Login = %q, want fromflag (--no-env must suppress INPUT_USER=fromenv)",
			inv.Login)
	}
	// --no-env suppresses only the INPUT_*/INPUTS layer; the GITHUB_TOKEN
	// fallback read by newInvocation still applies.
	if inv.Token.Reveal() != "ghp_mock_pat_valid" {
		t.Errorf("Token = %q, want fallback from GITHUB_TOKEN", inv.Token.Reveal())
	}
}

// TestUnified_BannerWritesToStderr pins the v3.0 behavior change: the
// startup banner goes to stderr (not stdout), so `--filename -` payloads
// streamed to stdout cannot be corrupted by banner bytes. A future
// refactor that flips the default would silently regress PNG output
// over stdout (and contaminate committed SVG diffs piped from stdout).
// PR #651 cap-1 review SHOULD-FIX #3.
func TestUnified_BannerWritesToStderr(t *testing.T) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cf, err := ParseFlags([]string{"--combined", "--dryrun"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env: []string{
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
		},
		Stdout:    &stdoutBuf,
		Stderr:    &stderrBuf,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith: %v", err)
	}
	// Banner must land on stderr (not stdout) so payload streams stay
	// clean for `--filename -` consumers.
	if !strings.Contains(stderrBuf.String(), "metrics-cli") {
		t.Errorf("banner missing from stderr; got %q", stderrBuf.String())
	}
	if strings.Contains(stdoutBuf.String(), "metrics-cli") ||
		strings.Contains(stdoutBuf.String(), "startup banner") {
		t.Errorf("banner leaked into stdout; got %q", stdoutBuf.String())
	}
}
