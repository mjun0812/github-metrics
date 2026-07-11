# GitHub データ取得方法リファレンス

採用済み各プラグインが GitHub のどのデータを、どの取得方法 (REST / GraphQL / HTML スクレイピング / ローカル計算) で得ているかの一覧。採用プラグインの確定リストは [`scope.md`](scope_ja.md) を参照。

> 初版調査日: 2026-06-04

---

## 目次

- [1. 共通APIクライアント](#1-共通apiクライアント)
- [2. 取得方法別エンドポイント一覧](#2-取得方法別エンドポイント一覧)
  - [2.1 REST API のみ](#21-rest-api-のみ)
  - [2.2 GraphQL のみ](#22-graphql-のみ)
  - [2.3 HTML スクレイピング](#23-html-スクレイピング)
  - [2.4 ローカル計算 (API呼び出しなし)](#24-ローカル計算-api呼び出しなし)
- [3. REST / GraphQL 両方で取得可能なデータ](#3-rest--graphql-両方で取得可能なデータ)
- [4. 認証トークン](#4-認証トークン)
- [5. 既知の制限、注意点](#5-既知の制限注意点)

---

## 1. 共通APIクライアント

| ファイル                        | 役割                                                                             |
| ------------------------------- | -------------------------------------------------------------------------------- |
| `internal/githubapi/graphql.go` | GraphQL クライアント (genqlient ラッパー)                                        |
| `internal/githubapi/rest.go`    | REST クライアント (httpx ラッパー)                                               |
| `internal/githubapi/scopes.go`  | `HEAD /` の `X-OAuth-Scopes` ヘッダーでトークンスコープを確認 (Classic PAT 向け) |

---

## 2. 取得方法別エンドポイント一覧

### 2.1 REST API のみ

GraphQL に相当するエンドポイントが存在しないため REST を使用。

| エンドポイント                                        | 取得情報                                                       | 使用プラグイン                      |
| ----------------------------------------------------- | -------------------------------------------------------------- | ----------------------------------- |
| `GET /users/{login}/events?per_page=100&page={n}`     | ユーザーの公開イベント一覧 (PushEvent / IssueEvent 等)         | activity, habits, languages(recent) |
| `GET /repos/{owner}/{repo}/commits/{sha}`             | 単一コミットの変更ファイル一覧                                 | habits, languages(recent)           |
| `GET /repos/{owner}/{repo}/compare/{before}...{head}` | コミット範囲の差分ファイル一覧                                 | habits, languages(recent)           |
| `GET /repos/{owner}/{repo}/stats/contributors`        | コントリビューター別コミット数、追加/削除行数 (202 ポーリング) | contributors                        |
| `GET /repos/{owner}/{repo}/contributors?per_page={n}` | リポジトリコントリビューター一覧 (名前 / コミット数)           | people(repo)                        |
| `GET /repos/{owner}/{repo}/stargazers?per_page={n}`   | リポジトリスターガザー一覧                                     | people(repo)                        |
| `GET /repos/{owner}/{repo}/subscribers?per_page={n}`  | リポジトリウォッチャー一覧                                     | people(repo)                        |
| `GET /users/{login}/starred?per_page=100&page={n}`    | ユーザーがスターしたリポジトリ一覧                             | repositories(starred)               |
| `GET /repos/{owner}/{repo}/traffic/views`             | リポジトリの PV / ユニーク訪問者数 (`repo` スコープ必須)       | traffic                             |
| `HEAD /` (`X-OAuth-Scopes` ヘッダー)                  | トークンのスコープ確認                                         | traffic                             |

### 2.2 GraphQL のみ

REST に相当するエンドポイントが存在しないため GraphQL を使用。

| フィールド / クエリ                                        | 取得情報                                                              | 使用プラグイン                         |
| ---------------------------------------------------------- | --------------------------------------------------------------------- | -------------------------------------- |
| `User(login)`                                              | ユーザー基本情報 (名前 / bio / アバター / フォロワー数等)             | base                                   |
| `Organization(login)`                                      | 組織基本情報                                                          | base                                   |
| `Repository(owner, repo)`                                  | 単一リポジトリ詳細                                                    | base                                   |
| `UserRepositories(login, first, after)`                    | ユーザーのリポジトリ一覧 (ページネーション付き)                       | base                                   |
| `user.contributionsCollection.contributionCalendar`        | コントリビューションカレンダー (週別 / 日別カウント、REST では非公開) | base → calendar / isocalendar で再利用 |
| `user.contributionsCollection.*`                           | コミット / Issue / PR / Review の年別統計                             | base                                   |
| `user.repositoriesContributedTo(orderBy: STARGAZERS_DESC)` | コントリビュートした他者リポジトリ一覧                                | notable                                |
| `user.followers(first: limit)`                             | フォロワー一覧                                                        | people                                 |
| `user.following(first: limit)`                             | フォロー中一覧                                                        | people                                 |
| `user.issues.reactions.content`                            | Issue のリアクション集計                                              | reactions                              |
| `user.issueComments.reactions.content`                     | Issue コメントのリアクション集計                                      | reactions                              |
| `user.sponsorshipsAsMaintainer(first: limit)`              | スポンサー一覧 (tier / 開始日)                                        | sponsors                               |
| `viewer.sponsorshipsAsSponsor(first: limit)`               | スポンサーしている維持者一覧 (tier / 総額)                            | sponsorships                           |
| `repository.stargazers(orderBy: STARRED_AT)`               | リポジトリのスターガザー時系列                                        | stargazers                             |
| `user.lists` / `list.items.repository`                     | Star Lists 一覧＋各リスト内リポジトリ (REST に対応エンドポイントなし) | starlists                              |
| `user.starredRepositories(orderBy: STARRED_AT_DESC)`       | スターしたリポジトリ一覧 (言語 / ライセンス / 統計付き)               | stars                                  |

### 2.3 HTML スクレイピング

GitHub API が対応していないため HTML を直接パース。

| URL                                      | 取得情報                                           | 使用プラグイン |
| ---------------------------------------- | -------------------------------------------------- | -------------- |
| `https://github.com/stars/{user}/topics` | スターした topics (topic 名 / 説明 / アイコン URL) | topics         |

実装: `goquery` で `a[href^="/topics/"]` セレクターにマッチするアンカーを収集。

### 2.4 ローカル計算 (API呼び出しなし)

base プラグインが取得済みのデータを加工するだけで追加 API 呼び出しなし。

| データソース                         | 生成情報                                 | 使用プラグイン            |
| ------------------------------------ | ---------------------------------------- | ------------------------- |
| base の `ContributionCalendar.Weeks` | 月別コントリビューションヒストグラム     | calendar                  |
| base の `ContributionCalendar.Weeks` | ISO 週カレンダー / streak / 統計         | isocalendar               |
| base の `RepositoryList.Languages`   | 言語別バイト分布 (standard mode)         | languages                 |
| base の各種統計値                    | 段階別アチーブメントバッジ               | achievements              |
| PushEvent 変更ファイル + go-enry     | 言語判定 (ファイル拡張子 / 内容から推定) | habits, languages(recent) |

---

## 3. REST / GraphQL 両方で取得可能なデータ

以下のデータは REST / GraphQL どちらでも取得できるが、現在の実装は一方を選択している。

| データ                                        | 現在の実装 | REST の代替                              | GraphQL の代替                               |
| --------------------------------------------- | ---------- | ---------------------------------------- | -------------------------------------------- |
| リポジトリ一覧                                | GraphQL    | `GET /users/{login}/repos`               | `user.repositories`                          |
| フォロワー / フォロー中                       | GraphQL    | `GET /users/{login}/followers` 等        | `user.followers` / `user.following`          |
| スターしたリポジトリ                          | GraphQL    | `GET /users/{login}/starred`             | `user.starredRepositories`                   |
| リポジトリのスターガザー一覧                  | GraphQL    | `GET /repos/{owner}/{repo}/stargazers`   | `repository.stargazers`                      |
| リポジトリのコントリビューター一覧 (名前のみ) | REST       | `GET /repos/{owner}/{repo}/contributors` | `repository.defaultBranchRef.target.history` |
| リポジトリのウォッチャー一覧                  | REST       | `GET /repos/{owner}/{repo}/subscribers`  | `repository.watchers`                        |

---

## 4. 認証トークン

### Classic PAT (現在使用)

- スコープ検出: `HEAD /` の `X-OAuth-Scopes` レスポンスヘッダー (`scopes.go`)
- 全プラグインが正常動作する
- 最低限必要なスコープ:
  - 基本 (公開情報のみ): `public_repo`
  - traffic プラグイン: `repo`

### Fine-grained PAT (将来対応が必要)

- `X-OAuth-Scopes` ヘッダーを返さないため、現在のスコープ検出が機能しない
- `traffic` プラグインが常にスキップされる
- 必要な権限: traffic → `Metadata: read`
- Classic PAT の廃止アナウンスが出た時点で対応予定

---

## 5. 既知の制限、注意点

### REST: コントリビューター統計が大規模リポジトリで 0 を返す

- `GET /repos/{owner}/{repo}/stats/contributors` は **10,000 コミット超**のリポジトリで `additions` / `deletions` が全件 0 を返す
- GitHub API の仕様上の制限 (回避不可)
- contributors プラグインで全件 0 の場合は警告ログを出力することを推奨

### REST: イベント API はリアルタイムではない

- `GET /users/{login}/events` は最大 **6 時間の遅延**が発生する場合がある
- 「This API is not built to serve real-time use cases」と GitHub ドキュメントに明記
- activity / habits / languages(recent) プラグインで影響を受ける可能性がある

### HTML スクレイピング: topics のクラス名が変化している

- 現在の GitHub HTML では `.topic-name` / `h3` / `p.f3` / `p.f4` クラスが存在しない
- topic 名はアンカー直下のテキストノードとして存在する
- 現在のコードはスラッグへのフォールバックで動作するが、表示名の取得には `a.Text()` を中間フォールバックに追加するべき (`http_navigator.go:109` 付近)

```go
// 推奨修正
name := firstNonEmptyText(a, "h3", ".topic-name", "p.f3", "p.f4", "p.lh-condensed", "p")
if name == "" {
    name = strings.TrimSpace(a.Text()) // アンカー直下テキストから取得
}
if name == "" {
    name = slug
}
```

### Fine-grained PAT: スコープ検出が機能しない

- `scopes.go` の `X-OAuth-Scopes` ヘッダー読み取りは Classic PAT 専用
- Fine-grained PAT では `X-Accepted-GitHub-Permissions` ヘッダーを使う必要がある
- 対策: スコープ事前チェックの代わりに楽観的呼び出し＋ 403 を graceful skip に変更する

```go
// 推奨修正 (traffic プラグイン例)
result, err := rest.TrafficViews(ctx, owner, repo)
if err != nil {
    var apiErr *githubapi.HTTPError
    if errors.As(err, &apiErr) && apiErr.StatusCode == 403 {
        return skip("insufficient permissions (repo write access required)")
    }
    return err
}
```
