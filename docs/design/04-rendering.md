# 04. レンダリングエンジン仕様

## 目次

- [1. パイプライン概要](#1-パイプライン概要)
- [2. EJS 互換テンプレートエンジン](#2-ejs-互換テンプレートエンジン)
- [3. SVG ラスタライズ (resvg)](#3-svg-ラスタライズ-resvg)
- [4. SVG 最適化](#4-svg-最適化)
- [5. PDF 出力](#5-pdf-出力)
- [6. Insights HTML 出力](#6-insights-html-出力)
- [7. JSON / Markdown 出力](#7-json--markdown-出力)
- [8. 絵文字・アイコン置換](#8-絵文字アイコン置換)
- [9. グラフ描画 (D3 相当)](#9-グラフ描画-d3-相当)
- [10. SVG ハッシュ (差分判定)](#10-svg-ハッシュ-差分判定)

---

## 1. パイプライン概要

`engine.Compute` は次の順に出力を生成する:

```
1. base plugin → data.user 等の共通データ収集
2. 各 plugin → data.plugins[name] にプラグイン固有 data
3. template.Run → 中で plugins.core を呼び、partial 順を確定
4. ejs.Render(image.svg, data) → 生 SVG (or Markdown)
5. (任意) twemoji / gemoji / octicon 置換
6. (任意) CSS 最適化 (purgecss + csso)
7. (任意) XML 整形 (xml-formatter)
8. (任意) SVG 最適化 (svgo / --optimize experimental)
9. render.Resize() → 高さは step 3-4 で確定済みなので、任意の `config.padding`
   を適用し、SVG はそのまま / PNG・JPEG は resvg でラスタライズ
10. {Rendered, Mime} を返す
```

Markdown / Markdown-PDF はステップ 4 で `markdown.go` に分岐し、独自フロー。

## 2. EJS 互換テンプレートエンジン

### 2.1 必要機能

EJS (`mde/ejs`) の以下サブセットを再現する:

| 構文 | 役割 |
|------|------|
| `<% code %>` | スクリプトレット (制御構文) |
| `<%= expr %>` | エスケープ付き出力 |
| `<%- expr %>` | 非エスケープ出力 |
| `<%# comment %>` | コメント |
| `<%- include('name', locals) %>` | partial 取り込み |
| `<%_` / `_%>` | 前後の空白除去 |
| `% (line-level)` | 廃止 |

markdown 出力時には `{ }` を delimiter とする 2 パスレンダリングを行う:

```text
1. <% %> でレンダリング
2. {%= %} を <%= %> に置換し、{ } delimiter で再度 EJS 評価
```

### 2.2 実装方針

- Node 版で動的な JS 評価が大量に行われているため、Go の `text/template` だけでは表現力が足りない。
- 対案 1: `github.com/robertkrimen/otto` を埋め込み EJS を解釈。
- 対案 2: 各テンプレートを **Go コードで書き直し**、共通 helper(`Image`, `Style`, `Fonts`, `Range`, `Partials`) を利用する。
- 採用方針: **対案 2**。EJS テンプレートを「partial 単位の Go ファイル」へ手動移植する。各 partial の本体は `internal/templates/<template>/partials/<name>.go` に置く。EJS の動的式は限定的 (`if`, `for`, `format()`, plugin 結果アクセス) なので、Go template + helper でほぼ書き換えられる。
- 互換性のため、Go テンプレートで生成した HTML は EJS 版と同じ DOM 構造を保つ。

### 2.3 共通 helper

`internal/render/helpers.go` に以下を実装:

| ヘルパ | 機能 |
|------|------|
| `format(n, opts)` | 数値フォーマッタ (Node `format`) |
| `format.Bytes(n)` | バイト整形 |
| `format.Percentage(n, opts)` | パーセント整形 |
| `format.Date(d, opts)` | 日付整形(タイムゾーン尊重) |
| `format.Ellipsis(s, n)` | 省略 |
| `S(n, "s")` | 単複サフィックス |
| `Image(asset, opts)` | base64 埋め込み画像 (data URI) |
| `Style(css)` | `<style data-optimizable="true">…</style>` 出力 |
| `Embed(name, q)` | markdown 用の `<img class="metrics-cacheable" ...>` 埋め込み |
| `Range(start, end, step)` | for ループ補助 |
| `Octicon(name, size)` | `:octicon-name-size:` プレースホルダ生成 |

## 3. SVG ラスタライズ (resvg)

最終出力はブラウザに依存しない。upstream は Headless Chromium で高さを実測し
`page.CaptureScreenshot` で PNG/JPEG を得ていたが、本移植では:

- **高さ**は生成時に確定する。各 partial が自分の消費高さを Go 側の
  フォントメトリクス (`internal/render/fontmetrics`, `x/image/font/sfnt` で
  埋め込み TTF を計測) から算出し、`classic.go` / `repository.go` が
  それを積み上げてルート `<svg height>` に直接書き込む
  (`height="99999"` プレースホルダも `foreignObject` も無い)。
- **PNG/JPEG**は確定済み SVG を [resvg](https://github.com/linebender/resvg)
  CLI (`render.Resvg`) でラスタライズする。JPEG は resvg が対応しないため、
  resvg の PNG を Go の `image/jpeg` で再エンコードする。

この構造により chromium への実行時依存が消え、distro のブラウザパッケージ
更新で描画が壊れる障害クラスを排除した (経緯: [#409](https://github.com/mjun0812/github-metrics/issues/409))。

### 3.1 `render.Resize` アルゴリズム (`internal/render/resvg.go`)

高さは既に確定しているので測定は行わない。処理は次のとおり:

1. `config.padding` をパース (`internal/render/padding.go`、後述 §3.3)。
2. `applyPadding`: padding が非自明ならルート `<svg>` の width / height /
   viewBox を `dim*mult + absolute` (ceil) に書き換える純粋な算術。既定
   (空 / `"0"`) では no-op でルートの寸法をそのまま返す。
3. `convert == "svg"`: padding 適用後の SVG をそのまま返す (resvg は呼ばない)。
4. `convert == "png" | "jpeg"`: SVG を resvg サブプロセスの stdin に流し込み、
   PNG を stdout から受け取る。JPEG はその PNG をデコードして `image/jpeg`
   で再エンコードする。

### 3.2 resvg サブプロセス

- 呼び出し: `resvg [フォントフラグ] - -c` (stdin から SVG、stdout へ PNG)。
  生成 SVG は画像を base64 で inline 済みなので resources-dir は不要。
- **フォントフラグは必須**。生成 SVG のフォントスタックは CSS 総称ファミリ
  (`sans-serif` / `monospace`) で終わるので、`--sans-serif-family` 等で
  Liberation ファミリ (`Liberation Sans` / `Serif` / `Mono`、Arial 等と
  メトリクス互換) にマップする。マップしないと resvg はスタックを解決できず
  **全 `<text>` を無言でスキップ**する。
- バイナリの解決順は `ResvgOpts.ExecPath` → 環境変数 `METRICS_RESVG_PATH`
  → `PATH` 上の `resvg`。構築時に stat して見つからなければ即エラー
  (`*InputError`) にし、無言劣化を防ぐ。
- resvg のプレビルド静的バイナリ (~6 MB) を Docker image / CI に同梱する。

### 3.3 padding パース

`config.padding` は入力互換のため upstream と同じ `"<絶対> + <相対>%"` 形式を
サポートする (`internal/render/padding.go`)。ブラウザ計測誤差を吸収するための
既定値だったが、計測が無くなったため既定は実質 no-op である。

### 3.4 デバッグオプション

upstream の puppeteer デバッグフラグ (`--puppeteer-disable-headless` /
`--puppeteer-debug` / `--puppeteer-wait-<event>`) はブラウザ前提のため、
本移植では入力互換のために受理するが動作には影響しない (no-op)。

## 4. SVG 最適化

### 4.1 CSS 最適化 (`svg.optimize.css`)

- `<style data-optimizable="true">…</style>` を抽出。
- PurgeCSS 相当の最適化(content HTML 内に出現しないセレクタを削除)。
- CSSO 相当の最小化。
- Go 実装: `internal/render/css.go` を新規に書き起こす。
  - Tokenizer + simple selector match (`tdewolff/parse/v2/css`)。
  - 出現セレクタの判定は `golang.org/x/net/html` でパース → セレクタを `cascadia` でマッチ。
  - minify は `tdewolff/minify/v2/css`。

### 4.2 XML 整形 (`svg.optimize.xml`)

- `xml-formatter` (Node) の代替: `encoding/xml` をベースに自作整形器。`lineSeparator="\n"`, `collapseContent=true` を実現。

### 4.3 SVG 最適化 (`svg.optimize.svg`)

- SVGO (Node) は experimental flag (`--optimize-svg`) 下でのみ呼ばれる。
- 初版 Go 移植では **対応外**。`raw` モード相当のスキップを既定にする。
- 必要が出たら `github.com/oklog/run` で `svgo` をサブプロセス起動するなどでバイパスを検討。

## 5. PDF 出力

`markdown-pdf` 出力の処理:

1. `marked.parse(rendered)` で Markdown を HTML に変換。
2. chromedp タブを開き、`<main class="markdown-body">…</main>` を `SetContent`。
3. 余白 (`paddings`) を `main { margin: ... }` で設定。
4. `@primer/css/dist/markdown.css` + ユーザー `style` をスタイルタグで追加。
5. chromedp の `page.PrintToPDF` でバイト列を取得。
6. MIME = `application/pdf`。

### 5.1 Markdown パーサ (marked 相当)

`github.com/yuin/goldmark` をベースに以下プラグインを有効化:

- `extension.GFM` (table, autolink, strikethrough, task list)
- `extension.Typographer`
- custom renderer (HTML)
- `bluemonday.UGCPolicy()` でサニタイズ (sanitize-html 相当)

### 5.2 sanitize-html

ユーザー入力(`q.markdown` で指定されたテンプレート source) は GitHub 上の raw ファイルなので、サニタイズしないと CSS / JS injection リスクがある。Node 版でも `sanitize-html` を介している。

Go 版は `bluemonday` で `allow-` を以下に限定:

- 既定の許可タグ + `<svg>` 関連 (`svg`, `path`, `circle`, `g`, `text`, `rect`, …)。
- `style` 属性は許可、`script` 系は不許可。

## 6. Insights HTML 出力

1. まず `engine.ComputeInsights(login)` で JSON データを得る (`convert="json"` の Compute を call)。
   - 固定 q: `template=classic` および以下プラグインを enable
     - `achievements` (threshold=X)
     - `isocalendar` (duration=full-year)
     - `languages` (limit=0)
     - `activity` (limit=100, days=0, timestamps=true)
     - `notable` (repositories=true)
     - `followup` (sections=repositories,user)
     - `introduction`, `topics` (mode=icons, limit=0)
     - `stars` (limit=6)
     - `reactions` (details=percentage)
     - `repositories` (pinned=6)
     - `sponsors`, `calendar` (limit=0)
   - 固定 plugins enable map: 上記同名 + `activity.markdown="extended"`, `languages.extras=false` 等。
2. 自分自身が立ち上げる metrics web インスタンスへ chromedp で `/insights/<login>?embed=1&localstorage=1` にアクセス。
3. `localStorage.setItem("local.metrics", json)` を `Evaluate` で実行 → reload。
4. `.container .user` セレクタが現れるまで待機 (`WaitVisible`)。
5. `document.querySelector("main").outerHTML` を取得し、`<style>` タグ (style.vars.css, style.css, insights/.statics/style.css) を inline 化して `<html>` でラップ。
6. MIME = `text/html`。

### 6.1 ローカル webserver

- Action 環境では `metrics-server` をサブプロセスで起動済み (action フェーズ §3.7)。
- Web 環境では自分自身が webserver。

### 6.2 単独実行

- Insights 出力は Web インスタンスへの自己 HTTP アクセスを伴うため、CLI モードでは `localhost:<port>` をテンポラリ起動する。
- Go 版では `httptest.NewServer` で同一バイナリ内に立ち上げる方が確実。

## 7. JSON / Markdown 出力

### 7.1 JSON

- `data` をシリアライズして返す。
- `Set` は `[]T`、`Map` は `map[string]any`、循環参照は `"[Circular]"` で潰す。
- MIME = `application/json`。
- Go では `json.Marshaler` 実装と `cycleDetector` で同等の挙動を実現する。

### 7.2 Markdown / Markdown-PDF

- `q.markdown` で指定された **テンプレートソース**(URL or repo path) を取得 (`raw.githubusercontent.com`)。
- ソース内の `{{ ... }}` を `<%= ... %>` に変換。
- 2 パスで EJS 評価:
  - Pass1: `<% %>` delimiter
  - Pass2: `{ }` delimiter
- `embed(name, q)` ヘルパで他テンプレート/プラグインの SVG を base64 化して `<img>` として埋め込む。
- 結果 HTML を `markdown-pdf` 時は §5 の PDF パイプラインへ。

### 7.3 markdown キャッシュ(Action)

- 出力中の `<img class="metrics-cacheable" data-name="..." src="data:...">` を Action 側が抽出し、リポジトリ内ファイルにコミットして URL 置換する ([02-action.md §4.3](./02-action.md#43-markdown-キャッシュ))。

## 8. 絵文字・アイコン置換

### 8.1 twemoji

- `@twemoji/parser` 相当: テキストから emoji を抜き出し `https://twemoji.maxcdn.com/v/...` (現公式 CDN は変化) で SVG を取得 → `<svg class="twemoji" ...>` に置換。
- ネットワーク呼び出しを伴うため、結果は `internal/render/twemoji_cache.go` で in-memory にキャッシュ。
- バンドル用に主要 emoji (国旗・天気・カレンダー) は `assets/twemoji/` に sample を埋め込む。

### 8.2 gemoji

- `GET https://api.github.com/emojis` で `name → url` map を取得。
- 文字列中の `:name:` を `<img class="gemoji" src="<base64>" width="16" height="16" />` に置換。
- 画像は `imgb64()` で base64 化(後述 §10)。

### 8.3 octicon

- `:octicon-<name>-<size>:` を `<svg>` に置換。
- `@primer/octicons` JSON データ (16/24px 含む全アイコン) を `assets/octicons/data.json` として埋め込む。
- `:octicon-<name>:` だけの場合は 16px をデフォルトで利用。

## 9. グラフ描画 (D3 相当)

Node 版 `utils.mjs` の `Graph.line`, `Graph.pie`, `Graph.time` を再実装。

| 関数 | 入力 | 出力 |
|------|------|------|
| `graph.Line(data, opts)` | `[{x,y,text?}]` | SVG (path) |
| `graph.Pie(data, opts)` | `map[string]float64` | SVG (path) |
| `graph.Time(data, opts)` | `[{x: Time, y: float}]` | SVG (path, scaleTime) |

実装: `gonum.org/v1/plot` を使うか、シンプルな SVG 文字列生成 helper を自作。アニメーション class はそのまま記述する。

## 10. SVG ハッシュ (差分判定)

- 入力: 完成 SVG 文字列。
- 用途: `output_condition=data-changed` のコミット可否判定。
- アルゴリズム詳細は [13-appendix.md §H](./13-appendix.md#h-svghash-の正規化アルゴリズム) を参照。
- Go 実装: `goquery` で `<footer>` 除去 → `<svg>` ノードを正規化 → MD5 (`crypto/md5`)。chromedp 不要。

## 11. 補助ユーティリティ

| 名称 | 役割 | Go 実装候補 |
|------|------|------------|
| `imgb64(url, opts)` | URL から画像取得 → resize → base64 data URI | `disintegration/imaging` + `image/png` |
| `record(page, frames)` | ヘッドレスブラウザでフレーム連続取得 | chromedp.Screenshot ループ |
| `gif(page, frames)` | アニメ GIF 生成 | `image/gif` |
| `highlight(code, lang)` | シンタックスハイライト | `chroma` (`html.Standalone` formatter) |
| `markdown(text, opts)` | Markdown → サニタイズ済 HTML | `goldmark` + `bluemonday` |
| `git()` | git 基本操作 (`hashObject` 等) | `go-git/go-git/v5` |
| `language(opts)` | 言語判定 (Linguist 互換) | `go-enry/go-enry/v2` |
| `format.Error(err, opts)` | エラー正規化 | `error` から `{Type, Message}` を抽出 |
