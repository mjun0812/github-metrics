package action

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

type staticRoundTripper struct {
	status int
	body   string
	err    error
}

func (s staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	status := s.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    req,
	}, s.err
}

type branchPostStatusRoundTripper struct{}

func (branchPostStatusRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/git/refs/heads/") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"object":{"sha":"basesha"}}`)),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Request:    req,
	}, nil
}

// TestNewCommitter_Errors covers constructor validation failures.
func TestNewCommitter_Errors(t *testing.T) {
	t.Parallel()
	if _, err := NewCommitter(nil, nil, nil); err == nil {
		t.Fatal("expected nil invocation error")
	}
	_, err := NewCommitter(nil, &Invocation{OutputAction: "commit"}, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "repo owner/name") {
		t.Fatalf("expected repo identity error, got %v", err)
	}
	_, err = NewCommitter(nil, &Invocation{
		OutputAction: "commit",
		RepoOwner:    "o",
		RepoName:     "r",
	}, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "nil REST") {
		t.Fatalf("expected nil REST error, got %v", err)
	}
}

// TestCommitter_RunUnknownAction covers the defensive dispatch branch.
func TestCommitter_RunUnknownAction(t *testing.T) {
	t.Parallel()
	c := &Committer{Action: "surprise"}
	err := c.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unrecognized") {
		t.Fatalf("expected unrecognized output_action error, got %v", err)
	}
}

// TestCommitter_RunNoneAction covers no-op action dispatch.
func TestCommitter_RunNoneAction(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"", "none"} {
		c := &Committer{Action: action}
		if err := c.Run(context.Background()); err != nil {
			t.Fatalf("action %q should be no-op: %v", action, err)
		}
	}
}

// TestCommitter_EnsureBranchEdges covers the direct branch lookup paths.
func TestCommitter_EnsureBranchEdges(t *testing.T) {
	t.Parallel()
	if err := (&Committer{}).ensureBranch(context.Background()); err != nil {
		t.Fatalf("empty branch should not need REST: %v", err)
	}

	mock := newPRRESTMock()
	mock.existingBranches["feature"] = true
	c := &Committer{REST: newRESTPR(t, mock), RepoOwner: "o", RepoName: "r", Branch: "feature"}
	if err := c.ensureBranch(context.Background()); err != nil {
		t.Fatalf("existing branch: %v", err)
	}

	c.Branch = "missing"
	err := c.ensureBranch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "auto-creation") {
		t.Fatalf("expected missing branch error, got %v", err)
	}

	c.REST = newRESTPR(t, staticRoundTripper{status: http.StatusTeapot, body: `{}`})
	c.Branch = "weird"
	err = c.ensureBranch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected unexpected status error, got %v", err)
	}
}

// TestCommitter_FetchPreviousSHAEdges covers previous-file lookup branches.
func TestCommitter_FetchPreviousSHAEdges(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	mock.contentsFor["/github-metrics.svg"] = `{"sha":"prevsha"}`
	c := &Committer{
		REST:      newRESTPR(t, mock),
		RepoOwner: "o",
		RepoName:  "r",
		Filename:  "github-metrics.svg",
	}
	got, err := c.fetchPreviousSHA(context.Background())
	if err != nil {
		t.Fatalf("fetchPreviousSHA: %v", err)
	}
	if got != "prevsha" {
		t.Fatalf("sha = %q, want prevsha", got)
	}

	mock.contentsFor["/github-metrics.svg"] = `{`
	if _, err := c.fetchPreviousSHA(context.Background()); err == nil {
		t.Fatalf("expected JSON decode error")
	}

	c.Branch = "main"
	c.REST = newRESTPR(t, staticRoundTripper{status: http.StatusAccepted, body: `{}`})
	if _, err := c.fetchPreviousSHA(context.Background()); err == nil {
		t.Fatalf("expected unexpected status error with branch ref")
	}
}

// TestCommitter_FetchPreviousSHAOnBranch covers the PR head branch lookup.
func TestCommitter_FetchPreviousSHAOnBranch(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	mock.contentsFor["head/github-metrics.svg"] = `{"sha":"headsha"}`
	c := &Committer{
		REST:      newRESTPR(t, mock),
		RepoOwner: "o",
		RepoName:  "r",
		Filename:  "github-metrics.svg",
	}
	got, err := c.fetchPreviousSHAOnBranch(context.Background(), "head")
	if err != nil {
		t.Fatalf("fetchPreviousSHAOnBranch: %v", err)
	}
	if got != "headsha" {
		t.Fatalf("sha = %q, want headsha", got)
	}

	delete(mock.contentsFor, "head/github-metrics.svg")
	got, err = c.fetchPreviousSHAOnBranch(context.Background(), "head")
	if err != nil {
		t.Fatalf("404 should be new file: %v", err)
	}
	if got != "" {
		t.Fatalf("404 sha = %q, want empty", got)
	}

	mock.contentsFor["head/github-metrics.svg"] = `{`
	if _, err := c.fetchPreviousSHAOnBranch(context.Background(), "head"); err == nil {
		t.Fatalf("expected JSON decode error")
	}
}

