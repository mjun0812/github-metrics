package action

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// prRESTMock satisfies the GitHub REST surface the PR / merge paths
// touch (repo lookup, ref CRUD, contents PUT, pulls POST, merge PUT).
// It records every method+path for ordering assertions.
type prRESTMock struct {
	mu               sync.Mutex
	defaultBranch    string
	branchSHAs       map[string]string // baseBranch → tip SHA
	existingBranches map[string]bool   // refs/heads/<branch> existence
	contentsFor      map[string]string // "branch/path" → base64 body for GET /contents/...
	prNumber         int
	prURL            string
	mergeStatus      int
	calls            []string // "METHOD path" history
}

func newPRRESTMock() *prRESTMock {
	return &prRESTMock{
		defaultBranch:    "main",
		branchSHAs:       map[string]string{"main": "tipsha"},
		existingBranches: map[string]bool{"main": true},
		contentsFor:      map[string]string{},
		prNumber:         42,
		prURL:            "https://github.com/o/r/pull/42",
		mergeStatus:      http.StatusOK,
	}
}

func (m *prRESTMock) record(method, path string) {
	m.calls = append(m.calls, method+" "+path)
}

func (m *prRESTMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := req.URL.Path
	method := req.Method
	m.record(method, path)
	h := http.Header{"Content-Type": []string{"application/json"}}
	switch {
	case method == http.MethodGet && strings.HasSuffix(path, "/repos/o/r"):
		body := `{"default_branch":"` + m.defaultBranch + `"}`
		return mkPRResp(req, http.StatusOK, h, body), nil
	case method == http.MethodGet && strings.Contains(path, "/git/refs/heads/"):
		branch := strings.TrimPrefix(path, "/repos/o/r/git/refs/heads/")
		if !m.existingBranches[branch] {
			return mkPRResp(req, http.StatusNotFound, h, `{}`), nil
		}
		sha := m.branchSHAs[branch]
		if sha == "" {
			sha = "deadbeef"
		}
		return mkPRResp(req, http.StatusOK, h, `{"object":{"sha":"`+sha+`"}}`), nil
	case method == http.MethodPost && strings.HasSuffix(path, "/git/refs"):
		bodyBytes, _ := io.ReadAll(req.Body)
		var doc struct{ Ref string }
		_ = json.Unmarshal(bodyBytes, &doc)
		newBranch := strings.TrimPrefix(doc.Ref, "refs/heads/")
		m.existingBranches[newBranch] = true
		m.branchSHAs[newBranch] = "newrefsha"
		return mkPRResp(req, http.StatusCreated, h, `{"ref":"`+doc.Ref+`","object":{"sha":"newrefsha"}}`), nil
	case method == http.MethodGet && strings.Contains(path, "/contents/"):
		ref := req.URL.Query().Get("ref")
		key := ref + "/" + strings.Split(path, "/contents/")[1]
		if body, ok := m.contentsFor[key]; ok {
			return mkPRResp(req, http.StatusOK, h, body), nil
		}
		return mkPRResp(req, http.StatusNotFound, h, `{}`), nil
	case method == http.MethodPut && strings.Contains(path, "/contents/"):
		return mkPRResp(req, http.StatusCreated, h, `{"content":{"sha":"abc"}}`), nil
	case method == http.MethodPost && strings.HasSuffix(path, "/pulls"):
		body, _ := json.Marshal(map[string]any{
			"number":   m.prNumber,
			"html_url": m.prURL,
		})
		return mkPRResp(req, http.StatusCreated, h, string(body)), nil
	case method == http.MethodPut && strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/merge"):
		return mkPRResp(req, m.mergeStatus, h, `{"merged":true,"sha":"mergedsha"}`), nil
	}
	return mkPRResp(req, http.StatusNotFound, h, `{}`), nil
}

