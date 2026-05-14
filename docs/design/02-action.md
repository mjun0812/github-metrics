# 02. CLI / GitHub Action 仕様

## 目次

- [1. 概要](#1-概要)
- [2. action.yml の入出力](#2-actionyml-の入出力)
- [3. 実行フェーズ](#3-実行フェーズ)
- [4. Committer (commit / PR / Gist) ロジック](#4-committer-commit--pr--gist-ロジック)
- [5. リトライとレート制御](#5-リトライとレート制御)
- [6. Docker と CLI バイナリ](#6-docker-と-cli-バイナリ)
- [7. CLI モード (action.yml に依存しない直接利用)](#7-cli-モード-actionyml-に依存しない直接利用)

---

## 1. 概要

GitHub Action としての利用は次の流れで行われる。

1. ユーザーは `lowlighter/metrics@v3` を `uses` で参照し、`with:` で入力を渡す。
2. `action.yml` の `runs.using: "composite"` がトリガされ、shell スクリプトが Docker イメージ `ghcr.io/lowlighter/metrics:<tag>` を pull / build して `docker run` する。
3. コンテナ内で `metrics-action` バイナリ (Go 版) が起動する。
4. `INPUTS` 環境変数(`toJson(inputs)`) を読み、各入力を `INPUT_<UPPER>` 互換で解釈する。
5. メトリクス計算 → ファイル出力 → commit/PR/gist 等の出力アクションを実行する。

Go 版でも上記フローを完全に踏襲する。`action.yml` 自体は今のまま流用し、Dockerfile 内の entrypoint だけが node から Go バイナリに置き換わる。

## 2. action.yml の入出力

### 2.1 inputs

`action.yml` には **1600 行超** の入力定義が含まれており、すべて `core/metadata.yml` から自動生成される。Go 版は

- `assets/plugins/<name>/metadata.yml` を `//go:embed` でバンドル
- 起動時に各 plugin の inputs 定義を組み上げ、`os.Getenv("INPUT_<KEY>")` で実値を取り出す

形にする。

入力カテゴリ:

| カテゴリ | 接頭辞 | 例 |
|--------|------|----|
| 基本 | (なし) | `user`, `template`, `token`, `filename` |
| Base content | `base_` | `base`, `base_indepth`, `base_hireable`, `repositories`, `repositories_batch` |
| Core 設定 | `config_` / `output_` / `committer_` / `delay` / `retries` … | `config_timezone`, `config_output`, `output_action`, `committer_branch` |
| プラグイン有効化 | `plugin_<name>` | `plugin_languages`, `plugin_achievements` |
| プラグイン設定 | `plugin_<name>_<opt>` | `plugin_languages_indepth`, `plugin_languages_limit` |
| デバッグ | `debug`, `debug_flags`, `debug_print`, `dryrun`, `use_mocked_data`, `use_prebuilt_image` |
| Notice / 設定 | `notice_releases`, `clean_workflows`, `setup_community_templates` |
| API | `github_api_rest`, `github_api_graphql` |

### 2.2 値の型変換

`metadata.yml` の `type:` フィールドに従い以下のように変換する:

| type | 解釈 |
|------|------|
| `boolean` | "yes"/"true"/"1" → `true` |
| `number` | `strconv.ParseFloat` (min/max があれば clamp) |
| `string` | そのまま |
| `array` | `format:` が `comma-separated` / `space-separated` / `newline-separated` のいずれか/組合せで split。`values:` が定義されていればホワイトリスト |
| `json` | `encoding/json` で Unmarshal |
| `token` | string、ログ出力時にマスク |

`metadata.plugins.*.inputs.action({core, preset})` 相当を `config.MetadataPlugin.InputsForAction(env, preset)` として実装する。

### 2.3 outputs

`action.yml` の `outputs:` セクション (生成ファイルパス、commit SHA など) を再現する:

| 出力 | 内容 |
|------|------|
| `metrics_url` | 生成された svg/markdown ファイルのリポジトリ内 URL |
| `metrics_sha` | レンダリング後の SHA |

これらは `core.setOutput()` の代わりに、`GITHUB_OUTPUT` 環境変数で指定されるファイルに `key=value` を append する (`@actions/core` v2 仕様)。

## 3. 実行フェーズ

実行順序は次の通り。Node 版 `source/app/action/index.mjs` も同じ流れ (リンクは出典/トレース用、実装に参照は不要)。

### 3.1 Initialization

- `github.context.eventName == "push"` で commit message に `[Skip GitHub Action]` または `Auto-generated metrics for run #N` が含まれていたら `exit 0` (skipped)。
- `setup()` を呼び、`Plugins`/`Templates`/`conf` をロード (`internal/engine/setup.go`)。
- Docker 環境(`!GITHUB_ACTIONS`) では `INPUT_OUTPUT_ACTION=none`, `INPUT_COMMITTER_TOKEN=INPUT_TOKEN`, `GITHUB_REPOSITORY=octocat/hello-world` を補完。

### 3.2 Core inputs

- `config_presets` をロード (`internal/config/presets.go`)。
- core プラグインの inputs(`metadata.plugins.core.inputs.action`) を読み込み、構造体 `CoreInputs` に詰める。
- `filename` のワイルドカード `*` を `output` に応じて拡張子に置換 (`*.svg`, `*.md`, `*.pdf`, `*.html`)。

### 3.3 Token validation

- `token` が未指定なら fail。
- `github_pat_` で始まれば fine-grained PAT を拒否 (GraphQL 非対応のため)。
- GraphQL クライアントは `githubv4.NewClient` を `Authorization: token <PAT>` で初期化。`github.api.graphql` でカスタム endpoint。
- REST クライアントは `go-github` を同様に。
- `mocked` 指定時は内蔵モックを利用 (Go 版は `internal/testutil/mocks` から differential 取得)。
- レート制限を `GET /rate_limit` で取得し、`quota_required_rest/graphql/search` を満たさなければ skipped 終了。
- `HEAD /` を打ち `X-OAuth-Scopes` ヘッダが有るかでトークン妥当性を確認。

### 3.4 新バージョン通知

`notice_releases=true` なら `GET /repos/lowlighter/metrics/releases` を打ち、`tag_name` が現バージョンより新しければ `::notice::` を出力。

### 3.5 GitHub アカウント解決

- `rest.users.getAuthenticated()` から `authenticated` ログインを取得 (失敗時は `GITHUB_REPOSITORY_OWNER` をフォールバック)。
- `user` 入力があればそれを優先。`q.repo` があればリポジトリも記録。

### 3.6 Committer 構築 → §4

### 3.7 Insights webserver (insights output 時)

- `output==="insights"` ならコンテナ内で `metrics-server` をバックグラウンド起動し、`stdout` に `Server ready !` が現れるまで待機(5 分タイムアウト)。
- Go 版では goroutine + `net.Dial` で待機する方が確実。

### 3.8 Plugins / Query 構築

- `metadata.plugins.<name>.inputs.action` を回し、`Plugins` レジストリ順に `plugin_<name>` が `true` のもののみ enable。
- 入力は二系統に振り分ける:
  - `type:token` → `plugins[name][key]` に格納 (秘匿)
  - その他 → `q["<name>.<key>"]` に格納
- `q[name] = true` でプラグイン全体を有効化フラグに。

### 3.9 Render

- `engine.Compute` を `retries`/`retries_delay` 付きで呼ぶ。
- 結果 `{Rendered, MIME, Errors}` を取得し、`debug_print` の時は標準出力に dump。

### 3.10 Output condition

- `output==="svg"` かつ `output_condition==="data-changed"` の時、commit/PR モードでは既存ファイルの SHA と比較してマージ要否を判定。
- 比較には `svg.Hash(rendered)` を使用 (footer の SHA 部分を除外したうえで MD5)。

### 3.11 Save & Output action

- `/renders/<filename>` に書き出し。
- `output_action` (`none` / `gist` / `commit` / `pull-request` / `pull-request-merge` / `pull-request-squash` / `pull-request-rebase`) に従って下記 §4 の処理を行う。

### 3.12 Workflow cleanup

`clean_workflows` (true) の場合、`GET /repos/<owner>/<repo>/actions/runs` を回して、`metrics` を生成したワークフローの旧 run を削除。

## 4. Committer (commit / PR / Gist) ロジック

### 4.1 committer 構造体

```go
type Committer struct {
    Token   string
    Gist    string  // gist 出力時のみ
    Commit  bool
    Message string  // ${filename} は実ファイル名に置換済み
    PR      bool
    Merge   string  // "" | "merge" | "squash" | "rebase"
    Branch  string  // base ブランチ
    Head    string  // head ブランチ (PR 時は `metrics-run-${runId}`)
    REST    *github.Client
    SHA     string  // 前回 render のオブジェクト SHA
}
```

### 4.2 commit 処理

1. `head` ブランチが無ければ `branch` の sha から作成。
2. `GET /repos/.../contents/<filename>` で前回 SHA を取得。
3. `PUT /repos/.../contents/<filename>` で content を upload (base64)。
4. `data-changed` 条件で SHA 一致なら skip。

### 4.3 markdown キャッシュ

markdown 出力の場合、生成 HTML に含まれる `<img class="metrics-cacheable" data-name=... src="data:image/...;base64,...">` を抽出し、`markdown_cache` ディレクトリ(既定 `.cache/`) 配下のファイルとして commit し、`<img src="https://github.com/.../blob/<branch>/<path>">` に書き換える。

### 4.4 PR 作成 / マージ

- `pull-request-*` モードでは `POST /repos/.../pulls` で PR を作る。
- `merge` が指定されていれば `PUT /repos/.../pulls/{num}/merge` でマージ。
- 失敗した場合は warning ログを出して継続(致命でない)。

### 4.5 Gist 出力

- `gist` モードでは render 結果を `PATCH /gists/{gist_id}` で更新 (`files: { [filename]: { content }}`)。
- `png`/`jpeg`/`markdown-pdf` の Gist 出力は明示的にエラー。
- gist の token は通常の `token` を使う(committer token に gist scope が無い前提)。

## 5. リトライとレート制御

| 入力 | 対象 |
|------|------|
| `retries` | render 全体 (デフォルト 3) |
| `retries_delay` | render リトライ間隔(秒) |
| `retries_output_action` | commit / PR / Gist 操作 |
| `retries_delay_output_action` | 同上の間隔 |
| `delay` | render 後の待機 (`output_action` が API でレート制限に当たるのを避ける) |

Go 版では `internal/engine/retry.go` に `Retry(ctx, fn, retries, delay)` を実装し、すべての対象で共通使用する。

> 起動バナーの整形ルール (Node 版 `info()` 関数互換のラベル幅・token マスク等) は [13-appendix.md §E](./13-appendix.md#e-action-起動バナーの整形ルール-info-互換) を参照。

## 6. Docker と CLI バイナリ

### 6.1 Dockerfile

- `FROM golang:1.23 AS build` で `go build -trimpath -ldflags='-s -w' -o /out/metrics-action ./cmd/metrics-action` を行う。
- ランタイムは `FROM gcr.io/distroless/cc-debian12` または `chromium` 同梱の `mcr.microsoft.com/playwright` ベースで Chromium バイナリを同梱。
- chromedp 用に `PUPPETEER_BROWSER_PATH` 互換の `METRICS_CHROME_PATH` を環境変数として注入。
- `WORKDIR /metrics`, `ENTRYPOINT ["/metrics-action"]`。

### 6.2 イメージタグ運用

- リリースタグ (`v3.x.x`) の単純な `vX.Y` を抽出して `ghcr.io/lowlighter/metrics:X.Y` を pull。
- beta なら `X.Y-beta`。
- ローカル fork は `metrics:forked-<version>` をビルド。

### 6.3 環境変数

| 変数 | 役割 |
|------|------|
| `INPUTS` | `${{ toJson(inputs) }}` を保持。Go バイナリはこの JSON を最優先で読み、無ければ `INPUT_<UPPER>` を読む |
| `INPUT_*` | 個別入力 |
| `GITHUB_*` | GitHub Actions 標準 (`GITHUB_REPOSITORY`, `GITHUB_EVENT_PATH`, `GITHUB_RUN_ID`, …) |
| `METRICS_RENDERS` | ホストの renders マウントポイント |
| `TZ` | タイムゾーン |
| `METRICS_CHROME_PATH` | chromedp 用 Chromium 実行パス |

### 6.4 終了コード

| 状態 | code |
|------|------|
| success | 0 |
| skipped | 0 |
| failed  | 1 |

`die=true` 設定でプラグイン内エラーが致命に分類されたとき、または GitHub API トークン無効時に 1 で終了。

## 7. CLI モード (action.yml に依存しない直接利用)

Go 版では `metrics-action --config inputs.yaml` で `INPUTS` 相当の JSON / YAML をローカルから読めるようにする (テスト・スクリプト用)。

```sh
metrics-action \
  --user lowlighter \
  --template classic \
  --token "$GITHUB_TOKEN" \
  --plugin "languages=true" \
  --plugin "languages.indepth=true" \
  --output svg \
  --filename out.svg
```

- 同じ入力解釈 (`metadata.yml` ベース) を流用する。
- `--dryrun` で commit/PR/gist を抑制。
- `--output` は `svg|png|jpeg|json|markdown|markdown-pdf|insights`。
- `--config inputs.yaml` で全入力を YAML 一括指定可能(`action.yml` の `with:` をそのまま貼り付け可)。
