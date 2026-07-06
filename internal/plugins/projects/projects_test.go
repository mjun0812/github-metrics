package projects_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/projects"
	"github.com/mjun0812/github-metrics/internal/templates"
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

// newREST builds a *REST whose `GET /` returns the supplied
// X-OAuth-Scopes header, allowing tests to drive Scope-gated logic.
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

func run(t *testing.T, scopes string) *projects.Result {
	t.Helper()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{},
		REST:   newREST(t, scopes),
	}
	out, err := projects.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*projects.Result)
}

func TestRun_MissingProjectScope_Skipped(t *testing.T) {
	t.Parallel()
	r := run(t, "repo, read:user")
	if !r.Skipped {
		t.Errorf("expected Skipped without read:project; got %+v", r)
	}
	if !strings.Contains(r.SkippedReason, "read:project") {
		t.Errorf("SkippedReason should mention read:project; got %q", r.SkippedReason)
	}
}

func TestRun_NoScopes_Skipped(t *testing.T) {
	t.Parallel()
	r := run(t, "")
	if !r.Skipped {
		t.Errorf("expected Skipped without scope header; got %+v", r)
	}
}

func TestRun_WithProjectScope_NotSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, "repo, read:project")
	if r.Skipped {
		t.Errorf("expected non-Skipped when read:project present; got %+v", r)
	}
}

// TestRun_LimitReadsStringInput pins the #661 fix: Action mode delivers
// plugin_projects_limit as a string (INPUT_* env), which the previous
// bare v.(int) assertion silently ignored (limit stayed at 4).
func TestRun_LimitReadsStringInput(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"plugin_projects_limit": "7"},
		REST:   newREST(t, "repo, read:project"),
	}
	out, err := projects.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*projects.Result)
	if r.Limit != 7 {
		t.Errorf("Limit = %d, want 7 (string input honored)", r.Limit)
	}
}

func TestRun_NilRESTSkipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}}
	out, _ := projects.Plugin.Run(context.Background(), pc)
	r := out.(*projects.Result)
	if !r.Skipped {
		t.Errorf("nil REST should yield Skipped; got %+v", r)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &projects.Result{Skipped: true, List: []projects.Project{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "projects.json")
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

// renderPartial wires a Result into a PartialContext and runs the
// classic projects partial, returning its SVG fragment.
func renderPartial(t *testing.T, r *projects.Result) string {
	t.Helper()
	data := plugins.NewData()
	data.SetPlugin(projects.Name, r)
	pc := &templates.PartialContext{Data: data, Inputs: map[string]any{}}
	frag, err := projects.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	return frag
}

// TestPartial_EmptyList_RendersHeader pins issue #475: upstream gates the
// projects section on `<% if (plugins.projects) { %>` (object presence)
// and unconditionally prints the `<%= totalCount %> Project<s>` header, so
// an enabled-but-empty projects result must still render a `0 Projects`
// header instead of an empty card.
func TestPartial_EmptyList_RendersHeader(t *testing.T) {
	t.Parallel()
	frag := renderPartial(t, &projects.Result{List: []projects.Project{}})
	if frag == "" {
		t.Fatalf("empty List must still render the section header; got empty fragment")
	}
	if !strings.Contains(frag, `data-section="projects"`) {
		t.Errorf("fragment missing projects section wrapper: %q", frag)
	}
	if !strings.Contains(frag, `0 Projects`) {
		t.Errorf("empty List must render `0 Projects` header; got %q", frag)
	}
	if !strings.Contains(frag, `<section class="project">`) {
		t.Errorf("fragment missing empty project section; got %q", frag)
	}
}

// TestPartial_Skipped_Empty ensures a Skipped result renders nothing even
// though the empty-list guard was relaxed for #475.
func TestPartial_Skipped_Empty(t *testing.T) {
	t.Parallel()
	frag := renderPartial(t, &projects.Result{Skipped: true, List: []projects.Project{}})
	if frag != "" {
		t.Errorf("Skipped result must render nothing; got %q", frag)
	}
}

// TestPartial_SingularHeader checks the `s()` pluralization: exactly one
// project must render `1 Project` (no trailing "s").
func TestPartial_SingularHeader(t *testing.T) {
	t.Parallel()
	frag := renderPartial(t, &projects.Result{List: []projects.Project{
		{Name: "Solo", URL: "https://github.com/users/u/projects/1"},
	}})
	if !strings.Contains(frag, `1 Project<`) {
		t.Errorf("single project must render `1 Project` header; got %q", frag)
	}
	if strings.Contains(frag, `1 Projects`) {
		t.Errorf("single project must not be pluralized; got %q", frag)
	}
}

// TestPartial_WithProjects renders the project rows (name link, optional
// description, updated date) and asserts the plural header.
func TestPartial_WithProjects(t *testing.T) {
	t.Parallel()
	frag := renderPartial(t, &projects.Result{List: []projects.Project{
		{
			Name:        "Roadmap",
			Description: "Q3 plan",
			URL:         "https://github.com/users/u/projects/2",
			UpdatedAt:   time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			Name: "Backlog",
			URL:  "https://github.com/users/u/projects/3",
		},
	}})
	if !strings.Contains(frag, `2 Projects`) {
		t.Errorf("two projects must render `2 Projects` header; got %q", frag)
	}
	if !strings.Contains(frag, `href="https://github.com/users/u/projects/2"`) {
		t.Errorf("project URL must be linked; got %q", frag)
	}
	if !strings.Contains(frag, `Roadmap`) || !strings.Contains(frag, `Backlog`) {
		t.Errorf("project names must render; got %q", frag)
	}
	if !strings.Contains(frag, `Q3 plan`) {
		t.Errorf("description must render when present; got %q", frag)
	}
	if !strings.Contains(frag, `Updated 2026-05-01`) {
		t.Errorf("updated date must render; got %q", frag)
	}
}
