package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_NoArgs prints the banner + usage and returns nil. This is
// the legacy M1 behavior preserved through #646 so anyone running
// `metrics-cli` with no args (and no INPUT_<UPPER> env layer) gets a
// friendly help screen instead of falling into the pipeline.
func TestRun_NoArgs(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut, nil); err != nil {
		t.Fatalf("run(nil): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "metrics-cli") {
		t.Errorf("stdout missing banner; got %q", got)
	}
	if !strings.Contains(got, "Usage") {
		t.Errorf("stdout missing usage text; got %q", got)
	}
}

// TestRun_Help_BootstrapFlag accepts --help / -h and exits cleanly.
func TestRun_Help_BootstrapFlag(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--help", "-h"} {
		var out, errOut bytes.Buffer
		if err := run([]string{flag}, &out, &errOut, nil); err != nil {
			t.Errorf("run(%q): %v", flag, err)
		}
	}
}

// TestRun_Version prints the version literal and returns nil.
func TestRun_Version(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run([]string{"--version"}, &out, &errOut, nil); err != nil {
		t.Fatalf("run --version: %v", err)
	}
	if strings.TrimSpace(out.String()) != version {
		t.Errorf("stdout = %q, want %q", out.String(), version)
	}
}

// TestRun_CLIFlagDispatch confirms that any non-bootstrap CLI arg
// routes into the unified action.Run pipeline. After #646 there is no
// separate Action-mode / CLI-mode dispatch — every invocation reads
// env vars AND CLI flags. We trigger the token-required branch (the
// cheapest deterministic exit) by scrubbing GITHUB_TOKEN/INPUT_TOKEN
// from the test process env.
func TestRun_CLIFlagDispatch(t *testing.T) {
	// t.Setenv is used to scrub real env (cannot t.Parallel()).
	// action.Run reads os.Environ() internally, so we must scrub
	// the host's GITHUB_TOKEN / INPUT_TOKEN to trigger the
	// missing-token branch deterministically.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("INPUT_TOKEN", "")
	var out, errOut bytes.Buffer
	// --combined avoids the per-plugin / output_action incompatibility
	// check so we reach the token validator (the cheapest deterministic
	// exit through the unified surface).
	err := run([]string{"--user", "octocat", "--combined", "--dryrun"}, &out, &errOut, nil)
	if err == nil || !strings.Contains(err.Error(), "token required") {
		t.Fatalf("expected token-required error from action.Run, got err=%v", err)
	}
}

// TestRun_EnvOnlyDispatch confirms that an invocation with no CLI args
// but with INPUT_<UPPER> env vars present routes into the unified
// pipeline (rather than printing the no-arg banner+usage). This is the
// "GitHub Actions runner sets `with:` keys, workflow has no `run:`"
// flow that drives most installations.
func TestRun_EnvOnlyDispatch(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("INPUT_USER", "octocat")
	t.Setenv("INPUT_COMBINED", "yes")
	t.Setenv("INPUT_DRYRUN", "yes")
	var out, errOut bytes.Buffer
	err := run(nil, &out, &errOut, nil)
	if err == nil || !strings.Contains(err.Error(), "token required") {
		t.Fatalf("expected token-required error from env-only dispatch, got err=%v", err)
	}
}

// TestRun_GitHubActionsEnvIgnored confirms that GITHUB_ACTIONS=true
// alone (no INPUT_*) is NOT enough to skip the no-arg banner+usage
// short-circuit. The pre-v3.0 binary dispatched purely on that env
// var; after #646 the binary ignores GITHUB_ACTIONS and looks for
// actual inputs (INPUT_*/INPUTS or CLI flags).
func TestRun_GitHubActionsEnvIgnored(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	// Scrub INPUT_* / GITHUB_TOKEN so only the marker is set.
	t.Setenv("INPUT_USER", "")
	t.Setenv("INPUT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut, nil); err != nil {
		t.Fatalf("run(nil) with GITHUB_ACTIONS=true alone: %v", err)
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Errorf("expected banner+usage when only GITHUB_ACTIONS is set; got %q", out.String())
	}
}

// TestSplitBootstrapArgs confirms the bootstrap-flag splitter
// separates --help / --version / --debug / --log-format from the rest
// so action.Run's own flag.FlagSet only sees its expected args.
func TestSplitBootstrapArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantCLI  []string
		wantBoot []string
	}{
		{"empty", nil, nil, nil},
		{"only_bootstrap", []string{"--debug", "--version"}, nil, []string{"--debug", "--version"}},
		{"only_cli", []string{"--user", "octocat"}, []string{"--user", "octocat"}, nil},
		{"mixed", []string{"--user", "octocat", "--debug"}, []string{"--user", "octocat"}, []string{"--debug"}},
		{"log_format_equal", []string{"--log-format=text", "--user", "x"}, []string{"--user", "x"}, []string{"--log-format=text"}},
		{"log_format_space", []string{"--log-format", "text", "--user", "x"}, []string{"--user", "x"}, []string{"--log-format", "text"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli, boot := splitBootstrapArgs(tc.args)
			if !equalArgs(cli, tc.wantCLI) {
				t.Errorf("cli = %v, want %v", cli, tc.wantCLI)
			}
			if !equalArgs(boot, tc.wantBoot) {
				t.Errorf("boot = %v, want %v", boot, tc.wantBoot)
			}
		})
	}
}

// TestHasActionInputs verifies the INPUT_<UPPER> / INPUTS detector
// that decides whether to short-circuit to the banner or hand off to
// the unified pipeline when no CLI args are present.
func TestHasActionInputs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  []string
		want bool
	}{
		{"empty", nil, false},
		{"unrelated_only", []string{"PATH=/usr/bin", "HOME=/root"}, false},
		{"github_actions_marker_alone", []string{"GITHUB_ACTIONS=true"}, false},
		{"input_user", []string{"INPUT_USER=octocat"}, true},
		{"input_token", []string{"PATH=/usr/bin", "INPUT_TOKEN=ghp_x"}, true},
		{"inputs_json", []string{"INPUTS={\"user\":\"x\"}"}, true},
		// Runner-emitted INPUT_FOO= entries for unset workflow inputs
		// MUST NOT trigger the dispatch — otherwise an empty `with:`
		// block would pre-empt the no-arg banner+usage short-circuit.
		{"empty_input_value", []string{"INPUT_USER=", "PATH=/usr/bin"}, false},
		{"empty_inputs_json", []string{"INPUTS="}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := hasActionInputs(tc.env); got != tc.want {
				t.Errorf("hasActionInputs(%v) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
