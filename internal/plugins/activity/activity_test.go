package activity_test

import (
	"context"
	"errors"
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
	"github.com/mjun0812/github-metrics/internal/plugins/activity"
)

// restMux is a tiny HTTP transport that returns canned responses keyed
// by URL path prefix. Tests register `(status, body)` per prefix and
// the mux serves the first matching entry.
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

func (m *restMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	var hit *restResp
	for prefix, r := range m.resp {
		if strings.HasPrefix(req.URL.RequestURI(), prefix) {
			rr := r
			hit = &rr
			break
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
		Inputs: map[string]any{"user": "octocat"},
		Data:   data,
		REST:   newREST(t, mux),
	}
	for k, v := range inputs {
		pc.Inputs[k] = v
	}
	return pc
}

// eventsBody renders a JSON body for /users/octocat/events with the
// supplied (type, repo, created_at, public) tuples.
func eventsBody(events ...string) string {
	if len(events) == 0 {
		return "[]"
	}
	return "[" + strings.Join(events, ",") + "]"
}

func ev(typ, repo string, when time.Time, public bool) string {
	return `{"type":"` + typ + `","repo":{"name":"` + repo +
		`"},"created_at":"` + when.UTC().Format(time.RFC3339) + `","public":` + boolStr(public) + `}`
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestRun_Normal — single page of mixed events, default inputs.
func TestRun_Normal(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(
			ev("PushEvent", "octocat/alpha", now.Add(-1*time.Hour), true),
			ev("PullRequestEvent", "octocat/beta", now.Add(-3*time.Hour), true),
		),
	)
	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := out.(*activity.Result)
	if !ok {
		t.Fatalf("Run returned %T", out)
	}
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if len(r.Events) != 2 {
		t.Fatalf("Events len = %d, want 2", len(r.Events))
	}
	// Date descending
	if r.Events[0].Date.Before(r.Events[1].Date) {
		t.Errorf("events not Date-descending: %+v", r.Events)
	}
}

// TestRun_Limit asserts the limit input caps the number of events kept.
func TestRun_Limit(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(
			ev("PushEvent", "octocat/a", now.Add(-1*time.Hour), true),
			ev("PushEvent", "octocat/b", now.Add(-2*time.Hour), true),
			ev("PushEvent", "octocat/c", now.Add(-3*time.Hour), true),
		),
	)
	pc := newPC(t, mux, map[string]any{
		"plugin_activity_limit": 1,
	})
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Errorf("Events len = %d, want 1", len(r.Events))
	}
}

// TestRun_EmptyEvents returns an empty array and asserts the plugin
// still returns Skipped=false with an empty Events slice (contract §2.5).
func TestRun_EmptyEvents(t *testing.T) {
	t.Parallel()
	mux := newRESTMux()
	mux.on("/users/octocat/events", http.StatusOK, "[]")
	pc := newPC(t, mux, nil)
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if r.Skipped {
		t.Errorf("Skipped = true, want false (empty != skipped)")
	}
	if len(r.Events) != 0 {
		t.Errorf("Events = %v, want []", r.Events)
	}
}

// TestRun_5xxRetryable asserts a transient 500 returns a
// *RetryableError so the engine can record it on Result.Errors.
func TestRun_5xxRetryable(t *testing.T) {
	t.Parallel()
	mux := newRESTMux()
	mux.on("/users/octocat/events", http.StatusInternalServerError, `{"message":"boom"}`)
	pc := newPC(t, mux, nil)
	_, err := activity.Plugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("error type = %T, want *RetryableError; err=%v", err, err)
	}
}

// TestRun_4xxNotRetryable asserts a permanent 4xx (e.g. 404 login does
// not exist) returns a regular error, NOT a *RetryableError. Retrying
// 4xx would never succeed and breaks the contract that *RetryableError
// is reserved for transient transport failures.
func TestRun_4xxNotRetryable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
	}{
		{name: "404_not_found", status: http.StatusNotFound},
		{name: "403_forbidden", status: http.StatusForbidden},
		{name: "401_unauthorized", status: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newRESTMux()
			mux.on("/users/octocat/events", tc.status, `{"message":"nope"}`)
			pc := newPC(t, mux, nil)
			_, err := activity.Plugin.Run(context.Background(), pc)
			if err == nil {
				t.Fatalf("expected error for %d", tc.status)
			}
			var re *xerrors.RetryableError
			if errors.As(err, &re) {
				t.Errorf("%d wrapped as *RetryableError; should be permanent. err=%v", tc.status, err)
			}
		})
	}
}

// TestRun_VisibilityFilter asserts visibility="private" drops public
// events.
func TestRun_VisibilityFilter(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(
			ev("PushEvent", "octocat/pub", now.Add(-1*time.Hour), true),
			ev("PushEvent", "octocat/priv", now.Add(-2*time.Hour), false),
		),
	)
	pc := newPC(t, mux, map[string]any{
		"plugin_activity_visibility": "private",
	})
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Fatalf("Events len = %d, want 1; %+v", len(r.Events), r.Events)
	}
	if r.Events[0].Repo != "octocat/priv" {
		t.Errorf("expected private repo only; got %s", r.Events[0].Repo)
	}
}
