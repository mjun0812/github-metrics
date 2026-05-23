# Data Model: 未配線 GraphQL plugin の data-fetch wiring (013)

各 plugin の Result 型は M4 / 011 で既に定義済。本 spec で「fill する」フィールドと「新規追加する」フィールドを以下に明示する。partial DOM (spec 011 確定) は変更しない。

## 1. sponsors.Result

| Field | Type | Status | Source (GraphQL) |
|---|---|---|---|
| `Skipped` | `bool` | filled (now: true → false on happy) | computed |
| `SkippedReason` | `string` | filled | computed |
| `Active` | `[]Sponsor` | **filled (was empty)** | `viewer.sponsorshipsAsMaintainer(activeOnly: true)` |
| `Past` | `[]Sponsor` | **filled (was empty)** | `viewer.sponsorshipsAsMaintainer(activeOnly: false)` − Active |
| `Goal` | `*Goal` | **filled (was nil)** | `viewer.sponsorsListing.goal` |
| `About` | `string` | **filled** | `viewer.sponsorsListing.shortDescription` |

`Sponsor`:

| Field | Type | Notes |
|---|---|---|
| `Login` | `string` | `sponsor.login` |
| `AvatarURL` | `string` | `sponsor.avatarUrl` |
| `EntityType` | `string` | "User" or "Organization" — `sponsor.__typename` |

`Goal`:

| Field | Type | Notes |
|---|---|---|
| `Title` | `string` | `goal.title` |
| `Description` | `string` | `goal.description` |
| `PercentComplete` | `int` | 0-100 |
| `Kind` | `string` | "TOTAL_SPONSORS_COUNT" / "MONTHLY_SPONSORSHIP_AMOUNT" |

**Note**: tier (`monthlyPriceInDollars`) は **本 spec の Result に含めない** (R-004)。

---

## 2. sponsorships.Result

| Field | Type | Status | Source |
|---|---|---|---|
| `Skipped` | `bool` | filled | computed |
| `SkippedReason` | `string` | filled | computed |
| `List` | `[]Maintainer` | **filled (was empty)** | `viewer.sponsorshipsAsSponsor(first:N)` |

`Maintainer`:

| Field | Type | Notes |
|---|---|---|
| `Login` | `string` | `sponsorable.login` |
| `AvatarURL` | `string` | `sponsorable.avatarUrl` |
| `EntityType` | `string` | "User" or "Organization" |

---

## 3. projects.Result

| Field | Type | Status | Source |
|---|---|---|---|
| `Skipped` | `bool` | filled | computed |
| `SkippedReason` | `string` | filled | computed |
| `Mode` | `AggregationMode` | unchanged (既存) | from `pc.Data.Computed` |
| `Limit` | `int` | unchanged (既存) | from `plugin_projects_limit` (default 4) |
| `List` | `[]Project` | **filled (was empty)** | `viewer.projectsV2(first:Limit, query: open)` |

`Project`:

| Field | Type | Notes |
|---|---|---|
| `Title` | `string` | `title` |
| `ShortDescription` | `string` | `shortDescription` |
| `UpdatedAt` | `time.Time` | `updatedAt` |
| `URL` | `string` | `url` |
| `Closed` | `bool` | `closed` (filter: open only by default) |

---

## 4. notable.Result

| Field | Type | Status | Source |
|---|---|---|---|
| `Skipped` | `bool` | filled | computed |
| `SkippedReason` | `string` | filled | computed |
| `List` | `[]NotableContribution` | **filled (was empty / Skipped)** | `viewer.repositories(first:N, ownerAffiliations:[OWNER], orderBy: STARGAZERS DESC)` |

`NotableContribution`:

| Field | Type | Notes |
|---|---|---|
| `NameWithOwner` | `string` | `nameWithOwner` |
| `Description` | `string` | `description` |
| `StargazerCount` | `int` | `stargazerCount` |
| `ForkCount` | `int` | `forkCount` |
| `Role` | `string` | "Maintainer" (basic mode 固定。indepth で "Contributor" 等を導入予定 — 014) |

---

## 5. stargazers.Result

| Field | Type | Status | Source |
|---|---|---|---|
| `Skipped` | `bool` | filled | computed |
| `SkippedReason` | `string` | filled | computed |
| `Mode` | `AggregationMode` | unchanged (既存) | from `pc.Data.Computed` |
| `Chart` | `Chart` | **filled (was empty)** | computed from owned repos' stargazers |

`Chart`:

| Field | Type | Notes |
|---|---|---|
| `Title` | `string` | "Stargazers" + optional " (latest N)" hint |
| `Series` | `[]MonthPoint` | **NEW (additive)** — month bucket cumulative |

`MonthPoint` (**新規追加**):

| Field | Type | Notes |
|---|---|---|
| `Month` | `time.Time` | 月初 (`YYYY-MM-01T00:00:00Z`) |
| `Cumulative` | `int` | この月時点での累積 star 数 (全 owned repos 合算) |

**Note**: 既存 `Result.Chart` 型を拡張する形。JSON tag 順は v1.0.0 を厳密に維持。

---

## 6. repositories.Result (Pinned のみ本 spec で更新)

| Field | Type | Status | Source |
|---|---|---|---|
| `Pinned` | `[]plugins.Repository` | **filled (was Featured コピー)** | `viewer.pinnedItems(first:6, types:[REPOSITORY])` |
| `Featured` | `[]plugins.Repository` | unchanged | (既存 M4 baseline) |
| `Starred` | `[]plugins.Repository` | unchanged | (012 PR で実 fetch 化済) |
| `Random` | `[]plugins.Repository` | unchanged | (M4 baseline で Fisher-Yates 実装済) |

`plugins.Repository` 型は変更しない。`Pinned` フィールドの値が Featured のコピーから本物の pin に切り替わるだけで shape は不変。

---

## 7. State Transitions

各 plugin の Run() における Result.Skipped の遷移:

```
[before 013]
  Skipped=true  (Run が直接返すか、scope 不足で gate された)

[after 013]
  ┌─ GraphQL == nil ────────────→ Skipped=true (test path 互換維持 / FR-001)
  ├─ scope insufficient ────────→ Skipped=true (existing scope-gate)
  ├─ rate budget < estimated ───→ Skipped=true (R-005)
  ├─ HTTP 5xx ──────────────────→ Skipped=true + Data.Errors に *RetryableError
  └─ happy ─────────────────────→ Skipped=false + populated fields
```

すべての分岐で他 plugin に副作用を出さない (`internal/engine/dispatch.go` の per-plugin isolation を遵守)。

## 8. JSON Shape の Backward Compatibility

- v1.0.0 で既に shipped している JSON フィールドの **名前 / 型 / 順序は変更しない** (Constitution §II)。
- 本 spec で「埋める」フィールドは shape 上は最初から存在していたもの (omitempty で消えていただけ)。
- 唯一の新規追加は `stargazers.Chart.Series []MonthPoint` で、これは additive (v1.0.0 では `Chart.Series` が常に nil でフィールド自体は struct に存在)。
