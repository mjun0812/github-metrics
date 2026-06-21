# 09. 設定・metadata 仕様

## 目次

- [1. settings.json (Web インスタンス)](#1-settingsjson-web-インスタンス)
- [2. action.yml inputs (Action / CLI)](#2-actionyml-inputs-action--cli)
- [3. 入力解決の優先順位](#3-入力解決の優先順位)
- [4. metadata.yml ローダ](#4-metadatayml-ローダ)
- [5. presets ローダ](#5-presets-ローダ)
- [6. 値のマスキング・サニタイズ](#6-値のマスキングサニタイズ)
- [7. JSON Schema 提供](#7-json-schema-提供)

---

## 1. settings.json (Web インスタンス)

### 1.1 場所と検索順

- リポジトリルートの `settings.json` を `os.ReadFile` で読む。
- 存在しない場合は欠損として扱い、`settings={ Port: 3000 }` のみで起動する。
- `sandbox=true` ではファイル読み込みをスキップし、メモリ上で defaults を使う。

### 1.2 スキーマ

Go 表現:

```go
type Settings struct {
    Token       string                          `json:"token"`
    Modes       []string                        `json:"modes"`         // ["embed", "insights"]
    Restricted  []string                        `json:"restricted"`    // allow-list
    MaxUsers    int                             `json:"maxusers"`      // 0=unlimited
    Cached      int                             `json:"cached"`        // ms
    Ratelimiter *Ratelimiter                    `json:"ratelimiter"`
    Port        int                             `json:"port"`
    Optimize    OptimizeFlag                    `json:"optimize"`      // bool | [css|xml|svg]
    Debug       bool                            `json:"debug"`
    DebugHeadless bool                          `json:"debug.headless"`
    Mocked      MockFlag                        `json:"mocked"`        // bool | "force"
    Repositories int                            `json:"repositories"`
    Padding     []string                        `json:"padding"`       // ["0","8 + 11%"]
    Outputs     []string                        `json:"outputs"`
    Hosted      struct{ By, Link string }       `json:"hosted"`
    OAuth       *OAuth                          `json:"oauth"`
    API         APISettings                     `json:"api"`
    Control     struct{ Token string }          `json:"control"`
    Community   struct{ Templates []string }    `json:"community"`
    Templates   struct{ Default string; Enabled []string } `json:"templates"`
    Extras      Extras                          `json:"extras"`
    PluginsDefault bool                          `json:"plugins.default"`
    Plugins     map[string]PluginSettings        `json:"plugins"`
    // sandbox / sandbox 自動付与
    Sandbox bool `json:"-"`
    Web *WebSettings `json:"web,omitempty"`
}

type PluginSettings struct {
    Enabled bool   `json:"enabled"`
    Extra   map[string]any `json:"-"` // token などフラットに格納される
}

type Ratelimiter struct {
    Max      int           `json:"max"`
    WindowMs time.Duration `json:"windowMs"`
    // express-rate-limit 互換のフィールドを保持
}

type Extras struct {
    Default bool         `json:"default"`
    Features any         `json:"features"` // false | true | []string
    Logged   []string    `json:"logged"`
}
```

> JSON 内の `//` キー (`settings.example.json` 参照) は **コメント** として扱う。Go 版は `json5` を使うか、`json.RawMessage` 解析時に `//` キーをフィルタする。

### 1.3 主要キーの意味

| キー | 意味 |
|------|------|
| `token` | GitHub PAT (必須)。`"NOT_NEEDED"` でトークン不要モード |
| `modes` | 有効モード: `embed`, `insights` |
| `restricted` | 許可ユーザー (`/<login>` 経由のリクエストはこのリストに含まれる場合のみ受理) |
| `maxusers` | 同時 cache 対象上限 |
| `cached` | レンダリングキャッシュ TTL (ms) |
| `ratelimiter` | `express-rate-limit` 互換 (Go では go-chi/httprate に変換) |
| `port` | listen ポート |
| `optimize` | `true` / `["css","xml","svg"]` |
| `outputs` | 受理する出力フォーマット |
| `hosted.by/link` | フッターに表示 |
| `oauth` | OAuth クライアント設定 |
| `api.rest/graphql` | GitHub Enterprise 用カスタム URL |
| `control.token` | `/.control/stop` で要求するトークン |
| `community.templates` | コミュニティテンプレート設定 |
| `templates.default/enabled` | デフォルト/許可リスト |
| `extras.default/features/logged` | 拡張機能フラグ |
| `plugins.default` | デフォルトで全プラグインを有効化するか |
| `plugins[name]` | 個別 plugin 設定 (`enabled`, `*.token` 等のシークレット) |

### 1.4 extras features 一覧

| key | 意味 |
|------|------|
| `metrics.setup.community.templates` | コミュニティテンプレートの動的取得を許可 |
| `metrics.setup.community.presets` | コミュニティプリセットの利用を許可 |
| `metrics.api.github.overuse` | GitHub API 大量利用 |
| `metrics.api.*` | 外部 API 呼び出し |
| `metrics.cpu.overuse` | CPU 集中処理 |
| `metrics.run.tempdir` | 一時ディレクトリ書込 |
| `metrics.run.git` | git 実行 |
| `metrics.run.licensed` | `licensed` バイナリ実行 (廃止予定) |
| `metrics.run.user.cmd` | 任意コマンド実行 (非推奨) |
| `metrics.run.puppeteer.scrapping` | chromedp スクレイピング |
| `metrics.run.puppeteer.user.css` | ユーザー CSS の chromedp 適用 |
| `metrics.run.puppeteer.user.js` | ユーザー JS の chromedp 実行 |
| `metrics.npm.optional.*` | optional dependency (Go 版では不要) |

## 2. action.yml inputs (Action / CLI)

### 2.1 構造

- `inputs:` 配下に `{key: {description, default}}` を持つ。
- すべての値は `core.metadata.yml` または `<plugin>/metadata.yml` の入力定義から **生成** されている (`.github/scripts/build.mjs`)。

### 2.2 Go 側の取り扱い

- ビルド時に `assets/plugins/*/metadata.yml` を `embed.FS` 化。
- `config.MetadataLoader` で 1 回ロードし、`Inputs(): []InputDef` を `metadata.plugins.core` 含めて取得。
- 各 plugin の `Inputs.Action(env, preset)` は `INPUT_<UPPER>` を読み、preset を合成して `map[string]any` を返す。
- Action 入力名 (`plugin_languages_indepth`) と Web 入力名 (`plugin_languages.indepth`) の相互変換は `metadata.to.Action(key)`, `metadata.to.Web(key)` に集約。

### 2.3 自動生成

`action.yml` 自体を将来更新可能にするため、Go 版に `metrics-action gen action-yml > action.yml` のサブコマンドを用意。Node 版 `.github/scripts/build.mjs` 相当の役割。

## 3. 入力解決の優先順位

入力値は以下の優先で決まる:

1. **`q` (URL クエリ / Action 入力)** … ユーザーが明示した値
2. **`preset` (config_presets で展開された値)**
3. **`settings.json` の `plugins.<name>.<key>` (global=true の入力のみ)**
4. **`metadata.yml` の `default`**

`global: yes` の入力に限り、3 が優先になる (Web インスタンスで管理者が固定する用途)。

### 3.1 型変換規則

| `type:` | 規則 |
|---------|------|
| `boolean` | `"yes"/"true"/"1"` → true。それ以外 → false。string で来た場合は trim・lower-case |
| `number` | `strconv.ParseFloat`、`min`/`max` を clamp、NaN は default |
| `string` | trim |
| `array` | `format` に従い split。`values:` でホワイトリスト(順序維持) |
| `json` | `json.Unmarshal`。失敗時は default |
| `token` | string (秘匿)。空 → 未指定 |

### 3.2 動的プレースホルダ

- `default: .user.login` のように `.path` 形式の値は data の field path を参照する。
- `default: .user.twitter` など。
- 解決順は dataprovider 実行後の `data.User` を参照する。

## 4. metadata.yml ローダ

```go
// internal/config/metadata.go

type MetadataLoader struct {
    Plugins   map[string]*PluginMetadata
    Templates map[string]*TemplateMetadata
    Package   *PackageMetadata
    Action    *ActionMetadata
    Env       struct{ GHActions bool }
}

func Load(fsys fs.FS) (*MetadataLoader, error)
```

- `assets/plugins/<name>/metadata.yml` を回って `*PluginMetadata` を組み立てる。
- `assets/templates/<name>/metadata.yml` 同上。
- `package.json` 相当のバージョン情報は `assets/version.txt` に置く (`go generate` で git tag から生成可能)。
- `action.yml` のパースは `gopkg.in/yaml.v3` で `Inputs` セクションだけ抽出。

### 4.1 inputs() 関数の Go 表現

```go
// Inputs はプラグイン入力定義集合
type Inputs struct {
    defs map[string]InputDef
}

func (i *Inputs) ForAction(env map[string]string, preset map[string]any) map[string]any
func (i *Inputs) ForWeb(q map[string]string) map[string]any
func (i *Inputs) ForData(data *Data, q Query, account string) map[string]any
```

ForData は dataprovider 実行後の動的プレースホルダ (`.user.login` 等) を解決する。

### 4.2 extras 解決

`PluginMetadata.Extras(name string, settings *Settings) bool` で、`features` が `true` か `[]string` で名前を含むかを判定。`error=false` モードを設けると features 不足時に panic せず false 返却で fallback できる (オプション)。

## 5. presets ローダ

### 5.1 入力

- `config_presets` (Action) / `config.presets` (Web) は **カンマ区切り**または改行区切り。
- 各要素は次のいずれか:

| 形式 | 例 | 解釈 |
|------|-----|------|
| `@<name>` | `@languages` | GitHub raw に組込み preset を取りに行く |
| `https://...` | `https://gist.github.com/user/abc.yml` | URL fetch |
| `local/path.yml` | `examples/foo.yml` | ローカルファイル(Action 環境時のみ) |

### 5.2 スキーマ

```yaml
schema: v1
with:
  plugin_languages: yes
  plugin_languages_indepth: yes
  plugin_languages_limit: 0
```

`with` 配下が `INPUT_*` 互換のキーで `q` に展開される。`metadata.yml` 中 `preset: no` の入力は許可しない。完全な制約と取り込み形式は [13-appendix.md §K](./13-appendix.md#k-presets-yaml-スキーマ-v1-例)。

### 5.3 Go 実装

```go
func LoadPresets(ctx, list string, meta *MetadataLoader, fetch HTTPGetter) (map[string]any, error)
```

- list を split → 各要素を URL/ローカル判定。
- ローカル/URL の YAML を読み、`schema: v1` だけ採用 (将来バージョン拡張のため)。
- `Allowed` set は metadata の入力名から `token` 型/`preset: no` を除外したものに限る。
- 値は `q` と同じ key 命名に正規化 (`metadata.to.Query(key)`)。

## 6. 値のマスキング・サニタイズ

- ログ出力時、`type: token` 入力は `(provided)` / `(missing)` / `(MOCKED TOKEN)` / `(NOT NEEDED)` に置き換える。マスク規則と整形ルールは [13-appendix.md §E](./13-appendix.md#e-action-起動バナーの整形ルール-info-互換)。
- `q` をデバッグ出力するときも token 値はマスクする。
- `INPUTS` JSON を保持する `os.Setenv` 経由の値はそのまま使う(GitHub Actions が外向き log で自動マスクする)。
- 入力値のうち `format=newline-separated` の `commits_authoring`, `users_ignored`, `repositories_skipped` は **配列**として保持。
- ファイル書き出し時には正規化されたパス (`filepath.Clean`) を使い、`..` を含む path traversal を拒否する。

## 7. JSON Schema 提供

互換性検証と外部ツール連携のため、以下を `api/` ディレクトリに JSON Schema として公開する:

| ファイル | 内容 |
|--------|------|
| `api/settings.schema.json` | settings.json のスキーマ |
| `api/action.schema.json` | action.yml inputs のスキーマ (各 `metadata.yml` から生成) |
| `api/insights.schema.json` | Insights API レスポンス |

Go では `embed.FS` 化し、`metrics-server` の `/.schema/settings` 等で配信できるようにする。
