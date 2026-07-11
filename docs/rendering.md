# Rendering

Output comes in 4 formats: **SVG / PNG / JPEG / JSON**. Since v4.0.0 (#409), rendering is completely browser-free, with no runtime dependency on Chromium / chromedp. The SVG's height is finalized on the Go side, and only rasterization (PNG / JPEG) is delegated to the external `resvg` binary.

## Table of Contents

- [1. Pipeline overview](#1-pipeline-overview)
- [2. Partials and height finalization](#2-partials-and-height-finalization)
- [3. Decoration pipeline (`render.Apply`)](#3-decoration-pipeline-renderapply)
- [4. resvg rasterization](#4-resvg-rasterization)
- [5. padding](#5-padding)
- [6. JSON output](#6-json-output)
- [7. SVG hash (data-changed detection)](#7-svg-hash-data-changed-detection)

---

## 1. Pipeline overview

After plugin execution, `engine.Compute` outputs according to `format` via `dispatchOutput` in `engine/dispatch.go`.

- **json**: Serializes `data` with `MarshalWithProvider` ([SS6](#6-json-output)).
- **svg / png / jpeg**:
  1. `template.Run(ctx, pc)` stacks the partials vertically and generates a single SVG string ([SS2](#2-partials-and-height-finalization)).
  2. It passes through the decoration stages of `render.Apply` in order (octicon substitution -> image inlining -> optional CSS purge / XML formatting) ([SS3](#3-decoration-pipeline-renderapply)).
  3. **svg**: The height is already finalized at generation time, so it is returned as-is (no rasterizer is called). padding is applied optionally ([SS5](#5-padding)).
  4. **png / jpeg**: The finalized SVG is rasterized by `render.Renderer.Resize` (resvg by default) ([SS4](#4-resvg-rasterization)).

Templates are implemented in Go code, not EJS. Some places retain the upstream `.ejs` filenames in comments, but these are just for tracking the porting source; no EJS engine exists at runtime.

## 2. Partials and height finalization

Upstream measured the rendered height using headless Chromium, measuring the HTML inside `foreignObject`. This port does not use a browser; instead, **each partial reports its own consumed height**:

```go
// internal/templates/template.go
type PartialFunc func(ctx context.Context, pc *PartialContext) (markup string, height int, err error)
```

- `height > 0`: The exact number of px consumed by that partial (native SVG).
- `height == 0`: Height not reported. The markup is still emitted, but sections with `height <= 0` are skipped with a warning during stacking.

`StackSections` in `internal/templates/chrome` accumulates each partial's reported height, arranges them vertically with `<g transform="translate(0,y)">`, and the template writes the finalized value directly into the root `<svg height>` (there is no `height="99999"` placeholder or measurement pass).

The SVG primitives (`WrapSection` / `SVGText` / `SVGField`, etc.) live in `internal/templates/chrome/svg.go`, and new partials should reuse them. Text width is measured by `internal/render/fontmetrics` (embedded Liberation Sans). Since the fallback font in the viewing browser can be up to ~13.5% wider, elements with fixed widths should take ~14% of headroom. See the "Rendering architecture" section of CLAUDE.md for implementation notes.

## 3. Decoration pipeline (`render.Apply`)

`render.Apply(stages, svg)` applies `PipelineStage` in order. Each stage is a best-effort chain that passes its input to the next stage even on error, and the caller decides whether the collected errors are fatal (`internal/render/pipeline.go`).

| Stage                       | Description                                                                                     | File              |
| --------------------------- | ----------------------------------------------------------------------------------------------- | ----------------- |
| octicon                     | Replaces `:octicon-<name>-<size>:` with an embedded SVG (data from `assets/octicons/data.json`) | `octicon.go`      |
| image inline                | Fetches remote / local images and inlines them as `data:` URIs                                  | `image_inline.go` |
| CSS optimization (optional) | Purges selectors that do not appear in the HTML and minifies with `tdewolff/minify`             | `css.go`          |
| XML formatting (optional)   | Re-indents and formats the `<svg>`                                                              | `xml_format.go`   |

Only octicon emoji are supported. SVGO-equivalent SVG optimization and twemoji / gemoji substitution are not adopted ([`scope.md`](scope.md#33-other-non-adopted-items)).

## 4. resvg rasterization

PNG / JPEG rasterize the finalized SVG with the `resvg` binary (`internal/render/resvg.go`, `render.NewResvg`).

- **PNG**: Streams the SVG into the `resvg` subprocess via stdin and receives the PNG via stdout (`rasterizePNG`). Since the generated SVG already has images inlined as base64, a resources-dir is not needed.
- **Resolution**: PNG/JPEG are rasterized at `render.RasterScale` (= 2) scale (`--zoom 2`). The SVG coordinate system's 480px width becomes a 960px output width. This is to prevent blurriness on high-DPI (Retina) displays; when embedding at the card's native size, specify the display width as in `<img width="480">`.
- **JPEG**: Since resvg does not output JPEG, it is first converted to PNG and then re-encoded with Go's standard `image/jpeg`.
- **SVG**: `Resize` only applies padding (no rasterization).

### 4.1 Binary resolution and fonts

- Resolution order: `ResvgOpts.ExecPath` -> environment variable `METRICS_RESVG_PATH` -> `resvg` on `PATH`. It is stat-checked at construction time, and if not found, it immediately errors out (`*InputError`) to prevent silent degradation.
- **Font flags are mandatory**. Since the generated SVG's font stack ends with CSS generic families (`sans-serif` / `monospace`), map them to the Liberation family with `--sans-serif-family "Liberation Sans"`, etc. Without this mapping, resvg silently skips all `<text>` elements.
- resvg does not interpret `var()` / `:root` / `:not`, etc. Colors are output as literal `fill` / `stroke` attributes (`@keyframes` are ignored and drawn with the inline final value, so animation CSS such as for gauges may be left as-is).

Prebuilt resvg binaries are bundled in the Docker image. Local testing uses `make test-resvg` (requires `METRICS_RESVG_PATH`).

## 5. padding

`config_padding` supports the upstream-compatible `"<absolute> + <relative>%"` format (`internal/render/padding.go`). It was originally meant to absorb browser measurement error, but now that measurement is gone, the default is effectively a no-op. Only when a non-trivial padding is specified does it arithmetically rewrite the root `<svg>`'s width / height / viewBox.

## 6. JSON output

Serializes and returns `data` (MIME `application/json`). The JSON keys of plugin results are fixed via each plugin's struct tags to match upstream (`data.plugins.<name>`).

## 7. SVG hash (data-changed detection)

Used to determine commit eligibility for `output_condition=data-changed` (`internal/render/svg_hash.go`, `Hash`).

- Hashes the outer HTML of the SVG, with the footer (`<g data-section="metadata">`) removed, using MD5 (32-character hex).
- Differences confined to the footer alone (timezone / version / generated time) produce the same hash.
- The implementation is pure Go (`goquery` + `crypto/md5`). There is no rasterizer dependency.
