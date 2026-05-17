package action

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

func TestRetry_FirstSuccess(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func() error { calls++; return nil })
	if err != nil {
		t.Errorf("err = %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_RetryableThenSuccess(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		if calls == 1 {
			return xerrors.NewRetryableError(errors.New("transient"))
		}
		return nil
	})
	if err != nil {
		t.Errorf("err = %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestRetry_RetryableExhausted(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	calls := 0
	want := errors.New("transient")
	err := p.Do(context.Background(), func() error {
		calls++
		return xerrors.NewRetryableError(want)
	})
	if err == nil {
		t.Fatalf("expected error after exhaustion")
	}
	var re *xerrors.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("err type = %T, want *RetryableError", err)
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4 (Retries=3 + 1)", calls)
	}
}

func TestRetry_NonRetryable_FailFast(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	calls := 0
	want := errors.New("plain")
	err := p.Do(context.Background(), func() error {
		calls++
		return want
	})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want plain sentinel via errors.Is", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (fail-fast)", calls)
	}
}

func TestRetry_WrappedRetryableErrorDetected(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		return fmt.Errorf("engine: %w", xerrors.NewRetryableError(errors.New("transient")))
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4 (errors.As must unwrap)", calls)
	}
}

func TestRetry_ContextCancel_DuringDelay(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: 100 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	err := p.Do(ctx, func() error {
		calls++
		return xerrors.NewRetryableError(errors.New("transient"))
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if calls < 1 || calls > 4 {
		t.Errorf("calls = %d, expected 1..4", calls)
	}
}

func TestRetry_ContextCancel_BeforeAttempt(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, Delay: time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := p.Do(ctx, func() error { calls++; return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0 (cancel before first attempt)", calls)
	}
}

func TestRetry_RetriesZero(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 0, Delay: time.Millisecond}
	calls := 0
	_ = p.Do(context.Background(), func() error {
		calls++
		return xerrors.NewRetryableError(errors.New("x"))
	})
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (Retries=0 = single attempt)", calls)
	}
}
