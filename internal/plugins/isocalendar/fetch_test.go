package isocalendar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// TestWindowStart pins the upstream range rule: half-year is now-180d and
// full-year is now-1y, both rewound to the previous Sunday 00:00:00 UTC
// (only the time-of-day is zeroed when the date is already a Sunday).
func TestWindowStart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		now      time.Time
		duration string
		want     time.Time
	}{
		// 2026-06-03 (Wed) - 180d = 2025-12-05 (Fri) → Sunday 2025-11-30.
		{"half-year", time.Date(2026, 6, 3, 8, 33, 25, 0, time.UTC), "half-year", time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC)},
		// 2026-06-03 - 1y = 2025-06-03 (Tue) → Sunday 2025-06-01.
		{"full-year", time.Date(2026, 6, 3, 8, 33, 25, 0, time.UTC), "full-year", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
		// 2026-05-29 - 180d already lands on Sunday 2025-11-30.
		{"already Sunday", time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), "half-year", time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC)},
	} {
		if got := windowStart(tc.now, tc.duration); !got.Equal(tc.want) {
			t.Errorf("%s: windowStart = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestChunkRanges_ContiguousNonOverlapping asserts the 4-week query
// slicing: chunks tile [start, now] with 1ms gaps at the boundaries so
// no day is reported twice, and the final chunk is clamped to now.
func TestChunkRanges_ContiguousNonOverlapping(t *testing.T) {
	t.Parallel()
	start := time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 3, 8, 33, 25, 0, time.UTC)
	ranges := chunkRanges(start, now)
	if len(ranges) != 7 {
		t.Fatalf("len(ranges) = %d, want 7", len(ranges))
	}
	if !ranges[0][0].Equal(start) {
		t.Errorf("first from = %v, want %v", ranges[0][0], start)
	}
	for i := 0; i < len(ranges)-1; i++ {
		wantNext := ranges[i][1].Add(time.Millisecond)
		if !ranges[i+1][0].Equal(wantNext) {
			t.Errorf("ranges[%d] from = %v, want %v (1ms after previous to)", i+1, ranges[i+1][0], wantNext)
		}
		if got := ranges[i+1][0].Sub(ranges[i][0]); got != chunkDays*24*time.Hour {
			t.Errorf("chunk %d width = %v, want %v", i, got, chunkDays*24*time.Hour)
		}
	}
	if want := now.Add(-time.Millisecond); !ranges[len(ranges)-1][1].Equal(want) {
		t.Errorf("last to = %v, want %v", ranges[len(ranges)-1][1], want)
	}
}

// chunkRecorderTransport serves the same canned UserIsocalendar payload
// for every request and records the requested from/to variables.
type chunkRecorderTransport struct {
	body  string
	froms []time.Time
}

func (c *chunkRecorderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	var payload struct {
		Variables struct {
			From time.Time `json:"from"`
		} `json:"variables"`
	}
	_ = json.Unmarshal(raw, &payload)
	c.froms = append(c.froms, payload.Variables.From)
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        http.StatusText(http.StatusOK),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(c.body)),
		ContentLength: int64(len(c.body)),
		Request:       req,
	}, nil
}

// TestFetchWindowedWeeks_ChunkedFetch wires a mock GraphQL client and
// asserts the half-year window is fetched in exactly 7 four-week
// chunks (180d + Sunday snap always spans 180–186 days), that the
// first chunk starts on a Sunday at 00:00 UTC, and that the returned
// weeks preserve GitHub's per-day count/color verbatim.
func TestFetchWindowedWeeks_ChunkedFetch(t *testing.T) {
	t.Parallel()
	transport := &chunkRecorderTransport{
		body: `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[
			{"firstDay":"2026-01-04","contributionDays":[
				{"date":"2026-01-04","contributionCount":3,"weekday":0,"color":"#40c463"},
				{"date":"2026-01-05","contributionCount":0,"weekday":1,"color":"#ebedf0"}
			]}
		]}}}}}`,
	}
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:      transport,
		DisableRetries: true,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	data := plugins.NewData()
	data.User = &plugins.User{Login: "octocat"}
	pc := &plugins.PluginContext{
		Data:    data,
		GraphQL: gql,
		Inputs:  map[string]any{"plugin_isocalendar": true},
	}

	weeks, err := fetchWindowedWeeks(context.Background(), pc, "half-year")
	if err != nil {
		t.Fatalf("fetchWindowedWeeks: %v", err)
	}
	if len(transport.froms) != 7 {
		t.Fatalf("GraphQL calls = %d, want 7", len(transport.froms))
	}
	first := transport.froms[0]
	if first.Weekday() != time.Sunday {
		t.Errorf("first chunk from weekday = %v, want Sunday", first.Weekday())
	}
	if first.Hour() != 0 || first.Minute() != 0 || first.Second() != 0 {
		t.Errorf("first chunk from time-of-day = %v, want 00:00:00", first)
	}
	if len(weeks) != 7 {
		t.Fatalf("weeks = %d, want 7 (one fixture week per chunk)", len(weeks))
	}
	day := weeks[0].Days[0]
	if day.ContributionCount != 3 || day.Color != "#40c463" || day.Weekday != 0 {
		t.Errorf("day = %+v, want count=3 color=#40c463 weekday=0", day)
	}
}

// TestRun_ThreadsEmptyGraphQLResponseToDataErrors guards #732 /
// PR #773: when the windowed contribution-calendar fetch returns
// githubapi.ErrEmptyGraphQLResponse (secondary rate limit path),
// isocalendar.Run must thread the failure to Data.Errors so operators
// can see the primary-path pushback even if the shared-calendar
// fallback also comes back empty and the plugin renders Skipped.
func TestRun_ThreadsEmptyGraphQLResponseToDataErrors(t *testing.T) {
	t.Parallel()
	transport := &chunkRecorderTransport{body: `{"data":null}`}
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:      transport,
		DisableRetries: true,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.User = &plugins.User{Login: "octocat"}
	pc := &plugins.PluginContext{
		Data:    data,
		GraphQL: gql,
		Inputs:  map[string]any{"plugin_isocalendar": true, "user": "octocat"},
	}

	if _, runErr := Plugin.Run(context.Background(), pc); runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	errsSnap := data.SnapshotErrors()
	if len(errsSnap) == 0 {
		t.Fatalf("Data.Errors is empty; ErrEmptyGraphQLResponse should be threaded through")
	}
	var matched bool
	for _, e := range errsSnap {
		if errors.Is(e, githubapi.ErrEmptyGraphQLResponse) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("Data.Errors does not carry ErrEmptyGraphQLResponse; got %v", errsSnap)
	}
}
