package traffic_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func newREST(t *testing.T, mux *githubapi.MockTransport) *githubapi.REST {
	t.Helper()
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

func scopeMux(scopes string) *githubapi.MockTransport {
	mux := githubapi.NewMockTransport()
	h := http.Header{}
	if scopes != "" {
		h.Set("X-OAuth-Scopes", scopes)
	}
	mux.Set("GET", "/", githubapi.MockResponse{Status: http.StatusOK, Header: h, Body: []byte(`{}`)})
	return mux
}

func TestRun_NoRepoScope_Skipped(t *testing.T) {
	t.Parallel()
	mux := scopeMux("read:user")
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"plugin_traffic": true},
		REST:   newREST(t, mux),
	}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if !r.Skipped {
		t.Errorf("expected Skipped without repo scope")
	}
	if !r.HideEmpty {
		t.Errorf("HideEmpty should default to true even on the skipped path; got false")
	}
}

// TestRun_HideEmpty_ExplicitFalse verifies `plugin_traffic_hide_empty:
// "no"` and `false` both turn off the filter (so legacy callers can
// re-enable the pre-#412 behaviour).
func TestRun_HideEmpty_ExplicitFalse(t *testing.T) {
	t.Parallel()
	for _, v := range []any{"no", false} {
		v := v
		mux := scopeMux("repo")
		pc := &plugins.PluginContext{
			Data:   plugins.NewData(),
			Inputs: map[string]any{"plugin_traffic": true, "plugin_traffic_hide_empty": v},
			REST:   newREST(t, mux),
		}
		out, _ := traffic.Plugin.Run(context.Background(), pc)
		r := out.(*traffic.Result)
		if r.HideEmpty {
			t.Errorf("HideEmpty for input %v (%T) = true, want false", v, v)
		}
	}
}

func TestRun_WithRepoScope_AggregatesViews(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	mux.SetJSON("GET", "/repos/octocat/alpha/traffic/views", `{"count":100,"uniques":40}`)
	mux.SetJSON("GET", "/repos/octocat/beta/traffic/views", `{"count":50,"uniques":20}`)

	data := plugins.NewData()
	data.Computed.RepositoryList = []plugins.Repository{
		{NameWithOwner: "octocat/alpha"},
		{NameWithOwner: "octocat/beta"},
	}
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{"plugin_traffic": true}, REST: newREST(t, mux)}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if r.Skipped {
		t.Fatalf("unexpected Skipped: %+v", r)
	}
	if r.Total.Count != 150 || r.Total.Uniques != 60 {
		t.Errorf("Total = %+v, want {Count:150,Uniques:60}", r.Total)
	}
	if len(r.Views) != 2 {
		t.Errorf("Views len = %d, want 2", len(r.Views))
	}
	if errs := pc.Data.SnapshotErrors(); len(errs) != 0 {
		t.Errorf("SnapshotErrors len = %d, want 0; errors: %v", len(errs), errs)
	}
}

func TestRun_NoRepositories_EmptyButNotSkipped(t *testing.T) {
	t.Parallel()
	mux := scopeMux("repo")
	pc := &plugins.PluginContext{
		Data: plugins.NewData(), Inputs: map[string]any{"plugin_traffic": true}, REST: newREST(t, mux),
	}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if r.Skipped {
		t.Errorf("empty RepositoryList should yield empty (non-Skipped) result")
	}
	if r.Total.Count != 0 {
		t.Errorf("Total.Count = %d, want 0", r.Total.Count)
	}
}

func TestRun_NilREST_Skipped(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: map[string]any{"plugin_traffic": true}}
	out, _ := traffic.Plugin.Run(context.Background(), pc)
	r := out.(*traffic.Result)
	if !r.Skipped {
		t.Errorf("nil REST should yield Skipped")
	}
}

