# Contract: sponsorships plugin Run() — data fetch

## Inputs

- `pc.GraphQL`: required
- `pc.Inputs["plugin_sponsorships_size"]` (default 12)

## GraphQL Query

```graphql
query ViewerSponsorships($first: Int!) {
  viewer {
    sponsorshipsAsSponsor(first: $first, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        sponsorable {
          __typename
          ... on User { login avatarUrl }
          ... on Organization { login avatarUrl }
        }
      }
    }
  }
}
```

## Output (Result)

- `Skipped=false`
- `List`: 各 `Maintainer` の `Login` / `AvatarURL` / `EntityType`

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `pc.GraphQL == nil` | `Skipped=true`, `SkippedReason="GraphQL client unavailable"` |
| scope 不足 | 既存 scope-gate 維持 |
| `pc.GraphQL.RateBudget() < 3` | `Skipped=true` + RetryableError |
| GraphQL 5xx | `Skipped=true` + RetryableError |
| 空配列 | `Skipped=false`, `List=[]` |

## Out of Scope

- past sponsorships (= cancelled) は本 spec では取得しない。upstream parity の表示が active のみ。
- partial DOM は変更しない
