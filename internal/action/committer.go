package action

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// Committer drives the output_action dispatch (commit / pull-request
// variants). The struct is populated by NewCommitter from an
// *Invocation; Run executes the configured action.
//
// Phase 3 ships only the `commit` path. The `pull-request*` variants
// land in Phase 4 (T029-T031); for now the Committer rejects them
// with a clear error so the integration boundary is explicit.
type Committer struct {
	REST   *githubapi.REST
	Policy RetryPolicy

	RepoOwner string
	RepoName  string
	Branch    string
	Filename  string
	Message   string
	Author    CommitterAuthor
	Action    string // "none", "commit", "pull-request[-merge|-squash|-rebase]"

	Body []byte

	// Result fields populated by Run.
	MetricsURL string // https://github.com/<owner>/<name>/blob/<branch>/<filename>
	Skipped    bool   // true when output_condition=data-changed and content matched
}

// CommitterAuthor carries the commit author identity used by the
// Contents API call.
type CommitterAuthor struct {
	Name  string
	Email string
}

// NewCommitter constructs a Committer from an *Invocation + raw body.
// Returns nil + error when the invocation's RepoOwner/RepoName is
// missing for a path that requires GitHub API access (commit /
// pull-request). The caller is expected to fail fast on such errors.
func NewCommitter(rest *githubapi.REST, inv *Invocation, body []byte) (*Committer, error) {
	if inv == nil {
		return nil, errors.New("NewCommitter: nil invocation")
	}
	c := &Committer{
		REST:      rest,
		Policy:    inv.RetryPolicy,
		RepoOwner: inv.RepoOwner,
		RepoName:  inv.RepoName,
		Branch:    inv.Branch,
		Filename:  inv.OutputFilename,
		Message:   inv.CommitterMessage,
		Author:    CommitterAuthor{Name: inv.CommitterAuthor, Email: inv.CommitterEmail},
		Action:    inv.OutputAction,
		Body:      body,
	}
	if c.needsRepoIdentity() {
		if c.RepoOwner == "" || c.RepoName == "" {
			return nil, errors.New("NewCommitter: repo owner/name missing (set GITHUB_REPOSITORY in Action mode)")
		}
		if rest == nil {
			return nil, errors.New("NewCommitter: nil REST client")
		}
	}
	return c, nil
}

func (c *Committer) needsRepoIdentity() bool {
	return c.Action != "" && c.Action != "none"
}

// Run dispatches to the per-action sequence.
func (c *Committer) Run(ctx context.Context) error {
	switch c.Action {
	case "", "none":
		return nil
	case "commit":
		return c.runCommit(ctx)
	case "pull-request", "pull-request-merge", "pull-request-squash", "pull-request-rebase":
		// T029-T031 land in Phase 4. Surface a clear error so US1
		// (P1 MVP) users who ask for PR mode early see the gap.
		return fmt.Errorf("committer: output_action=%q is implemented in M6 Phase 4 (T029-T031); use 'commit' for the MVP", c.Action)
	default:
		// Defensive: should not happen — output_action.go's Validate
		// catches unsupported values before NewCommitter is called.
		return fmt.Errorf("committer: unrecognized output_action %q", c.Action)
	}
}

// runCommit implements contracts/committer.md §2 commit path:
// ensureBranch → (optional data-changed check) → previous-sha lookup →
// PUT /contents. All API calls are wrapped by RetryPolicy.
func (c *Committer) runCommit(ctx context.Context) error {
	// Step 1: ensure target branch exists (create from default if missing).
	if err := c.Policy.Do(ctx, func() error { return c.ensureBranch(ctx) }); err != nil {
		return fmt.Errorf("committer: ensureBranch: %w", err)
	}

	// Step 2: previous-sha lookup (404 = new file; other 4xx/5xx propagate).
	prevSHA, prevErr := c.fetchPreviousSHA(ctx)
	if prevErr != nil {
		return fmt.Errorf("committer: fetch previous file: %w", prevErr)
	}

	// Step 3: commit.
	if err := c.Policy.Do(ctx, func() error { return c.putContents(ctx, prevSHA) }); err != nil {
		return fmt.Errorf("committer: putContents: %w", err)
	}

	c.MetricsURL = fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s",
		c.RepoOwner, c.RepoName, c.Branch, c.Filename)
	return nil
}

// ensureBranch verifies the target branch ref exists; creates it from
// the default branch when missing.
func (c *Committer) ensureBranch(ctx context.Context) error {
	if c.Branch == "" {
		// Empty branch means "use default" — nothing to ensure.
		return nil
	}
	path := fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", c.RepoOwner, c.RepoName, c.Branch)
	_, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return wrapRetryable(err, resp)
	}
	if resp != nil && resp.StatusCode == http.StatusOK {
		return nil // branch exists
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		// Branch missing — would need PUT /git/refs to create. For
		// the MVP commit path we expect the branch to already exist
		// (default branch). When it doesn't, surface a clear error.
		return fmt.Errorf("branch %q does not exist on %s/%s and auto-creation is not yet implemented in this phase",
			c.Branch, c.RepoOwner, c.RepoName)
	}
	return fmt.Errorf("unexpected status %d looking up branch %q", statusOf(resp), c.Branch)
}

// fetchPreviousSHA looks up the existing file's blob sha so PUT
// /contents can do conflict detection. Returns "" + nil when the file
// does not exist (404).
func (c *Committer) fetchPreviousSHA(ctx context.Context) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", c.RepoOwner, c.RepoName, c.Filename)
	if c.Branch != "" {
		path += "?ref=" + c.Branch
	}
	body, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return "", wrapRetryable(err, resp)
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return "", nil // new file
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching previous file", statusOf(resp))
	}
	var doc struct {
		SHA string `json:"sha"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return "", fmt.Errorf("decode contents response: %w", jerr)
	}
	return doc.SHA, nil
}

// putContents issues PUT /repos/.../contents/<filename> to create or
// update the rendered file in the target branch.
func (c *Committer) putContents(ctx context.Context, prevSHA string) error {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", c.RepoOwner, c.RepoName, c.Filename)
	payload := map[string]any{
		"message": c.Message,
		"content": base64.StdEncoding.EncodeToString(c.Body),
		"committer": map[string]string{
			"name":  c.Author.Name,
			"email": c.Author.Email,
		},
	}
	if c.Branch != "" {
		payload["branch"] = c.Branch
	}
	if prevSHA != "" {
		payload["sha"] = prevSHA
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.REST.Put(ctx, path, body, nil)
	if err != nil {
		return wrapRetryable(err, resp)
	}
	if resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
		return fmt.Errorf("unexpected status %d committing %s", statusOf(resp), c.Filename)
	}
	return nil
}

// wrapRetryable promotes a transient transport error or 5xx response
// to *xerrors.RetryableError so RetryPolicy can do its job. Non-5xx
// HTTP errors and non-transport-level errors pass through unchanged.
func wrapRetryable(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp != nil && resp.StatusCode >= 500 {
		return xerrors.NewRetryableError(err)
	}
	// Network error without a response — treat as transient.
	if resp == nil {
		return xerrors.NewRetryableError(err)
	}
	return err
}

func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
