# Phase 0 Research: M6 — GitHub Action / CLI

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は plan.md Phase 0 で列挙した 7 件の研究タスクに対する Decision / Rationale / Alternatives considered を記録する。spec の Clarifications session で解決済の 5 件 (Q1-Q5) は再記述しない。

---

## R-001: Docker image の base 選定

**Decision**: M3 で確立した chromium 同梱 image をベースに、`Dockerfile` を multi-stage build (`golang:1.26-alpine` builder + chromium runtime) で書き直す。最終 image サイズは ~500 MB 程度 (chromium が ~400 MB) を許容する。

**Rationale**:

- M3 PR #112 ですでに chromium 入り Dockerfile が動いており、`make test-chromedp` が green。`metrics-action` の chromedp 経由 SVG resize / PNG / JPEG 経路はこの image を継承するのが最短。
- alpine base に apt chromium を入れる alternative は、apt が無いので apk になり (alpine の `chromium` パッケージ)、版差で chromedp との接続が壊れた事例 (M3 R-003) があるため、リスク回避。
- ユーザは Docker image を pull するだけで Action は完結する。サイズが大きくても GitHub Actions の docker cache + reusable workflow キャッシュで毎回 pull するわけではない。

**Alternatives considered**:

- alpine + apk chromium: サイズ最小 (~200 MB) だが M3 で chromedp 接続不安定の事例あり。除外。
- distroless + system chromium: GoogleContainerTools/distroless は static binary 向けで、chromium のような巨大バイナリ + dynamic dep には不適。除外。
- multi-arch image (linux/amd64 + linux/arm64): GitHub Actions の Linux runner はすべて amd64 なので arm64 は不要。CLI binary の方 (clarification Q2) で arm64 をカバー。MVP では amd64 single-arch でリリース。

---

## R-002: `*xerrors.RetryableError` の判定 API

**Decision**: `internal/action/retry.go` で `errors.As(err, &re *xerrors.RetryableError)` を使う標準パターンに統一する。直接 type assertion (`err.(*xerrors.RetryableError)`) は wrapper チェーン (e.g. `fmt.Errorf("compute: %w", retryErr)`) で false になるため使わない。

**Rationale**:

- M4 PR #216 の Must Fix #1 でまさに「`interface{ Retryable() bool }` を局所定義した type assertion が常に false」問題を経験している。同じ落とし穴を action 経路で再現しないよう、最初から `errors.As` を統一規約にする。
- `errors.As` は wrapper チェーンを traverse するので、`engine.Compute` が `fmt.Errorf("engine: base: %w", retryErr)` で wrap してもチェーン先端の `*xerrors.RetryableError` を捕捉できる。
- Go 1.20+ 標準で、追加依存ゼロ。

**Alternatives considered**:

- direct type assertion: wrapper チェーンで失敗するため除外。
- `xerrors.IsRetryable(err) bool` ヘルパーを `internal/errors/` に追加: 1 行ラッパなので価値が低い + `errors.As` の方が Go 標準パターンとして可読性高い。`xerrors` 側に helper を追加する PR は将来検討可。

---

## R-003: `$GITHUB_OUTPUT` への append 仕様

**Decision**: `KEY=VALUE\n` の単純 append (single-line value) と、複数行値が必要なら heredoc 形式 (`KEY<<EOF\n...\nEOF\n`) の両方をサポート。`internal/action/outputs.go` に `SetOutput(key, value string)` ヘルパーを置き、value 内に改行が含まれる場合は自動で heredoc に切り替える。

**Rationale**:

- GitHub Actions の `@actions/core` v2 仕様 ([docs](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-output-parameter)):
  - 推奨: `echo "{name}={value}" >> "$GITHUB_OUTPUT"` (single-line)
  - 複数行: heredoc delimiter で囲む (delimiter は value に含まれない unique 文字列、典型 `EOF` + random suffix)
- `metrics_url` (URL 1 行) / `metrics_sha` (40 文字 hex) は single-line で十分。将来 `metrics_metadata` のような構造化 output を追加する余地を残すため、heredoc 自動切替を初版から実装する。
- delimiter は `strings.Replace` で value 内に存在しないことを確認しつつランダム suffix を付ける (典型 `EOF_<8byte hex>`)。

**Alternatives considered**:

- single-line only: `metrics_*` がすべて単行で済む現状なら十分だが、将来拡張時に "また仕様を追加" のコストを嫌う。
- single-line で改行を `%0A` URL エンコード: GitHub Actions の `set-output` workflow command (deprecated) で使われた古い形式。`@actions/core` v2 で heredoc に置き換わったため使わない。
- 専用ライブラリ (`sethvargo/go-githubactions`): 依存追加に値しない (本仕様は 30 行程度の自前ヘルパーで十分)。constitution 原則 V: 新規依存は必要十分性を厳しく審査。

---

## R-004: action.yml inputs の生成方法

**Decision**: `internal/tools/gen-action-yml/main.go` を新規追加し、`assets/plugins/<plugin>/metadata.yml` + `assets/templates/<template>/metadata.yml` を読み取って `action.yml` を機械生成する。手動 commit は禁止 (lefthook + CI で diff 検出)。

**Rationale**:

- upstream `lowlighter/metrics` の action.yml は **1600 行**、すべて plugin metadata から `@vercel/ncc` 相当のコード生成で作っている。手動 maintain は MVP 1 度限りの作業でも数十時間、以降の plugin input 追加 / 変更で毎回数時間。
- M1 で `assets/plugins/<plugin>/metadata.yml` は既に embed されており、`config.MetadataLoader` で読める。同じ情報源から action.yml を生成すれば、metadata 変更時の二重 maintain が消える。
- 生成ツールは Go で書く (`go run ./internal/tools/gen-action-yml --output ./action.yml`)。`make gen-action-yml` ターゲットを追加。
- 採用 21 plugin + core inputs のみ生成、不採用 19 plugin は無視 (constitution 原則 III: スコープ規律)。

