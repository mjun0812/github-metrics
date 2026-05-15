# Phase 1 Data Model: chromedp レンダリングパイプライン (M3)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は M3 spec の Key Entities を Go の型表現と検証規則に落とし込む。型名・フィールド名は constitution 原則 V に従い英語のままとする。本 spec で既存型へ加える変更は `engine.Deps` 1 フィールドのみで、他は新規パッケージ `internal/render/` に置く新規型。

## E-001: `render.Browser` (chromedp ラッパ)

**役割**: chromedp の `ExecAllocator` + 親 context を保持し、`NewTab(ctx) (tabCtx, cancel, err)` で子 tab context を発行する。N 回利用後に内部 recycle (mutex 内 `recycleLocked()`) で allocCtx を再生成する。M3 で初登場、M4 plugin scrape (`topics` / `starlists`) も同じ Browser を使い回す前提。

**定義**:

```go
// internal/render/chrome.go

// Browser owns a single chromedp ExecAllocator plus the parent context
// it spawns tabs from. A Browser is safe for concurrent use; NewTab
// serializes against the internal recycle mutex.
type Browser struct {
    opts        BrowserOpts
    allocCtx    context.Context
    allocCancel context.CancelFunc
    parentCtx   context.Context
    parentCancel context.CancelFunc
    mu          sync.Mutex // guards counter + recycle
    counter     int        // number of NewTab calls since the last
                            // (re)allocation
}

// BrowserOpts captures the construction-time configuration. Zero
// values map to upstream-compatible defaults.
type BrowserOpts struct {
    // ExecPath, when non-empty, overrides chromedp's auto-detect path
    // for the chromium / chrome binary. Falls back to the
    // METRICS_CHROME_PATH env var when empty.
    ExecPath string
    // RecycleEvery sets how many NewTab calls trigger an allocator
    // re-init. Zero means the default (200).
    RecycleEvery int
    // Headless toggles --headless. Default true. Set false only for
    // local debugging via --puppeteer-disable-headless.
    Headless bool
    // ExtraFlags are appended to the chromedp ExecAllocator option set
    // after the upstream-compatible defaults are applied.
    ExtraFlags []chromedp.ExecAllocatorOption
    // Logger is used for debug / warn events. nil falls back to
    // slog.Default().
    Logger *slog.Logger
}

// New constructs a Browser. The returned instance owns chromedp
// resources; callers MUST call Close when done.
func New(opts BrowserOpts) (*Browser, error)

// NewTab returns a child context bound to a fresh chromedp tab. The
// returned cancel func MUST be called by the caller (typically via
// defer).
func (b *Browser) NewTab(ctx context.Context) (tabCtx context.Context, cancel context.CancelFunc, err error)

// Close cancels the allocator and waits for cleanup. Idempotent.
func (b *Browser) Close() error
```

**不変条件**:

- `New` が成功した Browser は `Close` を呼ぶまで `allocCancel` が無効化されない。
- `RecycleEvery <= 0` のとき、内部既定 200 を使用する。
- `counter == RecycleEvery` の `NewTab` 呼び出しが (a) 既存 allocCtx を cancel、(b) 新規 ExecAllocator + parent context を作成、(c) counter を 0 にリセット、までを mutex 内で完結させる。recycle 中の `NewTab` はブロックする。
- `Close` 後の `NewTab` は `*errors.InputError` を返す。
- `BrowserOpts.ExecPath == ""` かつ env `METRICS_CHROME_PATH` が空のとき、chromedp の auto-detect に委ねる。

## E-002: `render.ResizeOpts` / `render.ResizeResult`

**役割**: `svg.Resize` の入力・出力。`engine.dispatchOutput` が `Request.Inputs` から組み立てて `Renderer.Resize` に渡す。

**定義**:

