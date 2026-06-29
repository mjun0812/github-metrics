package action

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
)

// TestRun_EntryPointRejectsUnsupportedOutputAction_FromEnv covers the
// unified Run entry point via the env layer (INPUT_<UPPER>) — i.e. the
// flow that the GitHub Actions runner drives.
func TestRun_EntryPointRejectsUnsupportedOutputAction_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "mjun0812/test-repo")
	t.Setenv("GITHUB_ACTOR", "octocat")
	t.Setenv("INPUT_USER", "octocat")
	t.Setenv("INPUT_TOKEN", "ghp_mock_pat_valid")
	t.Setenv("INPUT_OUTPUT_ACTION", "gist")
	t.Setenv("INPUT_DRYRUN", "yes")

	err := Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected unsupported output_action error")
	}
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("err type = %T, want *ConfigError; err=%v", err, err)
	}
}

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

// TestDefaultBuildDeps_MockedDataUsesFakeRendererOnly verifies the hermetic
// mocked-data collaborator setup without making requests.
func TestDefaultBuildDeps_MockedDataUsesFakeRendererOnly(t *testing.T) {
	t.Parallel()
	deps, err := defaultBuildDeps(context.Background(), &Invocation{
		Token:            config.NewToken("MOCKED_TOKEN"),
		UseMockedData:    true,
		GitHubAPIRest:    "http://mock.localhost",
		GitHubAPIGraphQL: "http://mock.localhost/graphql",
	})
	if err != nil {
		t.Fatalf("defaultBuildDeps: %v", err)
	}
	if deps.REST == nil || deps.GraphQL == nil {
		t.Fatalf("REST and GraphQL clients must be constructed")
	}
	if deps.Render == nil {
		t.Fatalf("mocked data should use a fake renderer")
	}
	if deps.HTTPClient != nil {
		t.Fatalf("mocked data should not create an image HTTP client")
	}
}

// TestDefaultBuildDeps_RealDataDefersRendererAndCreatesHTTPClient verifies
// the real-data construction path without issuing requests.
func TestDefaultBuildDeps_RealDataDefersRendererAndCreatesHTTPClient(t *testing.T) {
	t.Parallel()
	deps, err := defaultBuildDeps(context.Background(), &Invocation{
		Token:            config.NewToken("ghp_mock_pat_valid"),
		UseMockedData:    false,
		GitHubAPIRest:    "http://mock.localhost",
		GitHubAPIGraphQL: "http://mock.localhost/graphql",
	})
	if err != nil {
		t.Fatalf("defaultBuildDeps: %v", err)
	}
	if deps.REST == nil || deps.GraphQL == nil {
		t.Fatalf("REST and GraphQL clients must be constructed")
	}
	if deps.Render != nil {
		t.Fatalf("real data should defer renderer allocation to engine dispatch")
	}
	if deps.HTTPClient == nil {
		t.Fatalf("real data should create an image HTTP client")
	}
}
