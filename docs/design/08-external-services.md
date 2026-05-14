# 08. 外部 API / データソース仕様

## 目次

- [1. GitHub API](#1-github-api)
- [2. GitHub GraphQL クエリの管理](#2-github-graphql-クエリの管理)
- [3. GitHub OAuth](#3-github-oauth)
- [4. 第三者 API](#4-第三者-api)
- [5. Git / Linguist](#5-git--linguist)
- [6. ブラウザスクレイピング](#6-ブラウザスクレイピング)
- [7. 共通 HTTP クライアント方針](#7-共通-http-クライアント方針)
- [8. レート制限と再試行](#8-レート制限と再試行)

---

## 1. GitHub API

### 1.1 REST

- Go ライブラリ: `github.com/google/go-github/v66`。
- カスタム base URL を `settings.api.rest` または `github_api_rest` 入力で受ける (`https://api.github.example.com/v3/` のような GitHub Enterprise 想定)。
- 認証: `Authorization: token <PAT>` (classic) または `Bearer <PAT>` (fine-grained は GraphQL 不可なので reject)。
- 主要利用エンドポイント:
  | エンドポイント | 利用プラグイン |
  |--------------|---------------|
  | `GET /rate_limit` | Web instance / Action 起動時のレート確認 |
  | `HEAD /` | scope ヘッダ確認 |
  | `GET /users/{login}/events` (& `/orgs/{org}`) | activity, code, habits |
  | `GET /repos/{owner}/{repo}/commits/{sha}` | code, habits, languages.recent |
  | `GET /repos/{owner}/{repo}/stats/contributors` | lines, contributors |
  | `GET /repos/{owner}/{repo}/traffic/views` | traffic |
  | `GET /repos/{owner}/{repo}/contents/{path}` | action / markdown キャッシュ |
  | `PUT /repos/{owner}/{repo}/contents/{path}` | committer 上書き |
  | `GET /repos/{owner}/{repo}/releases` (lowlighter/metrics) | notice releases |
  | `POST /repos/{owner}/{repo}/pulls` | pull-request 作成 |
  | `PUT /repos/{owner}/{repo}/pulls/{n}/merge` | pull-request merge |
  | `GET /gists/{id}` / `PATCH /gists/{id}` | gist 出力 |
  | `GET /emojis` | gemoji 置換 |
  | `GET /users/{login}` etc. | 補助 |

### 1.2 GraphQL

- Go ライブラリ: `github.com/shurcooL/githubv4` (型生成ベース) または `github.com/Khan/genqlient` (codegen) を比較検討。
- 推奨: **genqlient**。`assets/plugins/<name>/queries/*.graphql` から **コード生成** することで型安全性を確保。
- カスタム endpoint を `settings.api.graphql` で受ける。
- 認証は REST と同じ token。
- 主要利用クエリは plugin ごとに `queries/*.graphql` として定義する。base プラグインのクエリ全文は [13-appendix.md §A](./13-appendix.md#a-base-プラグインの-graphql-クエリ全文)。

### 1.3 認証バリエーション

| トークン値 | 動作 |
|----------|------|
| `<PAT>` | 通常 |
| `NOT_NEEDED` | base plugin をスキップする (静的 partial のみ生成) |
| `MOCKED_TOKEN` | mocked サブシステム経由でレスポンスを返す (テスト用) |

`conf.settings.notoken = token === "NOT_NEEDED"` の getter を Go では `func (s *Settings) NoToken() bool` で実装する。

## 2. GitHub GraphQL クエリの管理

### 2.1 ロード

- 各 plugin の `queries/*.graphql` を `//go:embed queries/*.graphql` で取り込む。
- 変数置換 (オリジナル実装) は `$key` を文字列リプレースしていた (`queried.replace($login, value)`)。
  - Go 版は **genqlient** 採用なら variables を構造体で型安全に渡せる。`$login` 置換は廃止し、`variables` map に統一。
  - クエリ内の `$login` のような変数は `$login: String!` 形式のクエリ引数に書き換える必要がある。
- 互換性維持のため、**まず文字列置換版**を実装し、徐々に variables 版へ移行する hybrid 方針も可。

### 2.2 主要クエリ (base)

| ファイル | 用途 | 変数 |
|--------|------|------|
| `base/queries/user.graphql` | 基本ユーザー情報 | `$login` |
| `base/queries/user.x.graphql` | 拡張(カウント類, calendar) | `$login`, `$affiliations`, `$calendar.from`, `$calendar.to` |
| `base/queries/organization.graphql` | 組織 | `$login` |
| `base/queries/organization.x.graphql` | 拡張 | `$login`, `$affiliations` |
| `base/queries/calendar.graphql` | カレンダー (`contributionsCollection.contributionCalendar`) | `$login`, `$calendar.from`, `$calendar.to` |
| `base/queries/contributions.graphql` | contributions (一年単位) | `$login`, `$field`, `$range` |
| `base/queries/field.graphql` | 個別フィールド取得 (フォールバック) | `$login`, `$account`, `$field` |
| `base/queries/field.repositories.graphql` | repositories.totalCount / totalDiskUsage | 同上 |
| `base/queries/repositories.graphql` | リポジトリ詳細 (主要なフィールド一括) | `$login`, `$repositories`, `$after`, `$affiliations` |
| `base/queries/repository.graphql` | 単一リポジトリ | `$owner`, `$name` |

### 2.3 ページング

- GraphQL の `pageInfo.hasNextPage`/`endCursor` を尊重する。
- `repositories_batch` (default 100) ずつ取得。GitHub の制限 (100) を超えない。
- `pageInfo.hasNextPage == false` になるまで繰り返し、または `settings.repositories` 到達で停止。

### 2.4 失敗時フォールバック

- bulk クエリ (`base/user.x.graphql` 等) は GitHub timeout (`502`/`this may be the result of a timeout`) が起きやすい。
- 失敗時は **field 単位の単一クエリ**へ切り替える二段構えを維持する。詳細手順は [13-appendix.md §B](./13-appendix.md#b-base-プラグインの取得アルゴリズム-擬似コード)。

## 3. GitHub OAuth

- 用途: Web インスタンスでユーザー別 token をブラウザから取得し、`/.requests`/`embed` を user token で実行する。
- `settings.oauth = {id, secret, url}` で client 設定。
- フロー: [03-web-server.md §6](./03-web-server.md#6-oauth) 参照。
- 取消: `DELETE https://api.github.com/applications/{client_id}/grant` (Basic auth)。

## 4. 第三者 API

### 4.1 一覧

| サービス | エンドポイント | 認証 | 使用プラグイン |
|--------|--------------|------|---------------|
| WakaTime | `https://wakatime.com/api/v1/users/<u>/stats/<range>?api_key=` | API key | wakatime |
| Wakapi | カスタム base URL | API key | wakatime (self-hosted) |
| Google PageSpeed | `https://www.googleapis.com/pagespeedonline/v5/runPagespeed` | API key | pagespeed |
| AniList | `https://graphql.anilist.co` | (なし) | anilist |
| LeetCode | `https://leetcode.com/graphql` | (option Cookie) | leetcode |
| Stack Exchange | `https://api.stackexchange.com/2.3` | API key (推奨) | stackoverflow |
| Spotify | `https://api.spotify.com/v1` | OAuth refresh token | music (spotify) |
| LastFM | `https://ws.audioscrobbler.com/2.0/` | API key | music (lastfm) |
| Apple Music | `https://api.music.apple.com/v1/` | JWT | music (apple) |
| dev.to | `https://dev.to/api/articles` | (公開) | posts |
| Hashnode | `https://api.hashnode.com` | (公開) | posts |
| Twitter API v2 | `https://api.twitter.com/2/` | Bearer token (deprecated) | tweets |
| Twemoji CDN | `https://twemoji.maxcdn.com/v/.../svg/...` | (公開) | render twemojis |
| GitHub Emoji | `GET /emojis` | GitHub PAT | render gemojis |
| Steam Web API | `https://api.steampowered.com/` | API key | steam |
| chess.com | `https://api.chess.com/pub/player/<user>` | (公開) | community/chess |
| CoinGecko | `https://api.coingecko.com/api/v3` | (公開) | community/crypto |
| Alpha Vantage | `https://www.alphavantage.co/query` | API key | community/stock |
| Nightscout | カスタム URL | API secret | community/nightscout |
| Google Maps Geocoding | `https://maps.googleapis.com/maps/api/geocode/json` | API key | stargazers.worldmap |
| Splatoon SR | (固定 URL) | (なし) | community/splatoon |
| Open Graph | 任意 URL のメタ取得 | (公開) | posts (cover image) |
| GitHub Skyline | `https://skyline.github.com/...` | (公開) | skyline |

### 4.2 API キーの取り扱い

- API キー / token は `metadata.yml` で `type: token` 入力として定義し、Web UI からは隠す。
- ログ出力時は `mask(token)` で `xxx****` 化。
- ENV 経由で渡せるよう、`metrics-action --token-env WAKATIME_TOKEN=$WAKATIME_TOKEN` のような注入も用意する (Action 用 secrets と整合)。

### 4.3 HTTP リトライポリシ

- 4xx ユーザー入力エラーは再試行しない。
- 5xx と 429 は `Retry-After` を尊重しつつ最大 3 回再試行 (基本 1.5 倍 backoff)。
- DNS / TCP エラーは最大 3 回。

## 5. Git / Linguist

### 5.1 git clone

- プラグイン `languages.indepth` と `licenses` がリポジトリを `git clone` する。
- Go 実装は `go-git/go-git/v5` を `--depth=1` で実行 (`PlainCloneContext`)。
- 認証は HTTPS Basic (`http.BasicAuth{Username:"x-access-token", Password: token}`)。
- 一時ディレクトリは `os.MkdirTemp(...)`、終了時に削除。
- 個別タイムアウト: `plugin_languages_analysis_timeout_repositories` (default 7.5 分)。

### 5.2 linguist

- `go-enry/go-enry/v2` で言語判定。GitHub Linguist と同等の挙動を提供。
- `.gitignore` / `.gitattributes` を尊重するためのオプションを `enry.GetLanguageInfo` で行う。

### 5.3 simple-git の代替

- git 基本操作 (主に Action 内で render hash 計算前の blob hash) の用途:
  - `git hashObject(path)` → Go では `crypto/sha1` で blob ハッシュを自作 (git の `sha1("blob <size>\0" + content)`)。
  - `git diff` / `log` 系は使われていないので不要。

## 6. ブラウザスクレイピング

### 6.1 用途

- `topics`, `starlists`, `achievements`, `support`, `community/screenshot`, `community/poopmap`, `community/splatoon` プラグイン。
- `svg.resize`, `svg.pdf`, `svg.hash`, insights HTML render。

### 6.2 chromedp

- ベース: `github.com/chromedp/chromedp`。
- 単一の **長寿命 Browser** を `engine` 起動時に確保し、必要に応じて Tab を開閉する。
- 失敗時は `RestartBrowser()` で再生成。
- Docker 内 Chrome のフラグ:
  - `--no-sandbox`
  - `--disable-extensions`
  - `--disable-dev-shm-usage`
  - `--disable-gpu`
  - `--single-process` (resource 抑制)
  - `--disable-features=IsolateOrigins,site-per-process`
- 環境変数 `METRICS_CHROME_PATH` で実行パスを上書き。

### 6.3 ページ操作のリトライ

- ナビゲーションは最大 3 回、各 30 秒タイムアウト。
- DOM 評価 (`page.Evaluate`) は失敗時に warning ログを残し、当該プラグインのみ失敗扱いとする。

## 7. 共通 HTTP クライアント方針

### 7.1 ラッパ

```go
// internal/httpx/client.go
package httpx

type Client struct { ... }

func (c *Client) GetJSON(ctx, url, headers, dest) error { ... }
func (c *Client) Get(ctx, url, headers) ([]byte, http.Header, error) { ... }
func (c *Client) Post(ctx, url, headers, body) ([]byte, error) { ... }
```

- 内部で `net/http` + `hashicorp/go-retryablehttp` を使用。
- ベース URL / Bearer / API key を `Option` 関数で組み立て。
- 共通ユーザーエージェント: `metrics/<version> (+https://github.com/lowlighter/metrics)`。
- 既定タイムアウト 30 秒、download body 制限 50 MB。

### 7.2 axios.get(url, {headers, params, responseType})

HTTP GET は最も多く使われる呼び出し系。Go ラッパでは `BinaryGet` (バイナリ/画像) と `JSONGet` (JSON) の二系を提供。

### 7.3 imgb64

- 仕様: 画像取得 → resize → base64 data URI を返す。
- Go 版: `httpx.BinaryGet` で取得 → `image/png` で decode → `imaging.Resize` → `bytes.Buffer` に encode → `data:image/png;base64,<...>`。

## 8. レート制限と再試行

### 8.1 GitHub

- レート制限残量を `/rate_limit` で都度確認し、`requests.{rest,graphql,search}` 状態を `internal/githubapi/rate.go` で保持する。
- 残量 < `quota_required` のときは Action では skipped 終了、Web では request を 429 で reject。

### 8.2 Secondary rate limit

- GitHub の secondary rate limit (`x-ratelimit-resource: secondary`) が当たった場合は `Retry-After` ヘッダに従って待機。

### 8.3 第三者

- 4xx ユーザーエラー → reject。
- 429 / 5xx → backoff retry (max 3, base 500 ms, max 5 s)。
- Spotify など token refresh が必要なものは `internal/plugins/music/spotify_auth.go` 等で別実装。
