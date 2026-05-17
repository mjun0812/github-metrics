package action

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

	// Condition controls the data-changed gate. "always" (default)
	// commits unconditionally; "data-changed" runs HashComparator
	// and skips when bytes are equivalent. Spec FR-012 / FR-013.
	Condition string

	// RunID is the GitHub Actions run id used to scope the
	// pull-request head branch (`metrics-run-<RunID>`). Required for
	// the pull-request* paths; ignored for commit / none.
	RunID string

	Body []byte

	// Result fields populated by Run.
	MetricsURL string // https://github.com/<owner>/<name>/blob/<branch>/<filename> (or PR URL for pull-request*)
	Skipped    bool   // true when output_condition=data-changed and content matched
	PRNumber   int    // populated for pull-request* paths after PR creation
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
		Condition: inv.OutputCondition,
		RunID:     inv.RunID,
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
	case "pull-request":
		return c.runPullRequest(ctx, "")
	case "pull-request-merge":
		return c.runPullRequest(ctx, "merge")
	case "pull-request-squash":
		return c.runPullRequest(ctx, "squash")
	case "pull-request-rebase":
		return c.runPullRequest(ctx, "rebase")
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

	// Step 2 (optional): data-changed check.
	if c.Condition == "data-changed" {
		cmp := &HashComparator{
			REST:      c.REST,
			RepoOwner: c.RepoOwner,
			RepoName:  c.RepoName,
			Branch:    c.Branch,
			Filename:  c.Filename,
			NewBody:   c.Body,
		}
		var eq bool
		if err := c.Policy.Do(ctx, func() error {
			var inner error
			eq, inner = cmp.Equal(ctx)
			return inner
		}); err != nil {
			return fmt.Errorf("committer: data-changed: %w", err)
		}
		if eq {
			c.Skipped = true
			return nil
		}
	}

	// Step 3: previous-sha lookup (404 = new file; other 4xx/5xx propagate).
	prevSHA, prevErr := c.fetchPreviousSHA(ctx)
	if prevErr != nil {
		return fmt.Errorf("committer: fetch previous file: %w", prevErr)
	}

	// Step 4: commit.
	if err := c.Policy.Do(ctx, func() error { return c.putContents(ctx, c.commitBranch(), prevSHA) }); err != nil {
		return fmt.Errorf("committer: putContents: %w", err)
	}

	c.MetricsURL = fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s",
		c.RepoOwner, c.RepoName, c.commitBranch(), c.Filename)
	return nil
}

// commitBranch returns Branch when set, otherwise an empty string —
// which lets the GitHub API choose the repository's default branch.
// Exists as a method so runPullRequest can override it with the
// run-scoped head branch (`metrics-run-<RunID>`).
func (c *Committer) commitBranch() string {
	return c.Branch
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
// update the rendered file in the target branch. branch may be empty
// to commit to the default branch.
func (c *Committer) putContents(ctx context.Context, branch, prevSHA string) error {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", c.RepoOwner, c.RepoName, c.Filename)
	payload := map[string]any{
		"message": c.Message,
		"content": base64.StdEncoding.EncodeToString(c.Body),
		"committer": map[string]string{
			"name":  c.Author.Name,
			"email": c.Author.Email,
		},
	}
	if branch != "" {
		payload["branch"] = branch
	}
	if prevSHA != "" {
		payload["sha"] = prevSHA
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, resp, err := c.REST.Put(ctx, path, body, nil)
	if err != nil {
		return wrapRetryable(err, resp)
	}
	if resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
		return fmt.Errorf("unexpected status %d committing %s", statusOf(resp), c.Filename)
	}
	return nil
}

// runPullRequest implements contracts/committer.md §3 / §4.
// mergeMethod is "" for plain pull-request (no auto-merge), or one
// of "merge" / "squash" / "rebase" for the auto-merge variants.
func (c *Committer) runPullRequest(ctx context.Context, mergeMethod string) error {
	baseBranch := c.Branch // committer_branch input; empty = default

	if c.RunID == "" {
		return errors.New("committer: pull-request requires GITHUB_RUN_ID")
	}
	headBranch := fmt.Sprintf("metrics-run-%s", c.RunID)

	// 1. Ensure baseBranch exists (we'll fork from it).
	if err := c.Policy.Do(ctx, func() error { return c.ensureBranch(ctx) }); err != nil {
		return fmt.Errorf("committer: ensureBranch base: %w", err)
	}

	// 2. Optional data-changed check vs baseBranch.
	if c.Condition == "data-changed" {
		cmp := &HashComparator{
			REST:      c.REST,
			RepoOwner: c.RepoOwner,
			RepoName:  c.RepoName,
			Branch:    baseBranch,
			Filename:  c.Filename,
			NewBody:   c.Body,
		}
		var eq bool
		if err := c.Policy.Do(ctx, func() error {
			var inner error
			eq, inner = cmp.Equal(ctx)
			return inner
		}); err != nil {
			return fmt.Errorf("committer: data-changed: %w", err)
		}
		if eq {
			c.Skipped = true
			return nil
		}
	}

	// 3. Create the run-scoped head branch from baseBranch.
	if err := c.Policy.Do(ctx, func() error {
		return c.createBranchFromBase(ctx, baseBranch, headBranch)
	}); err != nil {
		return fmt.Errorf("committer: create head branch: %w", err)
	}

	// 4. Commit the file onto headBranch. previous sha lookup uses
	// headBranch (the freshly-created branch carries no file → 404
	// → empty prevSHA).
	prevSHA, prevErr := c.fetchPreviousSHAOnBranch(ctx, headBranch)
	if prevErr != nil {
		return fmt.Errorf("committer: fetch previous on head: %w", prevErr)
	}
	if err := c.Policy.Do(ctx, func() error { return c.putContents(ctx, headBranch, prevSHA) }); err != nil {
		return fmt.Errorf("committer: putContents head: %w", err)
	}

	// 5. POST /pulls.
	prNum, prURL, err := c.createPullRequest(ctx, baseBranch, headBranch)
	if err != nil {
		return fmt.Errorf("committer: create PR: %w", err)
	}
	c.PRNumber = prNum
	c.MetricsURL = prURL

	// 6. Auto-merge variants.
	if mergeMethod == "" {
		return nil
	}
	if err := c.Policy.Do(ctx, func() error { return c.mergePullRequest(ctx, prNum, mergeMethod) }); err != nil {
		return fmt.Errorf("committer: merge PR #%d: %w", prNum, err)
	}
	return nil
}

// createBranchFromBase issues PUT /git/refs/heads/<head> by first
// resolving baseBranch's tip SHA (GET /git/refs/heads/<base>) and
// then POSTing /git/refs. baseBranch="" resolves to the repo's
// default branch via the repository endpoint.
func (c *Committer) createBranchFromBase(ctx context.Context, baseBranch, headBranch string) error {
	resolvedBase, err := c.resolveBaseBranch(ctx, baseBranch)
	if err != nil {
		return err
	}
	baseSHA, err := c.fetchBranchSHA(ctx, resolvedBase)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{
		"ref": "refs/heads/" + headBranch,
		"sha": baseSHA,
	})
	_, resp, err := c.REST.Post(ctx, fmt.Sprintf("/repos/%s/%s/git/refs", c.RepoOwner, c.RepoName), body, nil)
	if err != nil {
		return wrapRetryable(err, resp)
	}
	if resp == nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		return fmt.Errorf("create branch %q: status %d", headBranch, statusOf(resp))
	}
	return nil
}

