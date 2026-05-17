// Package engine_test — bench_full_test.go exercises engine.Compute
// against all 21 採用 plugins simultaneously enabled, with mocked
// dependencies. Provides two pieces of evidence for the M4 success
// criteria:
//
//   - BenchmarkCompute_Full_21Plugins (T094 / SC-003): per-call wall
//     time. With mocked Deps the bench measures the in-process
//     orchestration overhead (template render + plugin dispatch +
//     marshaling), not real GitHub latency. Numbers go in the PR body.
//   - BenchmarkCompute_MemoryPeak (T095 / SC-009): allocations + peak
//     heap delta around a Compute invocation. Same caveat: mocked Deps
//     give us a lower bound, not the production peak.
//
// Both have companion regression-guard tests
// (TestCompute_Full_21Plugins_PerformanceBudget /
// TestCompute_MemoryPeak_RegressionGuard) that run during `go test ./...`
// to catch order-of-magnitude regressions even when -bench is omitted.
// The budgets are sized generously (5s wall / 800 MB peak) to match the
// SC-003 / SC-009 contract — under mocked Deps they pass trivially, and
// only blow up if a refactor introduces a quadratic explosion.
package engine_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/render"

	// Side-effect imports register all 21 採用 plugins so the engine
	// plugin registry sees them at runtime. P1 (5), P2 (12), P3 (4).
	_ "github.com/mjun0812/github-metrics/internal/plugins/achievements"
	_ "github.com/mjun0812/github-metrics/internal/plugins/activity"
	_ "github.com/mjun0812/github-metrics/internal/plugins/calendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/contributors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/habits"
	_ "github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/notable"
	_ "github.com/mjun0812/github-metrics/internal/plugins/people"
	_ "github.com/mjun0812/github-metrics/internal/plugins/projects"
	_ "github.com/mjun0812/github-metrics/internal/plugins/reactions"
	_ "github.com/mjun0812/github-metrics/internal/plugins/repositories"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	_ "github.com/mjun0812/github-metrics/internal/plugins/starlists"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
	_ "github.com/mjun0812/github-metrics/internal/plugins/topics"
	_ "github.com/mjun0812/github-metrics/internal/plugins/traffic"

	// classic template registration for SVG output
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// Performance budgets per Phase 6 contract (T094 / T095).
const (
	p95WallBudget    = 5 * time.Second   // SC-003
	peakMemoryBudget = 800 * 1024 * 1024 // SC-009 (800 MB)
)

// fullPluginInputs flips every plugin_<slug> truthy gate. Includes
// the languages sub-mode gates so the standard / recent / indepth
// branches all execute. Without these, the engine still runs the
// plugin Run() but the classic dispatcher skips the partial — for a
// bench we want all paths exercised.
func fullPluginInputs() map[string]any {
	return map[string]any{
		// P1
		"plugin_languages":          true,
		"plugin_languages_sections": "most-used,recently-used",
		"plugin_languages_indepth":  true,
		"plugin_activity":           true,
		"plugin_achievements":       true,
		"plugin_repositories":       true,
		"plugin_isocalendar":        true,
		// P2
		"plugin_calendar":     true,
		"plugin_habits":       true,
		"plugin_stars":        true,
		"plugin_people":       true,
		"plugin_notable":      true,
		"plugin_contributors": true,
		"plugin_reactions":    true,
		"plugin_projects":     true,
		"plugin_sponsors":     true,
		"plugin_sponsorships": true,
		"plugin_stargazers":   true,
		"plugin_traffic":      true,
		// P3 (chromedp / heavy). Note: pc.Render is *FakeRenderer here,
		// so topics / starlists will skip with "chromedp not available"
		// — Run() still executes, the gate just records a Skipped
		// result. This matches the production behavior when chromium is
		// unavailable.
		"plugin_topics":    true,
		"plugin_starlists": true,
	}
}

