# 03. Web サーバ仕様

## 目次

- [1. 起動とライフサイクル](#1-起動とライフサイクル)
- [2. ルーティング一覧](#2-ルーティング一覧)
- [3. ミドルウェア](#3-ミドルウェア)
- [4. embed モード](#4-embed-モード)
- [5. insights モード](#5-insights-モード)
- [6. OAuth](#6-oauth)
- [7. コントロールエンドポイント](#7-コントロールエンドポイント)
- [8. キャッシュとペンディング管理](#8-キャッシュとペンディング管理)
- [9. エラーレスポンス規約](#9-エラーレスポンス規約)
- [10. 静的アセットの配信](#10-静的アセットの配信)

---

## 1. 起動とライフサイクル

### 1.1 エントリポイント

`cmd/metrics-server/main.go` で `internal/server.Server` を起動する。

```go
func main() {
    s, err := server.New(server.Options{
        Sandbox: os.Getenv("SANDBOX") != "",
    })
    if err != nil { log.Fatal(err) }
    if err := s.ListenAndServe(); err != nil { log.Fatal(err) }
}
```

### 1.2 構築フロー

1. `config.Setup({Sandbox})` で `settings.json`、`metadata.yml`、plugin/template の登録情報を作る。
2. `Sandbox=true` のとき: `optimize=true`, `cached=0`, `plugins.default=true`, `extras.default=true`, `sandbox=true`、`mocked=true` を強制。
3. GitHub API クライアントを作る (`token`, `settings.api.rest`, `settings.api.graphql`)。
4. 各 plugin について
   - `plugins.default=true` のとき `settings.plugins.<name>.enabled` を未設定なら `true` にする。
   - `mocked=true` のとき plugin の `type:token` 入力を `"MOCKED_TOKEN"` で埋める。
5. レート情報を `/rate_limit` から取得し、15 分ごとと「リクエスト消費後」に refresh。
6. ルーティング登録、`http.Server.ListenAndServe(:port)` で待機。

### 1.3 起動ログ

次の表形式バナーを出す (Node 版互換):

```
───────────────────────────────────
── Server configuration ──────────
Listening on port         │ 3000
Modes                     │ embed, insights
...
Server ready !
```

Go 版では `slog` Default で `level=info` の単発メッセージとして出力する (Insights webserver 起動完了の判別用に `Server ready !` の文字列は必ず emit する)。

### 1.4 Graceful shutdown

- `SIGINT/SIGTERM` を受けたら `http.Server.Shutdown(ctx)` を呼ぶ。
- 進行中の `Compute` を取りこぼさないよう `pending sync.Map` の完了を待つ (最大 30 秒)。

## 2. ルーティング一覧

### 2.1 静的・メタ

| Method | Path | Description |
|--------|------|------|
| GET | `/` | `statics/index.html` |
| GET | `/index.html` | 同上 |
| GET | `/favicon.ico` | `statics/favicon.png` |
| GET | `/.favicon.png` | 同上 |
| GET | `/.opengraph.png` | `settings.web.opengraph` があれば 302、なければ `statics/opengraph.png` |
| GET | `/.version` | `package.version` text/plain |
| GET | `/.hosted` | `settings.hosted` JSON もしくは `null` |
| GET | `/.requests` | GitHub API rate 状態 JSON (REST/GraphQL/Search)。`X-Metrics-Session` header があれば user の rate を返す |
| GET | `/.modes` | `settings.modes` 配列 |
| GET | `/.extras` | `settings.extras` 表示。logged 拡張は session 認証時のみ |
| GET | `/.extras.logged` | `settings.extras.logged` |
| GET | `/.plugins` | 有効プラグイン一覧 `[{name, category, enabled, deprecated}]` |
| GET | `/.plugins.base` | `settings.plugins.base.parts` |
| GET | `/.plugins.metadata` | metadata Map (`name, icon, category, web, supports, scopes, deprecated`) |
| GET | `/.templates` | テンプレート一覧 `[{name, enabled}]` |
| GET | `/.templates/{template}` | テンプレート詳細 (`image/style/fonts/partials/views`) |
| GET | `/.templates/{template}/partials/*` | partial 静的配信 |

### 2.2 CSS / JS

`statics/*.css` および `node_modules` 経由で提供されていた以下のパスを Go 版では `//go:embed assets/web` でバンドルし配信する:

| Path | Source |
|------|--------|
| `/.css/style.css` | `statics/style.css` |
| `/.css/style.vars.css` | `statics/style.vars.css` |
| `/.css/style.prism.css` | prism-tomorrow.css (chroma で代替) |
| `/.js/app.js` | `statics/app.js` |
| `/.js/ejs.min.js` | (廃止: テンプレートはサーバ側でのみ) |
| `/.js/faker/*` | faker.js (フロント placeholder 用) |
| `/.js/axios.min.js` | フロント用 |
| `/.js/vue.min.js` | フロント用 |
| `/.js/vue.prism.min.js` | フロント用 |
| `/.js/prism.min.js` / `prism.yaml.min.js` / `prism.markdown.min.js` | フロント用 |
| `/.js/clipboard.min.js` | フロント用 |

> フロント実装は当面 Vue + Prism + Clipboard ベースを踏襲する。Go バイナリには `embed.FS` でそのまま埋め込む。
> 後継として Web UI を再実装する場合は別仕様で議論する。

### 2.3 キャッシュ操作

| Method | Path | Description |
|--------|------|------|
| GET | `/.uncache?user=<login>` | uncache token を発行 → `{token: "xxxx"}` |
| GET | `/.uncache?token=<token>` | 発行済 token に対応する user の cache を破棄 |

## 3. ミドルウェア

実装順:

1. **Compression** — `gzip` (`go-chi/chi` の `middleware.Compress(5)` 相当)。
2. **Rate Limit** — `settings.ratelimiter` がある時のみ。`max=0` で実質無効。`skip` 条件: cache hit ユーザーには適用しない。
3. **Cache-Control header** — `?cache=<ms>` クエリ or `settings.cached` の値で `public, max-age=<seconds>` を付与。`0` の時は `no-store, no-cache`。
4. **Trust Proxy** — `app.set('trust proxy', 1)` 相当。Go では `Request.RemoteAddr` の代わりに `X-Forwarded-For` を採用するため、`go-chi/middleware.RealIP` を入れる。

### 3.1 グローバル limiter

Node の `limiter = ratelimit({max: 60, windowMs: 60000})` を全静的・メタエンドポイントに適用。Go では `httprate.LimitByIP(60, time.Minute)` を `r.Group` 単位で適用。

### 3.2 Session header

- `X-Metrics-Session` ヘッダにユーザー毎の OAuth セッショントークンを乗せる。
- 該当セッションが `authenticated map` にあれば、その user の token で REST/GraphQL を作り直して embed/insights を実行する。

## 4. embed モード

### 4.1 ルート

| Method | Path | 内容 |
|--------|------|------|
| GET | `/embed/` | static `embed/index.html` |
| GET | `/embed/index.html` | 同上 |
| GET | `/.placeholders/*` | static `embed/placeholders` 配下 |
| GET | `/.js/embed/app.js` | static |
| GET | `/.js/embed/app.placeholder.js` | static |
| GET | `/{login}` | 個人/組織 metrics 生成 |
| GET | `/{login}/{repository}` | リポジトリ用 metrics 生成 (template=repository) |

### 4.2 `GET /:login/:repository?`

#### 入力検証

- `login` が `.` で始まる、または `/` を含むなら `next()` (実質ヒットしない静的パス回避)。
- `login` が `^[-\w]+$` でなければ 400 `Bad request: username seems invalid`。
- `settings.restricted` 配列があり `login` を含まなければ 403。

#### キャッシュ

- `cache.Get(login)` にヒットすれば `mime` を `Content-Type` に立てて返す。
- 同 login のリクエストが進行中なら待機(`pending sync.Map`)。

#### 最大同時ユーザー

- `settings.maxusers > 0` のとき、`cache.ItemCount() + 1 > maxusers` で 503。

#### リポジトリ alias

- `repository` パスがあれば `q.template = "repository"`, `q.repo = repository` を強制。

#### Render 呼び出し

- `extras.logged` を OAuth セッションに足す(設定が `features !== true` の時)。
- `q["config.presets"]` を `config.LoadPresets(q["config.presets"])` で取り込み、`q` にマージ。
- 出力フォーマットは `settings.outputs` のうち `q["config.output"]` を採用 (未指定なら `outputs[0]`)。
- `engine.Compute(ctx, ComputeRequest{Login, Q, Convert: format, Die, Verify})` を呼ぶ。
- 完了後、`cache.Put(login, {rendered, mime}, ttl)`、`Content-Type: mime` で返す。

#### エラーマッピング

§9 を参照。

### 4.3 クエリ仕様

- `q` には URL クエリ全てが流し込まれる。例: `?plugin_languages=true&plugin_languages.limit=8`
- `metadata.plugins.<name>.inputs.web` が定義された値のみ web 経由で受け入れる。`global: true` の入力はサーバ側 settings で上書きされる。
- web の入力解釈は `config.MetadataPlugin.InputsForWeb(query)` で実装。

## 5. insights モード

### 5.1 ルート

| Method | Path | 内容 |
|--------|------|------|
| GET | `/about/*` | 旧パス。`/insights/*` に 302 redirect |
| GET | `/insights/` | static `insights/index.html` |
| GET | `/insights/index.html` | 同上 |
| GET | `/insights/.statics/*` | static `statics/insights/` |
| GET | `/insights/:login` | static `insights/index.html` (SPA エントリ) |
| GET | `/insights/query/:login/` | 計算リクエスト (202 Accepted) |
| GET | `/insights/query/:login/:plugin/` | プラグイン単位の結果取得 |

### 5.2 `GET /insights/query/:login/`

- login 検証(`^[-\w]+$`)→ 400。
- すでに `pending["insights." + login]` がある(non-debug, non-mock)なら待機。
- `cache["insights." + login]` があれば返す。
- なければ非同期で `engine.ComputeInsights(login)` を起動し、202 で `{"processing": true, "plugins": [...プラグイン名]}` を返す。
- 完了時 plugin ごとに `cache.Put("insights." + login + "." + plugin, result)`、最終結果は `cache.Put("insights." + login, json, ttl)`。

### 5.3 `GET /insights/query/:login/:plugin/`

- plugin 名検証(`^\w+$`)。
- `cache["insights." + login + "." + plugin]` を返す (なければ 204 No Content)。

### 5.4 `engine.ComputeInsights`

固定の plugin/q セット (Node 版 `metrics.insights.q` / `metrics.insights.plugins`) を用いて `convert="json"` で実行。本仕様は [04-rendering.md](./04-rendering.md) §6 参照。

## 6. OAuth

`settings.oauth = {id, secret, url}` が設定されているとき有効化される。

| Method | Path | 内容 |
|--------|------|------|
| GET | `/.oauth/` | `statics/oauth/index.html` |
| GET | `/.oauth/index.html` | 同上 |
| GET | `/.oauth/script.js` | 同 dir 配下 |
| GET | `/.oauth/authenticate?scopes=...` | state を生成し GitHub OAuth へリダイレクト |
| GET | `/.oauth/authorize?code=&state=` | code を `https://github.com/login/oauth/access_token` と交換し session を発行 |
| GET | `/.oauth/redirect` | `statics/oauth/redirect.html` |
| GET | `/.oauth/revoke/:session` | GitHub に grant 削除 (`DELETE /applications/:id/grant`) |
| GET | `/.oauth/enabled` | `true` (OAuth 無効時は `false`) |

### 6.1 セッション管理

- `authenticated map[string]Session` (`{login, token}` を保持)。
- session 名は `crypto/rand` 64 バイトを hex (128 chars)。
- session を発行したら `?session=<hex>` を `redirect` URL の query に乗せる。
- `state` は `crypto/rand` 32 バイト hex (64 chars)。

### 6.2 GitHub OAuth flow

- 認可: `GET https://github.com/login/oauth/authorize?client_id&state&redirect_uri&scope&allow_signup=false`
- token 取得: `POST https://github.com/login/oauth/access_token` (form-urlencoded body)。Accept は `application/json` ではなく default (Node 版互換)。
- ユーザー検証: `GET https://api.github.com/user` (Authorization: `token <PAT>`)。

### 6.3 セキュリティ

- state は使い捨て (`states.delete(state)`)。
- session は HTTP-only Cookie ではなく `X-Metrics-Session` header 推奨。ブラウザは `localStorage` に保存する想定。
- 不正 session で `/.requests` を呼ぶと 401 が返り、session 削除。

## 7. コントロールエンドポイント

`settings.control.token` が設定されているときのみ:

| Method | Path | 内容 |
|--------|------|------|
| POST | `/.control/stop` | `Authorization: <token>` 一致で 202、5 秒後に `os.Exit(1)` |

Go 版では `os.Exit(1)` ではなく `server.Shutdown(ctx)` → `os.Exit(0)` とし、上位の Docker `restart` ポリシで再起動を期待する。

## 8. キャッシュとペンディング管理

| キー | 内容 | TTL |
|-----|------|-----|
| `<login>` | embed の `{Rendered, Mime}` | `?cache=<ms>` または `settings.cached` |
| `insights.<login>` | insights の最終 JSON | 同上 |
| `insights.<login>.<plugin>` | plugin 個別結果 | デフォルト無期限(全体 JSON 完了で上書き) |
| `actions.flush[<token>]` | `/.uncache` で発行された flush token | 一過性、消費で削除 |

実装: `internal/cache.Cache` を `patrickmn/go-cache` でラップ。`cache.Size()` を `ItemCount()` で代替。

### 8.1 同時実行制御

- `pending` は `sync.Map[string]chan struct{}` 相当。
- 同一 login へのリクエストが重複したら最初のリクエストの結果を待つ。
- mock / debug モードでは抑止せず常に実行 (テスト容易性のため)。

## 9. エラーレスポンス規約

| Code | Body | 条件 |
|------|------|------|
| 200 | (Content) | 成功 |
| 202 | JSON | insights 受付 |
| 204 | (empty) | plugin 結果未取得 |
| 400 | `Bad request: username seems invalid` | login validation 失敗 |
| 400 | `Bad request: plugin name seems invalid` | plugin validation 失敗 |
| 400 | `Bad request: unsupported template` | template 不一致 |
| 400 | `Bad request: invalid state` | OAuth state 不一致 |
| 400 | `Bad request: invalid session` | revoke 失敗 |
| 401 | `Unauthorized: invalid token` | `.control/*` |
| 401 | `Unauthorized: oauth failed` | OAuth 失敗 |
| 403 | `Forbidden: username not in allowed list` | restricted 不一致 |
| 404 | `Not found: unknown user or organization` | GitHub `NOT_FOUND` |
| 405 | `Method not allowed: this endpoint is not available` | mode 無効時の `/embed/*` `/insights/*` |
| 406 | `Not Acceptable: unsupported output format or account type for specified parameters` | template の `formats` / `supports` 不一致 |
| 429 | (rate-limit middleware) | `Too many requests: retry later` |
| 500 | `Internal Server Error: failed to execute request <req> (this may be the result of a timeout, or it could be a GitHub bug)` | GitHub timeout |
| 500 | `Internal Server Error: failed to process metrics correctly` | 一般エラー |
| 503 | `Service Unavailable: maximum number of users reached, only cached metrics are available` | maxusers 超 |

## 10. 静的アセットの配信

- `assets/web/statics/` 以下を `//go:embed` で取り込み、`http.FS(embed.FS).Open(...)` 経由で配信。
- ビルド時に `statics/style.css` 等を base CSS に組み込む。`prism-tomorrow.css` は `chroma` の `tomorrow-night` テーマ生成 CSS を Embedded 化する (互換性が必要なら原本を同梱)。
- `node_modules` 配下のファイル(`faker`, `axios`, `vue`, `prismjs`, `vue-prism-component`, `clipboard`) は手動で `assets/web/vendor/` に格納し、バージョン番号を `go.mod` ではなく `vendor.txt` で管理する。
