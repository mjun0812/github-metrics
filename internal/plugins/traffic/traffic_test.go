package traffic_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
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

func newREST(t *testing.T, mux *githubapi.MockTransport) *githubapi.REST {
	t.Helper()
	r, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return r
}

func scopeMux(scopes string) *githubapi.MockTransport {
	mux := githubapi.NewMockTransport()
	h := http.Header{}
	if scopes != "" {
		h.Set("X-OAuth-Scopes", scopes)
	}
	mux.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})
	return mux
}

func TestRun_NoRepoScope_Skipped(t *testing.T) {
	t.Parallel()
	mux := scopeMux("read:user")
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{},
		REST:   newREST(t, mux),
	}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if !r.Skipped {
		t.Errorf("expected Skipped without repo scope")
	}
	if !r.HideEmpty {
		t.Errorf("HideEmpty should default to true even on the skipped path; got false")
	}
}

// TestRun_HideEmpty_DefaultTrue verifies the new
// `plugin_traffic_hide_empty` input defaults to true when the key is
// absent from Inputs.
func TestRun_HideEmpty_DefaultTrue(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{},
		REST:   newREST(t, mux),
	}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if !r.HideEmpty {
		t.Errorf("HideEmpty default = false, want true")
	}
}

// TestRun_HideEmpty_ExplicitFalse verifies `plugin_traffic_hide_empty:
// "no"` and `false` both turn off the filter (so legacy callers can
// re-enable the pre-#412 behaviour).
func TestRun_HideEmpty_ExplicitFalse(t *testing.T) {
	t.Parallel()
	for _, v := range []any{"no", "false", "0", false} {
		v := v
		mux := scopeMux("repo")
		pc := &plugins.PluginContext{
			Data:   plugins.NewData(),
			Inputs: map[string]any{"plugin_traffic_hide_empty": v},
			REST:   newREST(t, mux),
		}
		out, _ := traffic.Plugin.Run(context.Background(), pc)
		r := out.(*traffic.Result)
		if r.HideEmpty {
			t.Errorf("HideEmpty for input %v (%T) = true, want false", v, v)
		}
	}
}

func TestRun_WithRepoScope_AggregatesViews(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	mux.SetJSON("GET", "/repos/octocat/alpha/traffic/views", `{"count":100,"uniques":40}`)
	mux.SetJSON("GET", "/repos/octocat/beta/traffic/views", `{"count":50,"uniques":20}`)

	data := plugins.NewData()
	data.Computed.RepositoryList = []plugins.Repository{
		{NameWithOwner: "octocat/alpha"},
		{NameWithOwner: "octocat/beta"},
	}
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{}, REST: newREST(t, mux)}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if r.Total.Count != 150 || r.Total.Uniques != 60 {
		t.Errorf("Total = %+v, want {Count:150,Uniques:60}", r.Total)
	}
	if len(r.Views) != 2 {
		t.Errorf("Views len = %d, want 2", len(r.Views))
	}
}

func TestRun_403RepoDroppedAndContinue(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	mux.Set("GET", "/repos/octocat/alpha/traffic/views", githubapi.MockResponse{
		Status: http.StatusForbidden,
		Body:   []byte(`{"message":"forbidden"}`),
	})
	mux.SetJSON("GET", "/repos/octocat/beta/traffic/views", `{"count":50,"uniques":20}`)
	data := plugins.NewData()
	data.Computed.RepositoryList = []plugins.Repository{
		{NameWithOwner: "octocat/alpha"},
		{NameWithOwner: "octocat/beta"},
	}
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{}, REST: newREST(t, mux)}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if r.Skipped {
		t.Fatalf("403 on one repo should not Skip the whole result")
	}
	if _, ok := r.Views["octocat/alpha"]; ok {
		t.Errorf("octocat/alpha (403) should be dropped")
	}
	if _, ok := r.Views["octocat/beta"]; !ok {
		t.Errorf("octocat/beta should still be present")
	}
}

func TestRun_NoRepositories_EmptyButNotSkipped(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	pc := &plugins.PluginContext{
		Data: plugins.NewData(), Inputs: map[string]any{}, REST: newREST(t, mux),
	}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if r.Skipped {
		t.Errorf("empty RepositoryList should yield empty (non-Skipped) result")
	}
	if r.Total.Count != 0 {
		t.Errorf("Total.Count = %d, want 0", r.Total.Count)
	}
}

func TestRun_NilREST_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if !r.Skipped {
		t.Errorf("nil REST should yield Skipped")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &traffic.Result{Skipped: true, Views: map[string]traffic.TrafficView{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "traffic.json")
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
