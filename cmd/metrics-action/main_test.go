package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_NoArgs prints the banner + usage and returns nil. This is
// the legacy M1 behavior preserved through M6 so anyone running
// `metrics-action` with no args gets a friendly help screen.
func TestRun_NoArgs(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut, nil); err != nil {
		t.Fatalf("run(nil): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "metrics-action") {
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

// TestRun_ActionMode_DispatchesEnvDetected confirms the
// GITHUB_ACTIONS=true env triggers the Action dispatch path. Without
// the required `user` input the dispatch surfaces a clear error from
// action.Run — verifying the routing wired up correctly.
func TestRun_ActionMode_DispatchesEnvDetected(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	err := run(nil, &out, &errOut, []string{"GITHUB_ACTIONS=true"})
	if err == nil {
		t.Fatalf("expected error from action.Run (no inputs supplied)")
	}
	// Either the M6 "input required" error or an upstream binding
	// error counts as evidence that action.Run was reached.
	msg := err.Error()
	if !strings.Contains(msg, "action") {
		t.Errorf("expected error from action.Run; got %q", msg)
	}
}

// TestRun_CLIMode_DispatchesWhenArgsProvided confirms a non-bootstrap
// arg routes to action.RunCLI (which returns the T039 sentinel for
// the Phase 3 skeleton).
func TestRun_CLIMode_DispatchesWhenArgsProvided(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	err := run([]string{"--user", "octocat"}, &out, &errOut, nil)
	if err == nil || !strings.Contains(err.Error(), "RunCLI") {
		t.Fatalf("expected action.RunCLI sentinel, got err=%v", err)
	}
}

// TestSplitBootstrapArgs confirms the bootstrap-flag splitter
// separates --help / --version / --debug / --log-format from the rest
// so action.RunCLI's own flag.FlagSet only sees its expected args.
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

// TestEnvValue resolves NAME=value pairs from a slice.
func TestEnvValue(t *testing.T) {
	t.Parallel()
	env := []string{"FOO=bar", "GITHUB_ACTIONS=true", "OTHER=x=y=z"}
	cases := map[string]string{
		"FOO":            "bar",
		"GITHUB_ACTIONS": "true",
		"OTHER":          "x=y=z",
		"MISSING":        "",
	}
	for name, want := range cases {
		if got := envValue(env, name); got != want {
			t.Errorf("envValue(%q) = %q, want %q", name, got, want)
		}
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
