# Implementation Plan: 未配線 GraphQL plugin の data-fetch wiring (013)

**Branch**: `013-unwired-graphql-data` | **Date**: 2026-05-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/013-unwired-graphql-data/spec.md`

## Summary

採用機能 §4.2 のうち未配線のまま残る 6 plugin (`sponsors` / `sponsorships` / `projects` / `notable` / `stargazers` / `repositories.Pinned`) の `Run()` を GraphQL fetch で実装し、Skipped を解消する。spec 011 で整っている partial DOM は本 spec で変更しない。GraphQL クエリは `internal/githubapi/queries/*.graphql` に追加 → `internal/tools/gen-graphql` で `graphql_gen.go` を再生成 → 各 plugin の Run から呼び出す方針。account kind は user mode に限定 (`viewer` 起点)、stargazers の chart は月単位 cumulative、sponsors の tier 価格は privacy 配慮で非露出、rate budget 不足時は per-plugin Skipped。

## Technical Context

**Language/Version**: Go 1.26.x (`go.mod` の `go` directive)

**Primary Dependencies**: `internal/githubapi` (`graphql.go` + `gen-graphql` で生成された `graphql_gen.go`)、`internal/errors` (`RetryableError`)、`internal/plugins/*` (既存の Result 型 / partial)

**Storage**: 該当なし。Plugin Result は in-memory のみで render パイプラインへ流れる。

**Testing**: `go test` + テーブルテスト + golden (data-fetch のみ。partial DOM は既存 golden 不変)。GraphQL は `internal/testutil/mocks` の GraphQL mux で差し替え。

**Target Platform**: Action / CLI 双方 (chromedp 不要)。

**Project Type**: cli + library。`cmd/metrics-action` および `cmd/metrics-cli` から呼ばれる plugin 群。

**Performance Goals**: 1 render あたり GraphQL 追加 cost < +50 points (SC-003)。レイテンシは既存 Compute 5 秒上限を維持。

**Constraints**: rate budget が枯渇したら per-plugin Skipped、他 plugin に影響を与えない。partial.go は変更不可。

**Scale/Scope**: 6 plugin × 平均 1〜2 query 追加。GraphQL gen 出力 1 ファイル更新。`Result` フィールドの追加は最小限。テスト 24 ケース以上 (各 plugin 4 ケース)。

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. 入力互換性 | OK | `action.yml` / `metadata.yml` / `settings.json` の入力名は変更しない。各 plugin の既存 `plugin_<slug>_*` 入力をそのまま使用。新規入力なし。 |
| II. 出力契約 | OK | JSON shape は additive 拡張のみ (sponsors の `goal` / stargazers の `chart.series` 等は既存 Result 型に定義済)。partial DOM (spec 011 確定) は不変。 |
| III. スコープ規律 | OK | 採用 §4.2 / §6.4 の plugin のみ。`tweets` / `wakatime` 等の不採用 plugin に新規コードを追加しない。compliance test (`TestCompliance_M4_AdoptedPlugins` / `TestNoUnadoptedPluginReference`) は破らない。 |
| IV. テーブルテスト + Golden | OK | 各 plugin に最低 4 ケース (happy / scope 不足 / 5xx / 空データ) のテーブルテスト追加。partial golden は不変、Run の Result golden を必要なら追加 (JSON shape の凍結用)。 |
| V. Go 規約 | OK | コードコメントは英語、log/slog、新規外部依存ゼロ (stdlib + 既存 `internal/githubapi` のみ)。GraphQL schema は既存の `internal/tools/gen-graphql` を流用。 |

**結論**: 全ゲート pass。新規制約や例外は不要。

## Project Structure

### Documentation (this feature)

```text
specs/013-unwired-graphql-data/
├── plan.md              # This file (/speckit-plan output)
├── spec.md              # /speckit-specify + /speckit-clarify output
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── contracts/           # Phase 1 (per-plugin Run の呼び出し契約)
│   ├── sponsors-fetch.md
│   ├── sponsorships-fetch.md
│   ├── projects-fetch.md
│   ├── notable-fetch.md
│   ├── stargazers-fetch.md
│   └── repositories-pinned-fetch.md
├── quickstart.md        # Phase 1 (実行確認手順)
├── checklists/
│   └── requirements.md  # speckit-specify 出力
└── tasks.md             # Phase 2 (/speckit-tasks で生成)
```

### Source Code (repository root)

```text
internal/githubapi/
├── queries/
│   ├── viewer_sponsors.graphql           # (new) sponsorshipsAsMaintainer + sponsorsListing
│   ├── viewer_sponsorships.graphql       # (new) sponsorshipsAsSponsor
│   ├── viewer_projects.graphql           # (new) projectsV2(first:)
│   ├── viewer_notable.graphql            # (new) viewer.repositories(first:, ownerAffiliations:[OWNER])
│   ├── viewer_stargazers_repos.graphql   # (new) viewer.repositories + stargazers(first:100, orderBy: starredAt)
│   └── viewer_pinned_items.graphql       # (new) viewer.pinnedItems(first:6, types:[REPOSITORY])
├── graphql.go                            # (edit) public wrappers for new gen functions
└── graphql_gen.go                        # (regenerate) generated client

internal/plugins/
├── sponsors/sponsors.go                  # (edit) Run wires sponsorshipsAsMaintainer + goal
├── sponsorships/sponsorships.go          # (edit) Run wires sponsorshipsAsSponsor
├── projects/projects.go                  # (edit) Run wires projectsV2 (scope-gate path kept)
├── notable/notable.go                    # (edit) Run wires owner traversal
├── stargazers/stargazers.go              # (edit) Run extended to user mode w/ month bucketing
└── repositories/repositories.go          # (edit) Pinned wires pinnedItems

tests/
└── integration/
    └── plugins_p2_test.go                # (new) e2e smoke covering newly-wired plugins (using mocks)
```

## Phase 0: Outline & Research

すべての NEEDS CLARIFICATION は spec の Clarifications で解消済。本 Phase は技術選択の根拠固めに絞り、`research.md` に集約する。

**Decisions** (詳細は [research.md](./research.md)):

1. **GraphQL クライアントの拡張方針**: 既存の `internal/tools/gen-graphql` (genqlient) パイプラインを再利用。新クエリは `internal/githubapi/queries/*.graphql` として追加し、`go generate ./internal/githubapi/...` で `graphql_gen.go` を再生成。base / people / reactions / stars と同パターン。
2. **stargazers の Series 集計戦略**: `stargazers(first: 100, orderBy: STARRED_AT DESC)` で最新 100 件のみ取得 → 月単位 bucket → cumulative count。100 件超は partial の "Showing latest 100 stars" hint で表現 (spec 011 確定 DOM 内に hint 領域あり)。
3. **notable のスコープ縮小**: basic only (stargazer/fork/name/description)。indepth は 014。
4. **sponsors の tier 非露出**: Result 型に tier フィールドを追加せず、GraphQL fragment にも含めない。
5. **rate budget overflow の取り扱い**: 各 plugin Run 冒頭で `pc.GraphQL.RateBudget()` (T-010) と「推定 cost」を比較。不足なら Skipped + `*RetryableError` を Data.Errors に積む。
6. **organization mode の取り扱い**: 本 spec では全 plugin user mode のみ。org mode は 014 以降 backlog。

## Phase 1: Design & Contracts

**Prerequisites**: research.md 完了

### 1. Data Model

`data-model.md` を生成し、6 plugin の既存 Result 型を表で列挙、本 spec で「埋める」フィールドと「追加する」フィールドを明確化する。新規追加は `stargazers.Chart.Series []MonthPoint` のみ。

### 2. Contracts

`contracts/<plugin>-fetch.md` × 6 ファイル。各 plugin の Run I/O 契約を明文化:

- **入力**: `pc.GraphQL`, `pc.Inputs["user"]`, `pc.Inputs["plugin_<slug>_*"]`、`pc.Data.Computed`
- **出力**: `Result` 型 (Skipped / List / Goal / Series 等)、副作用: `pc.Data.Errors` への `*RetryableError`
- **失敗時挙動**: rate budget 不足 / scope 不足 / 5xx / 空データの 4 分類で挙動を明示
- **partial 側 (spec 011 確定済) は変更しない** ことを明記

### 3. Quickstart

`quickstart.md` を生成。mjun0812 token と Chromium 手元前提で `scripts/capture-mjun-references.sh` を流用し、6 plugin の SVG を render → `specs/013-unwired-graphql-data/plugins/screenshots/` に PNG / SVG を出す手順。PR 説明にこの screenshots へのリンクを必須として明記する。

### 4. Agent context update

`CLAUDE.md` の `<!-- SPECKIT START -->` / `<!-- SPECKIT END -->` 内の Active plan 参照を `specs/013-unwired-graphql-data/plan.md` に切り替える。

**Output**: `data-model.md`, `contracts/*.md` × 6, `quickstart.md`, `CLAUDE.md` 更新

## Constitution Check (Post-Design Re-evaluation)

| Principle | Pre-design | Post-design | Notes |
|---|---|---|---|
| I. 入力互換性 | OK | OK | 既存 `plugin_<slug>_*` 入力のみ使用。新規入力なし。 |
| II. 出力契約 | OK | OK | partial DOM 不変。JSON は既存フィールドを埋めるのみ、shape の追加は `stargazers.Chart.Series` のみで additive。 |
| III. スコープ規律 | OK | OK | 採用 §6.4 plugin のみ。compliance test 不変。 |
| IV. テーブルテスト + Golden | OK | OK | 各 plugin 4 ケース、tests/integration に e2e 1 file 追加。partial golden 不変。 |
| V. Go 規約 | OK | OK | 新規 stdlib 利用なし、英語コメント、`log/slog` のみ。 |

**結論**: design 後も全 gate pass。

## Next

`/speckit-tasks` で task 一覧を生成し、`/speckit-implement` で実装着手。