// resolveBaseBranch returns baseBranch when non-empty, otherwise the
// repository's default branch from GET /repos/.../{owner}/{repo}.
func (c *Committer) resolveBaseBranch(ctx context.Context, baseBranch string) (string, error) {
	if baseBranch != "" {
		return baseBranch, nil
	}
	path := fmt.Sprintf("/repos/%s/%s", c.RepoOwner, c.RepoName)
	body, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return "", wrapRetryable(err, resp)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve default branch: status %d", statusOf(resp))
	}
	var doc struct {
		DefaultBranch string `json:"default_branch"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return "", fmt.Errorf("decode default branch: %w", jerr)
	}
	if doc.DefaultBranch == "" {
		return "", errors.New("default_branch missing in response")
	}
	return doc.DefaultBranch, nil
}

// fetchBranchSHA returns the object SHA at the tip of branch.
func (c *Committer) fetchBranchSHA(ctx context.Context, branch string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", c.RepoOwner, c.RepoName, branch)
	body, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return "", wrapRetryable(err, resp)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s sha: status %d", branch, statusOf(resp))
	}
	var doc struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return "", fmt.Errorf("decode ref: %w", jerr)
	}
	if doc.Object.SHA == "" {
		return "", fmt.Errorf("ref %s missing object.sha", branch)
	}
	return doc.Object.SHA, nil
}

// fetchPreviousSHAOnBranch is like fetchPreviousSHA but pins the
// branch explicitly (used by the pull-request path where the file
// lookup must target the freshly-created head branch, not the
// configured committer_branch).
func (c *Committer) fetchPreviousSHAOnBranch(ctx context.Context, branch string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s",
		c.RepoOwner, c.RepoName, c.Filename, branch)
	body, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return "", wrapRetryable(err, resp)
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch previous on %s: status %d", branch, statusOf(resp))
	}
	var doc struct {
		SHA string `json:"sha"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return "", fmt.Errorf("decode contents on %s: %w", branch, jerr)
	}
	return doc.SHA, nil
}

// createPullRequest issues POST /repos/.../pulls and returns the PR
// number + URL.
func (c *Committer) createPullRequest(ctx context.Context, baseBranch, headBranch string) (int, string, error) {
	resolvedBase, err := c.resolveBaseBranch(ctx, baseBranch)
	if err != nil {
		return 0, "", err
	}
	title := firstLineOf(c.Message)
	body, _ := json.Marshal(map[string]any{
		"title":                 title,
		"body":                  c.Message,
		"head":                  headBranch,
		"base":                  resolvedBase,
		"maintainer_can_modify": true,
	})
	respBody, resp, err := c.REST.Post(ctx, fmt.Sprintf("/repos/%s/%s/pulls", c.RepoOwner, c.RepoName), body, nil)
	if err != nil {
		return 0, "", wrapRetryable(err, resp)
	}
	if resp == nil || resp.StatusCode != http.StatusCreated {
		return 0, "", fmt.Errorf("create PR: status %d", statusOf(resp))
	}
	var doc struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if jerr := json.Unmarshal(respBody, &doc); jerr != nil {
		return 0, "", fmt.Errorf("decode PR response: %w", jerr)
	}
	return doc.Number, doc.HTMLURL, nil
}

// mergePullRequest issues PUT /repos/.../pulls/{n}/merge with the
// supplied merge_method (merge | squash | rebase).
func (c *Committer) mergePullRequest(ctx context.Context, prNumber int, method string) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", c.RepoOwner, c.RepoName, prNumber)
	body, _ := json.Marshal(map[string]any{
		"merge_method":   method,
		"commit_message": c.Message,
	})
	_, resp, err := c.REST.Put(ctx, path, body, nil)
	if err != nil {
		return wrapRetryable(err, resp)
	}
	if resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
		return fmt.Errorf("merge PR #%d (%s): status %d", prNumber, method, statusOf(resp))
	}
	return nil
}

// firstLineOf returns the first non-empty line of s (used for PR title).
func firstLineOf(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
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
