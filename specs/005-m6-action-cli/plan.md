# Implementation Plan: M6 — GitHub Action / CLI

**Branch**: `005-m6-action-cli` | **Date**: 2026-05-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-m6-action-cli/spec.md`

## Summary

M4 PR #217 マージ時点で `engine.Compute` + 21 採用 plugin + classic template + chromedp render pipeline はすべて完成しているが、本プロジェクトは「ライブラリ / SDK」として提供する形態ではなく、**ユーザ READMEに metrics を貼るための実行手段** を提供する必要がある。M6 は M1-M4 の成果物を `metrics-action` という単一バイナリにラップし、(a) GitHub Actions の `uses:` 行で起動する Action モード、(b) ローカル開発者 / スクリプトから直接叩く CLI モード、の 2 経路で公開する。upstream `lowlighter/metrics@latest` の **drop-in 代替** になることが M6 の存在意義。Spec の 4 user story (P1 個人開発者 Action / P1 data-changed / P2 PR モード / P2 CLI) を独立にテスト可能な単位として、Phase 1-3 (Setup + Foundational + US1 MVP) → Phase 4 (US1 data-changed + US2 PR) → Phase 5 (US2 CLI) → Phase 6 (Polish + Docker + Release pipeline) で段階リリースする。新規依存は spec clarification Q3 で確定した `*xerrors.RetryableError` 経路の再利用 + CLI flag パース用 `flag` (標準) のみ、外部ライブラリ追加ゼロ。

## Technical Context

**Language/Version**: Go 1.26 (M1-M4 から継続)。

**Primary Dependencies**:

- 標準: `context`, `os`, `os/exec`, `flag`, `encoding/json`, `gopkg.in/yaml.v3`, `time`, `strings`, `errors`, `fmt`, `path/filepath`
- M1-M4 から再利用:
  - `internal/engine` (Compute 本体)
  - `internal/config` (Settings / MetadataLoader / Token / LoadPresets)
  - `internal/githubapi` (REST / GraphQL クライアント)
  - `internal/httpx` (retry middleware)
  - `internal/errors` (`xerrors.RetryableError` — clarification Q3 で確定した retry trigger)
  - `internal/render` (`render.Hash` — `output_condition=data-changed` の Hash 比較に使用、M3 で landing 済)
  - `internal/plugins/*` (21 採用 plugin が side-effect import で register)
  - `internal/templates/classic` (classic template が register)
- **新規追加 (本 spec)**: なし。CLI フラグは標準 `flag`、YAML は既存の `gopkg.in/yaml.v3` (M1 で導入済) を流用。Docker / GHCR は外部ツールチェーンなので Go モジュールには影響しない。

新規 Go 依存ゼロ。「依存追加の採用根拠を PR 本文に記載 (constitution 原則 V)」の対象なし。

**Storage**:

- runtime 永続化なし。
- `/renders/<filename>` (Action モードの SVG/PNG 等の出力先) は Docker コンテナの ephemeral storage。Action 終了後は破棄される。
- CLI モードの `--filename <path>` はユーザ指定パス、`--filename -` で stdout。
- `output_condition=data-changed` は GitHub Contents API から既存ファイルを取得し `render.Hash` で比較 (M3 既存ロジック再利用)。

**Testing**:

- 通常テスト (`go test ./...`): chromedp 不要で全 action / cli 経路の table test + golden が緑。FakeRenderer (M3) + mocked GraphQL/REST (M1) + httptest server (GitHub Contents API mock) で完結。
- chromedp 依存テスト (`make test-chromedp`, build tag `chromedp`): 既存 M3/M4 の chromedp 経路は変更なし。M6 で新たに追加する chromedp テストは無し (CLI/Action は chromedp を engine 経由で間接利用するだけ)。
- heavy テスト (`make test-heavy`): 既存 M4 経路変更なし。M6 で追加なし。
- 上流互換性テスト: spec FR-008 (`_filename` ワイルドカード) + FR-009 (`dryrun`) + FR-014 (commit) + FR-015 (PR) は、`tests/integration/` 配下に既存の plugins_p* と同じパターンで Action / CLI 統合テストを追加する。token validation (FR-004) + retry classification (FR-007) + 未対応 output_action 拒否 (FR-015b) は `internal/action/` の単体テストでカバー。
- M4 で確立した `tests/compliance/compliance_test.go::TestCompliance_M4_AdoptedPlugins` + `TestNoUnadoptedPluginReference` を `internal/action/` + `cmd/metrics-action/` 配下まで scan 範囲に拡張 (SC-012)。

**Target Platform**:

- Action モード: GitHub Actions runner (Linux x86_64) — Docker image `ghcr.io/mjun0812/github-metrics:<tag>` を `docker run` する形 (action.yml `runs.using: docker`)。
- CLI モード: macOS / Linux (clarification Q2 で確定: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)。Windows は MVP スコープ外 (assumption)。
- Docker は既存 M3 chromedp 環境 (chromium 同梱) を継承。

**Project Type**: 単一 Go プロジェクト (`cmd/` + `internal/`)。M6 で 2 新規ディレクトリ:

- `cmd/metrics-action/` (Action + CLI 兼用バイナリの main)
- `internal/action/` (input parsing, committer, retry, token validation, banner)

**Performance Goals**:

- Action 起動 → 初回 SVG 出力 (mocked deps) **< 30 秒** (SC-001 mocked)。
- 同上 (real GitHub) **< 60 秒** (SC-001 real)。
- CLI 起動 → SVG stdout (mocked deps) **< 30 秒** (SC-008)。
- 上記のうち `engine.Compute` 部分は M4 で確立済の p95 = 190 ms (SC-003)。残り時間予算は IO / Docker overhead / committer API 呼び出しに割り当てる。

**Constraints**:

- **原則 I (input compat)**: action.yml の input 名・既定値・型は upstream `metadata.yml` と完全一致 MUST (採用 21 plugin + core 入力に限定)。`INPUTS` JSON 形式と `INPUT_<UPPER>` env var の両経路をサポート (clarification Q なしで spec 既定)。
- **原則 II (output contract)**: M6 は新規 plugin / template を追加しないので出力 DOM 変化なし。`metrics_url` / `metrics_sha` Action outputs (FR-010) は upstream の `@actions/core` v2 仕様 (`GITHUB_OUTPUT` env var に `key=value` を append) と一致。
- **原則 III (scope)**: 採用 21 plugin / classic template 外の機能 (Web UI / OAuth / insights / markdown / gist / repository template) を **追加しない** MUST。未対応 `output_action` 値 (`gist`, `markdown-*`) は **fail fast + migration message** で拒否 (clarification Q4 で確定、FR-015b)。
- **原則 IV (tests + golden)**: action / CLI の入力パース + retry + committer + token validation は table test、Action 起動 / CLI 起動 / dryrun output は integration test、Docker image build は M10 で扱う。SC-001 〜 SC-013 の 13 メトリクスがそれぞれ deterministic test case を持つ。
- **原則 V (Go conventions)**: 新規依存ゼロ (上記 Primary Dependencies 参照)。コードコメント英語、運用ログ英語固定 (clarification Q5 で確定、FR-003)、spec / commit / 会話は日本語。
- **Retry classification**: `*xerrors.RetryableError` でラップされた error のみ retry、それ以外 fail fast (clarification Q3 で確定、FR-007)。M4 で確立された marker を action 経路で再利用する。
- **Token masking**: 全 log 経路 (banner / retry log / error wrap / slog field) で `<PAT>` 文字列が漏れない (FR-022 / SC-011)。
- **Logging defaults**: `slog` Default level=info、operator-facing log は **English 固定** (clarification Q5)。`docs/design/13-appendix.md §E` の banner 整形ルールに準拠。

**Scale/Scope**:

- 1 GitHub Actions run = 1 metrics-action 起動 = 1 ユーザ分の SVG 生成 (concurrent 同一 binary 起動なし)。
- CLI モードも基本は single-shot (`--watch` / daemon mode は MVP 外、ユーザの cron / GitHub schedule に任せる)。
- 配布: GHCR Docker image (`ghcr.io/mjun0812/github-metrics:vX.Y.Z` + `:latest` + `:sha-<short>`、clarification Q1) + GitHub Releases binary 4 platforms (clarification Q2)。
- spec FR-001-FR-023 (23 件), SC-001-SC-013 (13 件), Edge cases 12 件, Assumptions 17 件 を実装範囲とする。

## Constitution Check

*GATE: Phase 0 research 前に PASS、Phase 1 design 後に再評価。*

| 原則 | 状態 | コメント |
|-----|-----|---------|
| **I. 入力互換性 (NON-NEGOTIABLE)** | ✅ PASS | action.yml inputs は upstream metadata.yml 由来 (21 採用 plugin + core)。`INPUTS` JSON / `INPUT_<UPPER>` env var の 2 経路、優先度も upstream 互換 (FR-001)。未対応の `output_action` 値だけ fail-fast (FR-015b) だが、これは入力解釈の問題ではなく値検証なので原則違反ではない (`metadata.yml` のキー自体は素通り)。 |
| **II. 出力契約 (DOM/JSON 単位)** | ✅ PASS | M6 は新規 plugin / template / DOM を追加しない。`metrics_url` / `metrics_sha` Action outputs (FR-010) は upstream `@actions/core` v2 仕様準拠。 |
| **III. スコープ規律** | ✅ PASS | Web (M5) / markdown / gist / insights / repository template / community plugins は **assumption / FR-015b で明示的 out-of-scope**。`internal/action/` + `cmd/metrics-action/` を `TestCompliance_M4_AdoptedPlugins` + `TestNoUnadoptedPluginReference` の scan 範囲に拡張する (SC-012)。 |
| **IV. テーブルテスト + Golden File** | ✅ PASS | SC-004 (7 controlled-error matrix) / SC-005 (data-changed 3 シナリオ) / SC-007 (output_action 失敗 4 + 未対応値 2 ケース) / SC-009 (CLI ↔ Action 5 paired test cases) / SC-010 (7 retry/quota/token paths) / SC-011 (4 token masking paths) と、SC は全部 deterministic test に落ちる。Golden は banner snapshot (SC-003) + 既存 plugin/template (M2-M4) を継承。 |
| **V. Go 規約と言語ポリシー** | ✅ PASS | 新規 Go 依存ゼロ。runtime log English 固定 (clarification Q5)、spec / commit / 会話日本語。`cmd/metrics-action/` + `internal/action/` の二層配置 (constitution の `cmd/` + `internal/` 規約準拠、`pkg/` 不使用)。 |

**Gate result**: 全 5 原則 PASS、Phase 0 / Phase 1 へ進行可能。再評価は Phase 1 完了後に再実施。

## Project Structure

### Documentation (this feature)

```text
specs/005-m6-action-cli/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── action-yml.md            # action.yml input schema (21 plugin + core inputs)
│   ├── cli-flags.md             # CLI フラグ仕様 (--config / --user / --template / --plugin / --output / --filename / --token / --token-env / --dryrun)
│   ├── committer.md             # commit / pull-request / pull-request-{merge,squash,rebase} 経路の挙動
│   ├── output-actions.md        # サポート値リスト + 未対応値 (`gist`, `markdown-*`) の fail-fast migration message
│   ├── retry-policy.md          # *xerrors.RetryableError 経路のみ retry、それ以外 fail-fast の決定論
│   └── token-validation.md      # github_pat_* 拒否 + scope 警告 + quota チェックの判定マトリクス
├── checklists/          # Quality gate checklists
│   └── requirements.md
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── metrics-action/
    └── main.go                  # 単一 binary の entry point (Action / CLI 兼用、GITHUB_ACTIONS env 検出で分岐)

internal/
├── action/                      # 新規パッケージ (M6 の追加コードはほぼここに集約)
│   ├── action.go                # FR-001 / FR-002 (skip 判定、INPUTS / INPUT_<UPPER> パース、Compute 駆動)
│   ├── banner.go                # FR-003 (startup banner、English 固定、docs/design/13-appendix.md §E 形式)
│   ├── cli.go                   # FR-017-FR-020b (CLI flag set、YAML config loader、--token-env)
│   ├── committer.go             # FR-014 / FR-015 (commit / PR / merge variants の committer)
│   ├── inputs.go                # FR-006 (presets 統合)、FR-008 (_filename ワイルドカード)
│   ├── output_action.go         # FR-015b (サポート値検証 + 未対応値 fail-fast migration message)
│   ├── outputs.go               # FR-010 ($GITHUB_OUTPUT への metrics_url / metrics_sha append)
│   ├── retry.go                 # FR-007 (*xerrors.RetryableError のみ retry、それ以外 fail-fast)
│   ├── token.go                 # FR-004 (github_pat_* 拒否 + scope check + quota check)
│   ├── data_changed.go          # FR-012 / FR-013 (output_condition=data-changed の Hash 比較)
│   ├── notice.go                # FR-021 (notice_releases / 新版アナウンス)
│   └── *_test.go                # 上記すべての単体テスト + table test
├── engine/                      # 既存 (M1-M4)
├── plugins/                     # 既存 (M1-M4、21 plugin)
├── templates/classic/           # 既存 (M2-M4)
├── render/                      # 既存 (M3 — render.Hash を data_changed.go から呼ぶ)
├── githubapi/                   # 既存 (M1 — committer.go から PUT /git/refs / PUT /contents / POST /pulls)
├── httpx/                       # 既存 (M1 — retry middleware)
├── errors/                      # 既存 (M1 — *xerrors.RetryableError)
├── config/                      # 既存 (M1 — Settings / MetadataLoader / LoadPresets)
└── ...

action.yml                       # 新規 — 21 採用 plugin + core inputs の Action 定義 (1600 行 upstream の subset)
Dockerfile                       # 新規 / 拡張 — multi-stage build、最終 image に metrics-action binary + chromium 同梱
.github/workflows/release.yml    # 新規 — semver tag → GHCR push + GitHub Releases binary upload

tests/
├── integration/
│   ├── action_test.go           # 新規 — Action モード integration (mocked GitHub + dryrun + commit / PR 経路)
│   ├── cli_test.go              # 新規 — CLI モード integration (--user / --template / --plugin / --dryrun / --filename)
│   └── (既存 M1-M4 のテストはそのまま)
└── compliance/
    └── compliance_test.go       # 既存を拡張 — scanRoots に "internal/action", "cmd/metrics-action" を追加
```

**Structure Decision**: 単一 Go プロジェクト (constitution 原則 V) を継続。新規追加は `cmd/metrics-action/main.go` (entry point) + `internal/action/` パッケージ + ルート `action.yml` + ルート `Dockerfile` (M10 で finalize) + `.github/workflows/release.yml` (M10 で finalize)。すべての runtime ロジックは `internal/action/` に集約し、`main.go` は単に `action.Run(ctx)` / `action.RunCLI(ctx)` を呼ぶだけの薄い shim にする。これにより `cmd/` 配下は薄く保ち、テストは `internal/action/` に対して書ける (Go テストは `_test.go` で internal も exported もカバーできる)。

## Complexity Tracking

> Constitution Check が PASS のため、本セクションは記入しない。違反案件は無し。

## Phase 0: Research Output

Phase 0 で解決すべき NEEDS CLARIFICATION は spec の `/speckit-clarify` セッションで 5 件すべて解決済 (Q1-Q5)。残存の研究タスクは下記 7 件:

1. `metrics-action` Docker image の base image 選定 — chromium 含む既存 M3 image を継承するか、より軽量な alpine + apt chromium に切り替えるか。
2. `*xerrors.RetryableError` を Action / CLI 経路から判定する API 設計 — direct type assertion vs `errors.As` 経由。
3. `GITHUB_OUTPUT` への append 仕様 (`@actions/core` v2) の正確な format (KEY=VALUE 改行区切り? heredoc 必要?)。
4. action.yml inputs を upstream metadata.yml subset として生成する方法 — 手動コピー vs `internal/tools/gen-action-yml/` ジェネレータ。
5. CLI flag パーサ選定 — 標準 `flag` vs `cobra` vs `spf13/pflag` 単体。
6. data-changed 用の既存ファイル取得 — `GET /repos/.../contents/<filename>` のレスポンス形式 (base64 encoded body) と `render.Hash` の互換性。
7. banner format の M3 / M2 で既存ログとの整合 — slog handler を切り替えるか、`internal/action/banner.go` で自前 fmt するか。

→ `research.md` で各項目に「Decision / Rationale / Alternatives considered」を記述。

## Phase 1: Design Output

Phase 0 が完了したら以下を生成:

1. **`data-model.md`** — spec の Key Entities (Action invocation context / Committer / Hash comparator / CLI flag set / Preset bundle / Retry policy) を Go 構造体定義レベルまで具体化。`internal/action/*.go` のファイル単位と対応させる。

2. **`contracts/`** — 6 ファイル (上記 Project Structure 参照):
   - `action-yml.md` — action.yml の inputs 定義 schema (21 plugin + core)。upstream metadata.yml との 1:1 対応マップ。
   - `cli-flags.md` — CLI フラグ仕様。各フラグの type / default / 互換 INPUT 名。
   - `committer.md` — commit / pull-request / `*-merge` / `*-squash` / `*-rebase` の経路毎の API 呼び出しシーケンス。
   - `output-actions.md` — サポート / 未対応値リスト + migration message テンプレート。
   - `retry-policy.md` — `*xerrors.RetryableError` の判定論理 + retry budget consumption rule。
   - `token-validation.md` — `github_pat_*` 検出 / scope mask / quota check の判定マトリクス。

3. **`quickstart.md`** — 開発者向け M6 着手手順 (新規 plugin 開発手順は M4 quickstart で確立済、M6 は action / CLI 開発の流れ + `metrics-action` ローカル実行手順 + Docker build + release pipeline を扱う)。

4. **Agent context update**: `CLAUDE.md` 内の `<!-- SPECKIT START -->` ブロックで `Active plan` を `specs/004-m4-github-plugins/plan.md` → `specs/005-m6-action-cli/plan.md` に差し替え (Phase 1 で IMPL_PLAN ができたタイミングで実施)。

## Phase 2: Task Generation

`/speckit-tasks` で生成 — `tasks.md` (M4 の T001-T100 と同じ書式) に分解。Phase 構成案:

- **Phase 1: Setup** (Makefile / Dockerfile / GHCR workflow / action.yml の骨格)
- **Phase 2: Foundational** (`internal/action/{action,inputs,token,retry,outputs,banner}.go` の base helpers)
- **Phase 3: US1 (P1 MVP)** — Action mode で commit 経路まで動かす
- **Phase 4: US2 (P1 data-changed + P2 PR)** — `data_changed.go` + `committer.go` PR variants
- **Phase 5: US3 (P2 CLI)** — `cli.go` + flag parsing + YAML config loader
- **Phase 6: Polish** — release pipeline 完成 + GitHub Releases binary cross-build + Docker image publish + compliance test 拡張 + quickstart 完走

Phase ごとに PR 分割 (M4 と同じ 4 PR 体制を踏襲)。

## Post-Design Constitution Re-Check

Phase 1 完了後に再評価。現時点では Phase 0 / Phase 1 設計を経ても constitution 違反項目は予測されないため、PASS 維持の見込み。