// TestCommitter_PutContentsIncludesBranchAndSHA covers update payload fields.
func TestCommitter_PutContentsIncludesBranchAndSHA(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	c := &Committer{
		REST:      newRESTPR(t, mock),
		RepoOwner: "o",
		RepoName:  "r",
		Filename:  "github-metrics.svg",
		Message:   "metrics",
		Author:    CommitterAuthor{Name: "m", Email: "m@x"},
		Body:      []byte(`<svg></svg>`),
	}
	if err := c.putContents(context.Background(), "main", "prevsha"); err != nil {
		t.Fatalf("putContents: %v", err)
	}
	var payload map[string]any
	// prRESTMock does not retain PUT bodies, so re-run through fakeREST for payload assertion.
	rest := newFakeREST()
	c.REST = newRESTPR(t, rest)
	if err := c.putContents(context.Background(), "main", "prevsha"); err != nil {
		t.Fatalf("putContents with fakeREST: %v", err)
	}
	for _, body := range rest.putBodies {
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
	}
	if payload["branch"] != "main" || payload["sha"] != "prevsha" {
		t.Fatalf("payload branch/sha = %v/%v", payload["branch"], payload["sha"])
	}
}

// TestCommitter_ResolveAndFetchBranchSHAEdges covers branch decode errors.
func TestCommitter_ResolveAndFetchBranchSHAEdges(t *testing.T) {
	t.Parallel()
	c := &Committer{RepoOwner: "o", RepoName: "r"}
	got, err := c.resolveBaseBranch(context.Background(), "release")
	if err != nil {
		t.Fatalf("non-empty base branch: %v", err)
	}
	if got != "release" {
		t.Fatalf("base = %q, want release", got)
	}

	mock := newPRRESTMock()
	mock.defaultBranch = ""
	c.REST = newRESTPR(t, mock)
	if _, err := c.resolveBaseBranch(context.Background(), ""); err == nil {
		t.Fatalf("expected missing default_branch error")
	}

	c.REST = newRESTPR(t, staticRoundTripper{body: `{"object":{}}`})
	if _, err := c.fetchBranchSHA(context.Background(), "empty"); err == nil {
		t.Fatalf("expected missing object.sha error")
	}
}

// TestCommitter_CreatePullRequestErrors covers direct PR creation failures.
func TestCommitter_CreatePullRequestErrors(t *testing.T) {
	t.Parallel()
	c := &Committer{
		REST:      newRESTPR(t, staticRoundTripper{status: http.StatusOK, body: `{}`}),
		RepoOwner: "o",
		RepoName:  "r",
		Message:   "metrics",
	}
	if _, _, err := c.createPullRequest(context.Background(), "main", "head"); err == nil {
		t.Fatalf("expected status error")
	}
	c.REST = newRESTPR(t, staticRoundTripper{status: http.StatusCreated, body: `{`})
	if _, _, err := c.createPullRequest(context.Background(), "main", "head"); err == nil {
		t.Fatalf("expected decode error")
	}
}

// TestCommitter_CreateBranchFromBaseErrors covers resolve, fetch, and POST
// failures while creating a PR head branch.
func TestCommitter_CreateBranchFromBaseErrors(t *testing.T) {
	t.Parallel()
	c := &Committer{
		REST:      newRESTPR(t, staticRoundTripper{status: http.StatusInternalServerError, body: `{}`}),
		RepoOwner: "o",
		RepoName:  "r",
	}
	if err := c.createBranchFromBase(context.Background(), "", "head"); err == nil {
		t.Fatalf("expected resolve default branch error")
	}
	if err := c.createBranchFromBase(context.Background(), "main", "head"); err == nil {
		t.Fatalf("expected fetch branch sha error")
	}

	c.REST = newRESTPR(t, branchPostStatusRoundTripper{})
	if err := c.createBranchFromBase(context.Background(), "main", "head"); err == nil {
		t.Fatalf("expected create branch status error")
	}
}

// TestFirstLineOf_Empty covers the all-empty message fallback.
func TestFirstLineOf_Empty(t *testing.T) {
	t.Parallel()
	if got := firstLineOf("\n \n\t"); got != "" {
		t.Fatalf("firstLineOf empty = %q, want empty", got)
	}
}

// TestWrapRetryableAndStatusOf covers small error classification helpers.
func TestWrapRetryableAndStatusOf(t *testing.T) {
	t.Parallel()
	if err := wrapRetryable(nil, nil); err != nil {
		t.Fatalf("nil err should stay nil: %v", err)
	}

	transportErr := errors.New("transport")
	for _, resp := range []*http.Response{nil, {StatusCode: http.StatusInternalServerError}} {
		err := wrapRetryable(transportErr, resp)
		var retryable *xerrors.RetryableError
		if !errors.As(err, &retryable) {
			t.Fatalf("err = %T, want RetryableError", err)
		}
	}

	err := wrapRetryable(transportErr, &http.Response{StatusCode: http.StatusBadRequest})
	if !errors.Is(err, transportErr) {
		t.Fatalf("4xx should preserve original error, got %v", err)
	}
	if statusOf(nil) != 0 {
		t.Fatalf("nil response status should be 0")
	}
}
