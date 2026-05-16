# Contract: base plugin の M4 拡張

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md#E-040](../data-model.md)

本書は M1 で骨格のみ実装された `internal/plugins/base` を M4 で完成形に拡張するための契約を定義する。対象は上流 task T-018 (organization branch) / T-019 (indepth クエリ) / T-020 (repositories paging) の 3 つ。

## 1. organization branch (T-018)

### 1.1 Account kind 分岐

`Run(ctx, pc)` の冒頭で `pc.Data.Account` を分岐する (既存):

```go
switch pc.Data.Account {
case "", plugins.AccountUser:
    return p.runUser(ctx, pc, login)
case plugins.AccountOrganization:
    return p.runOrganization(ctx, pc, login)  // ← M4 で実装
case plugins.AccountRepository:
    return nil, nil  // M7 territory
}
```

`AccountOrganization` は `pc.Inputs["user"]` の login を GraphQL で `organization(login:$login)` として解決した結果、`organization != nil && user == nil` の場合に決定する (`base` のクエリで両方を `OR` で取り、片方を選ぶ)。

### 1.2 organization 取得クエリ

GraphQL 1 ショットで以下を取得する。フィールド集合は **上流 `lowlighter/metrics` の queries.organization (`./org_repo/source/app/metrics/queries/organization.graphql`) と完全互換** とする (constitution 原則 I)。

```graphql
query Organization($login: String!, $repositoriesBatch: Int!, $membersBatch: Int!) {
  organization(login: $login) {
    login
    name
    description
    avatarUrl
    createdAt
    membersWithRole(first: $membersBatch) { totalCount nodes { login name avatarUrl } pageInfo { endCursor hasNextPage } }
    repositories(first: $repositoriesBatch, orderBy: {field: UPDATED_AT, direction: DESC}) {
      totalCount
      nodes { /* base.Repository と同じフィールド集合 */ }
      pageInfo { endCursor hasNextPage }
    }
  }
}
```

- `$repositoriesBatch` の既定値は `plugin_repositories_batch` (上流既定 100)。
- `$membersBatch` も同 input の値を共有 (上流挙動)。
- members / repositories はそれぞれ独立に paging する (§3 参照)。

### 1.3 `Data.Organization` への書き込み

```go
type Organization struct {
    Login       string
    Name        string
    Description string
    AvatarURL   string
    CreatedAt   time.Time
    MembersCount int
    Members      []OrgMember  // members の paging 結果を格納
    Repositories []Repository // 後述 §3 で paging 結果を格納
}
```

`pc.Data.Organization` は `Data.Account == "organization"` のときのみ non-nil MUST。user モードでは触らない。

## 2. indepth クエリ (T-019)

### 2.1 indepth 起動条件

以下のいずれかが true のとき base は indepth GraphQL を **追加で** 発行する:

- `pc.Inputs["plugin_repositories"]==true` かつ
- `pc.Inputs` のいずれかが indepth flag を立てている:
  - `plugin_repositories_pinned=true`
  - `plugin_followup` 系 (M4 では未採用なので無視)
  - `plugin_isocalendar=true`
  - `plugin_calendar=true`
  - `plugin_habits=true`
  - `plugin_notable=true` かつ `plugin_notable_indepth=true`

### 2.2 indepth クエリ内容

base 標準クエリに以下フィールドを **追加** する:

- `user.repositories.nodes` に `commits: defaultBranchRef { target { ... on Commit { history(first:0) { totalCount } } } }`
- `user.repositories.nodes` に `issues: issues { totalCount }`, `pullRequests: pullRequests { totalCount }`
- `user.contributionsCollection { contributionCalendar { totalContributions weeks { ... } } }`

これらは上流 `queries.user.indepth` と完全互換。

### 2.3 indepth エラー時の degraded path

indepth クエリが GraphQL secondary rate limit / timeout で失敗したとき:

1. `Result.Errors` に `*RetryableError` を 1 件追加
2. base 標準クエリの結果はそのまま `Data.Computed` に書き込む
3. indepth 限定フィールド (`Computed.TotalCommits` 等) は zero value のまま
4. `Run` は `(nil, nil)` で正常完了 MUST (panic / hard error にしない)

## 3. repositories paging (T-020)

### 3.1 paging 戦略 (batch-halving)

上流 `./org_repo/source/app/metrics/utils.mjs::nearestRepositoriesBatchSize` のロジックを Go で再実装する:

```text
batch_start = plugin_repositories_batch (default 100)
cursor = nil
acc = []

loop:
  try: query(first: batch_start, after: cursor)
  if 502 or timeout:
    batch_start = max(1, batch_start / 2)
    continue (同じ cursor で retry)
  acc += result.nodes
  if !result.pageInfo.hasNextPage: break
  cursor = result.pageInfo.endCursor
```

### 3.2 結果の Compute への反映

`pc.Data.Computed.Repositories = acc` を最終的に書き込む。各 `Repository` 構造体のフィールドは [data-model.md E-015 Repository](../data-model.md#E-015) と一致する。

### 3.3 paging エラー時の degraded path

batch=1 でも 5xx / timeout が継続する場合:

1. それまでに取得済の `acc` をそのまま `Computed.Repositories` に書き込む (部分結果)
2. `Result.Errors` に `*RetryableError` を 1 件追加 (`base: repositories paging: batch=1 failed after N retries`)
3. `Run` は `(nil, nil)` で正常完了

## 4. テスト

- `internal/plugins/base/base_test.go::TestRun_User` — user モードの table test (M1 既存)
- `internal/plugins/base/base_test.go::TestRun_Organization` — 新規。`Organization` 構造体が正しく埋まる
- `internal/plugins/base/indepth_test.go::TestIndepthQuery_Triggered` — indepth クエリの起動条件 (5 ケース) と結果集約
- `internal/plugins/base/repositories_test.go::TestPaging_BatchHalving` — 502 を 2 回返してから 200 を返す mocked GraphQL で batch が 100 → 50 → 25 に削減されつつ完走することを assert
- `internal/plugins/base/repositories_test.go::TestPaging_PartialFailure` — batch=1 でも 5xx 継続のケース。`Errors` に 1 件、`Computed.Repositories` は部分結果

## 5. 性能予算

- organization branch: < 500ms (mocked, 1 organization + 100 members + 100 repos)
- indepth クエリ: < 800ms (mocked、base 標準 + indepth 追加 = 2 GraphQL リクエスト)
- repositories paging (1000 件 / 10 ページ): < 2s (mocked)

合計 base 全体で < 3s を確保し、他 20 plugin と並列実行しても Compute フルパス SC-003 (< 5s) に収まる予算配分。
