package sponsors_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
)

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
		httpx.Options{Transport: mux, DisableRetries: true},
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

// TestRun_NoOAuthScopeGate verifies the plugin no longer gates on the
// `read:user` / `read:org` OAuth scope (#451). Upstream
// (org_repo/source/plugins/sponsors/index.mjs) renders the section
// regardless of scope, so a token carrying only `repo` must still yield a
// non-Skipped Result with the upstream default sections.
func TestRun_NoOAuthScopeGate(t *testing.T) {
	t.Parallel()
	r := run(t, "repo")
	if r.Skipped {
		t.Errorf("sponsors must not gate on read:user/read:org; got Skipped (%s)", r.SkippedReason)
	}
}

func TestRun_DefaultSections(t *testing.T) {
	t.Parallel()
	r := run(t, "read:user, read:org")
	if r.Skipped {
		t.Errorf("expected non-Skipped; got %+v", r)
	}
	// Default sections mirror upstream
	// assets/plugins/sponsors/metadata.yml (goal, list, about). Caller
	// can override with the `plugin_sponsors_sections` input.
	want := []string{"goal", "list", "about"}
	if len(r.Sections) != len(want) {
		t.Fatalf("expected Sections=%v; got %+v", want, r.Sections)
	}
	for i, s := range want {
		if r.Sections[i] != s {
			t.Errorf("expected Sections=%v; got %+v", want, r.Sections)
			break
		}
	}
}

// TestRun_SizeReadsStringInput pins the #661 fix: Action mode delivers
// plugin_sponsors_size as a string (INPUT_* env), which the previous
// bare v.(int) assertion silently ignored (size stayed at 24).
func TestRun_SizeReadsStringInput(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"plugin_sponsors_size": "48"},
		REST:   newREST(t, "read:user, read:org"),
	}
	out, err := sponsors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*sponsors.Result)
	if r.Size != 48 {
		t.Errorf("Size = %d, want 48 (string input honored)", r.Size)
	}
}

// TestRun_RepoMode_Skipped verifies the one remaining gate (RequireUserMode):
// in repository mode the per-user sponsors section has nothing to render, so
// it Skips. This is the mode gate, NOT an OAuth scope gate.
func TestRun_RepoMode_Skipped(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetRepo(&plugins.Repo{Owner: "mjun0812", Name: "github-metrics"})
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{}}
	out, _ := sponsors.Plugin.Run(context.Background(), pc)
	r := out.(*sponsors.Result)
	if !r.Skipped {
		t.Errorf("repository mode should Skip the user-mode sponsors section; got %+v", r)
	}
}
