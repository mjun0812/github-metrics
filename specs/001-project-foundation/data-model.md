# Phase 1 Data Model: プロジェクト土台 (M1 19 タスク)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は spec の Key Entities を Go の型表現と検証規則に落とし込む。型名・フィールド名は constitution 原則 V に従い英語のままとする。日本語説明は entity の意図 / 不変条件 / state transition の解説に限定する。

---

## E-001: `Settings` (config package)

**役割**: 上流 `settings.json` と互換のグローバル設定。Web インスタンスを実装しなくてもキー集合は保持する (原則 I)。

**定義**:

```go
type Settings struct {
    Token         string                       `json:"token"`
    Modes         []string                     `json:"modes"`
    Restricted    []string                     `json:"restricted"`
    MaxUsers      int                          `json:"maxusers"`
    Cached        int                          `json:"cached"` // ms
    Ratelimiter   *Ratelimiter                 `json:"ratelimiter"`
    Port          int                          `json:"port"`
    Optimize      OptimizeFlag                 `json:"optimize"`
    Debug         bool                         `json:"debug"`
    DebugHeadless bool                         `json:"debug.headless"`
    Mocked        MockFlag                     `json:"mocked"`
    Repositories  int                          `json:"repositories"`
    Padding       []string                     `json:"padding"`
    Outputs       []string                     `json:"outputs"`
    Hosted        Hosted                       `json:"hosted"`
    OAuth         *OAuth                       `json:"oauth"`
    API           APISettings                  `json:"api"`
    Control       Control                      `json:"control"`
    Community     CommunitySettings            `json:"community"`
    Templates     TemplatesSettings            `json:"templates"`
    Extras        Extras                       `json:"extras"`
    PluginsDefault bool                        `json:"plugins.default"`
    Plugins       map[string]PluginSettings    `json:"plugins"`
    Sandbox       bool                         `json:"-"`
    Web           *WebSettings                 `json:"web,omitempty"`
}

type Ratelimiter struct {
    Max      int           `json:"max"`
    WindowMs time.Duration `json:"windowMs"`
}

type PluginSettings struct {
    Enabled bool           `json:"enabled"`
    Extra   map[string]any `json:"-"` // token などフラットに格納される
}
```

**不変条件**:

- `Port == 0` ならローダが `3000` を補完する。
- `Sandbox == true` で `Optimize=true / Cached=0 / PluginsDefault=true / Extras.Default=true / Mocked=true` を強制設定する。
- `Token` が空でも error にしない。token validation (T-108) は M6 で扱う。

**状態遷移**: 不変 (起動時に 1 回ロードして read-only)。

---

## E-002: `PluginMetadata` / `TemplateMetadata` / `ActionMetadata` / `PackageMetadata`

**役割**: `assets/**/metadata.yml` および `action.yml`, `assets/version.txt` の構造写像。`inputs` 定義をすべて保持。

**定義** (要部のみ):

```go
type PluginMetadata struct {
    Name        string                 `yaml:"name"`
    Category    string                 `yaml:"category"`
    Description string                 `yaml:"description"`
    Inputs      map[string]InputDef    `yaml:"inputs"`
    Scopes      []string               `yaml:"scopes"`
    Extras      map[string]bool        `yaml:"extras"`
    SupportedIn []string               `yaml:"supported_in"`
}

type TemplateMetadata struct {
    Name        string                 `yaml:"name"`
    Description string                 `yaml:"description"`
    Formats     []string               `yaml:"formats"`
    Inputs      map[string]InputDef    `yaml:"inputs"`
    Web         WebTemplateMeta        `yaml:"web"`
}

type InputDef struct {
    Type    string   `yaml:"type"`     // string | number | boolean | array | token | json
    Default any      `yaml:"default"`
    Format  string   `yaml:"format"`   // comma-separated | space-separated | newline-separated
    Values  []string `yaml:"values"`
    Global  bool     `yaml:"global"`
    Preset  bool     `yaml:"preset"`   // false なら preset 適用対象外
    Extras  []string `yaml:"extras"`   // 必須 extras フラグ
}

type ActionMetadata struct {
    Name        string                  `yaml:"name"`
    Description string                  `yaml:"description"`
    Inputs      map[string]InputDef     `yaml:"inputs"`
    Branding    Branding                `yaml:"branding"`
    Runs        Runs                    `yaml:"runs"`
}

type PackageMetadata struct {
    Version string // assets/version.txt
}
```

