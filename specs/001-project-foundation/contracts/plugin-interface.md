# Contract: Plugin Interface + Registry

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/plugins` が公開する Plugin 契約を定義する。M1 段階で `base` (T-017/018/020) と `core` (T-021/022) はこの契約に従い実装される。M4 (T-041..068) で 21 個の plugin を追加する際の不変条件としても機能する。

## 1. `Plugin` インタフェース

```go
package plugins

import (
    "context"
)

type Plugin interface {
    // Name returns the canonical plugin identifier matching the directory
    // under assets/plugins/<name>/. Must be stable across versions.
    Name() string

    // Metadata returns the parsed metadata.yml for this plugin.
    // The returned pointer must be the same instance across calls.
    Metadata() *PluginMetadata

    // Run executes the plugin against the given PluginContext.
    // On success, returns the plugin-specific result value.
    // On failure, returns a typed error (see internal/errors).
    // Run MUST NOT panic on input errors; it must convert them to *InputError.
    Run(ctx context.Context, pc *PluginContext) (any, error)
}
```

### 1.1 不変条件

- `Name()` は ASCII の小文字英数字 + ハイフンのみで構成。
- `Metadata()` は `init()` 内で 1 度だけ unmarshal され、同じポインタを返す。
- `Run` が `nil, nil` を返すことは許容 (例: extras 無効時のスキップ)。
- `Run` 内で `pc.Data` を書き込んだ場合、それは plugin の所有データに限定 (`pc.Data.Plugins[Name()]`)。他 plugin のフィールドへの書き込みは MUST NOT。

## 2. `PluginContext`

```go
type PluginContext struct {
    Ctx        context.Context
    Settings   *config.Settings
    Inputs     map[string]any
    Logger     *slog.Logger
    HTTPClient *httpx.Client
    REST       *githubapi.REST
    GraphQL    *githubapi.GraphQL
    Data       *Data
    Metadata   *config.MetadataLoader
    Imports    PluginImports
}

type PluginImports interface {
    Get(name string) (any, bool)
}
```

- `Imports` は他 plugin の結果を read-only で参照するためのアクセッサ。順序依存を緩める目的で `engine` 側が injection する。
- `Inputs` は **当該 plugin に紐づくキーのみ** に絞り込まれた状態で渡される (例: `plugin_languages_*` のみが `languages` plugin に届く)。

## 3. レジストリ

```go
var (
    registry        = make(map[string]Plugin)
    testRegistry    map[string]Plugin // RegisterForTest 用 ephemeral 上書き
    registryMu      sync.RWMutex
)

// Register registers a plugin. Must be called from init() of the plugin
// package. Duplicate names cause panic.
func Register(p Plugin)

// Get returns the registered plugin by name. Returns (nil, false) if absent.
func Get(name string) (Plugin, bool)

// Each iterates over registered plugins in name-sorted order.
func Each(fn func(name string, p Plugin) error) error

// RegisterForTest temporarily overrides registration during a test.
// Returns a cleanup function that t.Cleanup must invoke.
func RegisterForTest(t TB, p Plugin) (cleanup func())
```

### 3.1 不変条件

- 二重登録 (同じ `Name()`) は `Register` 時に panic (T-014 Acceptance criterion)。
- `RegisterForTest` で同名 plugin を一時的に差し替えた場合、`cleanup` 呼び出しで元の registration に戻す。`testRegistry` は test 関数のスコープでのみ有効。
- `Each` は決定論的順序 (`sort.Strings(keys)`) で iterate する。順序非決定によるテストフレーキ防止。

## 4. `core` plugin 契約 (特殊)

`core` plugin は他 plugin の **オーケストレータ** であり、通常 plugin の `Run` 後に呼ばれるのではなく、`engine.Compute` 中で base の後、partial 群の前に `RunPlugins` を呼び出される。

```go
// in internal/plugins/core
func RunPlugins(ctx context.Context, pc *PluginContext, parallel int) error
```

- `parallel <= 0` の場合は `runtime.GOMAXPROCS(0)` を採用 (Edge Case)。
- `errgroup.SetLimit(parallel)` で並列度を制限。
- 個別 plugin の `error` は `pc.Data.Plugins[name] = err` に記録し、戻り値の `error` には集約しない (集約は engine の責務)。
- 個別 plugin の panic は `recover()` で捕捉し、`pc.Data.Plugins[name] = &errors.RetryableError{Cause: ...}` に変換。

## 5. `base` plugin 契約 (特殊)

`base` plugin は engine から **最初に** 呼ばれ、`pc.Data.User` 等の共通データを populate する。

```go
// Name returns "base".
// Run dispatches by account type:
//   user         → base.RunUser
//   organization → base.RunOrganization
//   repository   → base.RunRepository (本 spec の M1 対象外、M7 で取り扱う)
```

### 5.1 user / organization fallback

- bulk クエリ `user.x.graphql` を試行。
- 502 / "Something went wrong" を含む errors を受けた場合、`errors` 配列から field 名を抽出し unit query (`user.<field>.graphql`) で個別取得。
- repositories ページングは `pageInfo.hasNextPage == false` か `settings.repositories` 件数に達するまでループ。
- ページタイムアウト時は batch 半減リトライ (T-020 Acceptance criterion)。

## 6. テスト契約

- 各 plugin の `*_test.go` は **mocked GraphQL/REST 経由でテーブルテスト** を 1 件以上持つ (FR-030)。
- `RegisterForTest` 経由で 3 つの fake plugin (success / error / panic) を投入する `core.RunPlugins` の `core_test.go` を M1 で実装する (User Story 4 Acceptance Scenario 2)。
- 二重登録テストは `defer recover() != nil` で panic 検出。

## 7. 採用範囲外プラグインの追加禁止 (NON-NEGOTIABLE)

`docs/design/15-selection-answer.md` §7 (community plugins, ソーシャル / 外部 API plugins, 未採用 GitHub plugins) に該当する plugin の `Register()` 呼び出しを `internal/plugins/<unadopted>/init.go` に追加することは MUST NOT。違反は constitution 原則 III 違反として PR を reject。
