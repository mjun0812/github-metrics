# 05. プラグインシステム仕様

## 目次

- [1. 設計目標](#1-設計目標)
- [2. プラグインインタフェース](#2-プラグインインタフェース)
- [3. metadata.yml の解釈](#3-metadatayml-の解釈)
- [4. プラグイン登録 (Registry)](#4-プラグイン登録-registry)
- [5. base プラグインの特殊扱い](#5-base-プラグインの特殊扱い)
- [6. core プラグインの役割](#6-core-プラグインの役割)
- [7. 並列実行とエラー集約](#7-並列実行とエラー集約)
- [8. プラグイン入力解決の優先順位](#8-プラグイン入力解決の優先順位)
- [9. community プラグイン (将来拡張)](#9-community-プラグイン-将来拡張)

---

## 1. 設計目標

- プラグインは **独立したサブパッケージ** として実装し、`internal/plugins/<name>/` 配下に閉じる。
- 各プラグインは `Plugin` インタフェースを実装することで、エンジン側からは均一に扱える。
- ロード時のオーバーヘッドを抑えるため、コード時点で全プラグインを `Register()` する (動的 import は行わない)。
- `assets/plugins/<name>/{metadata.yml, queries/*.graphql, examples.yml}` を `//go:embed` でバンドルする。

## 2. プラグインインタフェース

```go
// internal/plugins/plugin.go
package plugins

import "context"

type Plugin interface {
    // 名前は metadata.yml のファイルディレクトリ名と一致 (例: "languages")
    Name() string
    // Metadata はディレクトリの metadata.yml に基づくスキーマ。
    Metadata() *config.PluginMetadata
    // Run は plugin の主処理。data.Plugins[name] へ書き込まずに、戻り値を返す。
    Run(ctx context.Context, p *PluginContext) (any, error)
}
```

`PluginContext` は [01-architecture.md §4](./01-architecture.md#4-主要データ型) の型を流用する。

### 2.1 RunResult の型

各プラグインの結果型はプラグイン固有で問題ない (engine 側は `any` として `data.Plugins[name]` に格納)。命名規約は

```go
package languages

type Result struct {
    Unique   int                 `json:"unique"`
    Sections []string            `json:"sections"`
    Details  []string            `json:"details"`
    Colors   map[string]string   `json:"colors"`
    Total    int64               `json:"total"`
    Stats    map[string]int64    `json:"stats"`
    Recent   *Recent             `json:"stats.recent,omitempty"`
    Lines    map[string]int      `json:"lines,omitempty"`
}
```

JSON 出力時のキーは Node 版と同じになるように `json:` タグを設定する。

### 2.2 エラー戻り値

- 戻り値 `error` を返した場合、`data.Plugins[name] = error` として扱う。
- `error` 自体は文字列化して `data.Errors` に積む。
- `q["plugins.errors.fatal"] == true` の場合は最初の non-nil で全体を panic させる。

### 2.3 サポートされるアカウント種別

`metadata.supports = [user, organization, repository]` を Go 側でも検証する。違反したら `error`(ErrUnsupportedAccount) を返す。

## 3. metadata.yml の解釈

### 3.1 構造

```yaml
name: 🗃️ Base content
category: core            # core | github | social | community
description: ...
deprecated: false         # optional
icon: 🌐                  # optional, README 用
examples:
  default1: <url>
supports:
  - user
  - organization
  - repository
scopes:
  - public_access
inputs:
  <key>:
    description: ...
    type: boolean | number | string | array | json | token
    default: ...
    min: 0                # number 専用
    max: 100              # number 専用
    values: [a, b, c]     # array 専用 (許可値)
    format: comma-separated | space-separated | newline-separated | [複数]
    example: ...
    global: yes           # settings から上書きできる入力
    preset: no            # preset で指定不可
    extras:               # extras 機能フラグでガード
      - metrics.api.github.overuse
```

### 3.2 Go 表現

```go
type PluginMetadata struct {
    Name        string
    Category    string
    Description string
    Deprecated  bool
    Icon        string
    Examples    map[string]string
    Supports    []string
    Scopes      []string
    Inputs      map[string]InputDef
    Extras      map[string]any  // 追加カスタム
}

type InputDef struct {
    Description string
    Type        InputType        // boolean / number / string / array / json / token
    Default     any
    Min, Max    *float64
    Values      []string
    Format      []string
    Example     string
    Global      bool
    Preset      bool
    Extras      []string
}
```

### 3.3 値解決

- 入力解決順序は [09-configuration.md §3](./09-configuration.md#3-入力解決の優先順位) を参照。
- 値の正規化 (`yes`/`no`/`true`/`false`、配列の split) は `config.NormalizeInput(def, raw)` で統一実装する。
- boolean キャストの正規表現は [13-appendix.md §F](./13-appendix.md#f-入力正規化ルール一覧-legacyconverter-互換) を参照。
- `.user.login` 等の動的プレースホルダは `Apply(data)` で置換する。

## 4. プラグイン登録 (Registry)

### 4.1 init() による登録

```go
// internal/plugins/registry.go

var registry = map[string]Plugin{}

func Register(p Plugin) {
    if _, ok := registry[p.Name()]; ok { panic("dup: " + p.Name()) }
    registry[p.Name()] = p
}

func Get(name string) (Plugin, bool) { … }

func Each(f func(Plugin)) { … }

// プラグイン側
// internal/plugins/languages/languages.go
package languages

func init() { plugins.Register(&Plugin{}) }
```

### 4.2 順序保証

- 実行順は `data.Partials` (template `partials/_.json`) と `q["config.order"]` の合成順に従う。
- registry は探索のため、順序保証はしない。

### 4.3 base / core の固定登録

- `internal/plugins/base` と `internal/plugins/core` は `internal/engine` から直接 import して呼び出す (registry でも引けるが、固定参照を持つ)。

## 5. dataprovider / header プラグインの特殊扱い

> NOTE: PR #601–#614 リファクタで、旧 `base` プラグインは `dataprovider` (データ取得、常時実行) と `header` (プロフィールカード表示、opt-in) に分割されました。以下は分割後の説明です。

### 5.1 dataprovider の役割

- `data.User`, `data.User.Calendar`, `data.User.ContributionsCollection`, `data.User.Repositories` 等の **共通データ** を取得する。
- 他プラグインはこのデータを再利用する (重複クエリの回避)。

### 5.2 dataprovider の動作

1. 入力 `plugin_header_indepth`, `plugin_header_hireable`, `repositories.{forks,affiliations,batch}` 等を取得。
2. アカウント種別ループ (`user`, `organization`) で bulk クエリ → 失敗時に field 単位 fallback。
3. `plugin_header_indepth=true` のとき、過去アカウントライフタイム全体に対する `contributionsCollection.*` を 4 週間単位で集計し、`search.commits(author:<login>)` で全期間の commits を補正。
4. リポジトリ詳細を `repositories.batch` 件ずつページング (timeout 時は batch を半減してリトライ)。
5. `postprocess.user(...)` / `postprocess.organization(...)` で派生フィールド (計算済 commits, license aggregation) を埋める。
6. `token=NOT_NEEDED` の場合は `postprocess.skip` のみ実行して終了。

具体的なアルゴリズム (擬似コード) と fallback フィールド一覧、`postprocess.skip` の初期値は [13-appendix.md §B](./13-appendix.md#b-base-プラグインの取得アルゴリズム-擬似コード) を参照。
GraphQL クエリ全文は [13-appendix.md §A](./13-appendix.md#a-base-プラグインの-graphql-クエリ全文) を参照。

### 5.3 出力 (data 直接書き換え)

```go
type Data struct {
    Base BaseData                 // sections enable map
    User UserData                 // GraphQL user / organization 結果
    Account string                // "user" | "organization" | "repository"
    // 他は他プラグインが書き込む
}

type UserData struct {
    Login string
    Name  string
    AvatarURL string
    CreatedAt time.Time
    Calendar  Calendar
    ContributionsCollection Contributions
    Repositories Repositories
    Packages Counter
    StarredRepositories Counter
    Watching Counter
    SponsorshipsAsSponsor Counter
    SponsorshipsAsMaintainer Counter
    Followers Counter
    Following Counter
    IssueComments Counter
    Organizations OrgList
    // organization 専用
    MembersWithRole *Counter
    // ... (詳細は 08-external-services.md / 06-plugins-detail.md)
}
```

### 5.4 リポジトリ取得

- `queries.base.repositories(login, after, batch)` を「pageInfo.hasNextPage が false になるまで」回す。
- フィルタ:
  - `repositories_forks=false` → `isFork: false`
  - `repositories_affiliations` → `ownerAffiliations`
  - `repositories_skipped` → 配列のうち owner/name でマッチするものを除外
- 取得数は `settings.repositories` (デフォルト 100)。

## 6. core プラグインの役割

`core` は **テンプレート起動直後** に呼ばれ、global 設定を `data` に注入し、各プラグインを `pending` (goroutine 群) に積む。

### 6.1 入力

| 入力 | 用途 |
|------|------|
| `config.animations` | `data.Animated` |
| `config.display` | `data.Large` / `data.Columns` (`large` / `columns`) |
| `config.timezone` | `data.Config.Timezone` |
| `config.base64` | `false` のとき `imgb64` をパススルー化 |
| `debug.flags` | デバッグフラグ集合 |

### 6.2 タイムゾーン処理

- `_timezone` をベースに、`time.LoadLocation(name)` でロード。
- 失敗時は `error="Failed to use timezone \"<name>\""` を `data.Config.Timezone` にセット。

### 6.3 plugin pending 投入

```go
for _, name := range registry.Names() {
    if !q.Bool(name) { continue }
    pending.Go(func() error {
        result, err := registry[name].Run(ctx, pctx)
        if err != nil { data.Plugins[name] = err; ... }
        else { data.Plugins[name] = result }
        callbacks.Plugin(login, name, err == nil, data.Plugins[name])
        return nil
    })
}
```

Go では `golang.org/x/sync/errgroup` を採用 (`Promise.all` 相当)。

### 6.4 `metadata.plugins.core.extras(name, settings)` 相当

- core plugin に同居する `extras()` 関数で `settings.extras.features` (または `extras.default`) を判定し、各プラグイン側で features をガードする。
- Go では `config.IsExtraEnabled(name, settings)` を提供する。

## 7. 並列実行とエラー集約

### 7.1 errgroup

- 並列度は `runtime.GOMAXPROCS(0)` を上限とした `errgroup.SetLimit` で制御。`settings.parallel` で上書き可能 (新規設定)。
- パニックは `recover()` で error に変換し、`data.Errors` に追加する。

### 7.2 エラー型

```go
type PluginError struct {
    Plugin string
    Err    error
    Type   string  // "user" | "github" | "internal"
    Message string
}
```

- `die=true` のとき最初に発生した error を `engine.Compute` の戻り値とする。
- `die=false` のとき footer に出力する。
- カテゴリ判定 (`user` / `github` / `template` / `unsupported` / `internal`) は [13-appendix.md §J](./13-appendix.md#j-プラグインエラーの分類規則) を参照。

### 7.3 callbacks

- `callbacks.Plugin(login, plugin, success, result)` が登録されていれば呼ぶ。
- insights モードでは plugin 完了ごとにキャッシュへ put する。

## 8. プラグイン入力解決の優先順位

`metadata.plugins.<name>.inputs(...)` の挙動を Go で再現する。優先順位は次の通り。

1. `q["<plugin>.<key>"]` (URL クエリ / `--plugin` フラグ)
2. preset で展開された値
3. `metadata.yml` の `default`

`global: yes` の入力は web インスタンスで利用者ではなくサーバ側 `settings.json` から値を取り、`q` を上書きする。

`type: token` の入力は `q` には乗らず、`conf.settings.plugins[name][key]` から取り出す。

## 9. community プラグイン (将来拡張)

- 既存実装 (Node 版) はディレクトリ単位の動的ロードで community プラグインを取り込んでいた。
- Go 版で動的ロードを行うと、Plug-in 安全性 (`go plugin` 制約、クロスコンパイル不可) の問題があるため初版では **見送り**。
- 代替案:
  - `community/<name>/` を `internal/plugins/community/` に同梱して built-in 化。
  - WASM プラグインを `tetratelabs/wazero` で実行する将来案を保留。
- 当面、利用頻度の高い `crypto`, `nightscout`, `stock`, `chess`, `splatoon`, `fortune`, `poopmap`, `screenshot`, `16personalities` を built-in に移植する (詳細は [06-plugins-detail.md](./06-plugins-detail.md))。
