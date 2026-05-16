package reactions_test

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/reactions"
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

type fixedMux struct {
	mu   sync.Mutex
	body string
}

func (f *fixedMux) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	body := f.body
	f.mu.Unlock()
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func newGQL(t *testing.T, body string) *githubapi.GraphQL {
	t.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: &fixedMux{body: body}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return gql
}

func TestRun_AggregatesIssuesAndComments(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":{
		"issues":{"totalCount":2,"nodes":[
			{"reactions":{"totalCount":3}},
			{"reactions":{"totalCount":5}}
		]},
		"issueComments":{"totalCount":2,"nodes":[
			{"reactions":{"totalCount":1}},
			{"reactions":{"totalCount":2}}
		]}
	}}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if r.Issues != 8 {
		t.Errorf("Issues = %d, want 8", r.Issues)
	}
	if r.Comments != 3 {
		t.Errorf("Comments = %d, want 3", r.Comments)
	}
	if r.Total != 11 {
		t.Errorf("Total = %d, want 11", r.Total)
	}
}

func TestRun_DetailsFlagSurfacesMap(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":{"issues":{"totalCount":0,"nodes":[]},"issueComments":{"totalCount":0,"nodes":[]}}}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true, "plugin_reactions_details": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if r.Details == nil {
		t.Errorf("Details should be non-nil (even empty) when _details=true")
	}
}

func TestRun_NilUser(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":null}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if r.Skipped {
		t.Errorf("nil user response should yield empty (non-Skipped) result")
	}
}

func TestRun_NilGraphQL_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{"user": "octocat", "plugin_reactions": true}}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if !r.Skipped {
		t.Errorf("nil GraphQL should yield Skipped")
	}
}

func TestRun_NoLogin_Skipped(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":null}}`
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}, GraphQL: newGQL(t, body)}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if !r.Skipped {
		t.Errorf("missing login should yield Skipped")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &reactions.Result{Issues: 5, Comments: 3, Total: 8, Days: 30}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "reactions.json")
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
