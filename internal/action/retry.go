package action

import (
	"context"
	"errors"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// DefaultRetries is the default `retries` input value when none is
// supplied (action.yml + CLI both default to this). Spec FR-007.
const DefaultRetries = 3

// DefaultRetryDelay is the default `retries_delay` input value when
// none is supplied. action.yml declares the input in seconds with a
// default of 300 (upstream parity). Spec FR-007.
const DefaultRetryDelay = 300 * time.Second

// DefaultOutputRetries is the default `retries_output_action` input
// value when none is supplied. action.yml defaults to 5 (upstream
// parity). This governs the committer's GitHub API calls, distinct
// from the rendering `retries`.
const DefaultOutputRetries = 5

// DefaultOutputRetryDelay is the default `retries_delay_output_action`
// input value when none is supplied. action.yml declares it in seconds
// with a default of 120 (upstream parity).
const DefaultOutputRetryDelay = 120 * time.Second

// RetryPolicy controls retry-on-RetryableError behavior for fn calls
// (typically engine.Compute or committer API operations).
type RetryPolicy struct {
	Retries int           // total attempts = Retries + 1
	Delay   time.Duration // wait between attempts
}

// Do invokes fn up to (Retries + 1) times. It retries when fn's
// returned error is wrapped (anywhere in the chain) in
// *xerrors.RetryableError. Any other error class fails fast — no
// further attempts, return immediately.
//
// errors.As is used (research R-002) so wrapped chains like
// fmt.Errorf("engine: %w", retryErr) are recognized correctly.
//
// Context cancellation aborts the loop; ctx.Err() is returned even
// when fn's last call had not yet failed.
func (p RetryPolicy) Do(ctx context.Context, fn func() error) error {
	attempts := p.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		var re *xerrors.RetryableError
		if !errors.As(err, &re) {
			// Non-retryable: fail-fast.
			return err
		}

		if i < attempts-1 {
			timer := time.NewTimer(p.Delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}
	return lastErr
}