// newRESTNoRetry builds a REST client with retries disabled so that
// rate-limit Retry-After headers do not block the test for tens of
// seconds.
func newRESTNoRetry(t *testing.T, mux *githubapi.MockTransport) *githubapi.REST {
	t.Helper()
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

// TestRun_DropClassification verifies that a repo whose views cannot be
// fetched is dropped without skipping the card, and that exactly one
// aggregated error names the matching bucket (rate limited / forbidden /
// failed). DisableRetries keeps rate-limit Retry-After headers from
// blocking the test; the httpx error handler then surfaces the 403 as a
// *httpx.RateLimitedError.
func TestRun_DropClassification(t *testing.T) {
	t.Parallel()
	rlHeader := http.Header{}
	rlHeader.Set("Retry-After", "1")
	rlHeader.Set("Content-Type", "application/json")
	cases := []struct {
		name      string
		alpha     githubapi.MockResponse
		want      string
		wantCount string
		notWant   string
	}{
		{"rate limited", githubapi.MockResponse{Status: http.StatusForbidden, Header: rlHeader, Body: []byte(`{"message":"rate limited"}`)}, "rate limit", "1/2", "forbidden"},
		{"plain forbidden", githubapi.MockResponse{Status: http.StatusForbidden, Body: []byte(`{"message":"forbidden"}`)}, "forbidden", "1/2", "rate limit"},
		{"server error", githubapi.MockResponse{Status: http.StatusInternalServerError, Body: []byte(`{"message":"internal server error"}`)}, "(failed)", "1/2", "forbidden"},
		{"decode failure", githubapi.MockResponse{Status: http.StatusOK, Body: []byte(`not-json`)}, "(failed)", "1/2", "forbidden"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			mux := scopeMux("repo")
			mux.Set("GET", "/repos/octocat/alpha/traffic/views", c.alpha)
			mux.SetJSON("GET", "/repos/octocat/beta/traffic/views", `{"count":50,"uniques":20}`)
			data := plugins.NewData()
			data.Computed.RepositoryList = []plugins.Repository{
				{NameWithOwner: "octocat/alpha"},
				{NameWithOwner: "octocat/beta"},
			}
			pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{"plugin_traffic": true}, REST: newRESTNoRetry(t, mux)}
			out, err := traffic.Plugin.Run(context.Background(), pc)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			r := out.(*traffic.Result)
			if r.Skipped {
				t.Fatalf("result should not be Skipped")
			}
			if _, ok := r.Views["octocat/beta"]; !ok {
				t.Errorf("octocat/beta should still be present in Views")
			}
			if _, ok := r.Views["octocat/alpha"]; ok {
				t.Errorf("octocat/alpha should be dropped from Views")
			}
			errs := data.SnapshotErrors()
			if len(errs) != 1 {
				t.Fatalf("SnapshotErrors len = %d, want 1; errors: %v", len(errs), errs)
			}
			msg := errs[0].Error()
			if !strings.Contains(msg, c.want) || !strings.Contains(msg, c.wantCount) {
				t.Errorf("error should contain %q and %q; got %q", c.want, c.wantCount, msg)
			}
			if strings.Contains(msg, c.notWant) {
				t.Errorf("error must not contain %q; got %q", c.notWant, msg)
			}
		})
	}
}

// TestRun_PluginDisabled_Skipped covers the gate-off path: without a
// truthy `plugin_traffic` input, Run must Skip before reaching the
// dataprovider or any API client. The GraphQL mux has no handlers, so
// any GraphQL call fails the test.
func TestRun_PluginDisabled_Skipped(t *testing.T) {
	t.Parallel()
	gql := mocks.NewGraphQLMux(t)
	rest := mocks.NewRESTMux(t)
	pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
	pc.Provider = dataprovider.New("octocat", "", pc.GraphQL, pc.REST, pc.Logger, dataprovider.Options{})

	out, err := traffic.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := out.(*traffic.Result)
	if !r.Skipped || r.SkippedReason != "plugin disabled" {
		t.Errorf("Skipped = %v, SkippedReason = %q; want true, %q", r.Skipped, r.SkippedReason, "plugin disabled")
	}
	if n := rest.TotalCalls(); n != 0 {
		t.Errorf("REST calls = %d, want 0", n)
	}
}
