package activity_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	"github.com/mjun0812/github-metrics/internal/templates"
)

// restMux is a tiny HTTP transport that returns canned responses keyed
// by URL path prefix. Tests register `(status, body)` per prefix and
// the mux serves the first matching entry.
type restMux struct {
	mu     sync.Mutex
	calls  int
	resp   map[string]restResp
	byPath map[string]int
}

type restResp struct {
	status int
	body   string
	err    error
	header http.Header
}

func newRESTMux() *restMux {
	return &restMux{resp: map[string]restResp{}, byPath: map[string]int{}}
}

func (m *restMux) on(path string, status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resp[path] = restResp{status: status, body: body}
}

// onWithHeader registers a response carrying custom headers — used to
// inject Retry-After so httpx.ClassifyRateLimit produces a
// *httpx.RateLimitedError.
func (m *restMux) onWithHeader(path string, status int, body string, header http.Header) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resp[path] = restResp{status: status, body: body, header: header}
}

// pathCalls is a per-path call counter used to assert dedup / fetch
// budget bounds.
func (m *restMux) pathCalls(prefix string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byPath[prefix]
}

func (m *restMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	var hit *restResp
	var hitPrefix string
	for prefix, r := range m.resp {
		if strings.HasPrefix(req.URL.RequestURI(), prefix) {
			rr := r
			hit = &rr
			hitPrefix = prefix
			break
		}
	}
	if hit != nil {
		m.byPath[hitPrefix]++
	}
	m.mu.Unlock()
	if hit == nil {
		return jsonResp(req, http.StatusNotFound, `{"message":"no fixture"}`), nil
	}
	if hit.err != nil {
		return nil, hit.err
	}
	resp := jsonResp(req, hit.status, hit.body)
	for k, vs := range hit.header {
		for _, v := range vs {
			resp.Header.Add(k, v)
		}
	}
	return resp, nil
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

// prEv renders a PullRequestEvent JSON entry carrying the diff-stat
// payload (additions/deletions/changed_files) GitHub embeds under
// payload.pull_request.
func prEv(repo string, when time.Time, public bool, additions, deletions, changed int) string {
	return `{"type":"PullRequestEvent","repo":{"name":"` + repo +
		`"},"created_at":"` + when.UTC().Format(time.RFC3339) +
		`","public":` + boolStr(public) +
		`,"payload":{"pull_request":{"number":1,"additions":` + strconv.Itoa(additions) +
		`,"deletions":` + strconv.Itoa(deletions) +
		`,"changed_files":` + strconv.Itoa(changed) + `}}}`
}

// prEvSummaryOnly renders a PullRequestEvent JSON entry shaped like the
// real events API payload: the embedded pull_request carries only the
// summary fields (number/url/...) and NO additions/deletions/
// changed_files. This is what GitHub actually returns from
// `/users/{login}/events` — see the curl-driven discovery in #553.
func prEvSummaryOnly(repo string, when time.Time, public bool, number int) string {
	return `{"type":"PullRequestEvent","repo":{"name":"` + repo +
		`"},"created_at":"` + when.UTC().Format(time.RFC3339) +
		`","public":` + boolStr(public) +
		`,"payload":{"pull_request":{"number":` + strconv.Itoa(number) +
		`,"url":"https://api.github.com/repos/` + repo + `/pulls/` + strconv.Itoa(number) + `"}}}`
}

// prDetail renders the `/repos/{owner}/{name}/pulls/{number}` JSON body
// shape this plugin consumes.
func prDetail(additions, deletions, changed int) string {
	return `{"additions":` + strconv.Itoa(additions) +
		`,"deletions":` + strconv.Itoa(deletions) +
		`,"changed_files":` + strconv.Itoa(changed) + `}`
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

// TestRun_DefaultLimit asserts the default display limit is 5 (matching
// upstream metadata) so the timeline stays short when no
// plugin_activity_limit is supplied.
func TestRun_DefaultLimit(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	evs := make([]string, 0, 7)
	for i := range 7 {
		evs = append(evs, ev("PushEvent", "octocat/a", now.Add(-time.Duration(i+1)*time.Hour), true))
	}
	mux.on("/users/octocat/events", http.StatusOK, eventsBody(evs...))
	pc := newPC(t, mux, nil) // no plugin_activity_limit → default
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if len(r.Events) != 5 {
		t.Errorf("default Events len = %d, want 5", len(r.Events))
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

// TestRun_PullRequestStats asserts a PullRequestEvent surfaces the diff
// stats (files changed / additions / deletions) from
// payload.pull_request, while non-PR events leave Files/Lines nil.
func TestRun_PullRequestStats(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(
			prEv("octocat/beta", now.Add(-1*time.Hour), true, 34, 5, 2),
			ev("PushEvent", "octocat/alpha", now.Add(-2*time.Hour), true),
		),
	)
	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 2 {
		t.Fatalf("Events len = %d, want 2", len(r.Events))
	}

	var pr, push *activity.ActivityEvent
	for i := range r.Events {
		switch r.Events[i].Type {
		case "PullRequestEvent":
			pr = &r.Events[i]
		case "PushEvent":
			push = &r.Events[i]
		}
	}
	if pr == nil || push == nil {
		t.Fatalf("missing expected events: %+v", r.Events)
	}
	if pr.Files == nil || pr.Lines == nil {
		t.Fatalf("PR event missing diff stats: %+v", pr)
	}
	if pr.Files.Changed != 2 {
		t.Errorf("Files.Changed = %d, want 2", pr.Files.Changed)
	}
	if pr.Lines.Added != 34 || pr.Lines.Deleted != 5 {
		t.Errorf("Lines = %+v, want {Added:34 Deleted:5}", pr.Lines)
	}
	if push.Files != nil || push.Lines != nil {
		t.Errorf("non-PR event should have nil Files/Lines; got Files=%+v Lines=%+v", push.Files, push.Lines)
	}
}

// TestRun_PullRequestStats_FallbackToPRDetail asserts the plugin
// fetches PR diff stats from `/repos/{owner}/{repo}/pulls/{number}`
// when the events payload only carries the PR summary (number/url),
// which is what the real `/users/{login}/events` API returns. Without
// this fallback the rendered card shows "0 files changed ++0 --0",
// the bug pinned by issue #553.
func TestRun_PullRequestStats_FallbackToPRDetail(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(prEvSummaryOnly("octocat/beta", now.Add(-1*time.Hour), true, 42)),
	)
	mux.on("/repos/octocat/beta/pulls/42", http.StatusOK, prDetail(34, 5, 2))
	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Fatalf("Events len = %d, want 1", len(r.Events))
	}
	pr := r.Events[0]
	if pr.Files == nil || pr.Lines == nil {
		t.Fatalf("PR event missing diff stats after fallback: %+v", pr)
	}
	if pr.Files.Changed != 2 {
		t.Errorf("Files.Changed = %d, want 2", pr.Files.Changed)
	}
	if pr.Lines.Added != 34 || pr.Lines.Deleted != 5 {
		t.Errorf("Lines = %+v, want {Added:34 Deleted:5}", pr.Lines)
	}
}

// TestRun_PullRequestStats_FallbackFailsGracefully asserts a failing PR
// detail fetch (4xx, 5xx, transport error) does not break the whole
// plugin — the PR event is still surfaced, but with Files/Lines left
// nil so the partial skips the "N files changed" line entirely.
// Rendering "0 files changed ++0 --0" is the original #553 bug; keeping
// the stats nil is the only safe degradation path.
func TestRun_PullRequestStats_FallbackFailsGracefully(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(prEvSummaryOnly("octocat/beta", now.Add(-1*time.Hour), true, 42)),
	)
	mux.on("/repos/octocat/beta/pulls/42", http.StatusNotFound, `{"message":"not found"}`)
	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Fatalf("Events len = %d, want 1", len(r.Events))
	}
	// Fallback failed → Files/Lines stay nil so the partial does not
	// render the misleading "0 files changed ++0 --0" details line.
	pr := r.Events[0]
	if pr.Files != nil || pr.Lines != nil {
		t.Errorf("failed fallback should leave Files/Lines nil; got Files=%+v Lines=%+v", pr.Files, pr.Lines)
	}
}