**検証規則**:

- `Inputs[name].Type` は許可リスト (`string`, `number`, `boolean`, `array`, `token`, `json`) のいずれかでなければならない。違反は **warn ログを出してスキップ** (Edge Case 「未知 input」)。
- `Inputs[name].Format` は `Type == "array"` の場合にのみ意味を持つ。それ以外で指定されていた場合は warn。
- 採用 21 plugin + base + core + classic/repository すべてのメタデータが embed.FS から欠落なくロードできること (FR-009, SC-003)。

---

## E-003: `MetadataLoader`

**役割**: `embed.FS` から複数 metadata を一括ロードし、key 変換テーブル (`to.Action`, `to.Web`, `to.Query`) を提供する集約 root。

**定義**:

```go
type MetadataLoader struct {
    Plugins   map[string]*PluginMetadata
    Templates map[string]*TemplateMetadata
    Action    *ActionMetadata
    Package   *PackageMetadata
}

func Load(fsys fs.FS) (*MetadataLoader, error)

func (m *MetadataLoader) ToAction(key string) string  // metadata.to.Action 相当
func (m *MetadataLoader) ToWeb(key string) string     // metadata.to.Web 相当
func (m *MetadataLoader) ToQuery(key string) string   // metadata.to.Query 相当
func (m *MetadataLoader) Extras(name string, s *Settings) bool
```

**不変条件**:

- ロード後の `Plugins` は採用 21 plugin + `base` + `core` のキーをすべて含む。
- ロード後の `Templates` は `classic` と `repository` のキーをすべて含む。

---

## E-004: `Inputs`

**役割**: action env / web query / data context から組み立てる正規化済み入力マップ。

**定義**:

```go
type Inputs struct {
    raw      map[string]any
    defs     map[string]InputDef
    resolver placeholderResolver
}

func (i *Inputs) ForAction(env map[string]string, preset map[string]any) (map[string]any, error)
func (i *Inputs) ForWeb(query url.Values) (map[string]any, error)
func (i *Inputs) ForData(data *Data, q map[string]any, account string) (map[string]any, error)

// 型ごとの正規化
func NormalizeInput(def InputDef, raw any) (any, error)

// 動的プレースホルダ解決: .user.login など
func resolvePlaceholders(s string, data *Data) (string, error)
```

**検証規則**:

- `def.Type == "token"` の値は `Token` 型 (E-007) として保持し、`String()` は `(provided)` を返す (FR-012)。
- `def.Type == "boolean"` の文字列キャスト規則は上流 `legacy-converter` と同一 (`"true" / "yes" / "1"` を true、他を false)。
- `def.Type == "array"` で `def.Format == "comma-separated"` の場合 `strings.Split(s, ",")` 後 trim/dedup。
- 未知 input は warn ログを残して **そのまま `raw` に格納** (Edge Case 対応)。後続コード側で `_, ok := i.raw[key]` で参照する。

---

## E-005: `Data`

**役割**: `engine.Compute` が組み立てる internal 状態。M1 段階では出力 (JSON/SVG) 化対象としては未使用。M2 以降の T-029 / T-023 系が消費する。

**定義** (要部):

```go
type Data struct {
    User     *User                    // GraphQL user / organization
    Account  AccountKind              // "user" | "organization" | "repository"
    Config   ComputedConfig           // core プラグインが populate
    Computed Computed                 // 集計値
    Plugins  map[string]any           // plugin 名 → 結果 / エラー
    Errors   []error                  // 致命的でないエラー集約
}

type User struct {
    Login                 string
    Name                  string
    CreatedAt             time.Time
    AvatarURL             string
    Repositories          *Connection[Repository]
    ContributionsCollection *ContributionsCollection
    Calendar              *Calendar
    // organization 用
    Packages              *Connection[Package]
    SponsorshipsAsSponsor *Connection[Sponsorship]
    SponsorshipsAsMaintainer *Connection[Sponsorship]
    MembersWithRole       *Connection[Member]
}

type Computed struct {
    Commits      int
    Repositories ComputedRepositories
}

type ComputedRepositories struct {
    Count        int
    Stargazers   int
    Forks        int
    Releases     int
    Watchers     int
    Issues       int
    PullRequests int
    Languages    map[string]int  // 言語名 → サイズ
}

type AccountKind string

const (
    AccountUser         AccountKind = "user"
    AccountOrganization AccountKind = "organization"
    AccountRepository   AccountKind = "repository"
)

type ComputedConfig struct {
    Timezone struct {
        Name  string
        Error error
    }
    Animations bool
    Display    string
    Base64     bool
}
```

