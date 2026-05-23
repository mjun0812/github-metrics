# Contract: projects plugin Run() — data fetch

## Inputs

- `pc.GraphQL`: required
- `pc.Inputs["plugin_projects_limit"]` (default 4)
- token scope: `read:project`

## GraphQL Query

```graphql
query ViewerProjects($first: Int!) {
  viewer {
    projectsV2(first: $first, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        title
        shortDescription
        updatedAt
        url
        closed
      }
    }
  }
}
```

## Output (Result)

- `Skipped=false`
- `Mode`: 既存 `plugins.AggregationMode(pc.Data)` の戻り値
- `Limit`: from input
- `List`: 各 Project の field を上記 mapping で詰める。`closed=true` は除外しない (upstream parity)

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `pc.GraphQL == nil` | `Skipped=true`, `SkippedReason="GraphQL client unavailable"` (既存挙動と同等) |
| scope 不足 (`read:project`) | 既存 scope-gate 維持: `Skipped=true`, `SkippedReason="missing read:project scope"` |
| `pc.GraphQL.RateBudget() < 3` | `Skipped=true` + RetryableError |
| GraphQL 5xx | `Skipped=true` + RetryableError |
| 空配列 | `Skipped=false`, `List=[]` |

## Out of Scope

- organization mode は対象外 (R-006)
- partial DOM は変更しない