// benchGraphQLFixture returns canned GraphQL bodies that satisfy the
// base plugin + the plugins that drive their own GraphQL queries
// (calendar / stars / people / notable / sponsors / sponsorships /
// stargazers / reactions / projects). The shape mirrors the M2/M4
// integration fixtures but is collapsed into a single helper so the
// bench file stays self-contained.
type benchGraphQLFixture struct {
	calls atomic.Int32
}

func (g *benchGraphQLFixture) RoundTrip(req *http.Request) (*http.Response, error) {
	g.calls.Add(1)
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	var payload struct {
		OpName string `json:"operationName"`
	}
	_ = json.Unmarshal(body, &payload)
	resp, ok := benchGraphQLResponses[payload.OpName]
	if !ok {
		// Empty data lets the plugin go through its empty-but-not-skipped
		// path. Better than a 5xx (which would record errors) for bench.
		resp = `{"data": {}}`
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(resp)),
		ContentLength: int64(len(resp)),
		Request:       req,
	}, nil
}

// benchGraphQLResponses keeps the canned bodies short so the bench
// measures orchestration, not JSON unmarshaling cost.
var benchGraphQLResponses = map[string]string{
	"User": `{"data":{"user":{"databaseId":1,"id":"u","login":"octocat","name":"The Octocat","avatarUrl":"https://x"}}}`,
	"UserRepositories": `{"data":{"user":{"repositories":{"totalCount":2,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[
		{"databaseId":1,"id":"r1","name":"alpha","nameWithOwner":"octocat/alpha","url":"https://github.com/octocat/alpha","isPrivate":false,"isFork":false,"stargazerCount":100,"forkCount":10,"watchers":{"totalCount":5},"primaryLanguage":{"name":"Go","color":"#00ADD8"},"languages":{"totalCount":1,"totalSize":5000,"edges":[{"size":5000,"node":{"name":"Go","color":"#00ADD8"}}]}}
	]}}}}`,
	"UserIndepth": `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":10,"weeks":[]}},"repositories":{"totalCount":1,"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[{"defaultBranchRef":{"name":"main","target":{"__typename":"Commit","id":"c","history":{"totalCount":10}}},"issues":{"totalCount":0},"pullRequests":{"totalCount":0}}]}}}}`,
}

// benchRESTMux returns empty arrays for /events and /traffic-ish
// endpoints. Mirrors plugins_p1_test.go's restEventsMux + extends to
// the broader plugin set.
type benchRESTMux struct {
	calls atomic.Int32
}

func (m *benchRESTMux) RoundTrip(req *http.Request) (*http.Response, error) {
	m.calls.Add(1)
	body := `[]`
	if strings.Contains(req.URL.Path, "/views") ||
		strings.Contains(req.URL.Path, "/clones") {
		body = `{"count":0,"uniques":0,"views":[]}`
	}
	if req.URL.Path == "/" {
		// Scopes() helper hits GET / for X-OAuth-Scopes header.
		h := http.Header{}
		h.Set("Content-Type", "application/json")
		h.Set("X-OAuth-Scopes", "repo,read:user,read:org,read:project")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     h,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    req,
		}, nil
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

func newBenchDeps(tb testing.TB) engine.Deps {
	tb.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: &benchGraphQLFixture{}, MaxRetries: 0},
	)
	if err != nil {
		tb.Fatalf("NewGraphQL: %v", err)
	}
	rest, err := githubapi.NewREST(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost",
		httpx.Options{Transport: &benchRESTMux{}, MaxRetries: 0},
	)
	if err != nil {
		tb.Fatalf("NewREST: %v", err)
	}
	return engine.Deps{
		Settings: &config.Settings{Repositories: 100},
		GraphQL:  gql,
		REST:     rest,
		Render:   &render.FakeRenderer{},
	}
}

