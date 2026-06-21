# 11. Go 移植方針・ライブラリ対応表

> NOTE: This document describes pre-refactor (pre-#601) architecture. See [15-selection-answer.md](./15-selection-answer.md) for the current state.

## 目次

- [1. 全体方針](#1-全体方針)
- [2. ライブラリ対応表](#2-ライブラリ対応表)
- [3. 移植の難所と対応策](#3-移植の難所と対応策)
- [4. 互換性とマイグレーション](#4-互換性とマイグレーション)
- [5. フェーズ別マイルストーン](#5-フェーズ別マイルストーン)
- [6. リスクと未解決事項](#6-リスクと未解決事項)

---

## 1. 全体方針

### 1.1 移植優先度

1. **Compute エンジン** … `internal/engine`, `internal/config`, `internal/plugins/base`, `internal/plugins/core`
2. **GitHub API クライアント** … REST + GraphQL ラッパ
3. **Render パイプライン** … SVG → chromedp → PNG/PDF/Markdown
4. **テンプレート (classic)** … 既存 SVG 構造を Go で再現
5. **主要プラグイン** … languages, activity, achievements, repositories, gists, lines, isocalendar, habits, topics, calendar, stars
6. **Web インスタンス** … `cmd/metrics-server`
7. **Action / CLI** … `cmd/metrics-action`
8. **追加プラグイン / テンプレート** … 残りすべて

各フェーズで Node 版と Go 版の出力 (`config.output=json`) を diff してリグレッションを検出する。

### 1.2 設計指針

- **シンプル優先**: 過剰な抽象化を避け、Go らしい命名と short package を採用。
- **エラーは値**: `errors.As` / `errors.Is` を活用。プラグイン側は wrap して返す。
- **context.Context**: すべての I/O と長時間処理に渡す。
- **構造体公開**: テストしやすいよう interface はギャップを絞る。
- **不変データの即値化**: octicons / twemoji / partials は `embed.FS` で確定値として埋め込む。

## 2. ライブラリ対応表

| Node パッケージ | 役割 | Go 採用候補 | コメント |
|---------------|------|------------|----------|
| `express` + `compression` + `express-rate-limit` | HTTP / gzip / rate-limit | `github.com/go-chi/chi/v5` + `chi/middleware` + `github.com/go-chi/httprate` | 標準的な組合せ。`trust proxy=1` は `middleware.RealIP` |
| `@actions/core`, `@actions/github` | GH Actions I/O | 自作小ラッパ + `GITHUB_OUTPUT` ファイル append | core ライブラリは Go 公式版なし |
| `@octokit/graphql`, `@octokit/rest` | GitHub API | `github.com/google/go-github/v66` + `github.com/Khan/genqlient` (or `shurcooL/githubv4`) | genqlient で型生成 |
| `axios` | HTTP client | `net/http` + `github.com/hashicorp/go-retryablehttp` | helper を `internal/httpx` に集約 |
| `memory-cache` | TTL cache | `github.com/patrickmn/go-cache` | `Set/Get` 互換 |
| `ejs` | テンプレート | Go コードに手動移植 (or `goravel/framework/foundation/console` 経由 `text/template`) | 動的 JS 評価は不要なので Go テンプレートで十分 |
| `marked` | Markdown → HTML | `github.com/yuin/goldmark` + `extension.GFM` | 拡張で table/strikethrough |
| `sanitize-html` | HTML サニタイズ | `github.com/microcosm-cc/bluemonday` | UGCPolicy + 必要タグ追加 |
| `prismjs` | コード強調 | `github.com/alecthomas/chroma/v2` | HTML 出力フォーマッタ |
| `puppeteer` | ヘッドレスブラウザ | `github.com/chromedp/chromedp` | Chrome 同梱 Docker image |
| `sharp` | 画像 resize | `github.com/disintegration/imaging` + `golang.org/x/image` | cgo 不要 |
| `gifencoder` | GIF encode | 標準 `image/gif` | |
| `png-js` | PNG 解析 | 標準 `image/png` | |
| `linguist-js` | 言語判定 | `github.com/go-enry/go-enry/v2` | Linguist 互換 |
| `simple-git` | git 操作 | `github.com/go-git/go-git/v5` | pure-Go |
| `rss-parser` | RSS parse | `github.com/mmcdole/gofeed` | Atom 対応 |
| `open-graph-scraper` | OG メタ取得 | `github.com/dyatlov/go-opengraph` | |
| `js-yaml` | YAML | `gopkg.in/yaml.v3` | |
| `csso` + `purgecss` | CSS 最適化 | `github.com/tdewolff/minify/v2/css` + 自作 PurgeCSS 相当 | PurgeCSS は HTML + CSS から未使用セレクタを除く実装 |
| `svgo` | SVG 最適化 | 自作 (experimental) | 初版は最低限の minify、本格対応は v2 |
| `xml-formatter` | XML 整形 | 自作 (encoding/xml) | |
| `d3` | グラフ | 自作 SVG 生成 helper / `gonum.org/v1/plot` | |
| `@twemoji/parser` | 絵文字置換 | 自作 `internal/render/twemoji` | unicode regex |
| `@primer/octicons` | アイコン | data.json を `embed` | |
| `@primer/css` | markdown CSS | 必要 CSS のみ `embed` | markdown-pdf 専用 |
| `vue`, `vue-prism-component`, `clipboard` | フロント UI | そのまま静的配信 | 後継仕様で再実装 |
| `jsdom` | DOM 操作 | `golang.org/x/net/html` + `github.com/PuerkitoBio/goquery` | server 側 limited |
| `minimatch` | glob | `github.com/bmatcuk/doublestar/v4` | |
| `node-fetch` | fetch polyfill | 標準 `net/http` | |
| `eslint` | linter | `golangci-lint` | |
| `dprint` | formatter | `gofumpt` | |
| `jest` | test | 標準 `testing` + `stretchr/testify` | |
| `@faker-js/faker` | フェイクデータ | `github.com/jaswdr/faker` | |
| `libxmljs2` | SVG verify | `encoding/xml` + 自作 schema check | |
| `crypto` (Node) | sha256/md5/random | 標準 `crypto/*` | |
| `vercel.json` (server config) | 静的 host | 削除 (静的アセットは Go バイナリに) | |

## 3. 移植の難所と対応策

### 3.1 EJS テンプレート

- 大量の `<% if ... %>`/`<% for ... %>`/動的 include を含む。
- 対応策: **partial を Go 関数として再実装**。共通部分は `image_template.go` に。EJS の `include()` は呼び出し先 partial を直接呼ぶ Go 関数で代替。
- リスク: HTML 構造差分による既存 CSS のレイアウト崩れ。golden test (XML 比較) で防ぐ。

### 3.2 chromedp の安定性

- chromedp で `getBoundingClientRect()` を計測する流れは puppeteer と同等に動く。
- リスク: Chrome の起動コストが高い (1.5〜3 秒)。
- 対応: 単一ブラウザを長寿命で保持。タブを並列に開いて Compute を平行実行。
- リスク: メモリリーク。
- 対応: N 回のリクエストごとに `BrowserCtx.Cancel()` → 再生成 (`SettingsBrowserRecycle=200` のような閾値)。

### 3.3 SVGO 互換

- SVG 最適化は Node 版でも experimental (`--optimize` flag) なので、初版 Go 移植では skip して問題ない。
- 後継版で `tdewolff/parse/v2/css` をベースに XML→AST→最適化を組む。

### 3.4 mocked API の網羅性

- 既存 Node テストは `tests/mocks/index.mjs` で API を `mock` する仕組み。
- Go 版では plugin 数だけ mock を書く必要があり、最低限 base+主要 10 plugin に絞って網羅率 80% を目標。

### 3.5 settings.json のコメントキー

- `"//": "..."` の二重キーは JSON 標準では invalid。
- Node 版は `JSON.parse` で後勝ち動作。
- 対応: 専用パーサで `//` で始まる key を捨てる。`tidwall/gjson` で枝刈り。

### 3.6 ユーザー任意の CSS/JS injection

- `extras.css`, `extras.js` で任意 CSS/JS を SVG に流し込める。
- chromedp 実行時のサンドボックス強化が必要。
- 対応: extras features フラグ (`metrics.run.puppeteer.user.css/js`) を尊重し、設定がなければ無視する。

### 3.7 dprint / oxfmt 等のフォーマット差

- 既存リポジトリは `dprint` + `eslint`。
- Go 版で `gofmt` / `gofumpt` を強制する。Markdown は `oxfmt` (リポジトリ規約) のまま。

### 3.8 GitHub Action `composite` runs

- 既存 `action.yml` は bash + docker run の composite。
- Go バイナリだけのコンテナに変更しても composite 構造は維持できる。
- ただし、新規ユーザーが「Go バイナリ単体」を直接 `runs.using: docker` + `image: ghcr.io/...` で参照する形にすると CI の起動が高速化する。次バージョンで切り替えるか検討。

### 3.9 メモリ消費

- 巨大 SVG (4MB 超) を扱う場面で `[]byte` を多用するとアロケーションが増える。
- 対応: `io.Reader`/`io.Writer` チェーンを徹底し、ストリームでパイプライン処理する。

## 4. 互換性とマイグレーション

### 4.1 入力互換

- すべての `action.yml` inputs キーと既定値は維持。
- `settings.json` キーも維持。
- `q` (URL クエリ) と `INPUT_*` (Action) の双方向。

### 4.2 出力互換

- SVG/Markdown は **DOM 構造単位** で互換 (バイト同一は目指さない)。
- JSON (`config.output=json`) は **キー単位で完全互換**。Insights JSON も同様。

### 4.3 Docker イメージ互換

- 既存 `ghcr.io/lowlighter/metrics:vX.Y` の利用者がそのまま Go 版に乗り換えられるよう、タグの上書き互換でリリース。
- ロールバック手段として `ghcr.io/lowlighter/metrics:legacy-vX.Y` を残す。

### 4.4 マイグレーションガイド (公開)

`docs/migration-to-go.md` をユーザー向けに用意:

- 変更点 (バイナリ実行、Chrome 同梱位置、puppeteer 関連 env 変数の置換)
- 既知の差異 (SVGO 無効、community templates 未対応)
- 既知の挙動変化 (フォントレンダリング、SVG 改行数)

## 5. フェーズ別マイルストーン

| フェーズ | 期間 | 内容 |
|---------|------|------|
| M1 | Week 1-2 | リポジトリ初期化 (`cmd/`, `internal/`), `config` ローダ, metadata loader, GitHub REST/GraphQL ラッパ |
| M2 | Week 3-4 | base plugin, core plugin, classic template (SVG 最小構成), JSON 出力。Node 比較で base.header の互換確認 |
| M3 | Week 5-6 | chromedp 統合 (svg.Resize, svg.Hash), PNG/JPEG 出力 |
| M4 | Week 7-9 | 主要 10 plugin (languages, activity, repositories, lines, gists, isocalendar, calendar, achievements, habits, stars) を移植 |
| M5 | Week 10-11 | Web インスタンス (`metrics-server`), キャッシュ, OAuth, rate-limit |
| M6 | Week 12-13 | GitHub Action (`metrics-action`) と committer 機能 |
| M7 | Week 14-15 | Markdown / markdown-pdf テンプレート, embed() |
| M8 | Week 16-17 | 残りプラグイン (wakatime, pagespeed, stackoverflow, leetcode, sponsors, sponsorships, …) |
| M9 | Week 18-19 | community プラグイン (built-in 化), tests, golden file |
| M10 | Week 20-21 | リリース準備, docs, migration guide, ベンチ最適化 |

## 6. リスクと未解決事項

### 6.1 リスク

| リスク | 緩和策 |
|------|-------|
| chromedp の bug でレンダリングが安定しない | 失敗時に再試行 + ブラウザ再起動。バージョン固定 (`chromedp@v0.10.0`) |
| GitHub GraphQL の breaking changes | `genqlient` で型生成し早期 detect |
| Twemoji / Octicons の上流変更 | バージョン固定。`assets/twemoji/index.json` に sha 記録 |
| CSS PurgeCSS の精度低下 | 主要セレクタは保護リスト (`!purge`) で除外 |
| 大量プラグインの維持コスト | ドキュメントを `metadata.yml` 中心に集約、テスト golden を増やす |
| Chrome (Chromium) の volume サイズ | CLI 専用イメージを別途配布 |

### 6.2 未解決事項 (要追議論)

- **EJS テンプレートの Go 化方針**: 自動変換 (`ejs-to-go` を書く) vs 手動移植。本仕様では手動を採用するが、コミット時 dprint で乖離検出する案あり。
- **community プラグインの動的ロード**: WASM (`wazero`)、`yaegi` (Go interpreter)、または built-in 化のどれを採るか。
- **PDF 出力時のフォント**: ヘッドレス Chrome 内のフォントを保証するため、`fonts-ipafont-gothic` 等を Docker に同梱必須。Go 単体配布では別途指示が必要。
- **Multi-arch chromedp**: arm64 (Apple Silicon) で chromium が依存する `libnss3` 等を要確認。
- **action.yml の自動生成**: Go 側で出力するか、現状の手書きを維持するか。`metrics-action gen action-yml` 案を採用予定。

これらは [00-overview.md](./00-overview.md) のスコープ更新と合わせて、フェーズ M1 開始前に確定する。