**状態遷移**:

```
Empty Data → base.Run (FR-022) で User populate
           → base.repositories.Run (FR-023) で Computed.Repositories populate
           → core.Run (FR-024) で ComputedConfig populate (errgroup 開始前)
           → core.RunPlugins (FR-025) で Plugins[name] populate (並列)
           → 必要に応じて Errors 集約
```

**不変条件**:

- `die=true` 経路では `Errors` への蓄積を待たず即時 return。
- `Plugins[name]` は `any` 型だが、慣習として **成功時は struct ポインタ、失敗時は `error` 値** を入れる。M2 で型安全な container に置換予定 (リファクタ余地)。

---

## E-006: `PluginContext`

**役割**: plugin の `Run` 時に渡されるコンテキスト集約。

**定義**:

```go
type PluginContext struct {
    Ctx         context.Context
    Settings    *Settings
    Inputs      map[string]any
    Logger      *slog.Logger
    HTTPClient  *httpx.Client
    REST        *githubapi.REST
    GraphQL     *githubapi.GraphQL
    Data        *Data           // mutable
    Metadata    *MetadataLoader
    Imports     PluginImports   // 他 plugin の出力参照ヘルパ
}

type PluginImports interface {
    Get(name string) (any, bool)
}
```

**不変条件**:

- `Data` への書き込みは plugin の `Run` 内でのみ許可。`engine` 側は read-only に近い扱い (集約のみ)。
- `Logger` には `WithLogin(ctx, login)` が事前に注入されている前提。

---

## E-007: `Token` (不透明値)

**役割**: `type: token` の入力値を、ログ・エラー・JSON ダンプから秘匿する不透明型。

**定義**:

```go
type Token struct {
    raw string
}

func NewToken(raw string) Token
func (t Token) String() string         // "(provided)" or ""
func (t Token) Reveal() string         // raw 値 (githubapi/auth のみ呼ぶ)
func (t Token) MarshalJSON() ([]byte, error) // "(provided)"
func (t Token) GoString() string       // "(provided)"
```

**不変条件**:

- `String()` / `MarshalJSON()` / `GoString()` のいずれも raw 値を返さない (FR-012)。
- `Reveal()` の呼び出し点は `internal/githubapi/auth.go` および test helper に限定する (lint で違反検出する余地あり)。

---

## E-008: `Resources` (rate)

**役割**: GitHub API のレート残量トラッキング。

**定義**:

```go
type Resources struct {
    REST    Quota
    GraphQL Quota
    Search  Quota
    mu      sync.RWMutex
}

type Quota struct {
    Limit     int
    Used      int
    Remaining int
    Reset     time.Time
}

func (r *Resources) Refresh(ctx context.Context, c *REST) error
func (r *Resources) Snapshot() Resources
```

**不変条件**:

- 並行書き込み (Refresh) と読み込み (Snapshot) は `sync.RWMutex` で排他制御。race detector clean (R-012)。
- `Refresh` 失敗時は前回値を保持。

---

## E-009: Error types (`internal/errors`)

**役割**: 型付きエラー定義。

**定義**:

```go
type InputError struct {
    Field string
    Cause error
}

type NotFoundError struct {
    Resource string
    Cause    error
}

type ForbiddenError struct {
    Reason string
    Cause  error
}

type UnsupportedFormatError struct {
    Format string
    Cause  error
}

type RetryableError struct {
    Cause error
}

func (e *InputError) Error() string
func (e *InputError) Unwrap() error
// ... 他も同様
```

**不変条件**:

- `errors.As(err, &InputError{})` で型識別可能。
- `errors.Is(err, ErrSentinel)` 用の sentinel 変数も提供 (例: `var ErrInputInvalid = &InputError{}` — 比較は `errors.As` で行う)。

---

## エンティティ間の関係

```
Settings ──┐
            ├──> PluginContext ──> plugin.Run ──> Data.Plugins[name]
Inputs ────┤                          ↓
MetadataLoader                      REST/GraphQL ──> Resources (Refresh)
            └──> Data ──> Output (M2 以降の責務)
```

すべての entity は `internal/` 配下に閉じ、`cmd/` からは `engine.Compute` のみが触れるよう encapsulate する。
