# 01. アーキテクチャ仕様

## 目次

- [1. 全体ブロック図](#1-全体ブロック図)
- [2. Go パッケージ構成](#2-go-パッケージ構成)
- [3. ランタイム依存](#3-ランタイム依存)
- [4. 主要データ型](#4-主要データ型)
- [5. 実行フロー](#5-実行フロー)
- [6. エラーモデル](#6-エラーモデル)
- [7. ロギング・トレース](#7-ロギングトレース)

---

## 1. 全体ブロック図

```
                        ┌────────────────────────────┐
                        │     Entry points           │
                        │  ┌─────────┬──────────┐    │
                        │  │  CLI    │  Web     │    │
                        │  │ (action)│ (server) │    │
                        │  └─────────┴──────────┘    │
                        └──────────────┬─────────────┘
                                       │
                                       ▼
                        ┌────────────────────────────┐
                        │ engine.Compute(q, deps)    │
                        │  - load conf               │
                        │  - run base plugin         │
                        │  - run user plugins        │
                        │  - run template            │
                        │  - render output           │
                        └────────────────────────────┘
                                       │
                  ┌────────────────────┼────────────────────┐
                  ▼                    ▼                    ▼
        ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
        │ github (GraphQL/ │ │ external (axios) │ │ render (svg/pdf) │
        │ REST clients)    │ │ wakatime, etc.   │ │ chromedp, svgo   │
        └──────────────────┘ └──────────────────┘ └──────────────────┘
```

## 2. Go パッケージ構成

```
metrics/
├── cmd/
│   ├── metrics-action/      … main (GitHub Action / CLI)
│   │   └── main.go
│   └── metrics-server/      … main (Web instance)
│       └── main.go
├── internal/
│   ├── action/              … GitHub Action 固有ロジック (input パース, commit, PR)
│   │   ├── action.go
│   │   ├── inputs.go
│   │   └── output.go
│   ├── server/              … Web ハンドラ
│   │   ├── server.go
│   │   ├── routes.go
│   │   ├── middleware.go    … rate-limit, cache, compression
│   │   ├── oauth.go
│   │   └── insights.go
│   ├── engine/              … 中核オーケストレータ
│   │   ├── engine.go        … Compute() ; metrics(login, q, deps)
│   │   ├── pipeline.go      … plugin pending/promise.all 相当
│   │   ├── partials.go      … テンプレート partials 並び
│   │   └── markdown.go      … markdown / embed() 機能
│   ├── config/              … settings.json / metadata.yml ロード
│   │   ├── settings.go
│   │   ├── metadata.go
│   │   ├── presets.go
│   │   └── inputs.go        … core inputs (`config.*`) 取り出し
│   ├── githubapi/
│   │   ├── client.go        … REST + GraphQL クライアント
│   │   ├── graphql.go
│   │   ├── rate.go          … requests 状態管理
│   │   └── auth.go
│   ├── plugins/             … プラグイン実装
│   │   ├── plugin.go        … インタフェース定義 + Registry
│   │   ├── base/
│   │   ├── core/
│   │   ├── languages/
│   │   ├── activity/
│   │   ├── achievements/
│   │   ├── …40+ plugins
│   │   └── community/       … 動的 (将来拡張)
│   ├── templates/           … テンプレート実装
│   │   ├── template.go      … Template interface + Registry
│   │   ├── classic/
│   │   ├── repository/
│   │   ├── terminal/
│   │   └── markdown/
│   ├── render/              … 出力変換
│   │   ├── svg.go           … SVGO 相当の最適化
│   │   ├── pdf.go           … HTML → PDF
│   │   ├── png.go           … chromedp スクリーンショット
│   │   ├── markdown.go
│   │   ├── chrome.go        … chromedp ラッパ (puppeteer 相当)
│   │   ├── twemoji.go
│   │   ├── gemoji.go
│   │   └── octicon.go
│   ├── ejs/                 … `<%= %>` の最小実装 (Go template 互換 wrapper)
│   ├── graph/               … D3 相当のグラフ生成 (line / pie / time)
│   ├── format/              … 数値・日付フォーマッタ
│   ├── git/                 … simple-git 相当 (go-git)
│   ├── linguist/            … go-enry ラッパ
│   ├── cache/               … memory cache
│   ├── logger/              … slog wrapper
│   └── errors/              … エラー定義
├── assets/                  … 埋め込みアセット (//go:embed)
│   ├── plugins/             … queries/*.graphql, metadata.yml
│   ├── templates/           … image.svg, style.css, partials/*.ejs
│   └── statics/             … web statics (CSS, JS, HTML, fonts)
├── api/                     … 公開スキーマ
│   ├── action.schema.json
│   ├── settings.schema.json
│   └── insights.schema.json
├── deploy/
│   ├── Dockerfile
│   └── docker-compose.yml
├── docs/                    … (ユーザ向け) README, CONTRIBUTING
├── specs/                   … (本仕様書)
├── go.mod
└── go.sum
```

### 命名規則

- パッケージ名は短い小文字単語 (`engine`, `plugins/languages`)。
- 公開 API は `internal/` には置かず、`pkg/` を必要に応じて追加する。初版では `internal/` のみで閉じる。
- ファイル名はスネークケース不要、Go 標準どおりの `lowercase.go`。

## 3. ランタイム依存

| カテゴリ | パッケージ | 用途 |
|--------|----------|------|
| Web | `github.com/go-chi/chi/v5` | ルータ |
| Compress | 標準 `compress/gzip` + chi middleware | gzip 圧縮 |
| Rate limit | `github.com/go-chi/httprate` | rate limiter |
| Cache | `github.com/patrickmn/go-cache` | in-memory TTL cache |
| GitHub REST | `github.com/google/go-github/v66` | REST |
| GitHub GraphQL | `github.com/shurcooL/githubv4` | GraphQL |
| HTTP client | 標準 `net/http` + `github.com/hashicorp/go-retryablehttp` | retry, axios 相当 |
| YAML | `gopkg.in/yaml.v3` | metadata.yml / preset |
| JSON5 / merge | `github.com/tidwall/gjson` (optional) | partial query parsing |
| Markdown | `github.com/yuin/goldmark` + `bluemonday` | marked + sanitize-html |
| HTML / DOM | `golang.org/x/net/html` + `github.com/PuerkitoBio/goquery` | jsdom 相当 |
| 構文ハイライト | `github.com/alecthomas/chroma/v2` | prismjs 相当 |
| SVG 最適化 | 自作 (svgo 互換サブセット) | svgo |
| ブラウザ自動化 | `github.com/chromedp/chromedp` | puppeteer 相当 |
| 画像処理 | `github.com/disintegration/imaging` + `golang.org/x/image` | sharp 相当 |
| GIF 生成 | 標準 `image/gif` | gifencoder 相当 |
| 言語解析 | `github.com/go-enry/go-enry/v2` | linguist-js |
| Git | `github.com/go-git/go-git/v5` | simple-git |
| RSS | `github.com/mmcdole/gofeed` | rss-parser |
| OpenGraph | `github.com/dyatlov/go-opengraph` | open-graph-scraper |
| Twemoji | 自作 (image fetch + base64) | twemoji parser |
| Octicons | 静的 SVG 埋め込み | primer/octicons |
| 暗号通貨 | 各種 HTTP API (CoinGecko 等) | crypto plugin |
| 株価 | Alpha Vantage 等 | stock plugin |
| Logging | 標準 `log/slog` | debug log |
| エラー | 標準 `errors` + `fmt.Errorf` | - |
| テスト | 標準 `testing` + `github.com/stretchr/testify` | jest 相当 |
| Fake / モック | `github.com/jaswdr/faker` | faker-js |

詳細は [11-go-migration.md](./11-go-migration.md) を参照。

## 4. 主要データ型

```go
// internal/engine/engine.go

// Query は HTTP クエリ / Action 入力をパースした正規化キーバリュー。
// 例: q["plugin_languages"] = "yes" / q["config.timezone"] = "Asia/Tokyo"
type Query map[string]any

// ComputeRequest は Compute の入力。
type ComputeRequest struct {
    Login    string           // GitHub user / org / repo owner
    Query    Query
    Convert  Format           // "svg" | "png" | "jpeg" | "json" | "markdown" | "markdown-pdf" | "insights"
    Die      bool
    Verify   bool
    Warnings []Warning
}

// Data はテンプレートに渡される最終データ構造。
type Data struct {
    Q          Query
    Animated   bool
    Large      bool
    Base       BaseData
    Config     ConfigData
    Errors     []PluginError
    Warnings   []Warning
    Plugins    map[string]any   // プラグイン別 result
    Computed   map[string]any
    Extras     Extras           // user css/js
    Postscripts []string
    Partials   []string         // 適用 partial 名 (順序)
    Account    AccountKind      // "user" | "organization" | "repository" | "bypass"
}

// PluginContext はプラグインに注入される依存。
type PluginContext struct {
    Login     string
    Q         Query
    Data      *Data
    REST      *githubapi.REST
    GraphQL   *githubapi.GraphQL
    Queries   QueryRegistry
    Imports   *Imports
    Callbacks Callbacks
    Conf      *config.Config
}

// Plugin は各プラグインが実装するインタフェース。
type Plugin interface {
    Name() string
    Run(ctx context.Context, pctx *PluginContext) (any, error)
}

// Template は各テンプレートが実装するインタフェース。
type Template interface {
    Name() string
    Run(ctx context.Context, pctx *PluginContext) error
}
```

`Imports` は Node 版 `utils.mjs` の差し込み object に相当し、`axios`(HTTP)、`puppeteer`(chromedp)、`svg`(svg utility)、`format` 関数群、`metadata`、`s()` などを集約する。

## 5. 実行フロー

### 5.1 Compute シーケンス

```
engine.Compute(ctx, req, deps) →
   1. テンプレート存在検証 (Templates[req.Query["template"]])
   2. Convert 形式の決定 (req.Convert / metadata.formats[0])
   3. partial 順の決定 (config.order と template の partials を merge)
   4. Imports を組み立てる (formatters, puppeteer, svg, plugins map, …)
   5. base plugin 実行 (Plugins["base"].Run)
      - GraphQL: queries.base.user(login)
      - 拡張クエリ (calendar / contributions / repositories)
      - data.user, data.calendar, data.contributions, … を埋める
   6. template.Run() 実行
      - 中で plugins.core を呼び、pending に各 plugin Run を投入
      - 並列実行
   7. 全 promise を await し、エラーをまとめる
   8. Convert に応じた最終レンダリング
      - json   → JSON エンコード
      - markdown / markdown-pdf → ejs.Render → svg.pdf() (PDF 時)
      - svg / png / jpeg → ejs.Render → svg.optimize → svg.resize
   9. {Rendered, MIME, Errors} を返す
```

### 5.2 Insights

通常の Compute(`convert="json"`) を呼んだ後、Web インスタンスを起動して JSON を `localStorage.metrics` にロード → chromedp で HTML を取得する。

## 6. エラーモデル

| カテゴリ | Go 表現 |
|--------|--------|
| プラグインエラー | `engine.PluginError{ Plugin string; Err error; Recoverable bool }` |
| 入力エラー | `engine.InputError` (400) |
| 認可エラー | `engine.ForbiddenError` (403) |
| ユーザー未検出 | `engine.NotFoundError` (404) |
| 出力フォーマット非対応 | `engine.UnsupportedFormatError` (406) |
| GitHub API エラー | `githubapi.APIError{ Status, Type, Errors }` |
| 一時失敗 | `engine.RetryableError` (リトライポリシで包む) |

- `die=true` のときは最初の致命エラーで panic 相当(`return error`)。
- `die=false` のときは `data.errors` に積み、最終出力に footer として表示する。

## 7. ロギング・トレース

- `slog` で構造化ログ出力。
- 主要ログメッセージは Node 版の `metrics/compute/<login> > …` を踏襲し、`logger=engine, login=<login>, stage=...` で出す。
- デバッグ時は `--puppeteer-disable-headless`, `--puppeteer-debug`, `--puppeteer-wait-<event>` 等を chromedp option に変換する。
- `settings.debug=true` で `slog` レベルが `debug` になる。
