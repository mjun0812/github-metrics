package reactions_test

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

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/reactions"
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

type fixedMux struct {
	mu      sync.Mutex
	body    string
	lastReq string
}

func (f *fixedMux) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	body := f.body
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		f.lastReq = string(raw)
	}
	f.mu.Unlock()
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func newGQL(t *testing.T, body string) *githubapi.GraphQL {
	t.Helper()
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: &fixedMux{body: body}, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return gql
}

func TestRun_AggregatesReactionContent(t *testing.T) {
	t.Parallel()
	// 2 issues + 3 issue comments scanned; reactions aggregated by
	// content: HEART x3, THUMBS_UP x2, ROCKET x1 (total 6).
	body := `{"data":{"user":{
		"issues":{"totalCount":2,"nodes":[
			{"reactions":{"totalCount":2,"nodes":[{"content":"HEART"},{"content":"HEART"}]}},
			{"reactions":{"totalCount":1,"nodes":[{"content":"ROCKET"}]}}
		]},
		"issueComments":{"totalCount":3,"nodes":[
			{"reactions":{"totalCount":1,"nodes":[{"content":"HEART"}]}},
			{"reactions":{"totalCount":2,"nodes":[{"content":"THUMBS_UP"},{"content":"THUMBS_UP"}]}},
			{"reactions":{"totalCount":0,"nodes":[]}}
		]}
	}}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if r.Total != 6 {
		t.Errorf("Total = %d, want 6", r.Total)
	}
	if r.Comments != 5 {
		t.Errorf("Comments = %d, want 5", r.Comments)
	}
	if got := r.List["HEART"].Value; got != 3 {
		t.Errorf("HEART value = %d, want 3", got)
	}
	if got := r.List["THUMBS_UP"].Value; got != 2 {
		t.Errorf("THUMBS_UP value = %d, want 2", got)
	}
	if got := r.List["ROCKET"].Value; got != 1 {
		t.Errorf("ROCKET value = %d, want 1", got)
	}
	// absolute display: score == percentage == value/total.
	if got := r.List["HEART"].Score; got < 0.49 || got > 0.51 {
		t.Errorf("HEART score = %v, want ~0.5", got)
	}
}

// TestRun_ClampsConnectionLimitTo100 guards #472: GitHub rejects a
// connection `first` above 100 with EXCESSIVE_PAGINATION, which fails
// the entire UserReactions query and blanks the card. The upstream
// plugin_reactions_limit default is 200, so the request must clamp it
// to GitHub's 100 ceiling.
func TestRun_ClampsConnectionLimitTo100(t *testing.T) {
	t.Parallel()
	mux := &fixedMux{body: `{"data":{"user":{"issues":{"totalCount":0,"nodes":[]},"issueComments":{"totalCount":0,"nodes":[]}}}}`}
	gql, err := githubapi.NewGraphQL(
		config.NewToken("MOCKED_TOKEN"),
		"http://mock.localhost/graphql",
		httpx.Options{Transport: mux, MaxRetries: 0},
	)
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true, "plugin_reactions_limit": 200},
		GraphQL: gql,
	}
	if _, err := reactions.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(mux.lastReq, `"commentsFirst":200`) {
		t.Errorf("commentsFirst must be clamped to 100, not 200 (EXCESSIVE_PAGINATION); req=%s", mux.lastReq)
	}
	if !strings.Contains(mux.lastReq, `"commentsFirst":100`) {
		t.Errorf("commentsFirst should clamp to 100; req=%s", mux.lastReq)
	}
}

func TestRun_RelativeDisplayScalesByMax(t *testing.T) {
	t.Parallel()
	// HEART x3 (max), THUMBS_UP x1. relative score = value/max.
	body := `{"data":{"user":{
		"issues":{"totalCount":0,"nodes":[]},
		"issueComments":{"totalCount":2,"nodes":[
			{"reactions":{"totalCount":3,"nodes":[{"content":"HEART"},{"content":"HEART"},{"content":"HEART"}]}},
			{"reactions":{"totalCount":1,"nodes":[{"content":"THUMBS_UP"}]}}
		]}
	}}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true, "plugin_reactions_display": "relative"},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if got := r.List["HEART"].Score; got != 1.0 {
		t.Errorf("HEART relative score = %v, want 1.0", got)
	}
	if got := r.List["THUMBS_UP"].Score; got < 0.32 || got > 0.34 {
		t.Errorf("THUMBS_UP relative score = %v, want ~0.333", got)
	}
}

