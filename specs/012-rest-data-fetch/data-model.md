# Phase 1 Data Model: REST Data-Fetch Wiring

**Feature**: `012-rest-data-fetch` | **Date**: 2026-05-22

本 feature は **既存 Result struct のフィールドを populate するだけ**で、新規 struct の追加は最小限。以下は touch する Result の現状 + 本 feature での変更を明示する。

## E-012-A: `traffic.Result` (既存、フィールド populate のみ)

```go
type Result struct {
    Skipped       bool                   `json:"skipped,omitempty"`
    SkippedReason string                 `json:"-"`
    Views         map[string]TrafficView `json:"views"`  // ★ 本 feature で populate
    Total         TrafficView            `json:"total"`  // ★ 本 feature で populate
}

type TrafficView struct {
    Count   int `json:"count"`
    Uniques int `json:"uniques"`
}
```

| Field | Before | After | 由来 |
|---|---|---|---|
| `Skipped` | 常に true | repo scope 無し時のみ true | scope check |
| `Views[repo]` | 空 map | per-repo `{count, uniques}` | `/repos/{repo}/traffic/views` |
| `Total` | zero value | 全 repo の count/uniques 和 | aggregate |

**Validation rules**:
- `repo` scope 無し時 → `Skipped=true`, `SkippedReason="missing repo scope"`
- `RepositoryList` 空 → `Skipped=false`, `Views={}`, `Total={Count:0, Uniques:0}` (partial が空セクションを抑制)

**State transitions**: N/A (immutable Result)

---

## E-012-B: `contributors.Result` (既存、user/org mode で populate 追加)

```go
type Result struct {
    Skipped       bool          `json:"skipped,omitempty"`
    SkippedReason string        `json:"-"`
    Mode          string        `json:"mode,omitempty"`
    List          []Contributor `json:"list"`        // ★ user/org mode で populate
    Sections      []string      `json:"sections"`
    Base          string        `json:"base"`
    Head          string        `json:"head"`
}

type Contributor struct {
    Login     string `json:"login"`
    AvatarURL string `json:"avatarUrl"`
    Commits   int    `json:"commits"`
    Additions int    `json:"additions"`  // 本 feature では常に 0 (contributor endpoint で取得不可)
    Deletions int    `json:"deletions"`  // 同上
}
```

| Field | Before | After | 由来 |
|---|---|---|---|
| `List` (user/org mode) | `[]` (常に空) | per-login aggregate | `/repos/{repo}/contributors` × Featured |
| `Additions` / `Deletions` | 0 | 0 (unchanged) | endpoint で取れない、partial は 0 ガード |
| `Mode` | "" | "user" or "organization" | engine から伝播 |

**Aggregation rule** (FR-006):
1. すべての Featured repo に対し並列で `/repos/{repo}/contributors?per_page=100` を取得
2. login をキーに `Commits` を sum
3. AvatarURL は最初に出現したエントリから採用 (login は同一人物なので一意)
4. `Commits` 降順、tie-break で `Login` 昇順でソート
5. `plugin_contributors_limit` (default 14) で truncate

**Repo-mode behavior**: 既存ロジック (M7 で配線済) — touch しない。

---

## E-012-C: `repositories.Result` (既存、Starred + Random フィールド populate)

```go
type Result struct {
    Skipped       bool                 `json:"skipped,omitempty"`
    SkippedReason string               `json:"-"`
    Featured      []plugins.Repository `json:"featured"`  // UNCHANGED
    Pinned        []plugins.Repository `json:"pinned,omitempty"`   // 別 PR 013 (GraphQL)
    Starred       []plugins.Repository `json:"starred,omitempty"`  // ★ 本 feature で populate
    Random        []plugins.Repository `json:"random,omitempty"`   // ★ 本 feature で populate
}
```

| Field | Before | After | 由来 |
|---|---|---|---|
| `Starred` | `nil` / `[]` | up-to-limit slice | `/users/{login}/starred?sort=created&direction=desc` |
| `Random` | `nil` / `[]` | deterministic shuffle of Featured (n entries) | `math/rand/v2` + seed |

**Starred fetch rule**:
- 1 ページ取得 (`per_page=100`)、`plugin_repositories_limit` (default 8) で head truncate
- 各エントリは GitHub REST `repository` shape → 既存 `plugins.Repository` shape にマップ:
  - `nameWithOwner` → `NameWithOwner`
  - `description` → `Description`
  - `html_url` → `URL`
  - `private` → `Visibility` ("public"/"private")
  - `fork` → `IsFork`
  - `stargazers_count` → `Stars`
  - `forks_count` → `Forks`
  - `watchers_count` → `Watchers`
  - `language` (string) → `Language.Name` (color 不明 → "")

**Random shuffle rule** (FR-009 / FR-010):
- 入力: `Featured` の copy + `plugin_repositories_random` (int, 出力 n) + `plugin_repositories_random_seed` (int)
- seed=0 (default) → `time.Now().UnixNano()` を使用 (per-run fresh)
- seed≠0 → `math/rand/v2.NewPCG(uint64(seed), 0)` で deterministic
- `len(Featured) == 0` → `Random=[]`
- `n > len(Featured)` → `n = len(Featured)` で clamp

---

## E-012-D: Plugin Inputs (additive only)

新規入力は **無し** (既存 input の意味を明確化するだけ)。Reference のため列挙:

| Input | Default | Used in |
|---|---|---|
| `plugin_traffic` | `false` | traffic Run gate |
| `plugin_traffic_skipped` | `[]` (CSV) | traffic per-repo skip list |
| `plugin_contributors` | `false` | contributors Run gate |
| `plugin_contributors_limit` | `14` | contributors truncate |
| `plugin_contributors_ignored` | `[]` (CSV) | login filter |
| `plugin_repositories_starred` | `false` | Starred fetch gate |
| `plugin_repositories_limit` | `8` | Starred + Featured 共通の per-section limit |
| `plugin_repositories_random` | `0` (int) | Random 出力数 (0 → 機能 off) |
| `plugin_repositories_random_seed` | `0` | deterministic seed (0 → per-run fresh) |

すべて upstream `metadata.yml` と同名・同 default。`plugin_repositories_random` は upstream の `repositories_random` を mirror。

---

## 関係性図

```
engine.Compute
    ├── base.Run         → Computed.RepositoryList ([]Repository)
    ├── repositories.Run → Result.Featured  ([]Repository)
    │                    → Result.Starred   ← /users/{login}/starred (★ FR-008)
    │                    → Result.Random    ← shuffle(Featured) (★ FR-009/010)
    ├── traffic.Run      → Result.Views     ← /repos/{repo}/traffic/views × RepositoryList (★ FR-001/002)
    │                    → Result.Total       (aggregate)
    └── contributors.Run → Result.List      ← /repos/{repo}/contributors × Featured (★ FR-005/006)
```

repositories.Featured が contributors の入力になるため、`engine.runPlugins` の dispatch 順は `repositories → contributors` を保証する必要がある。既存の concurrency dispatch は plugin 間依存をサポートしていないが、Featured は `Computed.RepositoryList` から構築可能なので contributors は **`Computed.RepositoryList` を直接読む** ことで dispatch order に依存しない設計とする。
