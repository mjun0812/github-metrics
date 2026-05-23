# Research: 未配線 GraphQL plugin の data-fetch wiring (013)

Phase 0 として、技術選択を確定し、`spec.md` の Clarifications で解決済の判断を実装上のガイドラインに落とす。

## R-001: GraphQL クライアントの拡張方針

- **Decision**: 既存 `internal/tools/gen-graphql` (genqlient ベース) パイプラインを再利用。新クエリは `internal/githubapi/queries/*.graphql` として追加 → `go generate ./internal/githubapi/...` で `graphql_gen.go` 再生成。
- **Rationale**: base / people / reactions / stars がすでに同パターン。複数 GraphQL クライアントを混在させると依存と保守コストが増える。`graphql_gen.go` は型安全な Result struct を提供し、原則 IV (テーブルテスト) の mock 差し替えがしやすい。
- **Alternatives**:
  - `shurcooL/githubv4` を別途持ち込み → 二重依存・型表現が分裂、棄却。
  - 手書きクライアント (`encoding/json` 直叩き) → 型安全性が失われテーブルテストとの相性が悪く、棄却。

## R-002: stargazers の Chart.Series 集計戦略

- **Decision**: 各 owned repo に対し `stargazers(first: 100, orderBy: { field: STARRED_AT, direction: DESC })` で最新 100 件のみ取得し、`starredAt` の `time.Time` を `YYYY-MM` キーで bucket。cumulative count は repo を跨いだ累積。`Chart.Series = [{Month: "2025-01-01T00:00:00Z", Cumulative: N}, ...]` を生成。
- **Rationale**: SC-003 (cost +50 limit) を守る。100 件超のレポは "Showing latest 100 stars" hint で表現 (partial の hint 領域は spec 011 で確定済の "starlists" 風 layout 内に小さな note 領域あり、もしくは Chart.Title に括弧書き)。upstream parity 表示粒度に合わせ month を採用。
- **Alternatives**:
  - 全件 paginate → >1000 star ユーザーで cost 暴走、棄却。
  - 週 / 日 bucket → upstream parity と異なる、棄却 (Q2 で確定)。
  - REST `/repos/.../stargazers` 直叩き → month aggregation なし、 paging が重い、棄却。

## R-003: notable のスコープ縮小

- **Decision**: basic mode のみ。`viewer.repositories(first: N, ownerAffiliations: [OWNER], orderBy: { field: STARGAZERS, direction: DESC })` で上位 N (デフォルト 5) を集計し、`nameWithOwner` / `stargazerCount` / `forkCount` / `description` を `Result.List` に詰める。pinned discussion / 最新 release / indepth は 014 以降。
- **Rationale**: rate cost 抑制 + partial が basic per-repo row のみを期待する DOM (spec 011 確定)。
- **Alternatives**: indepth 同時実装 → cost 暴走、テスト数 2x、棄却 (Q1 で確定)。

## R-004: sponsors の tier 非露出

- **Decision**: Result 型に tier フィールドを追加しない。GraphQL fragment にも `tier { monthlyPriceInDollars }` を含めない。
- **Rationale**: privacy 優先 (Q3 で確定)。tier 表示は将来 opt-in 入力 (`plugin_sponsors_tier=yes` 等) で別 spec 着手。
- **Alternatives**: optional fetch して JSON に書く → 漏洩 risk + rate cost +α、棄却。

## R-005: rate budget overflow の取り扱い

- **Decision**: 各 plugin Run の冒頭で `pc.GraphQL.RateBudget()` (T-010 で導入済の rate tracker) と「推定 cost」を比較。budget 不足なら `Skipped=true` + `SkippedReason="rate budget too low"` を返し、`*RetryableError` を `pc.Data.Errors` に積む。他 plugin は継続。
- **Rationale**: `internal/engine/dispatch.go` は per-plugin error isolation を持つため、他 plugin への副作用なし。
- **Alternatives**:
  - 部分 fetch して degrade → Result の "完全性" が曖昧になり partial が不整合 DOM を出す risk、棄却。
  - 全件強行 → SC-003 違反、棄却。

## R-006: organization mode の取り扱い

- **Decision**: 本 spec 全 plugin で user mode (= `viewer` 起点) のみ実装。`AccountKind == "organization"` は既存 Skipped path で継続。`stargazers` の `repository` mode (= specific repo の star) は M7 territory として継続 Skipped。
- **Rationale**: clarifications Q4 で確定。`viewer` 起点なら token 1 本で完結し、`AccountKind` 分岐の plugin 内部実装がシンプル。
- **Alternatives**: org mode を同時実装 → token に `read:org` 要件が増え scope-gate ロジックが厚くなる、テスト数 2x、棄却。

## R-007: GraphQL クエリの cost 見積もり

- **Decision**: 各クエリの想定 cost を以下の通り見積もる:
  - `viewer.sponsorshipsAsMaintainer(first: 12)`: ~3 points
  - `viewer.sponsorshipsAsSponsor(first: 12)`: ~3 points
  - `viewer.projectsV2(first: 5)`: ~3 points
  - `viewer.repositories(first: 5, ownerAffiliations:[OWNER], orderBy: STARGAZERS)`: ~5 points (notable)
  - `viewer.repositories(first: 10) { stargazers(first: 100) }`: ~30 points (stargazers, 10 repos × 3 points each estimated)
  - `viewer.pinnedItems(first: 6, types:[REPOSITORY])`: ~3 points
  - **合計**: ~47 points → SC-003 の +50 限界内
- **Rationale**: GitHub の GraphQL cost は connection の `first:` パラメータと nested resource count にほぼ比例。stargazers の nested fetch が最大、他は軽量。

## R-008: テスト戦略

- **Decision**: 各 plugin に最低 4 ケースの table-driven test を追加。
  - `Happy`: GraphQL から N 件 → Result.List に N 件
  - `ScopeMissing`: scope-gate path → Skipped + reason
  - `NetworkFailure`: 5xx → `*RetryableError` を Data.Errors、Skipped=true
  - `EmptyData`: 空配列 → Skipped=false、List=[] (Skipped にはしない)
- **Rationale**: 原則 IV (テーブルテスト + golden) の最小単位。各 plugin に共通のテスト形を持たせると追加コストが減る。
- **golden**: partial DOM は変更しないので既存 golden 不変。Result の JSON shape を凍結する golden は新規追加せず、テーブルテスト内の `reflect.DeepEqual` で代替 (spec 012 PR と同方針)。
- **integration**: `tests/integration/plugins_p2_test.go` に 6 plugin が同時に enable された state で各 plugin が non-Skipped で動くことを mock 経由で smoke test する。

## Open Questions

- なし (Clarifications で全解消済)。

## Sources

- `docs/design/15-selection-answer.md` §4.2 / §6.4 (採用 plugin リスト)
- `docs/design/12-tasks.md` T-052〜T-068 (上流 plugin task 定義)
- 既存 `internal/plugins/{base,people,reactions,stars}/*.go` (同パターンの実装例)
- GitHub GraphQL API docs (`viewer.sponsorshipsAsMaintainer`, `viewer.projectsV2`, `viewer.pinnedItems`, etc.)
