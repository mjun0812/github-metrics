package main

import (
	"slices"
	"testing"
)

// TestFullFlag_TogglesPluginInputs verifies the --full flag plumbing:
// when full=true, METRICS_FIXTURE_FULL=1 is appended to the upstream
// npm subprocess's environment so the YAML loader can opt into the
// M4 adopted plugin set.
func TestFullFlag_TogglesPluginInputs(t *testing.T) {
	cases := []struct {
		name        string
		full        bool
		wantPresent bool
	}{
		{"flag_off", false, false},
		{"flag_on", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildNpmCommand("/tmp/org_repo", "octocat", tc.full)

			// argv shape stays the same regardless of full.
			wantArgs := []string{"npm", "test", "--silent", "--", "--grep", "octocat"}
			if !slices.Equal(cmd.Args, wantArgs) {
				t.Fatalf("argv mismatch: got %v want %v", cmd.Args, wantArgs)
			}

			// Working directory must be the upstream checkout root.
			if cmd.Dir != "/tmp/org_repo" {
				t.Fatalf("cmd.Dir = %q, want %q", cmd.Dir, "/tmp/org_repo")
			}

			// Env contains METRICS_FIXTURE_FULL=1 iff full was true.
			got := slices.Contains(cmd.Env, "METRICS_FIXTURE_FULL=1")
			if got != tc.wantPresent {
				t.Fatalf("METRICS_FIXTURE_FULL=1 present=%v, want %v (env=%v)", got, tc.wantPresent, cmd.Env)
			}
		})
	}
}
