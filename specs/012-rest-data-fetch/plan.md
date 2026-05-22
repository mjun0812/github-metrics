# Implementation Plan: REST Data-Fetch Wiring for 4 Plugins

**Branch**: `012-rest-data-fetch` | **Date**: 2026-05-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/012-rest-data-fetch/spec.md`

## Summary

011 sweep で 19 plugin の partial (描画レイヤ) は upstream parity を達成したが、4 plugin (`traffic` / `contributors` user-org mode / `repositories.Starred` / `repositories.Random`) はデータ取得 (Run) が未配線で `Skipped=true` または空 Result を返している。本 feature では REST 経由で実データを取得し、既存 partial と `plugins.Repository` shape を一切変更せずに Result を populate する。GraphQL 系プラグイン (sponsors / sponsorships / projects / repositories.Pinned 等) と chromedp 系 (topics) は本 PR スコープ外。

技術的アプローチ:
- 既存 `pc.REST` クライアント (`internal/githubapi/rest.go`) を再利用。新規 transport / client は追加しない
- 並列度 4 / per-request 30s timeout は `languages.indepth` のパターンを踏襲
- 各 per-repo HTTP 失敗は `*RetryableError` で集約、他 repo は継続
- token-scope check は既存 `pc.REST.Scopes(ctx)` ヘルパを使用
- `repositories.Random` はネットワーク不要、`math/rand/v2` の deterministic seed で実装

## Technical Context

**Language/Version**: Go 1.26.3 (`go.mod` の `go` directive 固定。constitution §V 言語ポリシー準拠)

**Primary Dependencies**:
- 既存: `internal/githubapi` (REST + GraphQL クライアント、retry / rate-limit 対応済), `golang.org/x/sync/errgroup` (`languages.indepth` で既使用), `math/rand/v2` (Go 1.22+ stdlib、shuffle 用)
- 新規追加: **なし** (constitution §V: 新規外部依存は採用根拠を明記すること。本 feature はすべて stdlib / 既存内部パッケージで完結)

**Storage**: N/A (本 feature は in-memory のデータ集約のみ。永続化対象なし)

**Testing**:
- Unit: 既存 `internal/plugins/<slug>/*_test.go` パターン (テーブルテスト + golden)
- REST モック: `internal/testutil/mocks` の `RESTMux` を拡張、`MOCKED_TOKEN` 経路を維持 (constitution 原則 IV)
- Integration: `tests/integration/plugins_p1_test.go` パターンを真似て 4 plugin の end-to-end Compute を fixture で検証

**Target Platform**: Linux / macOS / Windows (cross-compile)、container 環境 (`metrics-action` Docker image)

**Project Type**: cli (`cmd/metrics-action`)

**Performance Goals**:
- `engine.Compute` 全体 < 5 秒 (constitution §Technical Constraints) を維持
- per-plugin 並列度 4 で 100 リポジトリ (`config_repositories=100`) を ≦ 30 秒で処理

**Constraints**:
- 既存 `data.plugins.{traffic,contributors,repositories}` JSON shape は維持 (`additive` のみ可、フィールド削除/型変更 MUST NOT — constitution 原則 II)
- 既存 partial (011 完了済) は無変更 (Result が populate されれば自動で正しく描画される設計)
- HTTP per-request timeout 30s、全体 timeout は plugin metadata デフォルト準拠

**Scale/Scope**:
- 採用 19 plugin のうち 4 plugin の `Run` のみを touch
- 影響範囲: 4 つの `<slug>.go` + 各テスト + integration test 1 ファイル
- 新規 LOC 目安 ~600 行 (実装) + ~400 行 (テスト)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原則 | Status | Notes |
|---|---|---|
| **I. 入力互換性** | ✅ PASS | `action.yml` / `metadata.yml` 不変。本 feature は既存 input (`plugin_traffic` / `plugin_contributors` / `plugin_repositories_*`) の意味を「Skipped → data populated」に変える挙動修正のみ。新規 input は `plugin_repositories_random` / `plugin_repositories_random_seed` のみ (upstream metadata と同名、no-op 互換) |
| **II. 出力契約** | ✅ PASS | JSON shape は additive (`data.plugins.repositories.starred` / `data.plugins.repositories.random` は既存だが現状空 → populated に)。DOM は無変更 (partial は 011 で完了済) |
| **III. スコープ規律** | ✅ PASS | 採用 19 plugin の枠内。新規プラグイン追加なし。`docs/design/15-selection-answer.md` §6 の `traffic` / `contributors` / `repositories` はすべて採用済 |
| **IV. テーブルテスト + Golden** | ✅ PASS | 各 plugin に対し table test + golden を追加。REST モックは `RESTMux` 経路、外部 HTTP は panic で防御 |
| **V. Go 規約** | ✅ PASS | 新規外部依存なし、stdlib `math/rand/v2` + 既存 `errgroup` のみ。`cmd/` / `internal/` 二層維持。コメントは英語、`docs/`/spec.md は日本語 (mixed sections OK) |

**Gate result**: 全 5 原則 PASS。research phase に進む。

## Project Structure

### Documentation (this feature)

```text
specs/012-rest-data-fetch/
├── plan.md                  # This file (/speckit-plan output)
├── spec.md                  # Already exists (/speckit-specify output)
├── research.md              # Phase 0 output
├── data-model.md            # Phase 1 output
├── quickstart.md            # Phase 1 output
├── contracts/               # Phase 1 output
│   ├── plugin-traffic.md
│   ├── plugin-contributors.md
│   ├── plugin-repositories-starred.md
│   └── plugin-repositories-random.md
├── checklists/
│   └── requirements.md      # Already exists
└── tasks.md                 # /speckit-tasks output (not yet)
```

### Source Code (repository root)

```text
internal/
├── plugins/
│   ├── traffic/
│   │   ├── traffic.go            # ★ Run wiring (新規 REST 呼び出し追加)
│   │   ├── traffic_test.go       # ★ テーブルテスト追加
│   │   └── partial.go            # NO CHANGE (011 完了済)
│   ├── contributors/
│   │   ├── contributors.go       # ★ user/org mode の REST 経路追加
│   │   ├── contributors_test.go  # ★ テーブルテスト追加
│   │   └── partial.go            # NO CHANGE (011 完了済)
│   └── repositories/
│       ├── repositories.go       # ★ Starred + Random 配線
│       ├── repositories_test.go  # ★ テーブルテスト追加
│       └── partial.go            # NO CHANGE (011 完了済)
├── githubapi/
│   └── rest.go                   # 既存。新規ヘルパが必要なら追加 (本 plan で要否判定)
└── testutil/
    └── mocks/
        └── rest_mux.go           # ★ 必要な REST endpoint fixtures を追加 (traffic/views, contributors, starred)

tests/
├── integration/
│   └── plugins_012_test.go       # ★ 4 plugin end-to-end Compute 検証
└── golden/
    └── json/
        ├── traffic.json          # ★ 新規 golden
        ├── contributors.json     # 既存 (拡張)
        └── repositories.json     # 既存 (拡張 — Starred + Random フィールド)
```

**Structure Decision**: 既存 `internal/plugins/<slug>/<slug>.go` + `<slug>_test.go` パターンを踏襲。 各 plugin ディレクトリは self-contained (Run / Result struct / parseInputs / テスト) のため、本 feature 追加で構造変更は発生しない。`pkg/` は constitution §V により導入しない。

## Complexity Tracking

Constitution Check は全 PASS のため、本セクションは N/A (記入不要)。
