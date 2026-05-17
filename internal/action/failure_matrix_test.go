package action

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"
)

// failureMatrixREST returns deterministic GitHub API failures keyed
// per scenario. Each scenario reproduces an FR-016 case where the
// committer fails: the Run pipeline MUST log a Warn + return nil
// (no error bubble) so workflow exit stays 0.
type failureMatrixREST struct {
	mu   sync.Mutex
	name string
}

func (f *failureMatrixREST) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := req.URL.Path
	method := req.Method

	h := http.Header{"Content-Type": []string{"application/json"}}
	rateBody := `{"resources":{"core":{"remaining":5000,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`

	// Always-OK plumbing endpoints (token scopes + rate limit).
	switch path {
	case "", "/":
		h.Set("X-OAuth-Scopes", "repo")
		return mkRespFailure(req, http.StatusOK, h, "{}"), nil
	case "/rate_limit":
		return mkRespFailure(req, http.StatusOK, h, rateBody), nil
	}

	// Scenario-specific failure injection.
	switch f.name {
	case "commit_API_403_branch_protection":
		if method == http.MethodPut && strings.Contains(path, "/contents/") {
			return mkRespFailure(req, http.StatusForbidden, h,
				`{"message":"protected branch hook declined"}`), nil
		}
	case "pr_creation_422_conflict":
		if method == http.MethodPost && strings.HasSuffix(path, "/pulls") {
			return mkRespFailure(req, http.StatusUnprocessableEntity, h,
				`{"message":"A pull request already exists"}`), nil
		}
	case "merge_409_conflict":
		if strings.HasSuffix(path, "/merge") {
			return mkRespFailure(req, http.StatusConflict, h,
				`{"message":"Pull Request is not mergeable"}`), nil
		}
	case "merge_method_unavailable":
		if strings.HasSuffix(path, "/merge") {
			return mkRespFailure(req, http.StatusMethodNotAllowed, h,
				`{"message":"Merge method not allowed"}`), nil
		}
	}

	// Permissive defaults so other steps proceed and reach the failure.
	switch {
	case strings.HasSuffix(path, "/repos/o/r"):
		return mkRespFailure(req, http.StatusOK, h, `{"default_branch":"main"}`), nil
	case method == http.MethodGet && strings.Contains(path, "/git/refs/heads/"):
		branch := strings.TrimPrefix(path, "/repos/o/r/git/refs/heads/")
		if branch == "main" {
			return mkRespFailure(req, http.StatusOK, h, `{"object":{"sha":"tipsha"}}`), nil
		}
		return mkRespFailure(req, http.StatusNotFound, h, `{}`), nil
	case method == http.MethodPost && strings.HasSuffix(path, "/git/refs"):
		return mkRespFailure(req, http.StatusCreated, h, `{"ref":"refs/heads/x","object":{"sha":"x"}}`), nil
	case method == http.MethodGet && strings.Contains(path, "/contents/"):
		return mkRespFailure(req, http.StatusNotFound, h, `{}`), nil
	case method == http.MethodPut && strings.Contains(path, "/contents/"):
		return mkRespFailure(req, http.StatusCreated, h, `{"content":{"sha":"abc"}}`), nil
	case method == http.MethodPost && strings.HasSuffix(path, "/pulls"):
		return mkRespFailure(req, http.StatusCreated, h, `{"number":42,"html_url":"https://github.com/o/r/pull/42"}`), nil
	}
	return mkRespFailure(req, http.StatusOK, h, "{}"), nil
}

func mkRespFailure(req *http.Request, status int, h http.Header, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// TestAction_OutputAction_FailureWarning_Matrix covers SC-007 前半:
// 4 deterministic committer failures MUST each warn-and-continue
// (Run returns nil, so the wrapping main returns exit 0).
func TestAction_OutputAction_FailureWarning_Matrix(t *testing.T) {
	cases := []struct {
		name         string
		action       string
		wantWarnHint string
	}{
		{"commit_API_403_branch_protection", "commit", "committer failed"},
		{"pr_creation_422_conflict", "pull-request", "committer failed"},
		{"merge_409_conflict", "pull-request-merge", "committer failed"},
		{"merge_method_unavailable", "pull-request-rebase", "committer failed"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Capture slog output to assert the Warn line.
			var logBuf bytes.Buffer
			oldDefault := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			t.Cleanup(func() { slog.SetDefault(oldDefault) })

			restTransport := &failureMatrixREST{name: tc.name}
			err := runWith(context.Background(), runOptions{
				Mode: ModeAction,
				Env: []string{
					"GITHUB_REPOSITORY=o/r",
					"GITHUB_ACTOR=octocat",
					"GITHUB_RUN_ID=12345",
					"INPUT_USER=octocat",
					"INPUT_TOKEN=ghp_mock_pat_valid",
					"INPUT_DRYRUN=no",
					"INPUT_OUTPUT_ACTION=" + tc.action,
				},
				Stdout:    io.Discard,
				OutputDir: t.TempDir(),
				BuildDeps: buildFailureMatrixDeps(t, restTransport),
			})
			if err != nil {
				t.Fatalf("Run must succeed (exit 0) on committer failure; got err=%v", err)
			}
			if !strings.Contains(logBuf.String(), tc.wantWarnHint) {
				t.Errorf("expected Warn log %q in slog output; got: %s", tc.wantWarnHint, logBuf.String())
			}
		})
	}
}

func buildFailureMatrixDeps(t *testing.T, transport http.RoundTripper) func(context.Context, *Invocation) (engine.Deps, error) {
	t.Helper()
	return func(_ context.Context, inv *Invocation) (engine.Deps, error) {
		restClient, err := githubapi.NewREST(inv.Token, "http://mock.localhost",
			httpx.Options{Transport: transport, DisableRetries: true})
		if err != nil {
			return engine.Deps{}, err
		}
		gqlClient, err := githubapi.NewGraphQL(inv.Token, "http://mock.localhost/graphql",
			httpx.Options{Transport: &fakeGraphQL{}, DisableRetries: true})
		if err != nil {
			return engine.Deps{}, err
		}
		return engine.Deps{
			Settings: &config.Settings{Repositories: 100},
			REST:     restClient,
			GraphQL:  gqlClient,
			Render:   render.NewFakeRenderer(),
		}, nil
	}
}
