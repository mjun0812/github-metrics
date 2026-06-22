package action

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// newInvocation: --filename behavior (regression for #614/#616 revert)
// ---------------------------------------------------------------------------

// TestNewInvocation_FilenameExplicit is the HIGHEST PRIORITY regression test.
// When --filename foo.svg is passed (an explicit non-"-" filename), the resulting
// inv.OutputFilename must be exactly "foo.svg".
// The reverted PR #614 introduced per-plugin mode; its bug was that explicit
// filenames would try to write /foo.svg instead of joining with OutputDir.
// This test pins the current (correct) behavior.
func TestNewInvocation_FilenameExplicit(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":     "octocat",
		"filename": "foo.svg",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(ModeCLI, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.OutputFilename != "foo.svg" {
		t.Errorf("OutputFilename = %q, want %q", inv.OutputFilename, "foo.svg")
	}
}

// TestNewInvocation_FilenameStdout verifies that --filename - yields OutputFilename == "-".
func TestNewInvocation_FilenameStdout(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":     "octocat",
		"filename": "-",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(ModeCLI, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.OutputFilename != "-" {
		t.Errorf("OutputFilename = %q, want %q", inv.OutputFilename, "-")
	}
}

// TestNewInvocation_FilenameWildcard verifies wildcard expansion:
// "github-metrics.*" + format "svg" → "github-metrics.svg".
func TestNewInvocation_FilenameWildcard(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":          "octocat",
		"filename":      "github-metrics.*",
		"config_output": "svg",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(ModeCLI, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.OutputFilename != "github-metrics.svg" {
		t.Errorf("OutputFilename = %q, want %q", inv.OutputFilename, "github-metrics.svg")
	}
}

// ---------------------------------------------------------------------------
// newInvocation: user / GITHUB_ACTOR fallback (Action mode)
// ---------------------------------------------------------------------------

func TestNewInvocation_UserFromGitHubActor(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{} // user is absent
	env := map[string]string{
		"GITHUB_ACTOR":      "octocat",
		"GITHUB_REPOSITORY": "octocat/test-repo",
	}
	inv, err := newInvocation(ModeAction, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.Login != "octocat" {
		t.Errorf("Login = %q, want %q", inv.Login, "octocat")
	}
}

func TestNewInvocation_UserEmpty_Errors(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{}
	env := map[string]string{} // no GITHUB_ACTOR either
	_, err := newInvocation(ModeAction, inputs, env, "/tmp")
	if err == nil {
		t.Error("expected error when user and GITHUB_ACTOR are both empty")
	}
}

// ---------------------------------------------------------------------------
// newInvocation: GITHUB_REPOSITORY parsing
// ---------------------------------------------------------------------------

func TestNewInvocation_GitHubRepositoryParsed(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "mjun0812"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(ModeAction, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.RepoOwner != "mjun0812" {
		t.Errorf("RepoOwner = %q, want %q", inv.RepoOwner, "mjun0812")
	}
	if inv.RepoName != "test-repo" {
		t.Errorf("RepoName = %q, want %q", inv.RepoName, "test-repo")
	}
}

// In CLI mode, GITHUB_REPOSITORY env is NOT parsed (only Action mode reads it).
func TestNewInvocation_GitHubRepository_CLIModeIgnored(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(ModeCLI, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	// In CLI mode, RepoOwner/RepoName come from the `repo` input, not env.
	// With no `repo` input, they stay empty.
	if inv.RepoOwner != "" {
		t.Errorf("CLI mode: RepoOwner should be empty, got %q", inv.RepoOwner)
	}
}

// ---------------------------------------------------------------------------
// newInvocation: optimize default injection
// ---------------------------------------------------------------------------

func TestNewInvocation_OptimizeAbsent_InjectsDefault(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(ModeAction, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	got, ok := inv.Inputs["optimize"]
	if !ok {
		t.Fatal("optimize key missing from Inputs")
	}
	gotSlice, ok := got.([]string)
	if !ok {
		t.Fatalf("optimize type = %T, want []string", got)
	}
	if len(gotSlice) != 2 || gotSlice[0] != "css" || gotSlice[1] != "xml" {
		t.Errorf("optimize = %v, want [css xml]", gotSlice)
	}
}

func TestNewInvocation_OptimizeExplicit_Preserved(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "optimize": "css"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(ModeAction, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.Inputs["optimize"] != "css" {
		t.Errorf("explicit optimize overwritten; got %v", inv.Inputs["optimize"])
	}
}

// ---------------------------------------------------------------------------
// newInvocation: RetryPolicy defaults
// ---------------------------------------------------------------------------

func TestNewInvocation_RetryPolicyDefaults(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(ModeAction, inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.RetryPolicy.Retries != DefaultRetries {
		t.Errorf("Retries = %d, want %d", inv.RetryPolicy.Retries, DefaultRetries)
	}
	if inv.RetryPolicy.Delay != DefaultRetryDelay {
		t.Errorf("Delay = %v, want %v", inv.RetryPolicy.Delay, DefaultRetryDelay)
	}
	_ = time.Millisecond // keep time import referenced
}
