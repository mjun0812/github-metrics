//go:build heavy

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	"github.com/mjun0812/github-metrics/internal/plugins/languages"
)

// makeHeavyRepo prepares a minimal git repo with the supplied
// (path, content) pairs and returns the directory.
func makeHeavyRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s parent: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for name := range files {
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add %s: %v", name, err)
		}
	}
	_, err = wt.Commit("seed", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return dir
}

// fsCloner copies a prepared source dir into the destination — same
// pattern as the indepth_heavy_test fakeCloner, but inlined here so
// the integration test doesn't reach into a sibling _test.go file.
type fsCloner struct {
	sources map[string]string
}

func (f *fsCloner) Clone(ctx context.Context, dst, url string) (string, error) {
	src, ok := f.sources[url]
	if !ok {
		return "", &cloneMissError{url: url}
	}
	return dst, filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

type cloneMissError struct{ url string }

func (e *cloneMissError) Error() string { return "fsCloner: unknown URL " + e.url }

// p3HeavyEventsMux serves /users/octocat/events with a fixed PushEvent
// payload + /repos/.../commits/<sha> with file lists for the recent
// plugin to analyse.
type p3HeavyEventsMux struct{}

func (m *p3HeavyEventsMux) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	switch {
	case strings.HasSuffix(path, "/events"):
		body := `[
			{"type":"PushEvent","repo":{"name":"octocat/alpha"},"created_at":"` +
			time.Now().UTC().Add(-1*time.Hour).Format(time.RFC3339) +
			`","public":true,"payload":{"commits":[{"sha":"shaA"}]}},
			{"type":"PushEvent","repo":{"name":"octocat/beta"},"created_at":"` +
			time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339) +
			`","public":true,"payload":{"commits":[{"sha":"shaB"}]}}
		]`
		return heavyJSONResponse(req, body), nil
	case strings.Contains(path, "/commits/shaA"):
		return heavyJSONResponse(req, `{"files":[{"filename":"main.go","additions":120,"deletions":0}]}`), nil
	case strings.Contains(path, "/commits/shaB"):
		return heavyJSONResponse(req, `{"files":[{"filename":"app.js","additions":80,"deletions":0}]}`), nil
	}
	return heavyJSONResponse(req, `{}`), nil
}

func heavyJSONResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

// TestComputeSVG_P3HeavyAllPlugins drives engine.Compute through the
// classic template with languages.recent + languages.indepth enabled
// and asserts the necessary DOM markers are present.
func TestComputeSVG_P3HeavyAllPlugins(t *testing.T) {
	tmp := t.TempDir()
	srcA := makeHeavyRepo(t, filepath.Join(tmp, "alpha"), map[string]string{
		"main.go": strings.Repeat("// hi\n", 100),
	})
	srcB := makeHeavyRepo(t, filepath.Join(tmp, "beta"), map[string]string{
		"app.js": strings.Repeat("// hi\n", 60),
	})
	cln := &fsCloner{sources: map[string]string{
		"https://github.com/octocat/alpha.git": srcA,
		"https://github.com/octocat/beta.git":  srcB,
	}}

	gqlFix := newGraphQLFixture()
	gqlFix.On("User", p1UserOctocat)
	gqlFix.On("UserRepositories", p1UserRepositories)
	gqlFix.onContributionDefaults()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: gqlFix, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: &p3HeavyEventsMux{}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}

	deps := engine.Deps{
		GraphQL: gql,
		REST:    rest,
		Render:  &render.FakeRenderer{},
	}

	inputs := map[string]any{
		"plugin_languages":          true,
		"plugin_languages_sections": "most-used,recently-used",
		"plugin_languages_indepth":  true,
		languages.IndepthClonerKey:  languages.IndepthCloner(cln),
	}

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   inputs,
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}

	out := string(res.Output)
	wantMarkers := []string{
		// Standard languages section is always present when the plugin
		// runs against a repo list with bytes.
		`<g class="languages-progress">`,
		// Recent + indepth sub-sections added by T078 / T081.
		`<g class="languages-recent"`,
		`<g class="languages-indepth">`,
	}
	for _, m := range wantMarkers {
		if !strings.Contains(out, m) {
			t.Errorf("SVG missing marker %q\n---\n%s\n---", m, snippet(out))
		}
	}
}

// TestComputeJSON_P3HeavyAllPlugins asserts the JSON output exposes
// both heavy plugin entries with non-skipped payloads.
func TestComputeJSON_P3HeavyAllPlugins(t *testing.T) {
	tmp := t.TempDir()
	srcA := makeHeavyRepo(t, filepath.Join(tmp, "alpha"), map[string]string{
		"main.go": "package main\nfunc main(){}\n",
	})
	srcB := makeHeavyRepo(t, filepath.Join(tmp, "beta"), map[string]string{
		"app.js": "console.log(1);\n",
	})
	cln := &fsCloner{sources: map[string]string{
		"https://github.com/octocat/alpha.git": srcA,
		"https://github.com/octocat/beta.git":  srcB,
	}}

	gqlFix := newGraphQLFixture()
	gqlFix.On("User", p1UserOctocat)
	gqlFix.On("UserRepositories", p1UserRepositories)
	gqlFix.onContributionDefaults()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: gqlFix, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: &p3HeavyEventsMux{}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	deps := engine.Deps{
		GraphQL: gql,
		REST:    rest,
		Render:  &render.FakeRenderer{},
	}
	inputs := map[string]any{
		"plugin_languages":          true,
		"plugin_languages_sections": "most-used,recently-used",
		"plugin_languages_indepth":  true,
		languages.IndepthClonerKey:  languages.IndepthCloner(cln),
	}
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "json",
		Inputs:   inputs,
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "application/json" {
		t.Fatalf("MIME = %q, want application/json", res.MIME)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Output, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	pluginsMap, _ := payload["plugins"].(map[string]any)
	if pluginsMap == nil {
		t.Fatalf("plugins map missing in JSON output:\n%s", string(res.Output))
	}
	for _, slug := range []string{"languages.recent", "languages.indepth"} {
		entry, ok := pluginsMap[slug].(map[string]any)
		if !ok {
			t.Errorf("data.plugins[%q] missing or wrong type: %T", slug, pluginsMap[slug])
			continue
		}
		// Both heavy plugins should NOT be skipped given the fixture deps.
		if sk, _ := entry["skipped"].(bool); sk {
			t.Errorf("data.plugins[%q] skipped=true; entry=%v", slug, entry)
		}
	}
}
