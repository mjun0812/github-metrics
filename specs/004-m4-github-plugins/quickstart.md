# Quickstart: GitHub プラグイン群 (M4) 開発手順

**Date**: 2026-05-16 | **Plan**: [plan.md](./plan.md)

本 spec の実装に着手する開発者向けの初期セットアップ手順。M3 までの開発手順 (`specs/003-chromedp-rendering-pipeline/quickstart.md`) を踏まえている前提で、本 spec 固有の差分のみを記載する。

## 1. 必要なツール

### 1.1 通常開発に必要なもの

M1〜M3 から変更なし:

- Go 1.26+ (`go.mod` の `go` directive と一致)
- `make`
- `git`

新規依存ライブラリ (go.mod に追加) はランタイムでは特別なツール不要 (pure Go):

- `github.com/go-enry/go-enry/v2` — 言語判定
- `github.com/go-git/go-git/v5` — git clone

### 1.2 chromedp 経路 (topics / starlists テスト)

M3 で導入済の chromium バイナリ準備 (`specs/003-chromedp-rendering-pipeline/quickstart.md §1.1`) をそのまま使う。`METRICS_CHROME_PATH` の設定が必要。

### 1.3 heavy 経路 (languages.recent / indepth テスト)

追加ツール不要。`go-enry` の言語判定 DB は埋め込み済、`go-git` の浅い clone もテスト fixture から組み立てるので外部 git server 不要。

## 2. 開発の進め方 (新規 plugin 追加 7 ステップ)

採用 21 plugin はすべて以下のパターンに従って実装する:

### Step 1: data-model.md の Result 構造体を確認

`specs/004-m4-github-plugins/data-model.md` で実装対象 plugin の Result 構造体 (例: E-013 ActivityResult) を読み、フィールド / `json:` tag を把握する。

### Step 2: contracts/ で入力と挙動を確認

該当 plugin の優先度に応じて `contracts/plugin-{p1,p2,p3}.md` の該当節を読み、入力 / データソース / アルゴリズム / エッジケースを把握する。

### Step 3: テストファースト (table test)

`internal/plugins/<name>/<name>_test.go` を作り、最低 5 ケースのテーブルテストを書く:

1. 正常系 (octocat 相当 fixture)
2. 入力上限到達
3. scope 不足 / 空応答
4. API エラー (5xx)
5. 入力フィルタ (alias / ignored / skipped 系)

例 (activity plugin):

```go
package activity_test

import (
    "context"
    "testing"

    "github.com/mjun0812/github-metrics/internal/plugins"
    "github.com/mjun0812/github-metrics/internal/plugins/activity"
    "github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestRun_Table(t *testing.T) {
    cases := []struct{
        name      string
        fixture   string
        inputs    map[string]any
        wantSkip  bool
        wantEvents int
    }{
        {"normal", "octocat_events.json", nil, false, 50},
        {"limit_1", "octocat_events.json", map[string]any{"plugin_activity_limit": 1}, false, 1},
        {"empty", "empty_events.json", nil, false, 0},
        {"api_5xx", "error_500.json", nil, false, 0},
        {"filter_visibility", "octocat_events.json", map[string]any{"plugin_activity_visibility": "private"}, false, 0},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            pc := mocks.NewPluginContext(t, tc.fixture, tc.inputs)
            out, err := activity.Plugin.Run(context.Background(), pc)
            // ... assertions ...
        })
    }
}
```

### Step 4: fixture を vendor