**Alternatives considered**:

- 手動コピー: 上記の理由で除外。
- upstream の Node スクリプトをそのまま使う: `./org_repo` の Node 依存を解決する必要があり、Go プロジェクト single-stack のメリットを毀損。
- runtime generation (action.yml なし、metrics-action が起動時に inputs を validate): GitHub Actions UI で `with:` の autocomplete / type check が効かなくなり、ユーザ体験が劣化。除外。

---

## R-005: CLI flag パーサ選定

**Decision**: 標準 `flag` パッケージ + 必要なら `--plugin key=value` の repeatable flag 用に最小ラッパー (1 関数 ~20 行)。`cobra` / `spf13/pflag` は導入しない。

**Rationale**:

- M6 の CLI は single-shot 専用 (subcommand なし、`metrics-action --user octocat ...` の 1 形式のみ)。subcommand / 子コマンドツリーが不要なので `cobra` の利点 (subcommand routing, help generation) が活きない。
- `spf13/pflag` の利点 (POSIX-long-style: `--user`) は標準 `flag` も `-user octocat` と `--user octocat` 両方受け付けるので等価。
- repeatable `--plugin key=value` は `flag.Func` で `flag.Func("plugin", "...", func(s string) error { ... })` 一発実装可能 (~10 行)。
- constitution 原則 V: 新規依存ゼロを優先。

**Alternatives considered**:

- `cobra`: subcommand 必要時に再評価。MVP には過剰。
- `spf13/pflag` 単体: `flag` パッケージで足りるなら追加依存に値しない。
- `urfave/cli/v2`: cobra と同じ理由で除外。

---

## R-006: data-changed 用の既存ファイル取得 + Hash 比較

**Decision**: `GET /repos/<owner>/<repo>/contents/<filename>?ref=<branch>` でレスポンス body の `content` フィールドを base64 decode → byte 列に → `render.Hash(svgBytes)` で M3 既存 Hash と比較。一致なら `Committer.Commit = false`。

**Rationale**:

- GitHub Contents API のレスポンスは `{"content": "<base64>", "encoding": "base64", ...}` 固定形式。`base64.StdEncoding.DecodeString` で復元するだけ。
- `render.Hash(svg []byte) string` は M3 で既に landing 済 + `internal/render/svg_hash.go` の単体テストで安定動作確認済。M6 の data_changed.go からそのまま呼べる。
- 既存ファイルが存在しない (404) 場合は「変わった」と判定して通常 commit に進む (FR-012 の "first run commits when no existing file" 経路)。

**Alternatives considered**:

- `GET /repos/.../git/blobs/<sha>` で blob 直接取得: contents API より階層が深い (`GET /git/refs/heads/<branch>` → tree → blob)、本用途では contents API のシンプル path で十分。
- byte 比較 (`bytes.Equal`): 上流 metrics の SVG は version 文字列 + 生成時刻が embed されるため、byte 比較すると毎回 diff になり data-changed が事実上機能しない。`render.Hash` は M3 で DOM 正規化済の比較なのでこの問題を回避できる。

---

## R-007: banner format と slog handler

**Decision**: `slog.Default()` (M1 で確立の JSON handler / text handler どちらか) はそのまま、banner だけは `fmt.Fprintln(os.Stdout, ...)` で生 stdout に直接書く。`internal/action/banner.go` に `PrintBanner(version, template, plugins []string, mode string)` ヘルパーを置き、`docs/design/13-appendix.md §E` の format に準拠する固定 ASCII art を生成する。

**Rationale**:

- banner は「人間が GitHub Actions の Run log を見たときに最初に視認する Identifier」なので、JSON 形式 (slog default) では読みづらい。固定 ASCII art が運用上の標準。
- 一方 banner 以後の info / warning / error log は slog の structured logging を活かしたい (filter / parser 親和性)。
- ASCII art を slog の `slog.Info(...)` で出すと改行が escape されて読めない handler がある (text handler は OK だが JSON handler は failure)。`fmt.Fprintln` で stdout 直書きすれば handler 差を吸収できる。
- SC-003 (snapshot diff against upstream Node banner format) の verify は、banner 出力を `bytes.Buffer` に redirect する test util を `internal/action/banner_test.go` に置き、ASCII art の semantic 部分 (version / plugin list / template / mode) を行単位で diff する。

**Alternatives considered**:

- `slog.Info("banner", "lines", [...])` で list output: log handler の改行扱いに依存、SC-003 snapshot test が脆くなる。
- 完全に slog 経由 (テキスト handler 強制): JSON handler を使いたい運用者の選択肢を奪う。
- banner 専用 logger (slog.New(... handler with custom format)): 1 経路のために独立 logger を持つのは over-engineering。`PrintBanner` ヘルパー 1 関数で十分。

---

## まとめ

7 件の研究タスクすべてに Decision を確定。新規依存ゼロを維持 (R-002 / R-003 / R-005)。M3 既存資産 (R-001 chromium image / R-006 render.Hash) を最大限再利用し、M6 追加の表面積を `cmd/metrics-action/` + `internal/action/` + `action.yml` + `Dockerfile` + `.github/workflows/release.yml` の 5 種類に集約する。

次は Phase 1 で `data-model.md` / `contracts/` (6 ファイル) / `quickstart.md` を生成し、CLAUDE.md の Active plan reference を `005-m6-action-cli/plan.md` に更新する。
