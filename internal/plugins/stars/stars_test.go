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
		{"starredAt":"2026-05-10T00:00:00Z","node":{"nameWithOwner":"alice/x","description":"hi","url":"https://github.com/alice/x","isFork":false,"stargazerCount":100,"forkCount":1,"primaryLanguage":{"name":"Go","color":"#00ADD8"},"licenseInfo":{"name":"MIT License","spdxId":"MIT"},"issues":{"totalCount":3},"pullRequests":{"totalCount":7}}},
		{"starredAt":"2026-05-09T00:00:00Z","node":{"nameWithOwner":"bob/y","description":null,"url":"https://github.com/bob/y","isFork":true,"stargazerCount":50,"forkCount":2,"primaryLanguage":null,"licenseInfo":null,"issues":{"totalCount":0},"pullRequests":{"totalCount":0}}}
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
	first := r.List[0]
	if first.NameWithOwner != "alice/x" {
		t.Errorf("first = %s, want alice/x", first.NameWithOwner)
	}
	// #469: language / license / fork / issue / PR metadata must now be
	// surfaced from the extended GraphQL query.
	if first.Language == nil || first.Language.Name != "Go" || first.Language.Color != "#00ADD8" {
		t.Errorf("first.Language = %+v, want Go/#00ADD8", first.Language)
	}
	if first.License != "MIT" {
		t.Errorf("first.License = %q, want MIT (spdxId)", first.License)
	}
	if first.Forks != 1 || first.Issues != 3 || first.PullRequests != 7 {
		t.Errorf("first counts = forks %d / issues %d / prs %d, want 1/3/7",
			first.Forks, first.Issues, first.PullRequests)
	}
	second := r.List[1]
	if !second.IsFork {
		t.Errorf("second.IsFork = false, want true")
	}
	if second.Language != nil {
		t.Errorf("second.Language = %+v, want nil", second.Language)
	}
	if second.License != "" {
		t.Errorf("second.License = %q, want empty", second.License)
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

// TestRun_LicenseNoAssertion verifies the upstream `format.license`
// fallback: a "NOASSERTION" spdxId renders the full license name.
func TestRun_LicenseNoAssertion(t *testing.T) {
	t.Parallel()
	mux := &graphqlMux{body: `{"data":{"user":{"starredRepositories":{"totalCount":1,"edges":[
		{"starredAt":"2026-05-10T00:00:00Z","node":{"nameWithOwner":"alice/x","description":"hi","url":"https://github.com/alice/x","isFork":false,"stargazerCount":1,"forkCount":0,"primaryLanguage":null,"licenseInfo":{"name":"Other","spdxId":"NOASSERTION"},"issues":{"totalCount":0},"pullRequests":{"totalCount":0}}}
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
	if got := r.List[0].License; got != "Other" {
		t.Errorf("License = %q, want Other (NOASSERTION falls back to name)", got)
	}
}

// TestPartial_RendersRepoMetadata pins that the per-repo info row now
// surfaces language, license, fork, issue and PR counts (#469).
func TestPartial_RendersRepoMetadata(t *testing.T) {
	t.Parallel()
	r := &stars.Result{
		List: []stars.StarredRepo{
			{
				NameWithOwner: "gin-gonic/gin",
				Description:   "Gin is a web framework",
				URL:           "https://github.com/gin-gonic/gin",
				IsFork:        false,
				Stars:         88500,
				Forks:         8620,
				Issues:        2390,
				PullRequests:  2190,
				Language:      &stars.Language{Name: "Go", Color: "#00ADD8"},
				License:       "MIT",
				StarredAt:     time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			},
		},
		Limit: 4,
	}
	data := plugins.NewData()
	data.SetPlugin(stars.Name, r)
	got, err := stars.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		`class="language"`,
		`fill="#00ADD8"`,
		`>Go<`,
		`>MIT<`,
		`>88.5k<`, // FormatCount(88500)
		`>8.6k<`,  // forks
		`>2.4k<`,  // issues
		`>2.2k<`,  // pull requests
		`data-forks="8620"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("partial output missing %q\n---\n%s", want, got)
		}
	}
}

func TestPartial_RendersRelativeStarredDates(t *testing.T) {
	restore := stars.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	})
	defer restore()

	r := &stars.Result{
		List: []stars.StarredRepo{
			{NameWithOwner: "alice/hourly", URL: "https://github.com/alice/hourly", StarredAt: time.Date(2026, 5, 10, 10, 30, 0, 0, time.UTC)},
			{NameWithOwner: "alice/daily", URL: "https://github.com/alice/daily", StarredAt: time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)},
			{NameWithOwner: "alice/old", URL: "https://github.com/alice/old", StarredAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(stars.Name, r)
	got, err := stars.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		"starred 2 hours ago",
		"starred 3 days ago",
		"starred Mar 01 2026",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

// TestPartial_RelativeStarredDateBoundaries pins the exact branch
// boundaries of formatStarredAt (rendered through the partial via the
// clock seam): the <1 day / <30 day / absolute-date cutoffs and the
// negative-duration clamp for future timestamps.
func TestPartial_RelativeStarredDateBoundaries(t *testing.T) {
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	restore := stars.SetNowForTest(func() time.Time { return now })
	defer restore()

	r := &stars.Result{
		List: []stars.StarredRepo{
			// Exactly now-24h: crosses into the <30 day branch as "1 day ago"
			// (singular, since days == 1).
			{NameWithOwner: "alice/oneday", URL: "https://github.com/alice/oneday", StarredAt: now.Add(-24 * time.Hour)},
			// Exactly now-30*24h: hits the default absolute-date branch.
			{NameWithOwner: "alice/thirty", URL: "https://github.com/alice/thirty", StarredAt: now.Add(-30 * 24 * time.Hour)},
			// Future timestamp (now+1h): negative duration clamps to zero
			// and must read "0 hours ago" (plural), not "0 hour ago".
			{NameWithOwner: "alice/future", URL: "https://github.com/alice/future", StarredAt: now.Add(1 * time.Hour)},
		},
	}
	data := plugins.NewData()
	data.SetPlugin(stars.Name, r)
	got, err := stars.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		"starred 1 day ago",
		"starred Apr 10 2026",
		"starred 0 hours ago",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "0 hour ago") {
		t.Fatalf("clamped future timestamp must read '0 hours ago' (plural), got %s", got)
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