`tests/fixtures/plugins/<name>/` に [data-model.md E-099](../data-model.md#E-099) のフォーマットで 4 種類の mocked レスポンスを置く:

- `<name>_octocat.json` (正常系)
- `<name>_empty.json` (空応答)
- `<name>_scope_missing.json` (該当 plugin のみ)
- `<name>_api_error.json` (5xx)

### Step 5: plugin 本体実装

`internal/plugins/<name>/<name>.go` で `Plugin` interface を実装。`init()` に `plugins.Register(Plugin)` を書く。Step 3 のテストが緑になることを確認する。

### Step 6: partial template

`internal/templates/classic/partials/<name>.svg.tmpl` を作成。[contracts/partial-classic-m4.md](./contracts/partial-classic-m4.md) の DOM 規約に従う。`Skipped=true` の早期 return ガードを必ず入れる。

### Step 7: golden file 生成

```bash
# JSON shape golden
go test ./internal/plugins/<name>/... -run TestRun_Golden -update

# partial fragment golden
go test ./internal/templates/classic/... -run TestPartial_<Name>_Golden -update

# 各 golden を git add してレビューに出す
```

## 3. Makefile ターゲット

M3 で導入済の target に **`test-heavy` を新規追加** する:

```makefile
test-heavy:
	go test -tags=heavy ./...

test-chromedp:        # M3 で既存
	METRICS_CHROME_PATH=... go test -tags=chromedp ./...

test:                 # M3 で既存
	go test ./...
```

CI (`.github/workflows/go-ci.yml`) には `test-heavy` ジョブを `test-chromedp` と並列で追加する (M3 で確立した 2 ジョブ並列構成を 3 ジョブに拡張)。

## 4. 上流互換性 fixture の再生成

M2 `internal/tools/sync-fixtures` (M2) を活用して octocat の上流出力を再取得し、`tests/fixtures/upstream/octocat.json` を 21 plugin 含む完全版に更新する:

```bash
# org_repo に upstream のクローンが必要 (.gitignore で除外済)
cd /Users/mjun/workspace/github-metrics
go run ./internal/tools/sync-fixtures --user octocat --full
```

`--full` フラグは本 spec 内で sync-fixtures を拡張する (T-001 相当の Setup タスクで実装)。完了後、`tests/compatibility/json_test.go` が 21 plugin 完全版に対する key diff = 0 を assert。

## 5. trouble shooting

### 5.1 go-enry の言語判定が上流と一致しない

go-enry は GitHub linguist の Go 移植だが、binary file detection や vendoring 判定のパッチが一部 lagging することがある。差異が出た場合は:

1. `go-enry.IsBinary(filename, content)` / `IsVendor(path)` を確認
2. 上流 metrics.json と Go 側 result.Plugins.languages.recent を diff
3. 差異が `aliases` / `ignored` で吸収可能なら入力 alias で対応
4. それでも合わないなら fixture 側でその言語の repo を skip

### 5.2 chromedp タブが leak する

`Browser.NewTab(ctx)` の戻り値 `cancel func()` を必ず `defer cancel()` で呼ぶ。M3 の Recycle 機構 (200 タブごとに allocCtx 作り直し) で leak は自動回収されるが、明示的 cancel を忘れると 1 Compute 内で 21 タブ作って Recycle に間に合わない。

### 5.3 base 拡張による既存テストの破壊

M4 で base の `runOrganization` / paging / indepth を追加する際、M1/M2 の base unit test が壊れる可能性がある。対策:

- M1/M2 既存テスト (`base_test.go::TestRun_User`) は touched しない MUST
- 新規テストは `base_test.go::TestRun_Organization` / `indepth_test.go::*` / `repositories_test.go::*` で個別のファイルに分離

## 6. PR セルフレビュー基準 (M4 plugin 単体)

PR を出す前に以下を確認:

- [ ] plugin の Run が 5 ケースのテーブルテストで全緑
- [ ] `Skipped=true` パスを最低 1 つカバーしている
- [ ] partial template が DOM 規約 ([contracts/partial-classic-m4.md §5](./contracts/partial-classic-m4.md#5-各-partial-の-dom-期待値-テスト-assertion-対象)) に従う
- [ ] JSON shape golden が上流 metrics.json の対応キーと一致
- [ ] PR 本文に新規依存追加根拠 (該当する場合のみ go-enry / go-git の代替検討と棄却理由) を記載
- [ ] constitution 原則 III の確認: 不採用 plugin のコードを追加していない (`compliance_test.go` 緑)
