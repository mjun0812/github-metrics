package contributors

import (
	"context"
	"time"
)

// StatsPendingMaxAttempts exposes the 202 retry bound to external tests.
const StatsPendingMaxAttempts = statsPendingMaxAttempts

// ContributorsMaxPages exposes the fallback pagination cap to external
// tests.
const ContributorsMaxPages = contributorsMaxPages

// SetSleepFn swaps the 202-retry sleep implementation for tests and
// returns a restore function. External tests use this to drive the
// retry loop without real wall-clock delays.
func SetSleepFn(fn func(ctx context.Context, d time.Duration)) (restore func()) {
	prev := sleepFn
	sleepFn = fn
	return func() { sleepFn = prev }
}
