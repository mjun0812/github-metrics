# Contract: repositories.Pinned plugin Run() — data fetch (本 spec 範囲は Pinned のみ)

## Inputs

- `pc.GraphQL`: required (nil の場合は Featured コピー fallback)
- `pc.Inputs["plugin_repositories_pinned"]`: bool (default false。既存 input)

## GraphQL Query

```graphql
query ViewerPinnedItems {
  viewer {
    pinnedItems(first: 6, types: [REPOSITORY]) {
      nodes {
        ... on Repository {
          nameWithOwner
          description
          stargazerCount
          forkCount
          isFork
          isPrivate
          primaryLanguage { name color }
          url
        }
      }
    }
  }
}
```

## Output (Result)

- `Result.Pinned`: pin 順 (最大 6 件) の `plugins.Repository` slice
- ピンが 0 件: `Pinned=[]` (空配列。Skipped にしない)

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `pc.GraphQL == nil` | Featured のコピー fallback (M4 baseline と互換、FR-006 of 012 と同方針) |
| `pc.GraphQL.RateBudget() < 3` | `Pinned=nil` + Data.Errors に RetryableError |
| GraphQL 5xx | `Pinned=nil` + Data.Errors に RetryableError |
| 空配列 | `Pinned=[]` |

## Out of Scope

- Starred は 012 PR で実 fetch 化済
- Featured / Random の挙動は本 spec で変更しない
- partial DOM は変更しない (spec 011 確定済)
