package action

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRun_EntryPointRejectsUnsupportedOutputAction_FromFlags covers the
// unified Run entry point via the CLI flag layer (no INPUT_<UPPER>).
// Both layers feed the same fail-fast path so the assertion is the same.
func TestRun_EntryPointRejectsUnsupportedOutputAction_FromFlags(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_mock_pat_valid")
	out := filepath.Join(t.TempDir(), "github-metrics.svg")
	err := Run(context.Background(), []string{
		"--user", "octocat",
		"--filename", out,
		"--plugin", "output_action=gist",
	})
	if err == nil {
		t.Fatal("expected unsupported output_action error")
	}
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("err type = %T, want *ConfigError; err=%v", err, err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output file should not be created; stat err=%v", statErr)
	}
}
