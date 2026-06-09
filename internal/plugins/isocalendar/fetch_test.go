package isocalendar

import (
	"context"
	"encoding/json"
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

// TestWindowStart_HalfYearSnapsToSunday pins the upstream range rule:
// half-year is now-180d rewound to the previous Sunday 00:00:00 UTC.
// 2026-06-03 (Wed) - 180d = 2025-12-05 (Fri) → Sunday 2025-11-30.
func TestWindowStart_HalfYearSnapsToSunday(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 3, 8, 33, 25, 0, time.UTC)
	got := windowStart(now, "half-year")
	want := time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("windowStart(half-year) = %v, want %v", got, want)
	}
	if got.Weekday() != time.Sunday {
		t.Errorf("windowStart(half-year).Weekday() = %v, want Sunday", got.Weekday())
	}
}

// TestWindowStart_FullYearSnapsToSunday — full-year is now-1y rewound
// to the previous Sunday. 2026-06-03 - 1y = 2025-06-03 (Tue) →
// Sunday 2025-06-01.
func TestWindowStart_FullYearSnapsToSunday(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 3, 8, 33, 25, 0, time.UTC)
	got := windowStart(now, "full-year")
	want := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("windowStart(full-year) = %v, want %v", got, want)
	}
}

// TestWindowStart_AlreadySunday — when now-180d already falls on a
// Sunday only the time-of-day is zeroed (upstream skips the day shift).
func TestWindowStart_AlreadySunday(t *testing.T) {
	t.Parallel()
	// 2025-11-30 (Sun) + 180d = 2026-05-29, so now-180d lands on the Sunday.
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	got := windowStart(now, "half-year")
	want := time.Date(2025, 11, 30, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("windowStart = %v, want %v", got, want)
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
		Transport:  transport,
		MaxRetries: 0,
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

// TestFetchWindowedWeeks_NoGraphQL — the degraded path contract: no
// client (unit-test harnesses) yields nil weeks and no error so Run
// falls back to slicing the shared indepth calendar.
func TestFetchWindowedWeeks_NoGraphQL(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData()}
	weeks, err := fetchWindowedWeeks(context.Background(), pc, "half-year")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if weeks != nil {
		t.Errorf("weeks = %v, want nil", weeks)
	}
}

// TestFetchWindowedWeeks_DisabledPluginSkipsFetch — core.RunPlugins
// invokes every registered plugin even when its input is off, so a
// disabled isocalendar must not issue any GraphQL traffic.
func TestFetchWindowedWeeks_DisabledPluginSkipsFetch(t *testing.T) {
	t.Parallel()
	transport := &chunkRecorderTransport{body: `{"data":{"user":null}}`}
	gql, err := githubapi.NewGraphQL(config.NewToken("ghp_test"), "", httpx.Options{
		Transport:  transport,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	data := plugins.NewData()
	data.User = &plugins.User{Login: "octocat"}
	pc := &plugins.PluginContext{Data: data, GraphQL: gql}

	weeks, err := fetchWindowedWeeks(context.Background(), pc, "half-year")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if weeks != nil {
		t.Errorf("weeks = %v, want nil", weeks)
	}
	if len(transport.froms) != 0 {
		t.Errorf("GraphQL calls = %d, want 0 for a disabled plugin", len(transport.froms))
	}
}
