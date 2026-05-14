# Implementation Plan: プロジェクト土台 (M1 19 タスク一括)

**Branch**: `001-project-foundation` | **Date**: 2026-05-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-project-foundation/spec.md`

## Summary

上流 `lowlighter/metrics` (Node.js 製) の Go 移植プロジェクトにおいて、後続全機能の前提となる **19 タスク分の骨格** を構築する。具体的には、`cmd/` + `internal/` レイアウトの初期化、`log/slog` ベースのロガー / 型付きエラー、`settings.json` + `metadata.yml` + action `INPUTS` のローダ、`Khan/genqlient` ベースの GitHub REST/GraphQL クライアントとレート可視化、`embed.FS` によるアセット同梱、Plugin/Template の interface + registry、base/core プラグインによる並列実行と `engine.Compute` の結線までを到達点とする。M1 段階では `template.Run` は no-op (実描画は M2 以降)、最終出力は internal `data` 構造体のみ。**入力互換性 (constitution 原則 I)** と **テーブル + golden テスト規律 (原則 IV)** をこの段階で固める。

## Technical Context

**Language/Version**: Go 1.23 (latest stable に追随。`go.mod` の `go` directive で固定。`<1.22` サポートは MUST NOT)。

**Primary Dependencies**:

- 標準: `log/slog`, `net/http`, `context`, `errors`, `embed`, `encoding/json`, `gopkg.in/yaml.v3`
- 並行: `golang.org/x/sync/errgroup`
- HTTP リトライ: `github.com/hashicorp/go-retryablehttp`
- GraphQL コード生成: `github.com/Khan/genqlient`
- テスト: `testing` (stdlib) + `github.com/stretchr/testify/assert`, `net/http/httptest`
- フォーマッタ / リンタ: `gofumpt`, `golangci-lint`, `govulncheck`

**Storage**:

- `embed.FS` で `assets/plugins/**` と `assets/templates/{classic,repository}/**` をバイナリ同梱。
- 起動時に `settings.json` を `os.ReadFile` で読み込み (Web インスタンスを実装しないが、ローダは互換維持)。
- 生成物の永続化は M1 段階では対象外 (M6 の Action committer フェーズで `/renders/<filename>` 書き出しを担当)。

**Testing**:

- テーブルテスト (`[]struct{...}{} + t.Run(tc.name, ...)`)。
- Golden file テスト (XML 正規化 + ハッシュ比較、`-update` フラグ更新)。
- mocked REST/GraphQL は M1 段階で最小スタブを `internal/githubapi` 内の test helper として置き、M9 (T-118/T-119) で `internal/testutil/mocks` に full 実装へ昇格する。
- `MOCKED_TOKEN` 経路を通らない外部呼び出しは即 panic させ、テスト網羅性を保証 (原則 IV)。

**Target Platform**:

- CI 実行環境: GitHub Actions `ubuntu-latest` (primary)、`macos-latest` (secondary smoke)。
- バイナリ配布: クロスコンパイル (`linux/amd64`, `linux/arm64`, `darwin/arm64`, `windows/amd64`)。
- Docker (M10 で取り扱う。M1 では Makefile に placeholder のみ)。

**Project Type**: 単一プロジェクト (`cmd/` + `internal/` + `assets/` レイアウト)。GitHub Action / CLI のハイブリッド。

**Performance Goals**:

- `make build && make test` ローカル完了 < 3 分。
- CI workflow (test + vet + lint + govulncheck) PR 上で < 7 分。
- `MetadataLoader.Load(embed.FS)` 起動時 < 200 ms。
- mocked `engine.Compute` (1 ユーザー分) < 2 秒 (chromedp 抜き)。
- `internal/logger`, `internal/errors`, `internal/format`, `internal/config` の unit test カバレッジ >= 80%。

**Constraints**:

- **入力互換性 (原則 I)**: `action.yml` および `settings.json` のキー差分が上流に対して 0。
- **出力契約 (原則 II)**: M1 段階では JSON 出力 / SVG 出力共に未到達。`engine.Compute` の internal `data` 構造体が次フェーズで JSON marshaller (T-029) の入力になる形のみ保証。
- **スコープ規律 (原則 III)**: Web インスタンス、Markdown / PDF 出力、Insights、community plugins、ソーシャル/外部 API plugins、未採用 GitHub plugins に属するコードは追加 MUST NOT。
- **テスト規律 (原則 IV)**: 公開 API はすべてテーブルテスト。出力構造を持つ機能 (formatters / metadata loader / engine.Compute) は golden file 必須。mocked 経路外の外部呼び出しは panic。
- **言語規約 (原則 V)**: コード内コメント / docstring / identifier は英語。本ドキュメントを含む `docs/` は日本語。`./org_repo` は参照のみ、git 履歴に含めない。
- **メモリ**: chromedp プロセスを除いた idle メモリ < 80 MB (M1 段階では chromedp 未統合のため余裕あり)。

**Scale/Scope**:

- 19 タスク (T-001..005, 007..010, 012..018, 020..022)。
- 想定パッケージ数: 約 11 (`cmd/metrics-action`, `cmd/metrics-cli`, `internal/logger`, `internal/errors`, `internal/format`, `internal/config`, `internal/httpx`, `internal/githubapi`, `internal/plugins`, `internal/plugins/base`, `internal/plugins/core`, `internal/templates`, `internal/engine`)。
- 想定総 LOC: 約 6,000〜8,000 行 (テスト込み、`assets/` の同梱メタデータは除く)。
- 想定 PR 数: 5〜8 (User Story 単位で分割)。

## Constitution Check

*GATE: Phase 0 前と Phase 1 後の両方で再評価する。*

### Initial Constitution Check (pre Phase 0)

| 原則 | 内容 | 適合性 | 根拠 |
|---|---|---|---|
| I. 入力互換性 (NON-NEGOTIABLE) | `action.yml` / `settings.json` キー完全互換 | ✅ PASS | FR-008/009/010/011 で互換読み込みを必須化、SC-002 で差分ゼロを測定。 |
| II. 出力契約 (DOM/JSON 単位) | JSON 完全互換、SVG/Markdown は DOM 同等 | ✅ PASS (該当範囲外) | M1 段階では出力フェーズ未到達。`engine.Compute` の internal data 構造体が後続 JSON marshaller への入力契約を担う。 |
| III. スコープ規律 | 採用機能のみ実装 | ✅ PASS | FR-027 で `15-selection-answer.md` §7 範囲のコード追加を禁止。19 タスクは §6.1 の土台部分集合。 |
| IV. テーブルテスト + Golden File | mocked + golden 強制 | ✅ PASS | FR-030 で全公開関数のテーブルテスト + 構造出力 golden 必須。FR-017 で `MOCKED_TOKEN` 経路外 panic。 |
| V. Go 規約と言語ポリシー | Go latest, cmd/+internal/, 日本語 docs / 英語 comments | ✅ PASS | FR-001 で Go 1.23 + cmd/+internal/、FR-029 で言語規約。 |

**Gate result**: PASS — 違反なし、Complexity Tracking 不要。

### Post-Design Constitution Check (after Phase 1)

| 原則 | 設計後の再評価 |
|---|---|
| I | `data-model.md` の `Settings` / `Inputs` / `MetadataLoader` がキー集合を保持。`contracts/cli.md` の `INPUT_*` / `INPUTS` JSON も上流互換。差分なし。 |
| II | `data-model.md` の `Data` 構造体が JSON marshaller の契約を内包。M2 で T-029 が `Marshal(*Data)` を実装する際の出発点となる。M1 範囲では出力なし。 |
| III | `contracts/` には Web / Markdown / PDF / Insights / community 関連のシンボルなし。`data-model.md` も同じ。 |
| IV | `contracts/plugin-interface.md` で `Plugin.Run` の return が `(any, error)` でテスト対象、`contracts/template-interface.md` で `PartialFunc` の入出力が golden 対象。`research.md` で mocking 方針確定。 |
| V | `data-model.md` の entity 名・型は英語。`contracts/` も英語。本 plan / quickstart / research / spec は日本語。 |

**Gate result**: PASS — 設計でも違反なし。

## Project Structure

### Documentation (this feature)

```text
specs/001-project-foundation/
├── plan.md              # 本ファイル
├── research.md          # Phase 0 成果物
├── data-model.md        # Phase 1 成果物 (Settings / Inputs / Data 等)
├── quickstart.md        # Phase 1 成果物 (開発者初期セットアップ手順)
├── contracts/           # Phase 1 成果物
│   ├── cli.md           # CLI フラグ + INPUTS / INPUT_* 環境変数
│   ├── plugin-interface.md     # Plugin interface + registry contract
│   ├── template-interface.md   # Template interface + PartialFunc contract
│   └── github-api.md    # HTTP / REST / GraphQL / rate ラッパ contract
├── checklists/
│   └── requirements.md  # /speckit-specify で生成済み
└── tasks.md             # /speckit-tasks 出力 (未作成)
```

### Source Code (repository root)

```text
github-metrics/
├── go.mod                                    # module github.com/mjun0812/github-metrics, go 1.23
├── Makefile                                  # build / test / lint / bench / gen / docker / e2e
├── .github/
│   └── workflows/
│       └── go-ci.yml                         # go test / vet / golangci-lint / govulncheck
├── .golangci.yml                             # staticcheck / gosec / revive / gofumpt
├── action.yml                                # 上流互換 (M10 で Go バイナリ呼び出しに切替)
├── cmd/
│   ├── metrics-action/main.go                # GitHub Action / CLI エントリ (T-001, M1 では空 main)
│   └── metrics-cli/main.go                   # CLI 専用 (T-117 で本実装、M1 では空 main)
├── internal/
│   ├── logger/                               # T-002: slog 設定 + ヘルパ
│   ├── errors/                               # T-002: 型付きエラー
│   ├── ctxutil/                              # T-002: WithLogin / LoginFromContext
│   ├── format/                               # T-013: フォーマッタ群
│   ├── config/
│   │   ├── settings.go                       # T-003: settings.json ローダ
│   │   ├── metadata.go                       # T-004: metadata.yml ローダ
│   │   ├── inputs.go                         # T-005: 型変換 + プレースホルダ
│   │   └── inputs_token.go                   # token 不透明値
│   ├── httpx/                                # T-007: HTTP リトライラッパ
│   ├── githubapi/
│   │   ├── auth.go                           # T-008: token 種別判定
│   │   ├── rest.go                           # T-008: REST クライアント
│   │   ├── graphql.go                        # T-009: genqlient ラッパ
│   │   ├── graphql_gen.go                    # genqlient 自動生成 (go generate)
│   │   ├── rate.go                           # T-010: Resources / Refresh
│   │   └── testhelper.go                     # M9 までの最小スタブ
│   ├── plugins/
│   │   ├── plugin.go                         # T-014: Plugin interface + registry
│   │   ├── registry.go                       # Register / Get / Each / RegisterForTest
│   │   ├── base/
│   │   │   ├── base.go                       # T-017: user
│   │   │   ├── organization.go               # T-018: org
│   │   │   └── repositories.go               # T-020: ページング
│   │   └── core/
│   │       ├── core.go                       # T-021: グローバル設定注入
│   │       └── run_plugins.go                # T-022: errgroup 並列
│   ├── templates/
│   │   └── template.go                       # T-015: Template interface + registry
│   └── engine/
│       └── engine.go                         # T-016: Compute オーケストレータ
├── assets/                                   # T-012: embed 対象
│   ├── plugins/<name>/{metadata.yml,queries/*.graphql,examples/*.yml}
│   ├── templates/{classic,repository}/{metadata.yml,...}
│   └── version.txt
├── tests/
│   ├── fixtures/
│   │   ├── settings/                         # T-003 ケース
│   │   ├── inputs/                           # T-005 ケース
│   │   └── github/                           # mocked REST/GraphQL fixtures (M1 最小、M9 で拡充)
│   ├── golden/                               # T-030 (FR-030) golden file
│   └── integration/
│       └── foundation_test.go                # US5: engine.Compute 結線 e2e
└── scripts/
    └── sync-assets.sh                        # T-012: ./org_repo から手動ではなく確定スクリプトで取得
```

**Structure Decision**: Go 標準の単一プロジェクトレイアウト (`cmd/` + `internal/`)。`pkg/` は constitution 原則 V により採用しない (公開ライブラリではない)。`internal/` 配下を 11 パッケージに分け、user story 単位で PR を切り出せる粒度にする。`tests/integration/` は internal package 間結線の確認用で、unit test は各パッケージ内 `*_test.go` に置く。

## Complexity Tracking

> Constitution Check は PASS につき記入不要。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (なし) | — | — |
