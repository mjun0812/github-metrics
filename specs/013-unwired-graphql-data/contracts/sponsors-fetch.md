# Contract: sponsors plugin Run() — data fetch

## Inputs

- `pc.GraphQL`: required (test path で nil の場合は既存 fallback)
- `pc.Inputs["user"]`: viewer の login (sanity check のみ。実 fetch は viewer 起点)
- `pc.Inputs["plugin_sponsors_size"]` (default 12): active + past 上限
- `pc.GraphQL.Scopes(ctx)`: 既存 scope-gate ロジック

## GraphQL Query

```graphql
query ViewerSponsors($activeFirst: Int!, $pastFirst: Int!) {
  viewer {
    sponsorsListing {
      shortDescription
      activeGoal {
        title
        description
        kind
        percentComplete
      }
    }
    active: sponsorshipsAsMaintainer(first: $activeFirst, activeOnly: true, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        sponsor: sponsorEntity {
          __typename
          ... on User { login avatarUrl }
          ... on Organization { login avatarUrl }
        }
      }
    }
    past: sponsorshipsAsMaintainer(first: $pastFirst, activeOnly: false, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        isActive
        sponsor: sponsorEntity {
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
- `Active`: list (sponsor の `Login` / `AvatarURL` / `EntityType`)
- `Past`: `past.nodes` のうち `isActive=false` のみ抽出
- `Goal`: nil (listing.activeGoal が nil) または populated
- `About`: `sponsorsListing.shortDescription`

## Failure Modes

| 状況 | 挙動 |
|---|---|
| `pc.GraphQL == nil` | `Skipped=true`, `SkippedReason="GraphQL client unavailable"`, fallback (M4 baseline と同様 empty Result) |
| scope 不足 (`read:user` + `read:org`) | 既存 scope-gate path 維持: `Skipped=true`, `SkippedReason="missing read:user / read:org scope"` |
| `pc.GraphQL.RateBudget() < 5` | `Skipped=true`, `SkippedReason="rate budget too low"`, `*RetryableError` を Data.Errors に push |
| GraphQL 5xx / network error | `Skipped=true`, `SkippedReason="GraphQL fetch failed: <err>"`, `*RetryableError` を Data.Errors |
| `viewer.sponsorshipsAsMaintainer` が空 | `Skipped=false`, `Active=[]`, `Past=[]`, `Goal=nil` (Skipped にはしない) |

## Out of Scope

- tier (`monthlyPriceInDollars`) は取得しない (R-004)
- organization mode は対象外 (R-006)
- partial DOM (spec 011 確定) は変更しない
