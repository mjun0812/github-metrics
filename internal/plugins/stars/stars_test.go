package stars_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/stars"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

// graphqlMux serves a single canned GraphQL response. status/body
// control success vs. failure.
type graphqlMux struct {
	mu     sync.Mutex
	status int
	body   string
	calls  int
}

func (m *graphqlMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	status, body := m.status, m.body
	m.mu.Unlock()
	if status == 0 {
		status = http.StatusOK
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

func newGQL(t *testing.T, mux *graphqlMux) *githubapi.GraphQL {
	t.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return gql
}

func TestRun_Normal(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{body: `{"data":{"user":{"starredRepositories":{"totalCount":2,"edges":[
		{"starredAt":"2026-05-10T00:00:00Z","node":{"nameWithOwner":"alice/x","description":"hi","url":"https://github.com/alice/x","stargazerCount":100,"forkCount":1}},
		{"starredAt":"2026-05-09T00:00:00Z","node":{"nameWithOwner":"bob/y","description":null,"url":"https://github.com/bob/y","stargazerCount":50,"forkCount":2}}
	]}}}}`}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_stars": true},
		GraphQL: newGQL(t, mux),
	}
	out, err := stars.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*stars.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if len(r.List) != 2 {
		t.Errorf("List len = %d, want 2", len(r.List))
	}
	if r.List[0].NameWithOwner != "alice/x" {
		t.Errorf("first = %s, want alice/x", r.List[0].NameWithOwner)
	}
}

func TestRun_LimitInput(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{body: `{"data":{"user":{"starredRepositories":{"totalCount":0,"edges":[]}}}}`}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_stars": true, "plugin_stars_limit": 8},
		GraphQL: newGQL(t, mux),
	}
	out, _ := stars.Plugin.Run(context.Background(), pc)
	r := out.(*stars.Result)
	if r.Limit != 8 {
		t.Errorf("Limit = %d, want 8", r.Limit)
	}
}

func TestRun_EmptyResult(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{body: `{"data":{"user":{"starredRepositories":{"totalCount":0,"edges":[]}}}}`}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_stars": true},
		GraphQL: newGQL(t, mux),
	}
	out, _ := stars.Plugin.Run(context.Background(), pc)
	r := out.(*stars.Result)
	if r.Skipped {
		t.Errorf("empty list != Skipped")
	}
	if len(r.List) != 0 {
		t.Errorf("List = %v, want empty", r.List)
	}
}

func TestRun_NilUser(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{body: `{"data":{"user":null}}`}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_stars": true},
		GraphQL: newGQL(t, mux),
	}
	out, _ := stars.Plugin.Run(context.Background(), pc)
	r := out.(*stars.Result)
	if r.Skipped {
		t.Errorf("nil user response should yield empty result not Skipped")
	}
}

func TestRun_5xxRetryable(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{
		status: http.StatusOK,
		body:   `{"data":null,"errors":[{"message":"Internal Server Error"}]}`,
	}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_stars": true},
		GraphQL: newGQL(t, mux),
	}
	_, err := stars.Plugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("error type = %T, want *RetryableError", err)
	}
}

func TestRun_NilGraphQL_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{"user": "octocat", "plugin_stars": true}}
	out, _ := stars.Plugin.Run(context.Background(), pc)
	r := out.(*stars.Result)
	if !r.Skipped {
		t.Errorf("nil GraphQL should yield Skipped")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &stars.Result{
		List: []stars.StarredRepo{
			{NameWithOwner: "alice/x", Description: "hi", Stars: 100, StarredAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
		},
		Limit: 4,
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "stars.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}
