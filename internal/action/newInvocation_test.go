package action

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// newInvocation: --filename behavior (regression for #614/#616 revert)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// newInvocation: optimize default injection
// ---------------------------------------------------------------------------

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
// newInvocation: committer_message placeholder expansion (regression for #744)
// ---------------------------------------------------------------------------

// TestNewInvocation_CommitterMessagePlaceholders pins the upstream-parity
// expansion: ${filename} resolves to the rendered output filename and ${run}
// to GITHUB_RUN_ID. Before the fix these landed as literal text in commits.
func TestNewInvocation_CommitterMessagePlaceholders(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":              "octocat",
		"combined":          "yes",
		"filename":          "card.svg",
		"committer_message": "Update ${filename} for run #${run}",
	}
	env := map[string]string{
		"GITHUB_REPOSITORY": "mjun0812/test",
		"GITHUB_RUN_ID":     "987654",
	}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	want := "Update card.svg for run #987654"
	if inv.CommitterMessage != want {
		t.Errorf("CommitterMessage = %q, want %q", inv.CommitterMessage, want)
	}
}

// TestNewInvocation_CommitterMessageDefaultRunPlaceholder covers the CLI code
// default ("Auto-generated metrics for run #${run}"): ${run} must expand even
// when the message input is not supplied.
func TestNewInvocation_CommitterMessageDefaultRunPlaceholder(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes"}
	env := map[string]string{
		"GITHUB_REPOSITORY": "mjun0812/test",
		"GITHUB_RUN_ID":     "42",
	}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	want := "Auto-generated metrics for run #42"
	if inv.CommitterMessage != want {
		t.Errorf("CommitterMessage = %q, want %q", inv.CommitterMessage, want)
	}
}

// ---------------------------------------------------------------------------
// newInvocation: RetryPolicy defaults
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// newInvocation: OutputRetryPolicy (regression for #746)
// ---------------------------------------------------------------------------

// TestNewInvocation_OutputRetryInputsWired pins that the dedicated
// output-action retry inputs are honored (and read in seconds), independent of
// the rendering retries. Before the fix these inputs were silently ignored.
func TestNewInvocation_OutputRetryInputsWired(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{
		"user":                        "octocat",
		"combined":                    "yes",
		"retries":                     3,
		"retries_delay":               300,
		"retries_output_action":       7,
		"retries_delay_output_action": 45,
	}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatalf("newInvocation: %v", err)
	}
	if inv.OutputRetryPolicy.Retries != 7 {
		t.Errorf("OutputRetryPolicy.Retries = %d, want 7", inv.OutputRetryPolicy.Retries)
	}
	if inv.OutputRetryPolicy.Delay != 45*time.Second {
		t.Errorf("OutputRetryPolicy.Delay = %v, want 45s", inv.OutputRetryPolicy.Delay)
	}
	// The rendering policy must remain independent of the output-action inputs.
	if inv.RetryPolicy.Retries != 3 {
		t.Errorf("RetryPolicy.Retries = %d, want 3 (unaffected)", inv.RetryPolicy.Retries)
	}
}

// TestNewCommitter_UsesOutputRetryPolicy pins that the committer consumes the
// output-action retry policy, not the rendering one.
func TestNewCommitter_UsesOutputRetryPolicy(t *testing.T) {
	t.Parallel()
	inv := &Invocation{
		OutputAction:      "none",
		RetryPolicy:       RetryPolicy{Retries: 3, Delay: 300 * time.Second},
		OutputRetryPolicy: RetryPolicy{Retries: 5, Delay: 120 * time.Second},
	}
	c, err := NewCommitter(nil, inv, []byte("x"))
	if err != nil {
		t.Fatalf("NewCommitter: %v", err)
	}
	if c.Policy != inv.OutputRetryPolicy {
		t.Errorf("Committer.Policy = %+v, want OutputRetryPolicy %+v", c.Policy, inv.OutputRetryPolicy)
	}
}
