//go:build heavy

package languages_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
)

// restMux is a tiny HTTP transport that returns canned responses keyed
// by URL path prefix.
type restMux struct {
	mu    sync.Mutex
	calls int
	resp  map[string]restResp
}

type restResp struct {
	status int
	body   string
	err    error
}

func newRESTMux() *restMux {
	return &restMux{resp: map[string]restResp{}}
}

func (m *restMux) on(path string, status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resp[path] = restResp{status: status, body: body}
}

func (m *restMux) onErr(path string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resp[path] = restResp{err: err}
}

func (m *restMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	var hit *restResp
	var bestLen int
	for prefix, r := range m.resp {
		if strings.HasPrefix(req.URL.RequestURI(), prefix) && len(prefix) > bestLen {
			rr := r
			hit = &rr
			bestLen = len(prefix)
		}
	}
	m.mu.Unlock()
	if hit == nil {
		return jsonResp(req, http.StatusNotFound, `{"message":"no fixture"}`), nil
	}
	if hit.err != nil {
		return nil, hit.err
	}
	return jsonResp(req, hit.status, hit.body), nil
}

func jsonResp(req *http.Request, status int, body string) *http.Response {
	if body == "" {
		body = "{}"
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
	}
}

func newREST(t *testing.T, mux http.RoundTripper) *githubapi.REST {
	t.Helper()
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	return rest
}

func newPC(t *testing.T, mux http.RoundTripper, inputs map[string]any) *plugins.PluginContext {
	t.Helper()
	data := plugins.NewData()
	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"user":                      "octocat",
			"plugin_languages":          true,
			"plugin_languages_sections": "most-used,recently-used",
		},
		Data: data,
		REST: newREST(t, mux),
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

// pushEvent renders a single PushEvent body with the supplied commit
// SHAs at the given timestamp.
func pushEvent(repo string, when time.Time, shas ...string) string {
	commits := make([]string, 0, len(shas))
	for _, s := range shas {
		commits = append(commits, fmt.Sprintf(`{"sha":%q}`, s))
	}
	return fmt.Sprintf(
		`{"type":"PushEvent","repo":{"name":%q},"created_at":%q,"payload":{"commits":[%s]}}`,
		repo, when.UTC().Format(time.RFC3339), strings.Join(commits, ","),
	)
}

func commitBody(files ...string) string {
	return `{"files":[` + strings.Join(files, ",") + `]}`
}

func file(name string, additions, deletions int) string {
	return fmt.Sprintf(`{"filename":%q,"additions":%d,"deletions":%d}`,
		name, additions, deletions)
}

// TestRecentRun_Normal — 3 commits with Go + JavaScript files.
func TestRecentRun_Normal(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		"["+strings.Join([]string{
			pushEvent("octocat/alpha", now.Add(-1*time.Hour), "sha1", "sha2"),
			pushEvent("octocat/beta", now.Add(-2*time.Hour), "sha3"),
		}, ",")+"]",
	)
	mux.on("/repos/octocat/alpha/commits/sha1", http.StatusOK,
		commitBody(file("main.go", 100, 0), file("README.md", 5, 0)))
	mux.on("/repos/octocat/alpha/commits/sha2", http.StatusOK,
		commitBody(file("api.go", 50, 10)))
	mux.on("/repos/octocat/beta/commits/sha3", http.StatusOK,
		commitBody(file("script.js", 80, 0)))

	pc := newPC(t, mux, nil)
	out, err := languages.RecentPlugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := out.(*languages.RecentResult)
	if !ok {
		t.Fatalf("Run returned %T", out)
	}
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %s", r.SkippedReason)
	}
	if len(r.Favorites) == 0 {
		t.Fatalf("Favorites empty; got %+v", r)
	}
	names := map[string]int{}
	for _, f := range r.Favorites {
		names[f.Name] = f.Size
	}
	if names["Go"] == 0 {
		t.Errorf("expected Go bytes > 0; favorites=%+v", r.Favorites)
	}
	if names["JavaScript"] == 0 {
		t.Errorf("expected JavaScript bytes > 0; favorites=%+v", r.Favorites)
	}
	// Repos collected.
	if len(r.Repos) != 2 {
		t.Errorf("Repos = %v, want 2", r.Repos)
	}
}

