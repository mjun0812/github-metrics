# Implementation Plan: classic テンプレート + JSON 出力 (M2)

**Branch**: `002-output-classic-json` | **Date**: 2026-05-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-output-classic-json/spec.md`

## Summary

M1 で `engine.Compute` が `Result{Data, Errors}` を返すところまで動いた。本 feature は予約された `Result.Output []byte` / `Result.MIME string` を実体化し、ユーザーが消費可能な **JSON** と **SVG (classic テンプレート)** の 2 系統を提供する。JSON は循環参照対応 + 上流互換キー集合、classic SVG は image.svg スケルトン + 4 partial (`base.header` / `introduction` stub / `base.activity+community` / `base.repositories`) + optional metadata footer で構成し、DOM 構造単位で上流と同等になる (constitution 原則 II)。partial はテンプレートで欠落するデータ (例: M4 で実装する `computed.calendar`) を安全に skip し、現状利用可能な field のみ描画する。

## Technical Context

**Language/Version**: Go 1.26 (M1 から継続)。

**Primary Dependencies**:

- 標準: `encoding/json`, `text/template`, `html/template`, `strings`, `time`, `reflect`
- M1 で導入済み: `gopkg.in/yaml.v3`, `Khan/genqlient`, `golang.org/x/sync/errgroup`, `hashicorp/go-retryablehttp`
- 新規追加なし (DOM 組み立ては string concatenation で十分、`text/template` は partial 順制御に使う)

**Storage**:

- `assets/templates/classic/image.svg` を `embed.FS` 経由でロード (M1 で確立済みパターン)
- `assets/templates/classic/partials/_.json` (本 spec の T-023 範囲で classic 専用 MVP リストへ書き換える)
- 出力 ([]byte) はインメモリ保持のみ。永続化は呼び出し側 (cmd/metrics-action) の責務 (M6)

**Testing**:

- JSON: `assert.JSONEq` 比較 + キー集合 diff (FR-018)
- SVG: XML 正規化 (whitespace tidy + 属性 sort + footer mask) + MD5 比較 (FR-017)
- golden file `tests/golden/{json,classic}/` 配下、`-update` フラグ更新
- 上流互換性 fixture: `tests/fixtures/upstream/octocat.json` (M2 で `make sync-fixtures` 経由で取得)

**Target Platform**: M1 と同じ (GitHub Actions runner / 配布バイナリ)。chromedp は本 spec の範囲外 (M3)。

**Project Type**: 単一 Go プロジェクト (cmd/+internal/)。

**Performance Goals**:

- `engine.Compute` (classic / json 経路) 1 ユーザー分 < 2 秒 (chromedp 抜き、SC-003)
- JSON Marshal 単体 < 100 ms / 1 ユーザー分の Data (内部目標)
- SVG concatenation < 50 ms / 4 partial 合計 (内部目標)

**Constraints**:

- **原則 I (input compat)**: 本 spec で新規 input は追加しない。`Request.Format`, `Request.Template` は M1 で確定済み。
- **原則 II (output contract)**: JSON はキー集合・型・shape を上流と完全互換 (SC-001)、SVG は DOM 同等性のみ (FR-008)。
- **原則 III (scope)**: classic / repository 以外の template、Markdown / PDF / Insights 出力は MUST NOT 追加。
- **原則 IV (tests + golden)**: JSON golden + SVG golden + 上流互換性 fixture テストが必須。
- **原則 V (Go conventions)**: code は英語、docs は日本語、`html/template` ではなく明示的な string concatenation を使う (XML 数値参照のため)。
- **下位互換**: M1 で空 `Result.Output` を呼び出していたコードはない (M1 段階で `engine.Compute` を呼ぶ唯一の場所はテスト)。本 spec で populate されても破壊変更にならない。

**Scale/Scope**:

- 本 spec: ~6 タスク (T-023, T-024, T-025, T-026, T-027, T-028, T-029)。Phase 2 (`/speckit-tasks`) で細分化。
- 想定追加パッケージ: `internal/engine/json.go` (新規ファイル、`internal/engine` 既存パッケージ内)、`internal/templates/classic/` (新規ディレクトリ + `classic.go` + 4 partial files)、`tests/{golden,fixtures,compatibility}/` 追加。
- 想定 LOC: 本体 +400, テスト +600。

## Constitution Check

*GATE: Phase 0 前と Phase 1 後の両方で再評価する。*

### Initial Constitution Check (pre Phase 0)

| 原則 | 内容 | 適合性 | 根拠 |
|---|---|---|---|
| I. 入力互換性 (NON-NEGOTIABLE) | `action.yml` / `settings.json` キー完全互換 | ✅ PASS | 新規 input なし。`Request.Format` / `Request.Template` は M1 で確定。 |
| II. 出力契約 (DOM/JSON 単位) | JSON 完全互換、SVG/Markdown は DOM 同等 | ✅ PASS | FR-001 (JSON キー集合互換) + FR-008 (SVG DOM 同等) + SC-001/002 で測定。 |
| III. スコープ規律 | 採用機能のみ実装 | ✅ PASS | classic + JSON のみ。Markdown / PDF / Insights / 他テンプレートは MUST NOT 追加 (Edge Cases / Assumptions で明示)。 |
| IV. テーブルテスト + Golden File | mocked + golden 強制 | ✅ PASS | FR-016 (JSON golden) + FR-017 (SVG golden + XML normalize + MD5) + FR-018 (上流 fixture diff)。 |
| V. Go 規約と言語ポリシー | Go latest, cmd/+internal/, 日本語 docs / 英語 comments | ✅ PASS | 既存 `internal/engine` / `internal/templates/` 配下に追加。`html/template` 不使用 (FR-011)、SVG 安全エスケープは明示的に行う。 |

**Gate result**: PASS — 違反なし、Complexity Tracking 不要。

### Post-Design Constitution Check (after Phase 1)

| 原則 | 設計後の再評価 |
|---|---|
| I | 新規入力なし。data-model に新 field なし、新 contract は engine 出力 (`Result.Output/MIME`) のみで上流互換性に影響しない。差分なし。 |
| II | `contracts/json-output.md` / `contracts/classic-template.md` で JSON キー集合および SVG DOM の正規化アルゴリズムを契約化。golden は決定論的に生成される。違反なし。 |
| III | `contracts/` には Markdown / PDF / Insights / 未採用 template の言及なし。違反なし。 |
| IV | `contracts/json-output.md` で `data.Plugins` への循環参照テストケースを明文化、`contracts/classic-template.md` で各 partial の golden 生成手順を明示。 |
| V | `data-model.md` の field 名、`contracts/` 中の Go identifier はすべて英語。partial を `html/template` ではなく string builder で組み立てる方針を `contracts/classic-template.md §3` で明示。 |

**Gate result**: PASS — 設計でも違反なし。

## Project Structure

### Documentation (this feature)

```text
specs/002-output-classic-json/
├── plan.md                # 本ファイル
├── research.md            # Phase 0 成果物 (decisions + alternatives)
├── data-model.md          # Phase 1 成果物 (Result 拡張 / Marshal / Partials の Go 型)
├── quickstart.md          # Phase 1 成果物 (開発者初期セットアップ手順)
├── contracts/             # Phase 1 成果物
│   ├── json-output.md         # トップレベル JSON shape, cycleDetector, 上流互換性
│   ├── classic-template.md    # image.svg → partial dispatch → footer の組立規則
│   └── result-dispatch.md     # Request.Format → Result.Output/MIME のディスパッチ規則
├── checklists/
│   └── requirements.md    # /speckit-specify で生成済み
└── tasks.md               # /speckit-tasks 出力 (未作成)
```

### Source Code (repository root)

M1 の構成から **新規** に追加されるファイルのみ列挙する (既存ファイルへの編集は Plan 内で明記する)。

```text
github-metrics/
├── internal/engine/
│   ├── engine.go          # 既存。Result 拡張 + Format ディスパッチを追加。
│   └── json.go            # 新規: Marshal(*plugins.Data) + cycleDetector。
├── internal/templates/classic/
│   ├── classic.go         # 新規: Template 実装 + image.svg 読込 + partial loop。
│   ├── classic_test.go    # 新規: Template.Check / Run の単体テスト。
│   └── partials/
│       ├── base_header.go              # 新規: PartialFunc。
│       ├── introduction.go             # 新規: stub (data.Plugins["introduction"] 不在で空文字列)。
│       ├── base_activity_community.go  # 新規: PartialFunc。
│       ├── base_repositories.go        # 新規: PartialFunc。
│       └── partials_test.go            # 新規: 各 partial の goldenfile テスト。
├── assets/templates/classic/partials/
│   └── _.json             # 上書き: M2 で実装する 4 partial のみへ書き換え (T-023 N-001)。
├── tests/
│   ├── fixtures/upstream/
│   │   └── octocat.json   # 新規: `make sync-fixtures` で取得する上流 JSON。
│   ├── golden/
│   │   ├── json/octocat.json          # 新規: 本実装 JSON 出力の期待値。
│   │   └── classic/octocat.svg        # 新規: 本実装 classic SVG 出力の期待値。
│   ├── compatibility/
│   │   └── json_test.go               # 新規: 上流 fixture と本実装 JSON のキー集合 diff (SC-001)。
│   └── integration/
│       └── output_test.go             # 新規: engine.Compute で json/svg 両経路を end-to-end 検証。
└── internal/tools/sync-fixtures/      # 新規: ./org_repo の出力例から octocat.json 等を取得するスクリプト。
    └── main.go
```

**Structure Decision**: M1 で確立した `internal/engine` / `internal/templates` を **拡張** する形で実装。新規パッケージ追加は最小限 (`internal/templates/classic` ディレクトリ 1 つのみ)。テストは既存の `tests/golden/`, `tests/compatibility/` 階層をそのまま使う。`internal/tools/sync-fixtures` は `check-compat` と同じ tool 階層に置く。

## Complexity Tracking

> Constitution Check は PASS につき記入不要。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (なし) | — | — |
