# Contract: Token validation

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-007

本書は token 入力の起動時 validation 3 段階 (型 reject / scope check / quota check) の判定マトリクスを定める。spec FR-004 + SC-010 の 7 retry/quota/token paths の根拠。

## 1. 段階 1: token 形式 reject

| Token prefix | 判定 | Action |
|--------------|------|--------|
| `ghp_*` (classic PAT) | OK | continue to 段階 2 |
| `gho_*` (OAuth user-to-server token) | OK | continue to 段階 2 |
| `ghs_*` (GitHub Apps server-to-server) | OK | continue to 段階 2 |
| `ghu_*` (GitHub Apps user-to-server) | OK | continue to 段階 2 |
| `github_pat_*` (fine-grained PAT) | **REJECT** | exit 1 + message |
| `MOCKED_TOKEN` | OK (mock-only) | continue (use_mocked_data 用の特例) |
| 空文字列 (`""`) + use_mocked_data=true | OK | continue (mock 専用経路) |
| 空文字列 + use_mocked_data=false | **REJECT** | exit 1 (token required) |
| その他 (未知 prefix、長さ不正等) | **WARNING** | continue (上流互換のため柔軟に) |

### `github_pat_*` reject message

```text
Error: fine-grained personal access tokens (github_pat_*) are not supported.
The metrics action requires GitHub-scope tokens that are difficult to express
in the fine-grained model (REST + GraphQL + Search rate limits + multiple
plugin-required scopes). Please use a classic personal access token (ghp_*)
instead. See https://github.com/settings/tokens for token generation.
```

### Token required message (token empty + use_mocked_data=false)

```text
Error: token is required to fetch real GitHub data.
Either:
  - Pass a valid PAT via `INPUT_TOKEN` (action.yml) or `--token-env <NAME>` (CLI),
  - Or set `use_mocked_data: yes` (action.yml) / `--plugin use_mocked_data=true` (CLI) for offline demo.
```

## 2. 段階 2: scope check (`HEAD /`)

```text
1. tokenValidator が `HEAD /` を REST client で叩く
2. Response の `X-OAuth-Scopes` header から現在の token scope を取得
3. RequiredScopes (採用 21 plugin の metadata.yml の scopes union) と diff:
   missing = RequiredScopes - X-OAuth-Scopes
4. missing が空でなくても、本 step では **action は止めない** — warning ログを出してそのまま継続
5. 該当 plugin (例: traffic plugin) は実際の Compute 時に scope 不足を検出して
   data.Plugins[<name>].skipped=true を set + ログ 1 行 (M4 で確立済)
```

### RequiredScopes union (21 採用 plugin の scope 集合)

| Scope | 必要 plugin |
|-------|------------|
| `repo` (public_repo で代用可) | traffic |
| `read:user` | sponsors, sponsorships |
| `read:org` | sponsors |
| `read:project` | projects |
| `read:discussion` | (未採用) |
| `read:packages` | (未採用) |
| (なし、public 情報のみ) | languages, activity, achievements, repositories, isocalendar, calendar, habits, stars, people, notable, contributors, reactions, stargazers, topics, starlists, languages.recent, languages.indepth |

### scope warning message

```text
Warning: token is missing the following scopes that some plugins require:
  - repo (needed by: traffic)
  - read:project (needed by: projects)
The corresponding plugins will return skipped=true in the output.
To enable them, regenerate the token with the missing scopes.
```

## 3. 段階 3: quota check (`GET /rate_limit`)

```text
1. tokenValidator が `GET /rate_limit` を叩く
2. Response から rate.{rest,graphql,search}.remaining を取得
3. plugin metadata の quota_required_{rest,graphql,search} を union (sum)
4. 各 API 種別ごとに:
   if remaining < quota_required:
     QuotaSufficient = false
5. QuotaSufficient=false なら:
   action は **exit 0 (skipped log を出して終了)** — workflow を fail させない (FR-004)
   warning ログ: "Insufficient API quota; current run skipped. retry at {reset_time}."
```

### quota check が exit 0 (skipped) になる理由

- 上流 lowlighter/metrics 互換の挙動。GitHub schedule で 30 分おきに workflow が走る運用で、毎回 quota check fail で **`failed` mark** が付くと、ユーザの GitHub UI が常時赤くなる
- skipped log で「次の reset まで待つ」と明示すれば、ユーザは workflow 履歴で skipped を見て理解できる
- 本当に致命的な (workflow 全体を止めるべき) quota 不足 = 24 時間連続で枯渇のパターンは、現状のロジックでも skipped ログが連続出力されるので grep / alert で検出可