func mkPRResp(req *http.Request, status int, h http.Header, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func newRESTPR(t *testing.T, mock http.RoundTripper) *githubapi.REST {
	t.Helper()
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mock, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return rest
}

func TestCommitter_PullRequest_NoMergeMethod(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	c := &Committer{
		REST: newRESTPR(t, mock), Policy: RetryPolicy{Retries: 0, Delay: 0},
		RepoOwner: "o", RepoName: "r",
		Branch: "main", RunID: "12345",
		Filename: "github-metrics.svg",
		Message:  "Auto-generated metrics for run #12345",
		Author:   CommitterAuthor{Name: "metrics", Email: "m@x"},
		Action:   "pull-request",
		Body:     []byte(`<svg></svg>`),
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", c.PRNumber)
	}
	if !strings.Contains(c.MetricsURL, "/pull/42") {
		t.Errorf("MetricsURL should be PR URL; got %q", c.MetricsURL)
	}
	// Verify the create-PR call happened.
	found := false
	for _, call := range mock.calls {
		if call == "POST /repos/o/r/pulls" {
			found = true
		}
	}
	if !found {
		t.Errorf("POST /pulls missing; calls=%v", mock.calls)
	}
}

func TestCommitter_PullRequestMerge_AutoMerges(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	c := &Committer{
		REST: newRESTPR(t, mock), Policy: RetryPolicy{Retries: 0, Delay: 0},
		RepoOwner: "o", RepoName: "r",
		Branch: "main", RunID: "12345",
		Filename: "x.svg", Message: "metrics",
		Author: CommitterAuthor{Name: "m", Email: "m@x"},
		Action: "pull-request-merge",
		Body:   []byte(`<svg></svg>`),
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	merged := false
	for _, call := range mock.calls {
		if strings.HasPrefix(call, "PUT") && strings.HasSuffix(call, "/merge") {
			merged = true
		}
	}
	if !merged {
		t.Errorf("expected PUT /pulls/N/merge; calls=%v", mock.calls)
	}
}

func TestCommitter_PullRequestSquash_AndRebase(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"pull-request-squash", "pull-request-rebase"} {
		t.Run(action, func(t *testing.T) {
			mock := newPRRESTMock()
			c := &Committer{
				REST: newRESTPR(t, mock), Policy: RetryPolicy{Retries: 0, Delay: 0},
				RepoOwner: "o", RepoName: "r",
				Branch: "main", RunID: "12345",
				Filename: "x.svg", Message: "metrics",
				Author: CommitterAuthor{Name: "m", Email: "m@x"},
				Action: action,
				Body:   []byte(`<svg></svg>`),
			}
			if err := c.Run(context.Background()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			// Verify merge call carries the right merge_method.
			// We can't inspect body here without exposing the body recorder;
			// the call sequence existence is the immediate signal.
			merged := false
			for _, call := range mock.calls {
				if strings.Contains(call, "/merge") {
					merged = true
				}
			}
			if !merged {
				t.Errorf("expected merge call for %s; calls=%v", action, mock.calls)
			}
		})
	}
}

func TestCommitter_PullRequest_MissingRunID(t *testing.T) {
	t.Parallel()
	mock := newPRRESTMock()
	c := &Committer{
		REST: newRESTPR(t, mock), Policy: RetryPolicy{Retries: 0, Delay: 0},
		RepoOwner: "o", RepoName: "r",
		Branch:   "main",
		Filename: "x.svg", Message: "metrics",
		Author: CommitterAuthor{Name: "m", Email: "m@x"},
		Action: "pull-request",
		Body:   []byte(`<svg></svg>`),
	}
	err := c.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "GITHUB_RUN_ID") {
		t.Errorf("expected GITHUB_RUN_ID error; got %v", err)
	}
}

func TestCommitter_DataChanged_SkipsCommit(t *testing.T) {
	t.Parallel()
	// Configure the contents endpoint so the GET /contents/... returns
	// a base64-encoded body identical to the new render. The committer
	// should observe Equal=true and skip the PUT.
	const body = `<svg><text>same</text></svg>`
	mock := newPRRESTMock()
	mock.contentsFor["main/github-metrics.svg"] = `{"content":"` + b64(body) + `","encoding":"base64"}`

	c := &Committer{
		REST: newRESTPR(t, mock), Policy: RetryPolicy{Retries: 0, Delay: 0},
		RepoOwner: "o", RepoName: "r",
		Branch:    "main",
		Filename:  "github-metrics.svg",
		Message:   "metrics",
		Author:    CommitterAuthor{Name: "m", Email: "m@x"},
		Action:    "commit",
		Condition: "data-changed",
		Body:      []byte(body),
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !c.Skipped {
		t.Errorf("expected Skipped=true on identical body")
	}
	for _, call := range mock.calls {
		if strings.HasPrefix(call, "PUT") && strings.Contains(call, "/contents/") {
			t.Errorf("PUT /contents must not run on data-changed match; calls=%v", mock.calls)
		}
	}
}
