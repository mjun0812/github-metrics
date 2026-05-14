# Contract: Template Interface + PartialFunc

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/templates` が公開する Template 契約を定義する。M1 段階では interface と registry のみを実装し、`classic` / `repository` の本体は M2 (T-023..028) / M7 (T-089) で積む。

## 1. `Template` インタフェース

```go
package templates

import (
    "context"
    "io/fs"
)

type Template interface {
    // Name returns the canonical template identifier matching the directory
    // under assets/templates/<name>/.
    Name() string

    // Metadata returns the parsed metadata.yml for this template.
    Metadata() *TemplateMetadata

    // FS returns the embed.FS rooted at assets/templates/<name>/ for
    // accessing partials, image.svg skeleton, and metadata.yml.
    FS() fs.FS

    // Check validates that the rendering request is compatible with this
    // template. Returns *UnsupportedFormatError when format is not supported,
    // or *InputError when account/repository requirements are not met.
    Check(q map[string]any, account string, format string) error

    // Run executes the template's render pipeline. In M1 the implementation
    // is a no-op stub returning an empty string. M2 onward populates this.
    Run(ctx context.Context, pc *PartialContext) (string, error)
}
```

### 1.1 不変条件

- `Name()` の戻り値は `metadata.yml` の `name:` と完全一致。
- `Check` は `q` を破壊しない (read-only)。
- `Run` の戻り値文字列は M1 段階では空文字 (no-op)。M2 で SVG 文字列を返すよう拡張。

## 2. `PartialFunc` / `PartialContext`

```go
type PartialFunc func(ctx context.Context, pc *PartialContext) (string, error)

type PartialContext struct {
    Ctx        context.Context
    Settings   *config.Settings
    Inputs     map[string]any
    Logger     *slog.Logger
    Data       *Data
    Metadata   *config.MetadataLoader
    Helpers    *render.Helpers // M3 で追加。M1 では nil 許容。
}
```

- 各 partial 関数は **冪等** であること: 同じ `Data` を渡せば同じ string を返す。
- 不在キー (例: 採用していない plugin の結果) を参照したときは **panic せず空文字列** を返すこと (T-024 Acceptance criterion)。

## 3. レジストリ

```go
var (
    registry   = make(map[string]Template)
    registryMu sync.RWMutex
)

// Register registers a template. Must be called from init() of the template
// package. Duplicate names cause panic.
func Register(t Template)

// Get returns the registered template by name. Returns (nil, false) if absent.
func Get(name string) (Template, bool)

// MustGet returns the template or returns *NotFoundError.
func MustGet(name string) (Template, error)
```

### 3.1 不変条件

- 二重登録は panic。
- `MustGet` は `engine.Compute` から呼ばれる (FR-026)。未知 template は `*errors.NotFoundError` を返す。

## 4. `Check` の判定規則

- `metadata.yml` の `formats:` に該当 format が含まれない → `*UnsupportedFormatError`。
- `account == "repository"` だが template が repository 向けでない (例: `classic`) → `*InputError{Field: "account"}`。M7 (T-089) の `repository` template で 406 相当を返す。
- `account != "repository"` で `q["repo"]` がセットされている → 上流互換のため warn ログのみ。エラーにはしない。

## 5. テスト契約

- M1 段階では `templates.Template` の **空実装** を持つ `noopTemplate` を `internal/templates/template_test.go` 内で定義し、registry テストに用いる。
- M2 (T-023) で `classic` 本体を実装する際は、`assets/templates/classic/partials/_.json` (N-001 で MVP 用に絞られたリスト) を `Run` 中で読み込み、各 partial を順次呼び出す。
- 各 partial の戻り値は SVG/Markdown DOM 構造を golden file 比較で検証 (constitution 原則 II / IV)。

## 6. 採用範囲外テンプレートの追加禁止 (NON-NEGOTIABLE)

`docs/design/15-selection-answer.md` §6.3 で採用された template は `classic` / `repository` のみ。`terminal`, `markdown`, `community` の登録は MUST NOT。違反は constitution 原則 III 違反。