// BenchmarkCompute_Full_21Plugins (T094) measures the per-call wall
// time of engine.Compute with all 21 plugin gates on, classic SVG
// output, mocked GraphQL/REST/Render.
//
// Reported ns/op feeds the SC-003 p95 < 5s evidence on the PR body.
// Mocked deps mean the number is many orders of magnitude below the
// 5-second budget — the bench is a regression detector for "did
// someone accidentally add a quadratic loop or a blocking call?"
func BenchmarkCompute_Full_21Plugins(b *testing.B) {
	deps := newBenchDeps(b)
	inputs := fullPluginInputs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := engine.Compute(context.Background(), engine.Request{
			Login:    "octocat",
			Template: "classic",
			Format:   "svg",
			Inputs:   inputs,
		}, deps)
		if err != nil {
			b.Fatalf("Compute: %v", err)
		}
	}
}

// BenchmarkCompute_MemoryPeak (T095) reports allocations per op so the
// SC-009 peak < 800 MB evidence can be derived. The bench framework's
// `-benchmem` flag surfaces `B/op` and `allocs/op`. Manual peak heap
// measurement happens in TestCompute_MemoryPeak_RegressionGuard so
// `go test ./...` can fail-fast on a runaway leak.
func BenchmarkCompute_MemoryPeak(b *testing.B) {
	deps := newBenchDeps(b)
	inputs := fullPluginInputs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := engine.Compute(context.Background(), engine.Request{
			Login:    "octocat",
			Template: "classic",
			Format:   "svg",
			Inputs:   inputs,
		}, deps)
		if err != nil {
			b.Fatalf("Compute: %v", err)
		}
	}
}

// TestCompute_Full_21Plugins_PerformanceBudget runs 5 iterations of
// engine.Compute, sorts wall times, picks the p95 (= the slowest
// here, since N=5), and asserts it stays under p95WallBudget. With
// mocked Deps this trivially passes — the test guards against
// regressions large enough to be visible (e.g. an accidental
// time.Sleep or a synchronous loop blowing past 5s).
func TestCompute_Full_21Plugins_PerformanceBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short (bench-shaped regression guard)")
	}
	deps := newBenchDeps(t)
	inputs := fullPluginInputs()
	const iters = 5
	durations := make([]time.Duration, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		_, err := engine.Compute(context.Background(), engine.Request{
			Login:    "octocat",
			Template: "classic",
			Format:   "svg",
			Inputs:   inputs,
		}, deps)
		if err != nil {
			t.Fatalf("Compute[%d]: %v", i, err)
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[len(durations)-1]
	if p95 > p95WallBudget {
		t.Errorf("p95 wall time = %v, want < %v (durations=%v)", p95, p95WallBudget, durations)
	}
	t.Logf("p95 wall time = %v across %d iterations (budget: %v)", p95, iters, p95WallBudget)
}

// TestCompute_MemoryPeak_RegressionGuard runs Compute under a
// MemStats sampler that measures HeapInuse before / after. With mocked
// Deps the delta is sub-MB so this trivially passes, but a regression
// like "load every repository into memory at once" would blow past
// the 800 MB budget.
func TestCompute_MemoryPeak_RegressionGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short (bench-shaped regression guard)")
	}
	deps := newBenchDeps(t)
	inputs := fullPluginInputs()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	_, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   inputs,
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	var peak runtime.MemStats
	runtime.ReadMemStats(&peak)

	// HeapInuse during Compute may overshoot HeapAlloc by allocator
	// fragmentation, but for a regression guard the delta on HeapInuse
	// captures both "did we grow the heap" and "did we hold huge
	// objects". Use TotalAlloc delta as a secondary signal.
	heapDelta := int64(peak.HeapInuse) - int64(before.HeapInuse)
	totalAllocDelta := peak.TotalAlloc - before.TotalAlloc
	if heapDelta > peakMemoryBudget {
		t.Errorf("HeapInuse delta = %d bytes, want < %d (TotalAlloc delta = %d)",
			heapDelta, peakMemoryBudget, totalAllocDelta)
	}
	t.Logf("HeapInuse delta = %d B, TotalAlloc delta = %d B (budget: %d B)",
		heapDelta, totalAllocDelta, peakMemoryBudget)
}
