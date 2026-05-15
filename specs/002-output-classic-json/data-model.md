# Phase 1 Data Model: classic + JSON 出力 (M2)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は M2 spec の Key Entities を Go の型表現と検証規則に落とし込む。型名・フィールド名は constitution 原則 V に従い英語のままとする。

## E-001: `engine.Result` (拡張)

**役割**: `engine.Compute` の戻り値。M1 で `Data` / `Errors` を持っていた。本 spec で `Output` / `MIME` を追加し、format 切替の結果をそのまま呼び出し側に流せるようにする。

**定義**:

```go
// internal/engine/engine.go
type Result struct {
    // Data is the populated internal state, unchanged from M1.
    Data *plugins.Data
    // Errors aggregates non-fatal plugin errors when Die == false (M1).
    Errors []error
    // Output is the serialized payload in the requested format.
    // Populated by Compute after the template / json stage runs.
    Output []byte
    // MIME is the IANA type matching Output:
    //   application/json   when Format == "json"
    //   image/svg+xml      when Format == "svg"
    //   image/png          when Format == "png"  (M2: contains SVG bytes
    //                                              with a warning; real
    //                                              PNG conversion lands
    //                                              with chromedp in M3)
    //   image/jpeg         when Format == "jpeg" (同上)
    MIME string
}
```

**不変条件**:

- `len(Output) > 0` whenever Compute returned without error
- `MIME` is one of the four IANA values above; never empty when Output is set
- Output bytes are valid for the declared MIME: `json.Valid(Output)` is true for `application/json`; `bytes.HasPrefix(Output, []byte("<svg"))` is true for `image/svg+xml`

## E-002: `engine.Marshal` (JSON 出力エンジン)

**役割**: `*plugins.Data` を上流互換キー集合の JSON にシリアライズする。循環参照は safety-net で潰す。

**定義**:

```go
// internal/engine/json.go
package engine

// Marshal serializes the populated Data structure into the upstream-
// compatible JSON shape. Cycles inside Data.Plugins are replaced with
// the literal string "[Circular]"; the call never panics on
// pathological inputs.
func Marshal(data *plugins.Data) ([]byte, error)

// cycleDetector walks an arbitrary value and returns a "JSON-safe"
// equivalent (cycles replaced, Go-specific collections normalized).
type cycleDetector struct {
    // seen tracks pointer addresses to detect re-entry.
    seen map[uintptr]struct{}
}

func newCycleDetector() *cycleDetector
func (c *cycleDetector) normalize(v any) any
```

**正規化規則** (R-003 + R-002):

- `nil`、`*T == nil`: そのまま `null`
- `map[string]any`: そのまま (キー順は `encoding/json` がソート)
- `map[K]V` (K != string): `[]struct{Key K; Value V}` の配列に変換、Key で安定ソート
- 集合的役割の `map[T]struct{}`: ソート済み `[]T`
- `[]T`: 各要素を normalize
- struct: フィールドごとに normalize、`json:"..."` タグを尊重
- `time.Time`: RFC 3339 string
- `error`: `{"error": err.Error()}` object
- `Token` (config.Token): `String()` 経由で `"(provided)"` または `""`
- 循環参照: `"[Circular]"` 文字列

**不変条件**:

- 同じ Data に対して呼び出すたびに **byte-identical** な出力を返す (golden file 安定)
- どんな循環参照入力でも panic / stack overflow なし
- 出力 byte slice は `json.Valid` を満たす

## E-003: `classic.Template`

**役割**: `Template` インタフェースの classic 実装。`init()` で registry に登録される。

**定義**:

```go
// internal/templates/classic/classic.go
package classic

const Name = "classic"

// Template is the singleton.
var Template templates.Template = &classicTemplate{}

func init() {
    templates.Register(Template)
}

type classicTemplate struct {
    fsys     embed.FS // assets/templates/classic/*
    metadata *config.TemplateMetadata
    partials []string // resolved partial order from partials/_.json
}

func (t *classicTemplate) Name() string                       { return Name }
func (t *classicTemplate) Metadata() *config.TemplateMetadata { return t.metadata }
func (t *classicTemplate) FS() fs.FS                          { return t.fsys }
func (t *classicTemplate) Check(q map[string]any, account, format string) error
func (t *classicTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error)
```

