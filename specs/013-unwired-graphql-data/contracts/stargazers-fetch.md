# Contract: stargazers plugin Run() — data fetch

## Inputs

- `pc.GraphQL`: required
- `pc.Inputs["plugin_stargazers_limit"]` (default 10, owned repos の取得上限)
- `pc.Data.Computed.AccountKind`: "user" の場合のみ動作 (organization は backlog)

## GraphQL Query

```graphql
query ViewerStargazersRepos($first: Int!) {
  viewer {
    repositories(first: $first, ownerAffiliations: [OWNER], orderBy: {field: STARGAZERS, direction: DESC}) {
      nodes {
        nameWithOwner
        stargazers(first: 100, orderBy: {field: STARRED_AT, direction: DESC}) {
          edges {
            starredAt
          }
        }
      }
    }
  }
}
```

## Aggregation Logic

```
collect = map[YYYY-MM] += 1 across all owned repos' stargazers.edges[].starredAt
sort by month asc
cumulative = running sum
Series = [{Month: month-start UTC, Cumulative: running_sum} for each month]
```

最新 100 件超過の repo は "Showing latest 100 stars" hint を `Chart.Title` 末尾に括弧書きで添える。

## Output (Result)

- `Skipped=false` (account kind == "user")
- `Mode`: 既存 `plugins.AggregationMode(pc.Data)`
- `Chart.Title`: `"Stargazers"` + optional ` (latest 100)` suffix
- `Chart.Series`: `[]MonthPoint{{Month, Cumulative}, ...}` ← 新規 additive

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `AccountKind != "user"` | `Skipped=true`, `SkippedReason="stargazers requires user account kind"` (既存 M4 メッセージを更新) |
| `pc.GraphQL == nil` | `Skipped=true`, `SkippedReason="GraphQL client unavailable"` |
| `pc.GraphQL.RateBudget() < 30` | `Skipped=true` + RetryableError |
| GraphQL 5xx | `Skipped=true` + RetryableError |
| `viewer.repositories` 空 | `Skipped=false`, `Chart.Series=[]` |

## Out of Scope

- repository mode (specific repo の stargazers chart) は M7 territory (継続 Skipped)
- organization mode (R-006)
- 全件 paginate (R-002)
- partial DOM は変更しない