```go
// internal/render/svg_resize.go

// ResizeOpts captures the per-call parameters of Resize. All slice
// fields are read-only after construction.
type ResizeOpts struct {
    // Convert ∈ {"", "svg", "png", "jpeg"}. Empty is treated as "svg".
    Convert string
    // Padding mirrors the upstream `config.padding` input format:
    // either a single string "<abs> + <rel>%" or two strings
    // [width-padding, height-padding] / one comma-separated string.
    Padding []string
    // Scripts is the list of user JS bodies to evaluate before
    // measuring. Each entry is wrapped into `(async () => { ... })()`
    // by the resize JS template (see contracts/svg-resize.md §2.1).
    Scripts []string
    // ViewportWidth / ViewportHeight default to 980/980 when zero.
    ViewportWidth  int
    ViewportHeight int
    // SettleDelay overrides the post-script sleep (default 2400ms).
    // Use only in tests; production code SHOULD pass zero.
    SettleDelay time.Duration
}

// ResizeResult is the value Resize returns on success.
type ResizeResult struct {
    // Body holds the serialized payload — SVG bytes (XMLSerializer
    // output) for "svg"/"" Convert, raw PNG bytes for "png", raw JPEG
    // bytes for "jpeg".
    Body []byte
    // Width / Height are the post-padding integer dimensions written
    // back into the <svg> root (Height only) and recorded for
    // diagnostics.
    Width  int
    Height int
    // MIME is the IANA type matching Body, one of
    // "image/svg+xml" | "image/png" | "image/jpeg".
    MIME string
}
```

**不変条件**:

- `Convert == ""` は `"svg"` と同義 (`ResizeOpts.normalize()` で正規化)。
- `Padding` が `nil` または `len(Padding)==0` のとき、内部既定 (相対 0%, 絶対 0) を適用。
- `ViewportWidth/Height` が 0 のとき、内部既定 980 を適用。
- `SettleDelay == 0` のとき、内部既定 2400ms を適用。テストで `1ms` を指定可能。
- `Resize` 成功時、`ResizeResult.MIME` は必ず非空。`Body` は必ず非空。
- `Width >= 1 && Height >= 1` (FR-007 の `Math.max(1, ...)` と一致)。

## E-003: `render.Renderer` (engine 注入用 interface)

**役割**: `engine.Deps` が依存する 1 メソッド interface。本番は `*render.Browser` がメソッドとして実装、テストは `render.FakeRenderer` を注入。

**定義**:

```go
// internal/render/renderer.go

// Renderer is the abstraction engine.Compute uses to convert a
// rendered SVG string into final SVG/PNG/JPEG bytes. It exists so the
// engine package never imports chromedp directly.
type Renderer interface {
    Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error)
}

// FakeRenderer returns deterministic 1x1 bytes for tests. Use only in
// _test.go files or build-tagged fixtures.
type FakeRenderer struct {
    // Width / Height override the returned ResizeResult dimensions
    // (default 1).
    Width, Height int
    // ErrOnConvert, when set, makes Resize return the configured
    // error for the matching Convert value (e.g. {"png": errSentinel}).
    ErrOnConvert map[string]error
}

func (f *FakeRenderer) Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error)
```

**不変条件**:

- `Browser.Resize(...)` は `Browser.NewTab` → JS evaluate → optional Screenshot の一連を 1 メソッドで提供する。
- `FakeRenderer.Resize` は固定の minimal PNG (`\x89PNG\r\n\x1a\n` を含む 67 byte fixture) / JPEG (`\xff\xd8\xff` 始まり) を返す。`Convert == "" || "svg"` のときは `in` をそのまま `Body` に乗せる (decoration 段の出力を passthrough)。

## E-004: `render.PipelineStage`

**役割**: `engine.Compute` Stage 4 後の装飾段を表現する関数型。順序定義 (`contracts/render-pipeline.md`) はこの型のスライスに帰着する。

**定義**:

```go
// internal/render/pipeline.go

// PipelineStage is the function signature each decoration / optimization
// step exposes. Stages are composed by Apply.
type PipelineStage struct {
    // Name is used in error wrapping and debug logs.
    Name string
    // Run takes the current SVG and returns the next SVG. Any error is
    // surfaced via Apply's StageErrors return; the input string is
    // forwarded to the next stage unchanged (best-effort chain).
    Run func(in string) (string, error)
}

// Apply runs the stages in order. The returned string is the final
// SVG after all (successful) stages; errors are returned alongside in
// registration order. Apply never panics on stage errors.
func Apply(stages []PipelineStage, in string) (string, []error)
```

**不変条件**:

- `Apply` 戻り値 `string` は必ず非空 (`in` 自身を passthrough する)。
- stage が error を返したら次段の入力は **そのままの `in`** (直前段の出力)。
- 戻り値 `[]error` のスライスは stage 実行順、stage 名で wrap (`fmt.Errorf("stage %q: %w", name, err)`)。

## E-005: `engine.Deps` (拡張)

**役割**: M2 で `{Settings, Metadata, Logger, HTTPClient, REST, GraphQL}` を持っていた。本 spec で `Render render.Renderer` を追加する。

**定義**:

