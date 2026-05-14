# 07. テンプレート仕様

## 目次

- [1. 一覧](#1-一覧)
- [2. テンプレートインタフェース](#2-テンプレートインタフェース)
- [3. 共通レイアウト](#3-共通レイアウト)
- [4. partial 機構](#4-partial-機構)
- [5. classic テンプレート](#5-classic-テンプレート)
- [6. repository テンプレート](#6-repository-テンプレート)
- [7. terminal テンプレート](#7-terminal-テンプレート)
- [8. markdown / markdown-pdf テンプレート](#8-markdown--markdown-pdf-テンプレート)
- [9. community テンプレート](#9-community-テンプレート)
- [10. テンプレート互換性チェック](#10-テンプレート互換性チェック)

---

## 1. 一覧

| 名前 | 用途 | supports | formats |
|------|------|----------|---------|
| `classic` | GitHub 風の万能テンプレート (既定) | user / organization | svg / png / jpeg / json |
| `repository` | リポジトリ単位の表示 | repository | svg / png / jpeg / json |
| `terminal` | SSH セッション風 | user / organization | svg / png / jpeg / json |
| `markdown` | EJS Markdown ファイル → 画像埋込 | user / organization / repository | markdown / markdown-pdf / json |
| `community` | コミュニティ提供テンプレート | (任意) | (動的) |

## 2. テンプレートインタフェース

```go
// internal/templates/template.go
package templates

type Metadata struct {
    Name        string
    Description string
    Examples    map[string]string
    Index       int
    Supports    []string  // user / organization / repository
    Formats     []string  // svg / png / jpeg / json / markdown / markdown-pdf
    Extends     string    // (optional) community で他テンプレートを継承
}

type Template interface {
    Name() string
    Metadata() *Metadata
    // Image SVG の元ソース。`//go:embed` で `image.svg` をバンドル。
    Image() []byte
    // Style CSS 元ソース。`//go:embed style.css`。
    Style() []byte
    // Fonts CSS 元ソース。`//go:embed fonts.css`。空でも可。
    Fonts() []byte
    // Partials は partials/_.json で定義された並び (デフォルト)。
    Partials() []string
    // PartialBody は partials/<name>.ejs に相当する Go 関数を返す。
    PartialBody(name string) PartialFunc
    // Run は core プラグイン呼出 + 派生データ計算 (markdown テンプレートの alias 設定など)。
    Run(ctx context.Context, p *plugins.PluginContext) error
    // Check は accounts/formats の互換チェック。
    Check(q config.Query, account string, format string) error
}

type PartialFunc func(w io.Writer, ctx PartialContext) error
```

`PartialContext` は `data` フル + helper を保持する。

## 3. 共通レイアウト

`classic` の `image.svg` を共通テンプレートのリファレンスとする (terminal/repository はほぼ同形)。
**完全な image.svg スケルトン** (warnings ブロック・footer 含む) は [13-appendix.md §D](./13-appendix.md#d-classic-imagesvg-のスケルトン) を参照。

要約:

```html
<svg xmlns="http://www.w3.org/2000/svg"
     width="{{ large ? 960 : columns ? '100%' : 480 }}"
     height="99999"
     class="{{ large ? 'large' : columns ? 'columns' : '' }}
            {{ !animated ? 'no-animations' : '' }}">
  <defs><style>{{ fonts }}</style></defs>
  <style data-optimizable="true">{{ style }}</style>
  <style>{{ extras.css }}</style>

  <foreignObject x="0" y="0" width="100%" height="100%">
    <div xmlns="http://www.w3.org/1999/xhtml" class="items-wrapper">
      {{ warnings ブロック }}
      {{ for partial in partials: include partial }}
      {{ base.metadata なら footer }}
    </div>
    <div id="metrics-end"></div>
  </foreignObject>
</svg>
```

### 3.1 重要なクラス

- `large` — `data.Large=true` で `width=960` レイアウト
- `columns` — `data.Columns=true` で `width=100%` + multi-column layout
- `no-animations` — `data.Animated=false` で CSS アニメーションを停止
- `items-wrapper` — partial の親
- `#metrics-end` — chromedp が `getBoundingClientRect()` で高さを測る基準要素

### 3.2 共通の data 名前空間

EJS 内で参照されるトップレベル名:

| 名前 | 中身 |
|------|------|
| `q` | Query (URL/Action 入力) |
| `account` | "user" / "organization" / "repository" |
| `animated` / `large` / `columns` | core が決めるレイアウト |
| `base[part]` | base section が有効か |
| `config` | core から派生の設定 (timezone 等) |
| `computed` | base / core で計算された派生値 |
| `errors` / `warnings` | エラー/警告 |
| `meta` | `version`, `generated`, `link` |
| `plugins.<name>` | 各プラグインの結果 |
| `user` | base の結果 |
| `style` / `fonts` / `extras.css` | テンプレート CSS |
| `partials` | このリクエストで有効な partial 名の Set |
| `s(n, "s")` | 単複サフィックス |
| `f` (format) | フォーマッタ群 |

## 4. partial 機構

### 4.1 既定順序

`assets/templates/<name>/partials/_.json` に文字列配列で並べる。`classic` のデフォルト並び (46 partial) は [13-appendix.md §C](./13-appendix.md#c-classic-テンプレート-partials-並び順-_json) を参照。

### 4.2 partial 抑止 / 追加

ユーザーが `config_order` (=`config.order`) を指定すると、その配列の **intersection** が実行順の先頭になる。アルゴリズム詳細は [13-appendix.md §I](./13-appendix.md#i-partial-補完規則-configorder--_json-の合成) を参照。

Go では `engine.MergePartials(userOrder, templateOrder)` として実装する。`config.order` が空のときは template 既定をそのまま使う。

### 4.3 partial 実装方針

- partial (EJS) を 1:1 で Go 関数に翻訳する。EJS の制御構文は Go の `text/template` に置換可能。
- 共通ヘッダ部分 (`base.header`, `base.activity+community`, `base.repositories`) は base プラグインの結果に依存するため、Go の partial 関数からは `data.User` 等を読む。
- partial 個別の実装は **テンプレート別** に持つ (`internal/templates/classic/partials/base_header.go` 等)。
- HTML エスケープは partial 側で `template.HTMLEscapeString` を使う。

### 4.4 partial 一覧 (classic)

`_.json` の順 (省略不可。完全な再現には全ファイル移植が必須):

```
base.header, introduction, base.activity+community, base.repositories,
lines, followup, discussions, languages, notable, projects, repositories,
gists, pagespeed, habits, topics, music, nightscout, posts, rss, tweets,
isocalendar, calendar, stars, starlists, stargazers, people, activity,
reactions, anilist, wakatime, skyline, support, stackoverflow, leetcode,
crypto, stock, achievements, screenshot, code, chess, sponsors,
sponsorships, poopmap, 16personalities, fortune, splatoon, steam
```

`base.*` partial は base プラグインの sub-section 出力。それ以外はプラグイン名と一致。

## 5. classic テンプレート

- `template.Run` は `plugins.core` を呼ぶだけ。
- `image.svg` / `style.css` / `fonts.css` は §3 の共通レイアウト。
- partials は §4.4。
- formats: `svg`, `png`, `jpeg`, `json`。

## 6. repository テンプレート

- `metadata.supports = [repository]`。
- partial 構成は repository 専用 (`base.header` の代わりに `base.repository`、`introduction`、`base.community`、`base.activity` のリポジトリ版)。
- `image.svg` は classic とほぼ同等だが、`q.repo` 指定が必須。
- `template.Run` は classic と同じ (`plugins.core`)。
- `Check` で `account !== "repository"` を 406 にする。

## 7. terminal テンプレート

- `metadata.supports = [user, organization]`。
- 見た目を SSH 風にするため、`image.svg` は `<g class="terminal">` を root にとり、`<text>` で行を埋める。
- partial 群は classic とほぼ同じだが、改行・モノスペース表現になる。
- `template.Run` は classic と同じ。

## 8. markdown / markdown-pdf テンプレート

### 8.1 概要

- `metadata.supports = [user, organization, repository]`、`formats = [markdown, markdown-pdf, json]`。
- `template.Run` で **alias 変数** を data に詰める (NAME, LOGIN, REGISTRATION_DATE, COMMITS, FOLLOWERS, FORKS, LANGUAGES, POSTS, …)。

### 8.2 ソース読み込み

- `q.markdown` がローカルパス or URL。URL でなければ `https://raw.githubusercontent.com/{owner}/{repo}/{branch}/{path}` に展開。`q.repo` が省略時は `q.user` リポジトリ。
- `axios.get` 相当で取得。失敗時はテンプレート組込の `image.svg` (Markdown サンプル) を使う。

### 8.3 レンダリング

- 取得した markdown source に対し:
  1. `{{ ... }}` を `<%= ... %>` に置換 (markdown 内で標準 EJS と衝突しないようにするための糖衣構文)。
  2. delimiter `<` `>` で EJS 評価 (`include`/`embed` 含む)。
  3. delimiter `{` `}` で EJS 再評価。
- `embed(name, q)` は別の plugin/template を内部で再帰呼び出し、結果を `<img class="metrics-cacheable" data-name="..." src="data:image/...;base64,...">` として埋め込む。
- markdown-pdf 出力は得られた HTML を §4.5 の chromedp PDF パイプラインへ渡す。

### 8.4 embed の挙動

- 渡された `q.base = true | string | string[]` は base.* セクションを再有効化する。
- `q.config.animations = false` を強制 (markdown-pdf)。
- output format は `q["config.output"]` ∈ `[svg, png, jpeg]` のみ受理。

### 8.5 alias data

`template.Run` で計算する変数 (Node `template.mjs` 同等):

```
NAME, LOGIN, REGISTRATION_DATE, REGISTERED_YEARS, LOCATION, WEBSITE,
REPOSITORIES, REPOSITORIES_DISK_USAGE, PACKAGES, STARRED, WATCHING,
SPONSORING, SPONSORS, REPOSITORIES_CONTRIBUTED_TO,
COMMITS, COMMITS_PUBLIC, COMMITS_PRIVATE, ISSUES,
PULL_REQUESTS, PULL_REQUESTS_REVIEWS, FOLLOWERS, FOLLOWING,
ISSUE_COMMENTS, ORGANIZATIONS,
WATCHERS, STARGAZERS, FORKS, RELEASES, VERSION,
LINES_ADDED, LINES_DELETED,
GISTS, GISTS_STARGAZERS,
LANGUAGES, POSTS, TWEETS, TOPICS
```

Go 実装では `map[string]any` を `data.Aliases` に持たせ、テンプレート評価時に top-level 名前空間にマージする。

## 9. community テンプレート

- `metadata.supports` は community 提供側に従う。
- `extends:` フィールドを持つ場合、元テンプレートの `template.mjs` を継承する。Go 版では `Template.Inherits()` で resolve。
- 安全性のため `template.mjs` (任意 JS) は **既定では実行しない**。`+trust` フラグ付きで個別許可した場合のみ実行候補。
- 初版 Go では community テンプレートを **取り込まない**。インタフェースは残し、後継仕様で対応する。

## 10. テンプレート互換性チェック

`Template.Check(q, account, format)` で次を判定:

1. `format ∈ template.formats` でなければ 406 (`not supported for: <reason>`)。
2. `account ∈ template.supports` でなければ 406。
3. `repository` テンプレートで `q.repo` が無ければ 400。
4. `markdown` 系で `config.output` を `markdown`/`markdown-pdf` 以外に強制すれば 400。

Web 側 (`03-web-server.md §9`) のエラー文言と一致させる。
