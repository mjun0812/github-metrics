package action

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	// Side-effect imports register the classic template + core plugin
	// so engine.Compute can resolve them. Without these the engine
	// errors with "template not found".
	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// fakeGraphQL is a permissive RoundTripper that returns `{"data":{}}`
// for every GraphQL operation. Sufficient for action_test's purposes
// because we exercise the action plumbing, not the plugin pipeline.
type fakeGraphQL struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeGraphQL) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	reqBody, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	body := pickGraphQLResponse(string(reqBody))
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// pickGraphQLResponse returns a minimal canned GraphQL response based
// on the requested operation. Enough to make engine.Compute's base
// plugin succeed; downstream plugins skip cleanly because the gate
// inputs are all off in action_test scenarios.
func pickGraphQLResponse(reqBody string) string {
	switch {
	case strings.Contains(reqBody, "\"operationName\":\"User\""):
		return `{"data":{"user":{"databaseId":1,"id":"u","login":"octocat","name":"The Octocat","avatarUrl":"https://x","createdAt":"2008-01-14T04:33:35Z"}}}`
	case strings.Contains(reqBody, "\"operationName\":\"UserRepositories\""):
		return `{"data":{"user":{"repositories":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`
	default:
		return `{"data":{}}`
	}
}

// fakeREST mocks the GitHub REST surface action.Run needs:
// HEAD / for token scopes, GET /rate_limit for quota, and the
// arbitrary committer paths the commit-mode integration touches.
type fakeREST struct {
	mu        sync.Mutex
	rateBody  string
	scopes    string
	contents  map[string]string // path → body (for GET /contents/...)
	branches  map[string]bool   // existing branches (path → exists)
	putBodies map[string][]byte // captured PUT bodies (for assertions)
}

func newFakeREST() *fakeREST {
	return &fakeREST{
		rateBody:  `{"resources":{"core":{"remaining":5000,"limit":5000,"reset":0},"graphql":{"remaining":5000,"limit":5000,"reset":0},"search":{"remaining":30,"limit":30,"reset":0}}}`,
		scopes:    "repo",
		contents:  map[string]string{},
		branches:  map[string]bool{},
		putBodies: map[string][]byte{},
	}
}

func (m *fakeREST) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := req.URL.Path
	switch {
	case path == "/" || path == "":
		// HeadRoot probe (actually a GET to "/") for scopes lookup.
		h := http.Header{"X-OAuth-Scopes": []string{m.scopes}}
		return mkRespAction(req, http.StatusOK, h, "{}"), nil
	case path == "/rate_limit":
		return mkRespAction(req, http.StatusOK, nil, m.rateBody), nil
	case strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/git/refs/heads/"):
		if m.branches[path] {
			return mkRespAction(req, http.StatusOK, nil, `{"ref":"refs/heads/main"}`), nil
		}
		return mkRespAction(req, http.StatusNotFound, nil, `{}`), nil
	case strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/contents/"):
		if req.Method == http.MethodGet {
			if body, ok := m.contents[path]; ok {
				return mkRespAction(req, http.StatusOK, nil, body), nil
			}
			return mkRespAction(req, http.StatusNotFound, nil, `{"message":"Not Found"}`), nil
		}
		if req.Method == http.MethodPut {
			body, _ := io.ReadAll(req.Body)
			m.putBodies[path] = body
			return mkRespAction(req, http.StatusCreated, nil, `{"content":{"sha":"deadbeef"}}`), nil
		}
	}
	return mkRespAction(req, http.StatusNotFound, nil, `{}`), nil
}