### quota required value (per plugin)

各 plugin の metadata.yml に `quota_required.rest|graphql|search` フィールドがあれば accumulate。明示が無い plugin は (REST: 1, GraphQL: 1, Search: 0) の **conservative default** で計算 (FR-004 の "verify quota" 解釈)。

## 4. 判定マトリクス (decision table)

| Step | OK condition | Failure action | Exit code |
|------|--------------|----------------|-----------|
| 1: token format | prefix が `ghp_/gho_/ghs_/ghu_/MOCKED_TOKEN` or (空 + use_mocked_data) | reject message | 1 |
| 1: token empty + real | n/a | token required message | 1 |
| 2: scope check | `X-OAuth-Scopes` ⊇ RequiredScopes | warning ログ + continue | continue |
| 3: quota check | 各 API remaining ≥ quota_required | skipped ログ + exit | **0** (skipped) |
| 3: rate_limit HEAD/GET 自体が 5xx | RetryableError | retry (FR-007) | (retry) |

## 5. 実装 (E-007 TokenValidator)

```go
package action

type TokenValidator struct {
    Token          config.Token
    REST           *githubapi.REST
    RequiredScopes []string
    QuotaRest      int
    QuotaGraphQL   int
    QuotaSearch    int
}

func (v *TokenValidator) Validate(ctx context.Context) (ValidationResult, error) {
    // Step 1: format
    raw := v.Token.String() // masked-on-stringify は別ロジック、ここでは raw を見る必要がある
    if strings.HasPrefix(raw, "github_pat_") {
        return ValidationResult{}, &InputError{Key: "token", Msg: githubPatRejectMessage}
    }
    if raw == "" && !v.useMockedData {
        return ValidationResult{}, &InputError{Key: "token", Msg: tokenRequiredMessage}
    }
    if raw == "MOCKED_TOKEN" {
        return ValidationResult{QuotaSufficient: true}, nil // mock 経路は scope / quota check skip
    }

    // Step 2: scope
    scopes, err := v.fetchScopes(ctx) // HEAD /
    if err != nil {
        return ValidationResult{}, xerrors.NewRetryableError(err)
    }
    missing := diff(v.RequiredScopes, scopes)

    // Step 3: quota
    rate, err := v.fetchRateLimit(ctx) // GET /rate_limit
    if err != nil {
        return ValidationResult{}, xerrors.NewRetryableError(err)
    }
    sufficient := rate.REST >= v.QuotaRest &&
                  rate.GraphQL >= v.QuotaGraphQL &&
                  rate.Search >= v.QuotaSearch

    return ValidationResult{
        MissingScopes:    missing,
        QuotaSufficient:  sufficient,
        QuotaResetAt:     rate.ResetAt,
    }, nil
}
```

## 6. テスト戦略 (SC-010 + SC-011)

`internal/action/token_test.go`:

1. `TestValidator_GithubPatRejected` — `github_pat_xxx` → InputError + 期待 message 部分一致
2. `TestValidator_TokenMissingNoMock` — `Token=""`, `use_mocked_data=false` → InputError
3. `TestValidator_TokenMissingWithMock` — `Token=""`, `use_mocked_data=true` → pass (ValidationResult.QuotaSufficient=true)
4. `TestValidator_MockedToken` — `Token=MOCKED_TOKEN` → pass (network 呼び出しなし)
5. `TestValidator_ScopeMissing_Warning` — `HEAD /` mock で X-OAuth-Scopes が部分集合 → MissingScopes に該当 scope が入る + 戻り値 error は nil (=continue)
6. `TestValidator_QuotaInsufficient_Skipped` — `GET /rate_limit` mock で REST remaining=10 < QuotaRest=100 → QuotaSufficient=false (caller が exit 0 skipped で抜ける)
7. `TestValidator_RateLimitFetch5xx_Retryable` — `GET /rate_limit` mock で 503 → RetryableError (FR-007 retry に乗る)

`internal/action/token_mask_test.go` (SC-011 token masking):

1. `TestTokenMask_BannerOutput` — banner に `<PAT>` が含まれない (token は `ghp_***masked***` 表記)
2. `TestTokenMask_ErrorWrap` — engine.Compute 失敗時の error string に `<PAT>` が含まれない
3. `TestTokenMask_RetryLog` — retry warning ログに `<PAT>` が含まれない
4. `TestTokenMask_SlogField` — `slog.Info("compute failed", "token", token)` で `<PAT>` が含まれない (config.Token の Stringer が機能していること)