// TestRecentRun_LinguistDisabled — extras toggle skips the plugin entirely.
func TestRecentRun_LinguistDisabled(t *testing.T) {
	t.Parallel()
	mux := newRESTMux()
	pc := newPC(t, mux, map[string]any{
		"extras.metrics.run.linguist": false,
	})
	out, _ := languages.RecentPlugin.Run(context.Background(), pc)
	r := out.(*languages.RecentResult)
	if !r.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if r.SkippedReason != "linguist disabled via extras" {
		t.Errorf("SkippedReason = %q", r.SkippedReason)
	}
	if mux.calls > 0 {
		t.Errorf("REST called %d times despite skip", mux.calls)
	}
}

// TestRecentRun_NoPushEvents — empty events returns Skipped=false with empty
// Favorites (contract §1.6).
func TestRecentRun_NoPushEvents(t *testing.T) {
	t.Parallel()
	mux := newRESTMux()
	mux.on("/users/octocat/events", http.StatusOK, "[]")
	pc := newPC(t, mux, nil)
	out, err := languages.RecentPlugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*languages.RecentResult)
	if r.Skipped {
		t.Errorf("Skipped = true, want false (empty != skipped)")
	}
	if len(r.Favorites) != 0 {
		t.Errorf("Favorites = %v, want []", r.Favorites)
	}
}

// TestRecentRun_DaysFilter — events older than _days are dropped.
func TestRecentRun_DaysFilter(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		"["+strings.Join([]string{
			// In window (1 day)
			pushEvent("octocat/recent", now.Add(-12*time.Hour), "shaR"),
			// Out of window (3 days ago, _days=1)
			pushEvent("octocat/old", now.Add(-72*time.Hour), "shaO"),
		}, ",")+"]",
	)
	mux.on("/repos/octocat/recent/commits/shaR", http.StatusOK,
		commitBody(file("recent.go", 30, 0)))
	mux.on("/repos/octocat/old/commits/shaO", http.StatusOK,
		commitBody(file("old.go", 99, 0)))

	pc := newPC(t, mux, map[string]any{
		"plugin_languages_recent_days": 1,
	})
	out, _ := languages.RecentPlugin.Run(context.Background(), pc)
	r := out.(*languages.RecentResult)
	if len(r.Repos) != 1 || r.Repos[0] != "octocat/recent" {
		t.Errorf("Repos = %v, want [octocat/recent]", r.Repos)
	}
	// The "old" commit must not appear in totals.
	for _, f := range r.Favorites {
		if f.Size > 30 {
			t.Errorf("Size %d for %s indicates old commit leaked", f.Size, f.Name)
		}
	}
}

// TestRecentRun_CategoryFilter — non-programming languages are dropped when
// categories="programming".
func TestRecentRun_CategoryFilter(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		pushEventsArray(pushEvent("octocat/mix", now.Add(-1*time.Hour), "sha1")),
	)
	mux.on("/repos/octocat/mix/commits/sha1", http.StatusOK,
		commitBody(
			file("main.go", 100, 0),
			file("README.md", 80, 0), // markup, filtered
		))
	pc := newPC(t, mux, map[string]any{
		"plugin_languages_recent_categories": "programming",
	})
	out, _ := languages.RecentPlugin.Run(context.Background(), pc)
	r := out.(*languages.RecentResult)
	for _, f := range r.Favorites {
		if f.Name == "Markdown" {
			t.Errorf("Markdown leaked despite programming-only filter: %+v", f)
		}
	}
	if len(r.Favorites) == 0 || r.Favorites[0].Name != "Go" {
		t.Errorf("Favorites[0] = %+v, want Go", r.Favorites)
	}
}

// TestRecentRun_5xxRetryable — events 500 → *RetryableError.
func TestRecentRun_5xxRetryable(t *testing.T) {
	t.Parallel()
	mux := newRESTMux()
	mux.on("/users/octocat/events", http.StatusInternalServerError, `{"message":"boom"}`)
	pc := newPC(t, mux, nil)
	_, err := languages.RecentPlugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("error type = %T, want *RetryableError; err=%v", err, err)
	}
}

func pushEventsArray(events ...string) string {
	return "[" + strings.Join(events, ",") + "]"
}