func TestRun_DetailsParsed(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":{"issues":{"totalCount":0,"nodes":[]},"issueComments":{"totalCount":0,"nodes":[]}}}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true, "plugin_reactions_details": "percentage, count"},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if len(r.Details) != 2 || r.Details[0] != "percentage" || r.Details[1] != "count" {
		t.Errorf("Details = %v, want [percentage count]", r.Details)
	}
}

func TestRun_NilUser(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":null}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if r.Skipped {
		t.Errorf("nil user response should yield empty (non-Skipped) result")
	}
}

// TestRun_EmptyGraphQLResponsePropagatesError guards #732: when GitHub
// hits a secondary rate limit and replies with `{"data": null}` at 200
// OK, the GraphQL client must surface the swallowed condition as an
// error and reactions must propagate it (wrapped in RetryableError)
// instead of rendering an empty card from a zero-valued response.
func TestRun_EmptyGraphQLResponsePropagatesError(t *testing.T) {
	t.Parallel()
	body := `{"data":null}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"user": "octocat", "plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	_, err := reactions.Plugin.Run(context.Background(), pc)
	if err == nil {
		t.Fatalf("expected error from empty GraphQL response, got nil")
	}
	if !errors.Is(err, githubapi.ErrEmptyGraphQLResponse) {
		t.Errorf("errors.Is(ErrEmptyGraphQLResponse) = false; err=%v", err)
	}
}

func TestRun_NilGraphQL_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{"user": "octocat", "plugin_reactions": true}}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if !r.Skipped {
		t.Errorf("nil GraphQL should yield Skipped")
	}
}

// TestRun_PluginDisabled_Skipped covers the gate-off path: when
// `plugin_reactions` is not truthy in the input map, Run must Skip
// before issuing any GraphQL call. The previous incarnation of this
// test was misnamed (`TestRun_NoLogin_Skipped`) — it relied on the
// gate-off behaviour rather than testing the no-login branch, so any
// regression in the login check passed silently.
func TestRun_PluginDisabled_Skipped(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":null}}`
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{}, GraphQL: newGQL(t, body)}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if !r.Skipped {
		t.Errorf("missing plugin_reactions flag should yield Skipped")
	}
	if r.SkippedReason != "plugin disabled" {
		t.Errorf("SkippedReason = %q, want %q", r.SkippedReason, "plugin disabled")
	}
}

// TestRun_NoLogin_Skipped exercises the actual no-login branch:
// `plugin_reactions: true` (passing the gate) + neither `user` nor
// `login` set (so `loginFromInputs("") == ""`). The plugin must Skip
// with SkippedReason "no login", not "plugin disabled".
func TestRun_NoLogin_Skipped(t *testing.T) {
	t.Parallel()
	body := `{"data":{"user":null}}`
	pc := &plugins.PluginContext{
		Data:    plugins.NewData(),
		Inputs:  map[string]any{"plugin_reactions": true},
		GraphQL: newGQL(t, body),
	}
	out, _ := reactions.Plugin.Run(context.Background(), pc)
	r := out.(*reactions.Result)
	if !r.Skipped {
		t.Errorf("missing login should yield Skipped")
	}
	if r.SkippedReason != "no login" {
		t.Errorf("SkippedReason = %q, want %q", r.SkippedReason, "no login")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &reactions.Result{
		List: map[string]reactions.Reaction{
			"HEART":     {Value: 5, Percentage: 0.625, Score: 0.625},
			"THUMBS_UP": {Value: 3, Percentage: 0.375, Score: 0.375},
		},
		Total:    8,
		Comments: 6,
		Days:     0,
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "reactions.json")
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

// TestPartial_Reactions_Golden locks the upstream 8-emoji gauge panel
// structure: one gauge SVG per reaction, a gauge-arc when score > 0, and
// the percentage detail span (plugin_reactions_details=percentage).
func TestPartial_Reactions_Golden(t *testing.T) {
	r := &reactions.Result{
		List: map[string]reactions.Reaction{
			"HEART":     {Value: 6, Percentage: 0.75, Score: 0.75},
			"THUMBS_UP": {Value: 2, Percentage: 0.25, Score: 0.25},
		},
		Total:    8,
		Comments: 200,
		Details:  []string{"percentage"},
		Days:     0,
	}
	data := plugins.NewData()
	data.SetPlugin(reactions.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, _, err := reactions.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "reactions.svg")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	// Structural markers required for upstream parity.
	if n := strings.Count(got, `class="gauge info"`); n != 8 {
		t.Errorf("gauge count = %d, want 8", n)
	}
	for _, marker := range []string{
		`from last 200 comments`,
		`<text x="60" y="60" dominant-baseline="central" text-anchor="middle" font-size="40" fill="#58A6FF">❤️</text>`,
		`stroke-dasharray="`, // HEART has score>0 so an arc is present
		`class="title nowrap"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}