**Check rules**:

- `account ∈ {"user", "organization"}`: pass. Anything else (esp. "repository") returns `*errors.InputError{Field: "account"}`.
- `format ∈ {"", "svg", "png", "jpeg", "json"}`: pass. Anything else returns `*errors.UnsupportedFormatError`.

**Run pipeline**:

1. Load `image.svg` skeleton
2. Render top-level `<svg>` with width/height (M2: fixed 480 / 99999, columns/large modes for M3)
3. Insert `<defs><style>` with fonts.css + style.css (raw, unminified)
4. Render `<foreignObject><div class="items-wrapper">`
5. Iterate `t.partials` in order, call each partial's `PartialFunc`, append result
6. Optional `<footer>` if `base.metadata == true`
7. Close `</div></foreignObject></svg>`
8. Return resulting string

## E-004: classic Partials

**役割**: 4 個の `PartialFunc` (M2 で実装)。各 partial は冪等、不在キーで空文字列を返す。

**定義**:

```go
// internal/templates/classic/partials/<name>.go
type partial struct {
    name string
    fn   templates.PartialFunc
}

var registered = []partial{
    {"base.header",                BaseHeader},
    {"introduction",               Introduction},
    {"base.activity+community",    BaseActivityCommunity},
    {"base.repositories",          BaseRepositories},
}

// BaseHeader renders the <section> header containing avatar, name,
// login, follower count, and joined-at date. Missing fields collapse
// to empty strings; the wrapper section is omitted entirely if
// Data.User is nil.
func BaseHeader(ctx context.Context, pc *templates.PartialContext) (string, error)

// Introduction is a stub. Returns empty string when
// Data.Plugins["introduction"] is nil (M1 / M2 default).
func Introduction(ctx context.Context, pc *templates.PartialContext) (string, error)

// BaseActivityCommunity stitches Data.User's activity and community
// totals into a two-column section.
func BaseActivityCommunity(ctx context.Context, pc *templates.PartialContext) (string, error)

// BaseRepositories surfaces Data.Computed.Repositories counts (count,
// stargazers, forks, watchers) formatted via internal/format.Format.
func BaseRepositories(ctx context.Context, pc *templates.PartialContext) (string, error)
```

**Common helpers** (`internal/templates/classic/escape.go`):

```go
// EscapeXML maps <, >, &, ', " to their XML numeric / entity references.
func EscapeXML(s string) string

// FormatCount returns format.Format(n) wrapped with `internal/format`
// helpers. Centralized so partials share the same number-shortening
// behavior.
func FormatCount(n int64) string
```

## E-005: `format.Format` のエスケープ規約 (再利用)

M1 で `internal/format` に既存。本 spec で新規定義なし。`FormatBytes`, `FormatPercentage` は M2 partial では未使用 (M4 plugin 時に活用)。

## エンティティ間の関係

```
engine.Compute
    │
    ├── Stage 1: base.Plugin.Run        (M1: populates Data.User, Data.Computed)
    ├── Stage 2: core.Plugin.Run        (M1: populates Data.Config)
    ├── Stage 3: core.RunPlugins        (M1: runs other plugins in parallel)
    └── Stage 4: format dispatch        (M2 NEW)
              ├── req.Format == "json"  → engine.Marshal(data) → Result.Output / MIME
              └── req.Format ∈ {svg,..} → tmpl.Run(...)         → Result.Output / MIME
                                            │
                                            └── classic.Run pipeline
                                                  ├── load image.svg skeleton
                                                  ├── iterate t.partials [4 funcs]
                                                  │     ├── BaseHeader
                                                  │     ├── Introduction (stub)
                                                  │     ├── BaseActivityCommunity
                                                  │     └── BaseRepositories
                                                  └── optional metadata <footer>
```

すべて `internal/` 配下で完結し、`cmd/` から触れるのは引き続き `engine.Compute` のみ。