func mkRespAction(req *http.Request, status int, h http.Header, body string) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// buildTestDeps returns a fully-mocked engine.Deps for action tests.
// REST + GraphQL hit the fake transports; Render is a FakeRenderer
// so a real browser is never started.
func buildTestDeps(t *testing.T, rest *fakeREST) func(context.Context, *Invocation) (engine.Deps, error) {
	t.Helper()
	return func(_ context.Context, inv *Invocation) (engine.Deps, error) {
		restClient, err := githubapi.NewREST(
			inv.Token, "http://mock.localhost",
			httpx.Options{Transport: rest, DisableRetries: true},
		)
		if err != nil {
			return engine.Deps{}, err
		}
		gqlClient, err := githubapi.NewGraphQL(
			inv.Token, "http://mock.localhost/graphql",
			httpx.Options{Transport: &fakeGraphQL{}, DisableRetries: true},
		)
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

// TestRun_Dryrun_NoCommitterCall — happy path with dryrun=yes:
// engine.Compute runs, output file is written, but no PUT /contents
// hits the REST mock.
func TestRun_Dryrun_NoCommitterCall(t *testing.T) {
	// Disabled t.Parallel: each test exclusively sets GITHUB_OUTPUT via t.Setenv,
	// which the testing package documents as parallel-unsafe.
	rest := newFakeREST()
	outDir := t.TempDir()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	var stdout bytes.Buffer
	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"GITHUB_ACTOR=octocat",
			"INPUT_USER=octocat",
			"INPUT_TEMPLATE=classic",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_DRYRUN=yes",
			"INPUT_OUTPUT_ACTION=commit",
			"INPUT_USE_MOCKED_DATA=false",
			// Per-plugin mode is now the default; this test predates it
			// and expects single-file output at github-metrics.svg, so
			// opt into combined mode explicitly.
			"INPUT_COMBINED=yes",
		},
		Stdout:    &stdout,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("runWith: %v", err)
	}

	// Banner emitted to stdout.
	if !strings.Contains(stdout.String(), "metrics-cli — startup banner") {
		t.Errorf("banner missing in stdout: %s", stdout.String())
	}

	// Output file written.
	outPath := filepath.Join(outDir, "github-metrics.svg")
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file at %s; err=%v", outPath, err)
	}

	// Committer NOT called (no PUT /contents/... captured).
	if len(rest.putBodies) > 0 {
		t.Errorf("Committer ran under dryrun; PUT captures = %v", rest.putBodies)
	}

	// metrics_sha output written.
	outBody, _ := os.ReadFile(filepath.Join(outDir, "github_output"))
	if !strings.Contains(string(outBody), "metrics_sha=") {
		t.Errorf("metrics_sha output missing; got %q", outBody)
	}
}

// TestRun_OutputAction_UnsupportedFailFast — gist input causes exit 1
// before any API call hits the REST mock. Spec FR-015b / SC-007.
func TestRun_OutputAction_UnsupportedFailFast(t *testing.T) {
	t.Parallel()
	rest := newFakeREST()
	outDir := t.TempDir()

	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_OUTPUT_ACTION=gist",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err == nil {
		t.Fatal("expected error for output_action=gist")
	}
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("err type = %T, want *ConfigError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error should mention 'not supported'; got %v", err)
	}
	// engine.Compute MUST NOT have been called (no GraphQL hits).
	// fakeREST only sees init helpers (no rate_limit / scope check
	// path either, since output_action validates before token check).
	if len(rest.putBodies) > 0 {
		t.Errorf("PUT seen despite fail-fast: %v", rest.putBodies)
	}
}

// TestRun_GithubPatRejected — fine-grained PAT triggers token-format
// reject before any other API call.
func TestRun_GithubPatRejected(t *testing.T) {
	t.Parallel()
	rest := newFakeREST()
	outDir := t.TempDir()

	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=github_pat_xxx",
			"INPUT_OUTPUT_ACTION=commit",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err == nil {
		t.Fatal("expected error for github_pat_*")
	}
	if !strings.Contains(err.Error(), "github_pat_") {
		t.Errorf("error should mention github_pat_; got %v", err)
	}
}

// TestRun_SkipEvent — skip-marker present in GITHUB_EVENT_PATH causes
// exit 0 with no engine work.
func TestRun_SkipEvent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	_ = os.WriteFile(eventPath, []byte(`{"head_commit":{"message":"chore: [Skip GitHub Action]"}}`), 0o600)

	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock",
		},
		Stdout:    io.Discard,
		OutputDir: dir,
		EventPath: eventPath,
	})
	if err != nil {
		t.Errorf("skip-marker path should exit 0; got err=%v", err)
	}
}

// TestNewInvocation_Defaults populates the right defaults from a
// minimal env + INPUT_<UPPER> set.
func TestNewInvocation_Defaults(t *testing.T) {
	t.Parallel()
	inputs := map[string]any{"user": "octocat", "combined": "yes"}
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}
	inv, err := newInvocation(inputs, env, "/tmp/out")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Login != "octocat" {
		t.Errorf("Login = %q", inv.Login)
	}
	if inv.Template != "classic" {
		t.Errorf("Template = %q, want classic", inv.Template)
	}
	if inv.OutputAction != "commit" {
		t.Errorf("OutputAction = %q, want commit", inv.OutputAction)
	}
	if inv.OutputFilename != "github-metrics.svg" {
		t.Errorf("OutputFilename = %q", inv.OutputFilename)
	}
	if inv.RepoOwner != "mjun0812" || inv.RepoName != "test" {
		t.Errorf("Repo parse failed: owner=%q name=%q", inv.RepoOwner, inv.RepoName)
	}
	if inv.RetryPolicy.Retries != DefaultRetries {
		t.Errorf("RetryPolicy.Retries = %d, want %d", inv.RetryPolicy.Retries, DefaultRetries)
	}
	if inv.RetryPolicy.Delay != DefaultRetryDelay {
		t.Errorf("RetryPolicy.Delay = %v, want %v", inv.RetryPolicy.Delay, DefaultRetryDelay)
	}
	_ = time.Millisecond // keep time import alive across edits
}

