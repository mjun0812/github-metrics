# Contract: P3 chromedp + heavy 依存プラグイン 4 個

**Date**: 2026-05-16 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) (E-011, E-012, E-023, E-024)

本書は P3 4 個の plugin の Run 規約を確定する。対象: **languages.recent** (T-042), **languages.indepth** (T-043), **topics** (T-053), **starlists** (T-054)。これらは chromedp / go-git / go-enry のいずれかに依存し、build tag で隔離される。

共通規約は [plugin-p1-mvp.md §共通契約](./plugin-p1-mvp.md#共通契約) と同じ。本書では plugin 固有差分と依存ランタイム整理を扱う。

## 1. languages.recent (T-042)

### 1.1 ファイル / build tag
- `internal/plugins/languages/recent.go` — build tag **無し** (`recent.go` 自体は通常コンパイル対象)
- `internal/plugins/languages/recent_heavy_test.go` — build tag `//go:build heavy`

go-enry は実 runtime には embedded data がロードされるが import コスト自体は小さいので runtime には build tag 不要。テストのみ重い fixture を使うので heavy で隔離する。

### 1.2 入力
- `plugin_languages_sections` に `"recently-used"` が含まれること
- `plugin_languages_recent_days` (int, 既定 14)
- `plugin_languages_recent_load` (int, 既定 100) — 取得 PushEvent 上限
- `plugin_languages_recent_categories` (csv, 既定 `"programming"`) — go-enry の category filter

### 1.3 データソース
- REST `/users/X/events` の PushEvent 最新 `_load` 件
- 各 commit について REST `/repos/X/commits/Y` で files の `filename` を取得
- 各 filename を `go-enry.GetLanguage(name, content)` (content は不要なら空) で言語判定

### 1.4 アルゴリズム
1. PushEvent から `created_at >= now - _days` の commit を抽出
2. 各 commit について files の filename を go-enry に渡す
3. 言語別バイト数 ( = 触られたファイルの `additions + deletions` の合計) を集計
4. T-041 と同じソート / `Other` 集約ロジックで上位 N 件

### 1.5 出力
[E-011 LanguagesRecentResult](../data-model.md#E-011)

### 1.6 エッジケース
- `metrics.run.linguist=false` extras フラグ (上流互換) → `Skipped=true, SkippedReason="linguist disabled via extras"`
- PushEvent 0 件 → `Favorites: []`, `Skipped=false` (空でも skipped にしない)

### 1.7 性能予算
- < 2s (mocked、100 PushEvent + 各 1 GET = 100 リクエスト相当を mocked HTTP で)
- go-enry の言語判定は filename だけなら μs オーダー、content 解析を有効にすると ms オーダー

## 2. languages.indepth (T-043)

### 2.1 ファイル / build tag
- `internal/plugins/languages/indepth.go` — build tag **無し**
- `internal/plugins/languages/indepth_heavy_test.go` — build tag `//go:build heavy`

go-git の `PlainClone` 自体は通常 runtime にも必要なので tag 不要、テストだけ tag 隔離。

### 2.2 入力
- `plugin_languages_indepth=true`
- `plugin_languages_analysis_timeout_repositories` (string, 既定 `"7.5min"`) — go-git clone + 解析の上限
- `plugin_languages_analysis_timeout` (string, 既定 `"15min"`) — 全体上限
- extras `metrics.cpu.overuse=true` + `metrics.run.git=true` + `metrics.run.linguist=true` のすべて必要

### 2.3 データソース
- `base.Computed.Repositories` の各 repo を `git.PlainClone(tempDir, Depth:1, URL:<repo.clone-url>)` で浅い clone
- clone 後、HEAD tree のファイル一覧を `tree.Files()` で走査、各ファイルの content (size) を go-enry に渡して言語判定

### 2.4 アルゴリズム
1. extras フラグを検査、いずれか欠落で `Skipped=true`
2. `base.Computed.Repositories` 各 repo について並列処理 (errgroup with concurrency limit 4):
   - `os.TempDir() + "/metrics-indepth-<pid>-<repo>"` に shallow clone
   - tree walk + go-enry 判定
   - 言語別バイト数を `LanguageBytes.Bytes` に集計
   - 完了後 `RemoveAll(tempDir)` (`defer` で確実に)
3. 個別 repo timeout (`_timeout_repositories`) 超過は当該 repo の `Errors` に記録、他 repo は continue
4. 全体 timeout (`_timeout`) 超過は loop break、それまでの結果を `Repositories` map に保存
5. `Total.Bytes` を全 repo の合計で計算

### 2.5 出力
[E-012 LanguagesIndepthResult](../data-model.md#E-012)

### 2.6 エッジケース
- private repo の clone URL が auth 必要 → `https://x-access-token:<PAT>@github.com/X/Y.git` 形式で auth 付与 (PAT scope に `repo` 必要)
- clone 中 disk full → `Errors` に `"disk full: <repo>"`、defer cleanup は best-effort
- shallow clone の `.git` ディレクトリは fixture テストでは `t.TempDir()` 配下に手動で組み立てる (実 git server は使わない)

### 2.7 性能予算
- 個別 repo: `_timeout_repositories` (7.5min) 内に完了。mocked では fixture から即時返却で < 100ms / repo
- 全体: `_timeout` (15min) 内
- 実 GitHub clone は本予算の上限事例。mocked テストで 100ms / repo を保てれば十分

## 3. topics (T-053, chromedp)

### 3.1 ファイル / build tag
- `internal/plugins/topics/topics.go` — build tag **無し** (runtime コードは chromedp に依存するが、`Deps.Render` 経由でアクセスし、`Deps.Render == nil` のときは `Skipped=true` で抜けるので通常コンパイルは可能)
- `internal/plugins/topics/topics_chromedp_test.go` — build tag `//go:build chromedp`

### 3.2 入力
- `plugin_topics_mode` (string, 既定 `"icons"`) — `"icons"` / `"list"`
- `plugin_topics_limit` (int, 既定 15)
- `plugin_topics_sort` (string, 既定 `"name"`) — `"name"` / `"starred-at"`
- extras `metrics.run.puppeteer.scrapping=true` (上流互換) MUST true

### 3.3 データソース
- chromedp で `https://github.com/stars/<login>/topics` ページにアクセス
- DOM から topic カード (`.starred-list-topics`) を抽出: `name`, `description`, `icon` (img src), `url`

### 3.4 アルゴリズム
1. extras `metrics.run.puppeteer.scrapping=false` → `Skipped=true, SkippedReason="puppeteer scrapping disabled via extras"`
2. `pc.Deps.Render == nil` → `Skipped=true, SkippedReason="chromedp not available"` (`Result.Errors` に `*RetryableError` 追加)
3. `Browser.NewTab(ctx)` でタブを開く
4. `chromedp.Navigate(URL)` + `chromedp.WaitVisible(".starred-list-topics")`
5. JS で DOM 抽出 → JSON で Go へ受け渡し
6. `_sort` に従って sort、上位 `_limit` 件

### 3.5 出力
[E-023 TopicsResult](../data-model.md#E-023)

### 3.6 エッジケース
- ページが空 (topics 0 件) → `List: []`, `Skipped=false`
- chromedp タイムアウト → `*RetryableError`, plugin skip

### 3.7 性能予算
- < 5s (実 chromedp 起動 + ページロード + DOM 解析を 1 タブで完結)
- mocked テスト (chromedp build tag) では < 8s 許容 (ヘッドレス起動オーバヘッド込み)

## 4. starlists (T-054, chromedp)

### 4.1 ファイル / build tag
- `internal/plugins/starlists/starlists.go` — build tag **無し**
- `internal/plugins/starlists/starlists_chromedp_test.go` — build tag `//go:build chromedp`

### 4.2 入力
- `plugin_starlists_languages` (bool, 既定 false) — true のとき各 list 内 repo で言語分析 (T-041 ロジック再利用)
- `plugin_starlists_limit` (int, 既定 4)
- extras `metrics.run.puppeteer.scrapping=true`

### 4.3 データソース
- chromedp で `https://github.com/stars/<login>/lists` を navigate
- DOM から starlist 一覧 → 各 list の詳細ページに navigate して repos を取得

### 4.4 アルゴリズム
1. extras / Render 不在で `Skipped=true` (topics と同じパス)
2. starlist 一覧ページから `name`, `description`, `count` を抽出
3. `_languages=true` のとき、各 list の repo 一覧 (chromedp 経由) を `base.Computed.Repositories` と join して T-041 標準 languages ロジックで言語集計
4. 上位 `_limit` 件

### 4.5 出力
[E-024 StarlistsResult](../data-model.md#E-024)

### 4.6 性能予算
- < 8s (複数 starlist 詳細ページに順次 navigate、`_languages=true` のとき言語集計込み)

## 5. テスト共通規約 (P3)

- 通常 `go test ./internal/plugins/languages/...` は heavy tag が無効なので languages.recent / indepth のテストは **コンパイルされず skip** (test file が build tag で除外される)
- `make test-heavy` (新規 Makefile target、`go test -tags=heavy ./...`) で languages.recent / indepth のテストを実行
- `make test-chromedp` (M3 既存) で topics / starlists のテストを実行
- 各 P3 plugin にも最低 5 ケースのテーブルテスト + golden file。fixture は build tag 隔離側に置く (`*_chromedp_test.go` / `*_heavy_test.go` 内)

## 6. CI ジョブ拡張

`.github/workflows/go-ci.yml` に以下のジョブを追加 (M3 で `test-chromedp` を既設):

```yaml
test-heavy:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
    - run: make test-heavy
```

`test-heavy` は go-enry / go-git の依存だけで CI 実行可能 (pure Go、chromium 不要)。

## 7. 合計性能予算 (P3)

P3 4 plugin は通常 CI では skip され、専用ジョブで実行されるので Compute フルパス SC-003 への影響は **ゼロ**。`make test-heavy` 自体は < 60s で完走することを目標 (CI ジョブ単独実行時間)。`make test-chromedp` は < 120s (chromium 起動コスト + 2 plugin の navigate + golden 検証込み)。
