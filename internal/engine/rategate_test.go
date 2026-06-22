package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/httpx"
)

// rateLimitJSON renders a /rate_limit body with the given remaining
// counts and a shared reset epoch for the REST + GraphQL pools.
func rateLimitJSON(restRemaining, graphqlRemaining int, reset int64) string {
	return fmt.Sprintf(`{
		"resources": {
			"core":    {"limit":5000,"used":0,"remaining":%d,"reset":%d},
			"graphql": {"limit":5000,"used":0,"remaining":%d,"reset":%d},
			"search":  {"limit":30,"used":0,"remaining":30,"reset":%d}
		},
		"rate": {"limit":5000,"used":0,"remaining":%d,"reset":%d}
	}`, restRemaining, reset, graphqlRemaining, reset, reset, restRemaining, reset)
}

// newGateDeps builds engine Deps whose REST client is backed by mock and
// whose GraphQL client carries the same (real-shaped) token kind. The
// GraphQL client is never called by the gate; only its TokenKind matters.
func newGateDeps(t *testing.T, token string, mock *githubapi.MockTransport) Deps {
	t.Helper()
	rest, err := githubapi.NewREST(config.NewToken(token), "", httpx.Options{
		Transport:  mock,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}
	gql, err := githubapi.NewGraphQL(config.NewToken(token), "", httpx.Options{
		Transport:  mock,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewGraphQL: %v", err)
	}
	return Deps{
		Logger:  slog.New(slog.NewTextHandler(&strings.Builder{}, nil)),
		REST:    rest,
		GraphQL: gql,
	}
}

func rateLimitCalls(mock *githubapi.MockTransport) int {
	n := 0
	for _, c := range mock.Calls() {
		if c.Path == "/rate_limit" {
			n++
		}
	}
	return n
}

// TestRateGate_OffByDefault asserts the gate makes no /rate_limit call
// when the mode input is absent (default off).
func TestRateGate_OffByDefault(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(0, 0, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	if err := runRateGate(context.Background(), map[string]any{}, deps); err != nil {
		t.Fatalf("runRateGate (off): %v", err)
	}
	if got := rateLimitCalls(mock); got != 0 {
		t.Errorf("off mode must not call /rate_limit; got %d calls", got)
	}
}

// TestRateGate_OffExplicit asserts an explicit "off" likewise skips.
func TestRateGate_OffExplicit(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(0, 0, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	inputs := map[string]any{rateGuardInput: "off"}
	if err := runRateGate(context.Background(), inputs, deps); err != nil {
		t.Fatalf("runRateGate (off): %v", err)
	}
	if got := rateLimitCalls(mock); got != 0 {
		t.Errorf("off mode must not call /rate_limit; got %d calls", got)
	}
}

// TestRateGate_FailBelowThreshold asserts fail mode errors with the
// pool name and reset time in the message when a pool is depleted.
func TestRateGate_FailBelowThreshold(t *testing.T) {
	t.Parallel()

	reset := time.Now().Add(15 * time.Minute).Truncate(time.Second)
	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(5, 4990, reset.Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	inputs := map[string]any{rateGuardInput: "fail"}
	err := runRateGate(context.Background(), inputs, deps)
	if err == nil {
		t.Fatal("fail mode below threshold must error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "rest") {
		t.Errorf("error should name the rest pool: %q", msg)
	}
	if !strings.Contains(msg, "5 remaining") {
		t.Errorf("error should report remaining count: %q", msg)
	}
	if !strings.Contains(msg, reset.UTC().Format(time.RFC3339)) {
		t.Errorf("error should report reset time %s: %q", reset.UTC().Format(time.RFC3339), msg)
	}
	if got := rateLimitCalls(mock); got != 1 {
		t.Errorf("fail mode should call /rate_limit exactly once; got %d", got)
	}
}

// TestRateGate_FailHealthyPools asserts fail mode is a no-op when both
// pools are above the threshold (one /rate_limit call, no error).
func TestRateGate_FailHealthyPools(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(4990, 4990, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	inputs := map[string]any{rateGuardInput: "fail"}
	if err := runRateGate(context.Background(), inputs, deps); err != nil {
		t.Fatalf("healthy pools must pass: %v", err)
	}
	if got := rateLimitCalls(mock); got != 1 {
		t.Errorf("gate should call /rate_limit once; got %d", got)
	}
}

// TestRateGate_MockedTokenBypass asserts a MOCKED_TOKEN run never calls
// /rate_limit even in fail mode (the mocked guard would panic on a real
// host).
func TestRateGate_MockedTokenBypass(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(0, 0, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "MOCKED_TOKEN", mock)

	inputs := map[string]any{rateGuardInput: "fail"}
	if err := runRateGate(context.Background(), inputs, deps); err != nil {
		t.Fatalf("mocked token must bypass the gate: %v", err)
	}
	if got := rateLimitCalls(mock); got != 0 {
		t.Errorf("mocked token must not call /rate_limit; got %d", got)
	}
}

// TestRateGate_NoTokenBypass asserts the NOT_NEEDED sentinel bypasses
// the gate.
func TestRateGate_NoTokenBypass(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(0, 0, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "NOT_NEEDED", mock)

	inputs := map[string]any{rateGuardInput: "wait"}
	if err := runRateGate(context.Background(), inputs, deps); err != nil {
		t.Fatalf("NOT_NEEDED must bypass the gate: %v", err)
	}
	if got := rateLimitCalls(mock); got != 0 {
		t.Errorf("NOT_NEEDED must not call /rate_limit; got %d", got)
	}
}

// TestRateGate_WaitSleepsUntilReset asserts wait mode sleeps for the
// (reset+slack - now) interval. A fake clock keeps the test instant and
// records the requested sleep duration.
func TestRateGate_WaitSleepsUntilReset(t *testing.T) {
	t.Parallel()

	base := time.Now().Truncate(time.Second)
	reset := base.Add(2 * time.Minute)

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(5, 4990, reset.Unix()))
	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaa"), "", httpx.Options{Transport: mock, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}

	var slept time.Duration
	sleepCalls := 0
	g := &rateGate{
		rest:   rest,
		res:    githubapi.NewResources(),
		logger: slog.New(slog.NewTextHandler(&strings.Builder{}, nil)),
		now:    func() time.Time { return base },
		sleep: func(_ context.Context, d time.Duration) error {
			sleepCalls++
			slept = d
			return nil
		},
	}

	if err := g.run(context.Background(), rateGuardWait); err != nil {
		t.Fatalf("wait mode: %v", err)
	}
	if sleepCalls != 1 {
		t.Fatalf("wait mode should sleep exactly once; got %d", sleepCalls)
	}
	want := reset.Sub(base) + rateGuardSlack
	if slept != want {
		t.Errorf("slept %s, want %s (reset+slack-now)", slept, want)
	}
}

// TestRateGate_WaitRealSleep exercises the production sleepCtx path with
// a tiny reset offset so the real sleep stays well under a second.
func TestRateGate_WaitRealSleep(t *testing.T) {
	t.Parallel()

	// Reset is in the past so the computed wait is <= slack; the gate
	// returns immediately without a long sleep. The real sleepCtx path
	// is still driven for the healthy (wait <= 0) short-circuit.
	reset := time.Now().Add(-time.Minute)
	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(5, 4990, reset.Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	start := time.Now()
	if err := runRateGate(context.Background(), map[string]any{rateGuardInput: "wait"}, deps); err != nil {
		t.Fatalf("wait (past reset) should return immediately: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("past-reset wait should be instant; took %s", elapsed)
	}
}

// TestRateGate_WaitCapExceeded asserts wait mode errors (rather than
// blocking) when the required wait exceeds the hard cap.
func TestRateGate_WaitCapExceeded(t *testing.T) {
	t.Parallel()

	base := time.Now().Truncate(time.Second)
	reset := base.Add(rateGuardWaitCap + 10*time.Minute)

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(5, 4990, reset.Unix()))
	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaa"), "", httpx.Options{Transport: mock, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}

	g := &rateGate{
		rest:   rest,
		res:    githubapi.NewResources(),
		logger: slog.New(slog.NewTextHandler(&strings.Builder{}, nil)),
		now:    func() time.Time { return base },
		sleep: func(_ context.Context, _ time.Duration) error {
			t.Fatal("sleep must not be called when the wait exceeds the cap")
			return nil
		},
	}

	err = g.run(context.Background(), rateGuardWait)
	if err == nil {
		t.Fatal("wait beyond the cap must error")
	}
	if !strings.Contains(err.Error(), "exceeds cap") {
		t.Errorf("error should mention the cap: %q", err.Error())
	}
}

// TestRateGate_WaitContextCancelled asserts wait mode honours context
// cancellation while sleeping.
func TestRateGate_WaitContextCancelled(t *testing.T) {
	t.Parallel()

	base := time.Now().Truncate(time.Second)
	reset := base.Add(5 * time.Minute)

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(5, 4990, reset.Unix()))
	rest, err := githubapi.NewREST(config.NewToken("ghp_aaaa"), "", httpx.Options{Transport: mock, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewREST: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel up-front; sleepCtx must observe it immediately

	g := &rateGate{
		rest:   rest,
		res:    githubapi.NewResources(),
		logger: slog.New(slog.NewTextHandler(&strings.Builder{}, nil)),
		now:    func() time.Time { return base },
		sleep:  sleepCtx, // real ctx-aware sleep
	}

	err = g.run(ctx, rateGuardWait)
	if err == nil {
		t.Fatal("cancelled context during wait must error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error should be the context cancellation: %q", err.Error())
	}
}

// TestRateGate_RefreshError asserts a /rate_limit failure surfaces a
// descriptive error rather than silently passing the gate.
func TestRateGate_RefreshError(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.Set("GET", "/rate_limit", githubapi.MockResponse{Status: 500, Body: []byte(`{}`)})
	deps := newGateDeps(t, "ghp_aaaa", mock)

	err := runRateGate(context.Background(), map[string]any{rateGuardInput: "fail"}, deps)
	if err == nil {
		t.Fatal("refresh failure must surface an error")
	}
	if !strings.Contains(err.Error(), "refresh /rate_limit") {
		t.Errorf("error should mention the refresh failure: %q", err.Error())
	}
}

// TestRateGate_UnknownModeIsOff asserts an unrecognized mode value is
// treated as "off" (no /rate_limit call), so a typo never silently
// enables a blocking gate.
func TestRateGate_UnknownModeIsOff(t *testing.T) {
	t.Parallel()

	mock := githubapi.NewMockTransport()
	mock.SetJSON("GET", "/rate_limit", rateLimitJSON(0, 0, time.Now().Add(time.Hour).Unix()))
	deps := newGateDeps(t, "ghp_aaaa", mock)

	if err := runRateGate(context.Background(), map[string]any{rateGuardInput: "yes-please"}, deps); err != nil {
		t.Fatalf("unknown mode should be treated as off: %v", err)
	}
	if got := rateLimitCalls(mock); got != 0 {
		t.Errorf("unknown mode must not call /rate_limit; got %d", got)
	}
}

// TestSleepCtx_ExpiresNormally verifies sleepCtx returns nil when the
// timer fires before the context is cancelled.
func TestSleepCtx_ExpiresNormally(t *testing.T) {
	t.Parallel()

	start := time.Now()
	err := sleepCtx(context.Background(), 10*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("sleepCtx returned non-nil error: %v", err)
	}
	if elapsed < 5*time.Millisecond {
		t.Errorf("sleepCtx returned too early: %s", elapsed)
	}
}

// TestSleepCtx_CancelledContext verifies sleepCtx returns ctx.Err() when the
// context is already cancelled before it is called.
func TestSleepCtx_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling sleepCtx

	err := sleepCtx(ctx, time.Hour)
	if err == nil {
		t.Fatal("sleepCtx should return an error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("sleepCtx error = %v, want context.Canceled", err)
	}
}