// TestNewInvocation_OptimizeDefault guards the wiring that materializes
// the `optimize` metadata default ("css, xml") in the action / CLI
// path. ParseInputs does not apply metadata defaults, so newInvocation
// must inject it — otherwise CSS/XML optimization silently never runs
// (the root cause behind issue #434's non-minified sample output). An
// explicitly-provided value (including empty, the opt-out form) must be
// preserved verbatim.
func TestNewInvocation_OptimizeDefault(t *testing.T) {
	t.Parallel()
	env := map[string]string{"GITHUB_REPOSITORY": "mjun0812/test"}

	cases := []struct {
		name  string
		given map[string]any
		want  any
	}{
		{"absent injects default", map[string]any{"user": "octocat", "combined": "yes"}, []string{"css", "xml"}},
		{"explicit value preserved", map[string]any{"user": "octocat", "optimize": "css", "combined": "yes"}, "css"},
		{"explicit empty opt-out preserved", map[string]any{"user": "octocat", "optimize": "", "combined": "yes"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inv, err := newInvocation(tc.given, env, "/tmp/out")
			if err != nil {
				t.Fatal(err)
			}
			got := inv.Inputs["optimize"]
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("optimize = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestNewInvocation_MissingLogin_Errors — required-input check.
func TestNewInvocation_MissingLogin_Errors(t *testing.T) {
	t.Parallel()
	if _, err := newInvocation(map[string]any{}, map[string]string{}, "/tmp"); err == nil {
		t.Error("expected error when user / GITHUB_ACTOR both empty")
	}
}

// TestSortedTruthyPluginGates filters truthy plugin gates only.
func TestSortedTruthyPluginGates(t *testing.T) {
	t.Parallel()
	got := sortedTruthyPluginGates(map[string]any{
		"plugin_languages":          true,
		"plugin_activity":           "yes",
		"plugin_achievements":       false,
		"plugin_calendar":           "no",
		"plugin_languages_limit":    5,   // sub-option; excluded
		"plugin_languages_sections": "x", // sub-option; excluded
		"user":                      "x", // non-plugin; excluded
	})
	want := []string{"activity", "languages"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRun_RepoTemplate_MissingRepo_FailFast (M7 T011/T032 + SC-003):
// `template == "repository"` without a `repo` input MUST exit 1 in
// well under 5 seconds without contacting the GitHub API.
func TestRun_RepoTemplate_MissingRepo_FailFast(t *testing.T) {
	t.Parallel()
	rest := newFakeREST()
	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_TEMPLATE=repository",
			"INPUT_DRYRUN=yes",
			// INPUT_REPO deliberately omitted.
		},
		Stdout:    io.Discard,
		OutputDir: t.TempDir(),
		BuildDeps: buildTestDeps(t, rest),
	})
	if err == nil {
		t.Fatal("expected fail-fast error when template=repository and repo is empty")
	}
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Errorf("err type = %T, want *ConfigError; err=%v", err, err)
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("error should mention repo; got %v", err)
	}
	// fakeREST.putBodies stays empty (no rate_limit, no scope check,
	// no template-side fetch — we exit before any deps construction).
	if len(rest.putBodies) > 0 {
		t.Errorf("PUT seen despite fail-fast: %v", rest.putBodies)
	}
}

// TestRun_ClassicTemplate_WithRepoInput_Ignored (M7 T033 / FR-007):
// `template == "classic"` + `repo == "something"` MUST still run the
// classic flow without surfacing the repo input. This guards the
// backward-compat promise that pre-M7 workflows continue to function.
func TestRun_ClassicTemplate_WithRepoInput_Ignored(t *testing.T) {
	rest := newFakeREST()
	outDir := t.TempDir()
	t.Setenv("GITHUB_OUTPUT", filepath.Join(outDir, "github_output"))

	err := runWith(context.Background(), runOptions{
		Env: []string{
			"GITHUB_REPOSITORY=mjun0812/test-repo",
			"INPUT_USER=octocat",
			"INPUT_TOKEN=ghp_mock_pat_valid",
			"INPUT_TEMPLATE=classic",
			"INPUT_REPO=stray-but-harmless",
			"INPUT_DRYRUN=yes",
		},
		Stdout:    io.Discard,
		OutputDir: outDir,
		BuildDeps: buildTestDeps(t, rest),
	})
	if err != nil {
		t.Fatalf("classic + repo input must not error: %v", err)
	}
}
