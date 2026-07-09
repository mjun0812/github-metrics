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
	inv, err := newInvocation(inputs, env, "/tmp/out")
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
	inv, err := newInvocation(inputs, env, "/tmp/out")
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
		// combined mode opt-in: the per-plugin default forbids the commit
		// committer; this test asserts filename resolution only.
		"combined": "yes",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.OutputFilename != "github-metrics.svg" {
		t.Errorf("OutputFilename = %q, want %q", inv.OutputFilename, "github-metrics.svg")
	}
}

// TestNewInvocation_ConfigOutputAuto_ResolvesToSVG pins the resolution
// of the action.yml default `config_output: auto` (forwarded verbatim as
// INPUT_CONFIG_OUTPUT=auto by the Actions runner): it must resolve to
// the template default "svg" so filename wildcards and
// template.CheckFormat don't see the literal "auto".
func TestNewInvocation_ConfigOutputAuto_ResolvesToSVG(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":          "octocat",
		"config_output": "auto",
		"combined":      "yes",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.Format != "svg" {
		t.Errorf("Format = %q, want %q", inv.Format, "svg")
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
	inputs := map[string]any{"combined": "yes"} // user is absent; combined opt-in to skip per-plugin fail-fast
	env := map[string]string{
		"GITHUB_ACTOR":      "octocat",
		"GITHUB_REPOSITORY": "octocat/test-repo",
	}
	inv, err := newInvocation(inputs, env, "/tmp/out")
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
	_, err := newInvocation(inputs, env, "/tmp")
	if err == nil {
		t.Error("expected error when user and GITHUB_ACTOR are both empty")
	}
}

// ---------------------------------------------------------------------------
// newInvocation: GITHUB_REPOSITORY parsing
// ---------------------------------------------------------------------------

func TestNewInvocation_GitHubRepositoryParsed(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "mjun0812", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
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

// After the v3.0 mode unification (#646), GITHUB_REPOSITORY is parsed
// in every invocation — there is no longer a CLI-mode skip. The env
// var still no-ops when absent (local CLI without the runner sets it),
// but when present it populates RepoOwner / RepoName regardless of
// whether the invocation came from the GitHub Actions runner or the
// shell. This test pins that always-on behaviour.
func TestNewInvocation_GitHubRepository_AlwaysParsed(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test-repo"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.RepoOwner != "mjun0812" || inv.RepoName != "test-repo" {
		t.Errorf("RepoOwner/RepoName = %q/%q, want mjun0812/test-repo",
			inv.RepoOwner, inv.RepoName)
	}
}

// ---------------------------------------------------------------------------
// newInvocation: optimize default injection
// ---------------------------------------------------------------------------

func TestNewInvocation_OptimizeAbsent_InjectsDefault(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
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
	inputs := map[string]any{"user": "octocat", "optimize": "css", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.Inputs["optimize"] != "css" {
		t.Errorf("explicit optimize overwritten; got %v", inv.Inputs["optimize"])
	}
}

// ---------------------------------------------------------------------------
// newInvocation: token resolution chain (#647)
//
// After the v3.0 removal of --token / --token-env, the binary resolves the
// GitHub PAT through a single deterministic chain:
//
//	inputs["token"] (= INPUT_TOKEN via ParseInputs) > env["GITHUB_TOKEN"]
//	                                                > empty (delegated to
//	                                                  TokenValidator stage 1)
// ---------------------------------------------------------------------------

func TestNewInvocation_Token_InputTokenWinsOverGitHubToken(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":     "octocat",
		"token":    "input_token_value",
		"dryrun":   true,
		"combined": "yes",
	}
	env := map[string]string{"GITHUB_TOKEN": "github_token_value"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if got := inv.Token.Reveal(); got != "input_token_value" {
		t.Errorf("token = %q, want %q (INPUT_TOKEN must beat GITHUB_TOKEN)",
			got, "input_token_value")
	}
}

func TestNewInvocation_Token_GitHubTokenFallback(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":     "octocat",
		"dryrun":   true,
		"combined": "yes",
	} // no inputs["token"]
	env := map[string]string{"GITHUB_TOKEN": "github_token_value"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if got := inv.Token.Reveal(); got != "github_token_value" {
		t.Errorf("token = %q, want fallback to GITHUB_TOKEN", got)
	}
	// The fallback also seeds inputs["token"] so downstream readers
	// (banner / validators) see the same resolved value.
	if got := inv.Inputs["token"]; got != "github_token_value" {
		t.Errorf("inputs[\"token\"] = %v, want fallback value", got)
	}
}

func TestNewInvocation_Token_InputTokenAloneActionPath(t *testing.T) {
	t.Parallel()
	// Action-mode happy path: GitHub Actions runner sets INPUT_TOKEN
	// from `with: token:`; GITHUB_TOKEN is absent. ParseInputs would
	// already have copied INPUT_TOKEN into inputs["token"].
	inputs := map[string]any{
		"user":     "octocat",
		"token":    "input_token_only",
		"combined": "yes",
	}
	env := map[string]string{"GITHUB_REPOSITORY": "octocat/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if got := inv.Token.Reveal(); got != "input_token_only" {
		t.Errorf("token = %q, want %q", got, "input_token_only")
	}
}

func TestNewInvocation_Token_NeitherSet_DelegatesToValidator(t *testing.T) {
	t.Parallel()
	// newInvocation does NOT itself error on a missing token — the
	// TokenValidator stage 1 surfaces that diagnostic so the
	// use_mocked_data / MOCKED_TOKEN paths can still bypass it.
	inputs := map[string]any{
		"user":     "octocat",
		"dryrun":   true,
		"combined": "yes",
	}
	env := map[string]string{}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if got := inv.Token.Reveal(); got != "" {
		t.Errorf("token = %q, want empty string (delegated to validator)", got)
	}
}

// ---------------------------------------------------------------------------
// newInvocation: RetryPolicy defaults
// ---------------------------------------------------------------------------

func TestNewInvocation_RetryPolicyDefaults(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
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

// TestNewInvocation_RetriesDelayInSeconds pins the action.yml contract:
// `retries_delay` is declared in seconds ("Delay between each retry (in
// seconds)"), so retries_delay=10 must yield a 10-second delay — not
// 10 milliseconds as the pre-fix implementation consumed it.
func TestNewInvocation_RetriesDelayInSeconds(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes", "retries_delay": 10}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.RetryPolicy.Delay != 10*time.Second {
		t.Errorf("Delay = %v, want 10s", inv.RetryPolicy.Delay)
	}
}
