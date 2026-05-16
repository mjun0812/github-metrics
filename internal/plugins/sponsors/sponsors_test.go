package sponsors_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
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

func newREST(t *testing.T, scopes string) *githubapi.REST {
	t.Helper()
	mux := githubapi.NewMockTransport()
	h := http.Header{}
	if scopes != "" {
		h.Set("X-OAuth-Scopes", scopes)
	}
	mux.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})
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

func run(t *testing.T, scopes string) *sponsors.Result {
	t.Helper()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{},
		REST:   newREST(t, scopes),
	}
	out, err := sponsors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*sponsors.Result)
}

func TestRun_NoScope_Skipped(t *testing.T) {
	t.Parallel()
	r := run(t, "repo")
	if !r.Skipped {
		t.Errorf("expected Skipped without read:user or read:org; got %+v", r)
	}
}

func TestRun_ReadUserOnly_NotSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, "read:user")
	if r.Skipped {
		t.Errorf("read:user alone should satisfy gate; got Skipped")
	}
}

func TestRun_ReadOrgOnly_NotSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, "read:org")
	if r.Skipped {
		t.Errorf("read:org alone should satisfy gate; got Skipped")
	}
}

func TestRun_BothScopes_NotSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, "read:user, read:org")
	if r.Skipped {
		t.Errorf("expected non-Skipped with both scopes; got %+v", r)
	}
	if len(r.Sections) == 0 || r.Sections[0] != "sponsors" {
		t.Errorf("expected Sections=[sponsors]; got %+v", r.Sections)
	}
}

func TestRun_NilREST_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}}
	out, _ := sponsors.Plugin.Run(context.Background(), pc)
	r := out.(*sponsors.Result)
	if !r.Skipped {
		t.Errorf("nil REST should yield Skipped; got %+v", r)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &sponsors.Result{Skipped: true, Sponsors: []sponsors.Sponsor{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "sponsors.json")
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
	_ = strings.Contains // silence unused
}
