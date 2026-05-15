# Implementation Plan: GitHub プラグイン群 (M4)

**Branch**: `004-m4-github-plugins` | **Date**: 2026-05-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-m4-github-plugins/spec.md`

## Summary

M3 PR #112 マージ時点では classic SVG に "avatar + login + 基本統計" 程度しか乗っておらず、上流 `lowlighter/metrics` 採用構成 (主要言語 / 活動 / バッジ / pinned / contribution カレンダー) は全く再現できていない。本 feature は採用 21 プラグインを **3 つの user story (P1 MVP 5 個 / P2 GraphQL+REST 12 個 / P3 chromedp+heavy 4 個)** に分割して実装する。同時に M1 から残されていた `base` プラグインの未完部分 (organization ブランチ完成 / indepth クエリ / repositories ページング batch-halving) を M4 で完成形に拡張する。新規依存は `github.com/go-enry/go-enry` (言語分類) + `github.com/go-git/go-git` (浅い clone) の 2 つに限定し、その他は M2/M3 で導入済みのライブラリのみで完結させる (constitution 原則 V)。

## Technical Context

**Language/Version**: Go 1.26 (M1/M2/M3 から継続)。

**Primary Dependencies**:

- 標準: `context`, `encoding/json`, `time`, `strings`, `sort`, `sync`, `errors`, `fmt`, `regexp`
- M1/M2/M3 で導入済み: `log/slog`, `Khan/genqlient`, `hashicorp/go-retryablehttp`, `golang.org/x/sync/errgroup`, `chromedp/chromedp`, `PuerkitoBio/goquery`
- **新規追加 (本 spec)**:
  - `github.com/go-enry/go-enry/v2` — GitHub linguist の Go 移植。`languages.recent` / `languages.indepth` で commit diff 内のファイル拡張子 → 言語マッピングに使う。alternatives は `src-d/enry/v2` (古い) と独自実装 (linguist の正規表現ロジックを再現するコストが高すぎる)。
  - `github.com/go-git/go-git/v5` — pure Go の git クライアント。`languages.indepth` で浅い clone (`Depth: 1`) を行う。alternatives は libgit2 (cgo 必須でクロスコンパイル困難) と `os/exec` で git CLI 呼び出し (Docker イメージに git バイナリ必須)。go-git は M3 で導入済みの go.sum と矛盾なし。

新規依存は **採用根拠 (代替検討と棄却理由) を `/speckit-tasks` 出力時 PR 本文に必ず記載** する (constitution 原則 V)。

**Storage**:

- runtime 永続化はなし。`languages.indepth` の clone は `os.TempDir()/metrics-indepth-<pid>-<repo>` に作り、`Run` 終了 (or `defer cleanup`) で `RemoveAll` する。
- `tests/golden/classic/m4/` 階層を新設し、各 plugin の partial SVG fragment golden を 1 個ずつ置く。
- `tests/fixtures/plugins/<name>/` 階層に mocked GraphQL/REST レスポンス JSON を置く。

**Testing**:

- 通常テスト (`go test ./...`): chromedp / git / linguist 不要で全 plugin の table test + golden を緑にする。FakeRenderer (M3) + mocked GraphQL/REST (M1) で完結。
- chromedp 依存テスト (`make test-chromedp`, build tag `chromedp`): `topics` / `starlists` プラグインの実 chromedp scrape を 1 ケースずつ。
- heavy テスト (`make test-heavy`, build tag `heavy`, 新規): `languages.recent` / `languages.indepth` の go-enry + go-git 経路。実 git clone は使わず、`t.TempDir()` 配下に組み立てた小さな .git ディレクトリを fixture として使う。
- 上流互換性テスト (`tests/compatibility/json_test.go`): M2 で確立済の `sync-fixtures` 実行結果 (`tests/fixtures/upstream/octocat.json`) と本 spec で完成形となる `data.Plugins.<name>` を key/型レベルで diff 0 (SC-004)。

**Target Platform**: 既存 (GitHub Actions runner / 配布バイナリ)。`topics` / `starlists` は chromedp ジョブで、`languages.indepth` は通常ジョブで動く (go-git は pure Go なので chromium 非依存)。Docker は M10 で linguist + git バイナリは追加不要 (go-git + go-enry が pure Go)。

**Project Type**: 単一 Go プロジェクト (`cmd/` + `internal/`)。

**Performance Goals**:

- `Compute` フルパス (21 plugin 並列 + chromedp 込み、mocked deps) p95 **< 5 秒** (SC-003 / constitution Technical Constraints)。
- 各 plugin 単体 p95 **< 1 秒** (mocked deps、FR-013)。
- `languages.indepth` のみ別予算: `plugin_languages_analysis_timeout_repositories` 既定 **7.5 分 / repo**、全体タイムアウト **15 分** (上流既定値、`docs/design/06-plugins-detail.md §2.12` 準拠)。
- メモリピーク **< 800 MB** (SC-009、mocked 並列 21 プラグイン)。

**Constraints**:

- **原則 I (input compat)**: 各 plugin の `plugin_<name>_<opt>` キー名・型・既定値は上流 `metadata.yml` と完全一致。未知キーは黙って素通り MUST。
- **原則 II (output contract)**: `data.Plugins[<name>]` の Go 構造体は上流 `metrics.json` の `data.plugins.<name>` と key / 型 / null 表現が完全一致 MUST。SVG partial は DOM 単位互換 (テキストリテラルの差分は許容)。
- **原則 III (scope)**: 採用 21 プラグインのみ実装。不採用 19 個 (FR-018) のコードを新規追加 MUST NOT。`compliance_test.go` が `internal/plugins/` 配下を scan して regression を防ぐ。
- **原則 IV (tests + golden)**: 各 plugin に最低 5 ケースのテーブルテスト + 1 つの SVG partial golden + 1 つの JSON shape golden (`tests/golden/json/m4/<plugin>.json`)。
- **原則 V (Go conventions)**: 新規依存は go-enry + go-git の 2 つのみ。採用根拠は PR 本文に明記。コードコメント英語、docs 日本語。
- **後方互換**: M2/M3 で `engine.Compute` / `core.RunPlugins` を呼ぶ既存テストは本 spec で touched しない (`Result.Output` / `Result.MIME` / `Result.Errors` 契約は維持)。
- **scope 不足の挙動統一**: `read:project` / `read:user` / `read:org` / `repo` 不足を検出した plugin は `data.Plugins[<name>].skipped=true` を記録、WARN ログ 1 行、`Result.Errors` には何も追加しない (FR-010)。検出ロジックは `pc.REST.Scopes()` (M1 で導入済の token scopes helper) を使う。

**Scale/Scope**:

- 本 spec の実装単位: 21 個の plugin パッケージ + base plugin 拡張 (T-018/T-019/T-020 相当) + classic partial 21 個 + テスト + golden + classic top-level template の組み立て改修。
- 想定追加パッケージ: `internal/plugins/{languages,activity,achievements,repositories,isocalendar,calendar,habits,stars,topics,starlists,people,notable,contributors,reactions,projects,sponsors,sponsorships,stargazers,traffic}/` (19 ディレクトリ、`languages` 配下に `recent` / `indepth` サブモジュール)。
- 想定 LOC: 本体 +5500 (各 plugin 平均 ~250 行)、partial template +2000、テスト +4000。
- タスク粒度: `/speckit-tasks` で 21 plugin × (test ファースト 1 + 実装 1 + partial 1 + golden 1) = ~84 タスク + base 拡張 3 タスク + Setup/Foundational/Polish = 100 前後の見込み。

## Constitution Check

*GATE: Phase 0 前と Phase 1 後の両方で再評価する。*

### Initial Constitution Check (pre Phase 0)

| 原則 | 内容 | 適合性 | 根拠 |
|---|---|---|---|
| I. 入力互換性 (NON-NEGOTIABLE) | `metadata.yml` / `action.yml` キー完全互換 | ✅ PASS | 採用 21 plugin の `plugin_<name>_<opt>` キーはすべて M1 で `metadata.yml` に取り込み済。新規キー追加は **MUST NOT**。本 spec で新規 input 定義はゼロ。 |
| II. 出力契約 (DOM/JSON 単位) | JSON 完全互換、SVG/Markdown は DOM 同等 | ✅ PASS | 各 plugin の Go 構造体は上流 `data.plugins.<name>` と完全互換。SVG partial は DOM 単位互換。`tests/compatibility/json_test.go` (M2 で導入済) で key/型 diff 0 を担保 (SC-004)。 |
| III. スコープ規律 | 採用機能のみ実装 | ✅ PASS | 採用 21 plugin のみ。不採用 19 個 (FR-018) を `compliance_test.go` の scan 対象に追加し regression 防止。社外 API 系プラグイン (anilist 等) は **追加しない** MUST。 |
| IV. テーブルテスト + Golden File | mocked + golden 強制 | ✅ PASS | 各 plugin に table test + SVG partial golden + JSON shape golden。chromedp / heavy 経路は build tag で隔離 (FR-017)。 |
| V. Go 規約と言語ポリシー | Go latest, cmd/+internal/, 日本語 docs / 英語 comments | ✅ PASS | 新規依存 (go-enry, go-git) は採用根拠を本 plan + PR 本文に明示。新規ファイルはすべて `internal/plugins/<name>/` 配下。 |

**Gate result**: PASS — 違反なし、Complexity Tracking 不要。

### Post-Design Constitution Check (after Phase 1)

| 原則 | 設計後の再評価 |
|---|---|
| I | `contracts/plugin-base-extension.md` で base plugin の `Inputs` 解釈は M1 既存セットに準拠 (新規キーなし)。各 plugin contract で `plugin_<name>_*` キー一覧を列挙し、上流 metadata.yml と diff 0 (`scripts/check-metadata-keys.sh` を `/speckit-tasks` で計画)。差分ゼロ。 |
| II | `data-model.md` の各 PluginResult 構造体は上流 `metrics.json` の対応キーを `json:` tag で 1:1 マッピング。`contracts/partial-classic-m4.md` で SVG partial の DOM 階層 (`<g class="languages">` 等) を確定し、テキストリテラル差分のみ許容することを明示。 |
| III | `contracts/plugin-{p1,p2,p3}.md` の 3 ファイルに登場する plugin は採用 21 個のみ。不採用 19 個は **登場しない** ことを `compliance_test.go` が機械的に検証 (SC-007)。 |
| IV | `quickstart.md` に "新規 plugin 追加時の test ファースト手順" を 7 ステップで明示。各 contract に golden file path 命名規則を列挙。 |
| V | data-model 内の Go 型は英語、コメント英語。新規依存 (go-enry / go-git) の採用根拠は research.md R-002 / R-003 で詳述。 |

**Gate result**: PASS — 設計でも違反なし。

## Project Structure

### Documentation (this feature)

```text
specs/004-m4-github-plugins/
├── plan.md                            # 本ファイル
├── research.md                        # Phase 0 (採用判断 + 新規依存決定記録)
├── data-model.md                      # Phase 1 (21 PluginResult 構造体 + base 拡張型)
├── quickstart.md                      # Phase 1 (新規 plugin 追加手順 + テスト戦略)
├── contracts/                         # Phase 1
│   ├── plugin-base-extension.md       # base plugin の M4 拡張 (org branch / indepth / repos paging)
│   ├── plugin-p1-mvp.md               # P1 5 個 (languages 標準 / activity / achievements / repositories / isocalendar)
│   ├── plugin-p2-graphql.md           # P2 12 個 (habits / calendar / stars / people / notable / contributors / reactions / projects / sponsors / sponsorships / stargazers / traffic)
│   ├── plugin-p3-heavy.md             # P3 4 個 (languages.recent / languages.indepth / topics / starlists)
│   └── partial-classic-m4.md          # classic テンプレートに追加する 21 partial の DOM 階層と命名
├── checklists/
│   └── requirements.md                # /speckit-specify 出力済
└── tasks.md                           # /speckit-tasks 出力 (未生成)
```

### Source Code (repository root)

M3 までの構成に **新規** に追加されるファイルのみ列挙する。既存ファイルへの編集は本 plan 内で個別に明記する。

```text
github-metrics/
├── internal/plugins/                                # 既存。M4 で 19 plugin パッケージを追加。
│   ├── base/                                        # 既存。本 spec で完成形に拡張。
│   │   ├── base.go                                  # 既存。org ブランチ + indepth + paging を拡充。
│   │   ├── organization.go                          # 新規: organization-account 専用クエリ。
│   │   ├── indepth.go                               # 新規: commits / issues / pull-requests の追加クエリ。
│   │   ├── repositories.go                          # 既存。batch-halving + cursor 再帰を実装。
│   │   └── base_test.go                             # 新規/拡張: org / indepth / paging のテーブルテスト。
│   ├── languages/                                   # 新規 (P1: standard, P3: recent + indepth)。
│   │   ├── languages.go                             # 標準モード (P1: T-041)。
│   │   ├── recent.go                                # build tag=heavy (P3: T-042)。
│   │   ├── indepth.go                               # build tag=heavy (P3: T-043)。
│   │   ├── languages_test.go                        # 標準モードテーブルテスト。
│   │   ├── recent_heavy_test.go                     # build tag=heavy。
│   │   └── indepth_heavy_test.go                    # build tag=heavy。
│   ├── activity/                                    # 新規 (P1: T-044)。
│   ├── achievements/                                # 新規 (P1: T-045)。
│   ├── repositories/                                # 新規 (P1: T-046)。base.Computed.Repositories を消費。
│   ├── isocalendar/                                 # 新規 (P1: T-049)。
│   ├── calendar/                                    # 新規 (P2: T-050)。
│   ├── habits/                                      # 新規 (P2: T-051)。
│   ├── stars/                                       # 新規 (P2: T-052)。
│   ├── topics/                                      # 新規 (P3: T-053)。build tag=chromedp テスト。
│   ├── starlists/                                   # 新規 (P3: T-054)。build tag=chromedp テスト。
│   ├── people/                                      # 新規 (P2: T-055)。
│   ├── notable/                                     # 新規 (P2: T-056)。
│   ├── contributors/                                # 新規 (P2: T-059)。
│   ├── reactions/                                   # 新規 (P2: T-062)。
│   ├── projects/                                    # 新規 (P2: T-063)。
│   ├── sponsors/                                    # 新規 (P2: T-064)。
│   ├── sponsorships/                                # 新規 (P2: T-065)。
│   ├── stargazers/                                  # 新規 (P2: T-066)。worldmap option は地理 API なしで `nil` 返却 (M4 では実装しない、将来 N-task)。
│   └── traffic/                                     # 新規 (P2: T-068)。
├── internal/templates/classic/
│   ├── classic.go                                   # 既存。21 plugin 分の partial 呼び出しを追加。
│   └── partials/                                    # 新規ディレクトリ。
│       ├── languages.svg.tmpl                       # 新規 partial (21 個)。
│       ├── activity.svg.tmpl
│       ├── achievements.svg.tmpl
│       ├── ...                                      # 残り 18 partial 略 (各 plugin に 1 個)。
│       └── traffic.svg.tmpl
├── internal/githubapi/
│   └── scopes.go                                    # 既存 (M1 想定) または新規: `REST.Scopes() []string` の helper。各 plugin が scope 不足を判定するのに使う。
├── assets/plugins/
│   └── base/schema.graphql                          # 既存 (M2)。M4 で `organization` / `pinnedItems` / `contributionsCollection` を含む節を追加し、genqlient で型再生成。
├── tests/
│   ├── golden/
│   │   ├── classic/                                 # 既存。`octocat.svg` (M2) を M4 で再生成 (21 plugin 分が増える)。
│   │   ├── classic/m4/                              # 新規ディレクトリ。各 plugin の partial fragment golden。
│   │   │   ├── languages.svg
│   │   │   ├── activity.svg
│   │   │   └── ...                                  # 21 個。
│   │   └── json/m4/                                 # 新規ディレクトリ。各 plugin の JSON shape golden。
│   │       ├── languages.json
│   │       ├── activity.json
│   │       └── ...                                  # 21 個。
│   ├── fixtures/
│   │   └── plugins/                                 # 新規階層。各 plugin の mocked レスポンス JSON。
│   │       ├── languages/octocat_repos.json
│   │       ├── activity/octocat_events.json
│   │       └── ...
│   ├── integration/
│   │   ├── plugins_p1_test.go                       # 新規。5 plugin (P1) を classic + JSON で e2e。
│   │   ├── plugins_p2_test.go                       # 新規。12 plugin (P2) を分割 e2e。
│   │   ├── plugins_p3_chromedp_test.go              # 新規 (build tag=chromedp)。topics / starlists。
│   │   └── plugins_p3_heavy_test.go                 # 新規 (build tag=heavy)。languages.recent / indepth。
│   └── compatibility/
│       └── json_test.go                             # 既存 (M2)。M4 完了時に 21 plugin を含む完全 fixture に差し替え。
├── Makefile                                         # 既存。`test-heavy` / `gen-graphql-m4` ターゲット追加。
└── .github/workflows/go-ci.yml                      # 既存。`test-heavy` ジョブを `test-chromedp` と並列で追加。
```

**Structure Decision**: M1/M2 で確立した `internal/plugins/<name>/` 単位の独立パッケージ構成を踏襲。各 plugin は自己完結 (`<name>.go` + `<name>_test.go` + 必要なら `<name>_partial.go.tmpl` の参照) で、依存は `pc.GraphQL` / `pc.REST` / `pc.Imports.Get("base")` のみ。chromedp / heavy 経路は build tag で隔離し、通常 CI ジョブは pure Go で完結。classic テンプレートはトップレベル `classic.go` から partial を呼び出すだけで、partial 同士は独立 (テスト時に partial 単体で golden 検証可能)。`languages` だけサブモジュールが 3 つ (`languages.go` / `recent.go` / `indepth.go`) になるが、これは上流の `plugin_languages_sections` 切り替えと完全に対応するので妥当な分割。

## Complexity Tracking

> Constitution Check は PASS につき記入不要。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (なし) | — | — |