```go
// internal/engine/engine.go
type Deps struct {
    Settings   *config.Settings
    Metadata   *config.MetadataLoader
    Logger     *slog.Logger
    HTTPClient *httpx.Client
    REST       *githubapi.REST
    GraphQL    *githubapi.GraphQL
    // Render performs chromedp-backed SVG resize / convert. Nil is
    // permitted: when Format ∈ {"svg","png","jpeg"} and Render is nil,
    // Compute falls back to a default Browser allocated on first use
    // (M3 introduces this field).
    Render render.Renderer
}
```

**不変条件**:

- `Render == nil && Format == "json"`: 影響なし (json 経路は Renderer を呼ばない)。
- `Render == nil && Format ∈ {"svg","png","jpeg"}`: Compute 内で 1 度だけ `render.New(BrowserOpts{})` を遅延生成し、Compute 終了時に `Close()`。
- `Render != nil`: Compute は Render を借りるだけで `Close()` しない (所有権は呼び出し側)。
- テストは `&render.FakeRenderer{}` を渡すことで chromedp 実起動を完全に回避できる。

## E-006: `assets/octicons/data.json` (生成物)

**役割**: octicon 置換テーブル。`internal/tools/gen-octicons/main.go` が primer/octicons から生成し、runtime は `embed.FS` でロードする。

**スキーマ**:

```json
{
  "_meta": {
    "source": "primer/octicons@19.x",
    "generated_at": "2026-05-15T..."
  },
  "icons": {
    "star": {
      "16": "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"16\" height=\"16\" viewBox=\"0 0 16 16\"><path d=\"...\"/></svg>",
      "24": "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"24\" height=\"24\" viewBox=\"0 0 24 24\"><path d=\"...\"/></svg>"
    },
    "repo": {
      "16": "<svg ...>",
      "24": "<svg ...>"
    }
    // ... 600+ icons
  }
}
```

**不変条件**:

- すべての icon は 16 と 24 のキーを両方持つ (primer/octicons の規約)。
- SVG fragment は self-closing でない `<svg>...</svg>` 形式、`class="octicon"` 属性は **置換時に注入** (data.json には含めない)。
- 生成 tool は冪等 (同じ input から同じ output)。

## 既存型への影響まとめ

| 型 | 変更内容 | 後方互換性 |
|---|---|---|
| `engine.Deps` | `Render render.Renderer` フィールド追加 | 既存呼び出しコード (zero-value で構築) は型エラーにならない。`Render == nil` のとき遅延起動でフォールバック。 |
| `engine.Result.Output` (内容) | `Format ∈ {"svg","png","jpeg"}` のとき、Template.Run 結果ではなく装飾+Resize 後の bytes が入る | M2 で `Output` を読んでいた箇所は `MIME` のみ判定しているため、SVG bytes の中身差異は影響なし。テストの golden は更新が必要 (M2 の SVG golden を M3 で再生成)。 |
| `engine.Result.MIME` | 変更なし | — |
| `engine.dispatchOutput` (内部) | png/jpeg 経路の warn ログ削除、`Renderer.Resize` 呼び出しに差し替え | 内部関数のため公開 API には影響なし。 |
| `tests/golden/classic/octocat.svg` (M2 で作成) | 装飾 + Resize 後の値で再生成が必要 | golden 更新フラグ `-update` で対応 (FR-020)。 |

## バリデーション規則 (FR との対応)

| 検証ルール | 検証箇所 | 対応 FR |
|---|---|---|
| `Browser.New` で `ExecPath` が指定されたが実体が存在しない | `Browser.New` 内で `os.Stat` チェック | FR-002 |
| `RecycleEvery <= 0` で zero-fallback (200) | `BrowserOpts.normalize()` | FR-003 |
| `ResizeOpts.Convert` 未知値 | `Resize` 入口で `*UnsupportedFormatError` | FR-008 |
| `padding` パース失敗 | `parsePadding(s)` 内で警告ログ + 既定値 | FR-007 / Edge Case |
| `<svg height="auto">` 入力 | `Resize` 内で `setAttribute` をスキップ | FR-007 |
| `Hash(in)` で `<footer>` が複数 | 最初の 1 件のみ削除 | FR-012 |
| `ReplaceOcticons` 未知 octicon | `:octicon-doesnotexist-24:` をそのまま素通し | FR-014 / SC-005 |
| `OptimizeCSS` で ID セレクタ (`#metrics-end` 等) | 無条件で保持 | FR-015 / SC-006 |
| `Apply` 中の stage error | `[]error` に蓄積、入力を passthrough | FR-018 |
