# アーキテクチャ

`github-metrics` は GitHub ユーザー / 組織 / リポジトリの統計を集約し、プロフィールカードを SVG / PNG / JPEG / JSON で出力するツールである。upstream [`lowlighter/metrics`](https://github.com/lowlighter/metrics) の subset を Go に移植したもので、採用範囲は [`scope.md`](scope_ja.md) に定義する。

## 目次

- [1. 2 つの起動形態](#1-2-つの起動形態)
- [2. パイプライン](#2-パイプライン)
- [3. プラグインシステム](#3-プラグインシステム)
- [4. テンプレート](#4-テンプレート)
- [5. パッケージ構成](#5-パッケージ構成)

---

## 1. 2 つの起動形態

単一バイナリ `cmd/metrics-cli` が **GitHub Action** と **CLI** の両方を兼ねる (#646 でモードを統合)。実行時に `GITHUB_ACTIONS` で分岐せず、入力を次の順で重ね合わせる (後勝ち):

1. `INPUT_<UPPER>` 環境変数 (Action ランナーが `action.yml` の入力から生成) と `INPUTS` JSON
2. CLI フラグ (`--user` / `--template` / `--plugin key=value` など)

入力の命名体系とフラグ一覧は [`configuration.md`](configuration_ja.md) を参照。パースした入力は `internal/action.Run` に渡され、`engine.Compute` を呼ぶ。

## 2. パイプライン

中核は `internal/engine` の 1 関数:

```go
func Compute(ctx context.Context, req Request, deps Deps) (*Result, error)
```

- `Request{Login, Repo, Template, Format, Account, Inputs, Parallel, Die}`
- `Deps{Settings, Metadata, Logger, HTTPClient, REST, GraphQL, Render}`
- `Result{Data *plugins.Data, Errors []error, Output []byte, MIME string, Provider plugins.Provider}`

`Compute` の流れ:

1. **テンプレート解決**: `templates.MustGet(req.Template)` を引き、`Check(account, format)` で対応可否を検証する。
2. **データ枠の準備**: `plugins.Data` を組み立てる。
3. **レートゲート**: 起動時に GitHub API のレートリミット残量を確認する (#529)。
4. **dataprovider の生成**: `dataprovider.New(...)` が、プロフィール / リポジトリ / カレンダーを **遅延 / メモ化**して取得する Provider を返す (#603)。共有データはここに集約され、各プラグインが重複取得を避けて読む。
5. **core プラグイン (Stage 1)**: `core.Plugin.Run` が `config_*` を解釈して `data.Config` / `data.Computed` を埋める。
6. **各プラグイン (Stage 2)**: `core.RunPlugins(ctx, pc, req.Parallel)` が登録済みの残りプラグインを `golang.org/x/sync/errgroup` で並列実行する。各結果は `data.Plugins[name]` に、エラーはプラグイン単位で同マップに格納される (`die=false` 時は footer に集約)。
7. **出力ディスパッチ (Stage 3)**: `engine/dispatch.go` が `format` で分岐する。

出力分岐 (`dispatchOutput`):

- **json**: `MarshalWithProvider` で `data` をシリアライズする。
- **svg / png / jpeg**: `template.Run(ctx, pc)` で SVG を生成し、`render.Apply` の装飾パイプライン (octicon 置換 → 画像の data URI インライン化 → 任意の CSS purge / XML 整形) を通す。
  - **svg**: 高さは生成時に確定済みのため、そのまま返す (ラスタライザは呼ばない)。
  - **png / jpeg**: 確定済み SVG を `render.Renderer.Resize` (既定は resvg) でラスタライズする。

レンダリングの詳細 (native SVG の高さ確定、resvg、装飾ステージ) は [`rendering.md`](rendering_ja.md) を参照。

## 3. プラグインシステム

### 3.1 インタフェースと登録

各プラグインは `internal/plugins/<name>/` に閉じたサブパッケージで、`init()` から `plugins.Register` を呼んで自己登録する (`internal/plugins/plugin.go` の `map[string]Plugin` レジストリ、重複名は panic)。動的ロードは行わず、コンパイル時に全プラグインが決まる。

```go
type Plugin interface {
    Name() string
    Requires() []DataKey
    Run(ctx context.Context, pc *PluginContext) (any, error)
}
```

- `Run` の戻り値 `any` がそのプラグインの結果で、`data.Plugins[name]` に格納される。他プラグインは `PluginContext.Imports.Get(name)` で参照できる。
- `Requires()` は依存する共有データキーを申告する (dataprovider の取得計画に使われる)。
- `metadata.yml` は `assets/plugins/<name>/metadata.yml` として `//go:embed` でバンドルされ、入力定義 / `supports` (対応アカウント種別) / 必要スコープを持つ。

### 3.2 特殊プラグイン

- **`core`**: データプラグインではなく、Stage 1 の設定注入と Stage 2 の並列ランナーを担うオーケストレーション用プラグイン (`internal/plugins/core`)。
- **`base`**: activity+community / repositories のサマリーパネルを描く opt-in プラグイン。`dataprovider` からのみ読む。
- `internal/plugins/pluginutil` と `internal/plugins/requirestesting` はプラグインではなく共有ヘルパ。

採用プラグインの一覧と分類は [`scope.md`](scope_ja.md) を参照。

## 4. テンプレート

登録テンプレートは **`classic`** (user / organization) と **`repository`** (単一リポジトリ) の 2 つのみ。`terminal` / `markdown` は不採用 ([`scope.md`](scope_ja.md#31-web-インスタンス-旧-m5))。

```go
type Template interface {
    Name() string
    Metadata() *config.TemplateMetadata
    FS() fs.FS
    Check(q map[string]any, account, format string) error
    Run(ctx context.Context, pc *PartialContext) (string, error)
}
```

`Run` は partial 群を縦に積み上げて 1 枚の SVG 文字列を返す。partial のシグネチャは高さ自己申告付きの `(markup string, height int, err error)` で、詳細は [`rendering.md`](rendering_ja.md#2-partial-と高さの確定) にある。

> `internal/templates/chrome` は **ブラウザとは無関係**の共有パッケージで、カードの枠 (footer / base セクション / CSS ローダ / `chrome_*` セクションゲート) を `classic` と `repository` に提供する。`chrome_*` 入力の「chrome」も UI 用語である。

## 5. パッケージ構成

```
cmd/metrics-cli/       … 単一エントリポイント (Action / CLI)
internal/
├── action/            … 入力パース (ParseInputs / CLIFlags)、commit / PR / data-changed
├── engine/            … Compute オーケストレータ + dispatch
├── dataprovider/      … 共有データの遅延 / メモ化取得 (#603)
├── config/            … settings / metadata / 入力正規化
├── githubapi/         … REST (httpx) + GraphQL (genqlient) クライアント + scope 検出
├── plugins/           … 各プラグイン (core / base / header / 19 データプラグイン)
├── templates/         … classic / repository + 共有 chrome パッケージ
├── render/            … SVG 装飾パイプライン + resvg ラスタライズ + fontmetrics
├── format/            … 数値 / 日付フォーマッタ
└── ...                … logger / errors / httpx など
assets/                … //go:embed するプラグイン / テンプレート metadata / CSS / フォント
tests/                 … golden / compliance / integration
```

`internal/` が Go の可視性境界であり、公開 API (`pkg/`) は持たない。公開面は Action / CLI バイナリのみ。
