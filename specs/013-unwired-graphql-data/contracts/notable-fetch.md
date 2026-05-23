# Contract: notable plugin Run() — data fetch

## Inputs

- `pc.GraphQL`: required
- `pc.Inputs["plugin_notable_limit"]` (default 5)

## GraphQL Query

```graphql
query ViewerNotable($first: Int!) {
  viewer {
    repositories(first: $first, ownerAffiliations: [OWNER], orderBy: {field: STARGAZERS, direction: DESC}) {
      nodes {
        nameWithOwner
        description
        stargazerCount
        forkCount
        isFork
        isPrivate
      }
    }
  }
}
```

## Output (Result)

- `Skipped=false`
- `List`: top N owned repos (fork/private を除外しない方針 — upstream parity)

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `pc.GraphQL == nil` | `Skipped=true`, `SkippedReason="GraphQL client unavailable"` |
| `pc.GraphQL.RateBudget() < 5` | `Skipped=true` + RetryableError |
| GraphQL 5xx | `Skipped=true` + RetryableError |
| 空配列 | `Skipped=false`, `List=[]` |

## Out of Scope

- pinned discussion / latest release / indepth は 014 以降 (Clarifications Q1)
- `Role` フィールドは現状 "Maintainer" 固定 (basic mode)
- organization mode は対象外 (R-006)
- partial DOM は変更しない
