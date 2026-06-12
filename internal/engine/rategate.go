package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// rateGuardInput is the input key that selects the startup rate-limit
// gate behavior. Modes: "off" (default), "fail", "wait".
const rateGuardInput = "config_rate_limit_guard"

// Rate-guard modes.
const (
	rateGuardOff  = "off"
	rateGuardFail = "fail"
	rateGuardWait = "wait"
)

// rateGuardThreshold is the flat per-pool floor (#529). A run is gated
// when a pool the run will use has fewer than this many requests
// remaining. A constant keeps the config surface to a single mode
// switch instead of three per-pool knobs (the upstream
// quota_required_* inputs are a separate, dormant fail-soft mechanism).
const rateGuardThreshold = 100

// rateGuardWaitCap bounds how long "wait" mode may block before it
// gives up and errors. A pool reset is at most an hour away, but we cap
// well below that so a misconfigured run never hangs a CI job for the
// full window.
const rateGuardWaitCap = 30 * time.Minute

// rateGuardSlack is added to a pool's reset time before resuming, so we
// never race the server clock and re-trip the gate on the first request.
const rateGuardSlack = 5 * time.Second

// rateGate carries the collaborators the gate needs, with clock seams
// (now / sleep) so tests can drive "wait" mode without real delays.
type rateGate struct {
	rest    *githubapi.REST
	graphql *githubapi.GraphQL
	res     *githubapi.Resources
	logger  *slog.Logger
	now     func() time.Time
	sleep   func(ctx context.Context, d time.Duration) error
}

// runRateGate is the engine entry point. It reads the mode from inputs
// and, when enabled and the deps are live (non-mocked token), refreshes
// /rate_limit once and either waits or fails when a pool is below the
// threshold. Returns nil when the gate is disabled or the pools are
// healthy.
func runRateGate(ctx context.Context, inputs map[string]any, deps Deps) error {
	mode := normalizeRateGuardMode(inputs)
	if mode == rateGuardOff {
		return nil
	}
	if deps.REST == nil || deps.GraphQL == nil {
		// Without both REST and GraphQL there is nothing the gate can
		// meaningfully protect; skip rather than guess.
		return nil
	}
	if isMockedOrNoToken(deps.REST.TokenKind()) {
		// MOCKED_TOKEN / NOT_NEEDED runs must not call /rate_limit
		// (FR-017 mocked guard would panic on a real host).
		return nil
	}

	g := &rateGate{
		rest:    deps.REST,
		graphql: deps.GraphQL,
		res:     githubapi.NewResources(),
		logger:  deps.Logger,
		now:     time.Now,
		sleep:   sleepCtx,
	}
	return g.run(ctx, mode)
}

// run performs the refresh-then-decide cycle.
func (g *rateGate) run(ctx context.Context, mode string) error {
	if err := g.res.Refresh(ctx, g.rest); err != nil {
		return fmt.Errorf("engine: rate-limit gate: refresh /rate_limit: %w", err)
	}
	snap := g.res.Snapshot()

	g.logger.Info(
		"rate-limit gate",
		"mode", mode,
		"rest_remaining", snap.REST.Remaining,
		"rest_reset", snap.REST.Reset,
		"graphql_remaining", snap.GraphQL.Remaining,
		"graphql_reset", snap.GraphQL.Reset,
		"threshold", rateGuardThreshold,
	)

	low := lowPools(snap)
	if len(low) == 0 {
		return nil
	}

	switch mode {
	case rateGuardFail:
		return rateGuardError(low)
	case rateGuardWait:
		return g.waitForReset(ctx, low)
	default:
		return nil
	}
}

// lowPool is one pool that fell below the threshold.
type lowPool struct {
	name      string
	remaining int
	reset     time.Time
}

// lowPools returns the REST / GraphQL pools below the threshold. Search
// is excluded: no adopted plugin consumes the search API quota, so
// gating on it would block runs for a pool they never touch.
func lowPools(snap githubapi.ResourcesSnapshot) []lowPool {
	var out []lowPool
	if snap.REST.Remaining < rateGuardThreshold {
		out = append(out, lowPool{"rest", snap.REST.Remaining, snap.REST.Reset})
	}
	if snap.GraphQL.Remaining < rateGuardThreshold {
		out = append(out, lowPool{"graphql", snap.GraphQL.Remaining, snap.GraphQL.Reset})
	}
	return out
}

// waitForReset sleeps until the latest relevant reset (+slack), bounded
// by rateGuardWaitCap. Returns an error when the wait would exceed the
// cap or the context is cancelled.
func (g *rateGate) waitForReset(ctx context.Context, low []lowPool) error {
	var latest time.Time
	for _, p := range low {
		if p.reset.After(latest) {
			latest = p.reset
		}
	}
	wait := latest.Add(rateGuardSlack).Sub(g.now())
	if wait <= 0 {
		// Reset is already in the past (clock skew or a stale snapshot);
		// nothing to wait for.
		return nil
	}
	if wait > rateGuardWaitCap {
		return fmt.Errorf(
			"engine: rate-limit gate: wait of %s exceeds cap %s; %s",
			wait.Round(time.Second), rateGuardWaitCap, describePools(low),
		)
	}
	g.logger.Info(
		"rate-limit gate: waiting for reset",
		"wait", wait.Round(time.Second),
		"resume_at", latest.Add(rateGuardSlack),
	)
	return g.sleep(ctx, wait)
}

// rateGuardError builds the descriptive fail-mode error naming each low
// pool, its remaining count, and its reset time.
func rateGuardError(low []lowPool) error {
	return fmt.Errorf(
		"engine: rate-limit gate: insufficient quota (threshold %d); %s",
		rateGuardThreshold, describePools(low),
	)
}

// describePools renders the low pools into a stable human-readable
// fragment reused by both the fail error and the wait-cap error.
func describePools(low []lowPool) string {
	parts := make([]string, 0, len(low))
	for _, p := range low {
		parts = append(parts, fmt.Sprintf(
			"%s pool has %d remaining, resets at %s",
			p.name, p.remaining, p.reset.Format(time.RFC3339),
		))
	}
	return strings.Join(parts, "; ")
}

// normalizeRateGuardMode reads the mode input, lower-casing and
// defaulting unknown / absent values to "off" so a typo never silently
// enables a blocking gate.
func normalizeRateGuardMode(inputs map[string]any) string {
	v, ok := inputs[rateGuardInput]
	if !ok {
		return rateGuardOff
	}
	s, ok := v.(string)
	if !ok {
		return rateGuardOff
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case rateGuardFail:
		return rateGuardFail
	case rateGuardWait:
		return rateGuardWait
	default:
		return rateGuardOff
	}
}

// isMockedOrNoToken reports whether the token kind must bypass the gate
// (mocked transport or the explicit no-token sentinel).
func isMockedOrNoToken(kind githubapi.TokenKind) bool {
	return kind == githubapi.TokenMocked || kind == githubapi.TokenNone
}

// sleepCtx blocks for d or until ctx is cancelled, whichever comes
// first. Returns ctx.Err() on cancellation.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