// TestPartial_PullRequestStats asserts the rendered partial includes the
// upstream "N files changed ++A --D" details line for a PR event.
func TestPartial_PullRequestStats(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(activity.Name, &activity.Result{
		Events: []activity.ActivityEvent{
			{
				Type:       "PullRequestEvent",
				Repo:       "octocat/beta",
				Date:       time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
				Visibility: "public",
				Files:      &activity.EventFiles{Changed: 2},
				Lines:      &activity.EventLines{Added: 34, Deleted: 5},
			},
		},
		Days: 14,
	})
	got, _, err := activity.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	for _, want := range []string{
		`<g class="code">`,
		`2 files changed`,
		`++34 --5`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("partial missing %q in:\n%s", want, got)
		}
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

// TestRun_VisibilityDefaultIncludesPrivate pins the absent-input
// default to "all" (assets/plugins/activity/metadata.yml): private
// events the token can see must be surfaced without an explicit
// plugin_activity_visibility input. Regression guard for the
// hardcoded "public" default that hid every private event (found via
// the #465 DOM-contract failure on the regenerated doc samples).
func TestRun_VisibilityDefaultIncludesPrivate(t *testing.T) {
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
	pc := newPC(t, mux, nil)
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if len(r.Events) != 2 {
		t.Fatalf("Events len = %d, want 2 (public + private); %+v", len(r.Events), r.Events)
	}
}

// TestRun_SkipPrivateForcesPublicVisibility asserts the cross-plugin
// repositories_skip_private input (#656) hides private events even
// though the default visibility is "all".
func TestRun_SkipPrivateForcesPublicVisibility(t *testing.T) {
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
		"repositories_skip_private": "yes",
	})
	out, _ := activity.Plugin.Run(context.Background(), pc)
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Fatalf("Events len = %d, want 1 (public only); %+v", len(r.Events), r.Events)
	}
	if r.Events[0].Repo != "octocat/pub" {
		t.Errorf("expected public repo only; got %s", r.Events[0].Repo)
	}
}

