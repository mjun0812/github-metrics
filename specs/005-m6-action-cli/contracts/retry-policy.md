# Contract: Retry policy

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-006

本書は spec clarification Q3 で確定した「`*xerrors.RetryableError` でラップされた error のみ retry、それ以外 fail-fast」の判定論理 + retry budget consumption rule を定める。

## 1. 判定ロジック (research R-002 で確定)

```go
package action

import (
    "context"
    "errors"
    "time"

    xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

type RetryPolicy struct {
    Retries int           // default 3
    Delay   time.Duration // default 300ms
}

// Do invokes fn up to Retries+1 times.
// Retries only when the error is wrapped in *xerrors.RetryableError.
// All other errors fail fast.
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
            // Non-retryable: fail-fast (no further attempts).
            return err
        }

        if i < attempts-1 {
            select {
            case <-time.After(p.Delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    return lastErr
}
```

**重要**: `errors.As(err, &re)` を使う (research R-002)。`err.(*xerrors.RetryableError)` の direct type assertion は wrapper チェーン (例: `fmt.Errorf("engine: %w", retryErr)`) で false になるため使わない。

## 2. retry budget consumption

| シナリオ | attempts 消費 | 結果 |
|---------|---------------|------|
| 1 回目で success | 1 | nil return |
| 1 回目 RetryableError、2 回目 success | 2 | nil return |
| 全 4 回 RetryableError (Retries=3, attempts=4) | 4 | 最後の error を return |
| 1 回目 non-retryable (例: 4xx, config error) | 1 | 即 return (残り 3 回は使わない、fail-fast) |
| context cancel during delay | < 4 | `ctx.Err()` を return |

## 3. retry 対象とする error class (FR-007)

| Error class | Source | Retry? |
|-------------|--------|--------|
| `*xerrors.RetryableError` (engine 5xx wrap) | M4 で確立 | ✅ Yes |
| `*xerrors.RetryableError` (network timeout wrap) | M1 httpx middleware | ✅ Yes |
| `*xerrors.RetryableError` (rate-limit 429 wrap) | M1 httpx middleware | ✅ Yes |
| 通常の `error` (4xx HTTP, login not found, repo not visible) | engine | ❌ No (fail-fast exit 1) |
| `*xerrors.InputError` (token=github_pat_*, missing required input) | M6 action token.go | ❌ No (fail-fast exit 1) |
| `*ConfigError` (output_action 未対応値) | M6 output_action.go | ❌ No (fail-fast exit 1) |
| `context.DeadlineExceeded` / `context.Canceled` | ctx | ❌ No (即 return) |

## 4. デフォルト値

`retries` / `retries_delay` 入力 (action.yml / CLI):

| Input | Default | Range | Source |
|-------|---------|-------|--------|
| `retries` | 3 | 0-10 (over 10 で warning + clamp) | spec FR-007 |
| `retries_delay` | 300ms | 0-30000ms (over 30s で warning + clamp) | spec FR-007 |

`Retries=0` で「retry なし、1 回試行のみ」(fail-fast 専用)。CLI mode で debug 時に `--plugin retries=0` で diagnose しやすくする。

## 5. usage in action.go

```go
package action

func (a *Invocation) Compute(ctx context.Context, deps engine.Deps) (*engine.Result, error) {
    policy := RetryPolicy{
        Retries: readInt(a.Inputs, "retries", 3),
        Delay:   time.Duration(readInt(a.Inputs, "retries_delay", 300)) * time.Millisecond,
    }
    var res *engine.Result
    err := policy.Do(ctx, func() error {
        var e error
        res, e = engine.Compute(ctx, a.toRequest(), deps)
        return e
    })
    return res, err
}
```

同じ pattern を `Committer.PutContents` / `Committer.CreatePR` / `Committer.Merge` などにも適用 ([committer.md §5](./committer.md#5-retry-policy-適用範囲) 参照)。

## 6. テスト戦略 (SC-010: 7 retry/quota/token paths)

`internal/action/retry_test.go`:

1. `TestRetry_FirstSuccess` — fn が一発で nil
2. `TestRetry_RetryableThenSuccess` — fn が 1 回 RetryableError → 2 回目 success
3. `TestRetry_RetryableExhausted` — fn が全 4 回 RetryableError → 最後の error return
4. `TestRetry_NonRetryable_FailFast` — fn が plain error 1 回 → 残り retry は使われない (call counter で確認)
5. `TestRetry_ContextCancel_DuringDelay` — Delay 中に ctx cancel → ctx.Err() return
6. `TestRetry_ContextCancel_BeforeAttempt` — fn 呼び出し前に ctx cancel → ctx.Err() return
7. `TestRetry_RetriesZero` — Retries=0 で 1 回試行のみ
8. `TestRetry_WrappedRetryableError` — `fmt.Errorf("engine: %w", retryErr)` が errors.As で正しく検出される (research R-002 担保)

`tests/integration/action_test.go` で end-to-end (engine mocked) も 1 ケースカバー (`TestAction_RetryableError_Retried`)。
