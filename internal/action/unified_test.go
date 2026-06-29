package action

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// captureInvocation returns a BuildDeps that records inv into out
// then aborts the pipeline with errCaptured. Tests use it to assert
// the resolved Invocation shape without running the full engine
// compute path.
var errCaptured = errors.New("test: invocation captured")

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

// TestUnified_FlagsOnly verifies the local CLI flow where the user
// supplies every input through flags (no INPUT_<UPPER> set).
func TestUnified_FlagsOnly(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{
		"--user", "octocat",
		"--combined",
		"--dryrun",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env:       []string{"GITHUB_TOKEN=ghp_mock_pat_valid"},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith flags-only: %v", err)
	}
	if inv.Login != "octocat" {
		t.Errorf("Login = %q, want octocat (from --user)", inv.Login)
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
}

// TestUnified_NoEnvKeepsGitHubTokenFallback pins the boundary: --no-env
// suppresses the INPUT_*/INPUTS layer (the action.yml input vocabulary)
// but NOT the GITHUB_TOKEN env fallback that newInvocation reads when
// inputs["token"] is empty. This is intentional — local debug runs
// often rely on `gh auth token` populating GITHUB_TOKEN, and refusing
// to read it would force every user to pass --plugin token=... too.
func TestUnified_NoEnvKeepsGitHubTokenFallback(t *testing.T) {
	t.Parallel()
	cf, err := ParseFlags([]string{
		"--no-env",
		"--user", "octocat",
		"--combined",
		"--dryrun",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env:       []string{"GITHUB_TOKEN=ghp_mock_pat_valid"},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith --no-env + GITHUB_TOKEN: %v", err)
	}
	if inv.Token.Reveal() != "ghp_mock_pat_valid" {
		t.Errorf("Token = %q, want fallback from GITHUB_TOKEN",
			inv.Token.Reveal())
	}
}

// TestUnified_SourceAttributionLog verifies that assembleInputs emits
// a single slog.Debug record whose attrs name the originating layer of
// each resolved input key. Mocks the slog default handler so the test
// can inspect every emitted record.
func TestUnified_SourceAttributionLog(t *testing.T) {
	var captured []slog.Record
	cap := &recordCapturer{
		Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
		records: &captured,
	}
	prev := slog.Default()
	slog.SetDefault(slog.New(cap))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cf, err := ParseFlags([]string{
		"--user", "fromflag",
		"--combined",
		"--dryrun",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env: []string{
			"INPUT_USER=fromenv",     // overridden by --user
			"INPUT_TEMPLATE=classic", // env-only
			"INPUT_TOKEN=ghp_mock_pat_valid",
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith: %v", err)
	}

	// Find the "inputs resolved" debug record. Build a key→source map
	// from its attrs and assert the per-key attribution.
	var resolved slog.Record
	var found bool
	for _, r := range captured {
		if r.Message == "inputs resolved" {
			resolved = r
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an 'inputs resolved' debug record; got %d records", len(captured))
	}
	sources := map[string]string{}
	resolved.Attrs(func(a slog.Attr) bool {
		sources[a.Key] = a.Value.String()
		return true
	})
	want := map[string]string{
		"user":     "flag", // CLI flag overrode INPUT_USER
		"template": "env",  // env layer only
		"token":    "env",  // INPUT_TOKEN — env layer
		"dryrun":   "flag", // CLI flag
		"combined": "flag", // CLI flag
	}
	for k, w := range want {
		if got := sources[k]; got != w {
			t.Errorf("source[%q] = %q, want %q (full map: %v)", k, got, w, sources)
		}
	}
}

// TestUnified_InputsJSONSourceLabel verifies the INPUTS JSON layer
// receives its dedicated `inputs_json` source label so debug logs
// distinguish it from individual INPUT_<UPPER> entries.
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

func TestUnified_InputsJSONSourceLabel(t *testing.T) {
	var captured []slog.Record
	cap := &recordCapturer{
		Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
		records: &captured,
	}
	prev := slog.Default()
	slog.SetDefault(slog.New(cap))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cf, err := ParseFlags([]string{"--combined", "--dryrun"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var inv *Invocation
	err = runCLIWith(context.Background(), cf, runOptions{
		Env: []string{
			`INPUTS={"user":"fromjson","token":"ghp_mock_pat_valid"}`,
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: captureInvocation(&inv),
	})
	if !errors.Is(err, errCaptured) {
		t.Fatalf("runCLIWith INPUTS json: %v", err)
	}
	for _, r := range captured {
		if r.Message != "inputs resolved" {
			continue
		}
		var userSrc string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "user" {
				userSrc = a.Value.String()
				return false
			}
			return true
		})
		if userSrc != "inputs_json" {
			t.Errorf("user source = %q, want %q", userSrc, "inputs_json")
		}
		return
	}
	t.Fatal("no 'inputs resolved' debug record captured")
}