// TestRun_PullRequestStats_DedupAcrossEvents asserts the /pulls/{n}
// fallback is called exactly once per (repo, number) even when the
// same PR surfaces through several PullRequestEvent rows (opened /
// synchronize / closed). Without dedup the activity card would burn
// 2-4x the REST budget on identical fetches.
func TestRun_PullRequestStats_DedupAcrossEvents(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(
			prEvSummaryOnly("octocat/beta", now.Add(-1*time.Hour), true, 42),
			prEvSummaryOnly("octocat/beta", now.Add(-2*time.Hour), true, 42),
			prEvSummaryOnly("octocat/beta", now.Add(-3*time.Hour), true, 42),
		),
	)
	mux.on("/repos/octocat/beta/pulls/42", http.StatusOK, prDetail(34, 5, 2))
	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 3 {
		t.Fatalf("Events len = %d, want 3", len(r.Events))
	}
	for i, e := range r.Events {
		if e.Files == nil || e.Lines == nil {
			t.Errorf("event[%d] missing stats: %+v", i, e)
			continue
		}
		if e.Files.Changed != 2 || e.Lines.Added != 34 || e.Lines.Deleted != 5 {
			t.Errorf("event[%d] wrong stats: %+v", i, e)
		}
	}
	if n := mux.pathCalls("/repos/octocat/beta/pulls/42"); n != 1 {
		t.Errorf("PR detail fetched %d times for PR #42, want 1 (dedup)", n)
	}
}

// TestRun_PullRequestStats_LimitCapsFetchBudget asserts the fallback
// fetch is bounded by the display `limit`, not the raw `load` count.
// Without the fix the worst case would burn 300 REST calls per run on
// events that get truncated away.
func TestRun_PullRequestStats_LimitCapsFetchBudget(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	// 10 distinct PR events, all eligible — limit=3 must keep the
	// fallback fetch at 3 calls, not 10.
	evs := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		evs = append(evs,
			prEvSummaryOnly("octocat/beta", now.Add(-time.Duration(i+1)*time.Hour), true, 100+i))
	}
	mux.on("/users/octocat/events", http.StatusOK, eventsBody(evs...))
	// Register a detail fixture for every PR so an over-fetch surfaces
	// as a higher pathCalls total, not as 404s.
	for i := 0; i < 10; i++ {
		mux.on(fmt.Sprintf("/repos/octocat/beta/pulls/%d", 100+i),
			http.StatusOK, prDetail(1, 1, 1))
	}
	pc := newPC(t, mux, map[string]any{"plugin_activity_limit": 3})
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 3 {
		t.Fatalf("Events len = %d, want 3", len(r.Events))
	}
	// Count total /pulls/ fetches across all PR numbers.
	var total int
	for i := 0; i < 10; i++ {
		total += mux.pathCalls(fmt.Sprintf("/repos/octocat/beta/pulls/%d", 100+i))
	}
	if total != 3 {
		t.Errorf("PR detail fetched %d times total, want 3 (one per kept event)", total)
	}
}

// TestRun_PullRequestStats_RateLimitedSurfacesAsAppendError asserts a
// rate-limited /pulls/{n} fetch records a single aggregated
// AppendError on Data so the #531 surface-degradation contract holds.
// The PR event itself stays surfaced with Files/Lines nil so the
// partial skips the stats line.
func TestRun_PullRequestStats_RateLimitedSurfacesAsAppendError(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mux := newRESTMux()
	mux.on(
		"/users/octocat/events",
		http.StatusOK,
		eventsBody(prEvSummaryOnly("octocat/beta", now.Add(-1*time.Hour), true, 42)),
	)
	// 403 + Retry-After + X-RateLimit-* → httpx.ClassifyRateLimit (and
	// our fetcher's direct ClassifyRateLimit call) returns a
	// *httpx.RateLimitedError on the non-2xx branch.
	h := http.Header{}
	h.Set("Retry-After", "1")
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	mux.onWithHeader("/repos/octocat/beta/pulls/42",
		http.StatusForbidden, `{"message":"rate limited"}`, h)

	pc := newPC(t, mux, nil)
	out, err := activity.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*activity.Result)
	if len(r.Events) != 1 {
		t.Fatalf("Events len = %d, want 1", len(r.Events))
	}
	if r.Events[0].Files != nil || r.Events[0].Lines != nil {
		t.Errorf("rate-limited fetch should leave Files/Lines nil; got %+v", r.Events[0])
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 {
		t.Fatalf("SnapshotErrors len = %d, want 1; errors: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "rate limit") {
		t.Errorf("error message should mention rate limit; got %q", errs[0].Error())
	}
}
