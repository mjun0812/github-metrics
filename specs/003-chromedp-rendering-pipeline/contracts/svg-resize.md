# Contract: SVG リサイズと chromedp 評価 (M3)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/render.Resize` および `render.Browser` の I/O 仕様、chromedp に渡す JS 評価スクリプトの本文、padding パース規則、PNG/JPEG 変換規則を契約として固定する。

## 1. `Resize` シグネチャと不変条件

```go
// internal/render/svg_resize.go
func (b *Browser) Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error)
```

| 項目 | 規則 |
|---|---|
| 入力 `in` | 装飾段 (octicon / css / xml) を通過した SVG 文字列。`<svg>` ルートを 1 件、`#metrics-end` 要素を末尾 (またはレイアウト上の終端) に 1 件含む前提。 |
| `opts.Convert == "" \|\| "svg"` | XMLSerializer で SVG bytes を返す。`MIME = "image/svg+xml"`。 |
| `opts.Convert == "png"` | `page.CaptureScreenshot(clip, format=png, omitBackground=true)`。`MIME = "image/png"`。 |
| `opts.Convert == "jpeg"` | 同 (format=jpeg)。`MIME = "image/jpeg"`。 |
| `opts.Convert ∉ {"", "svg", "png", "jpeg"}` | `*errors.UnsupportedFormatError` を返す。 |
| `<svg height="auto">` | `height` 属性は書き換えない。計測のみ。 |
| chromedp `Run` エラー | `*errors.RetryableError` で wrap して返す。`Resize` 内ではリトライしない (上位 `internal/action/retry.go` に委ねる)。 |
| `ctx` キャンセル | 即座に `ctx.Err()` を返す。タブはクリーンアップ。 |

## 2. chromedp 評価スクリプト

### 2.1 本体 (JS 文字列)

`docs/design/13-appendix.md §G` の upstream puppeteer スクリプトを基にしつつ、**実装では prepare → Sleep → measure の 3 ステップに分割**する。理由: Runtime.evaluate の `awaitPromise` が `JSON.stringify` 返却で modern Chrome (137+) 上で不安定に振る舞ったため、settle delay を chromedp 側 (`chromedp.Sleep`) で駆動し JS はすべて同期にした (T026 impl note 参照)。padding と scripts は Go 側で `json.Marshal` → `quoteForJS` でテンプレートに直埋めする (`chromedp.Evaluate` の args bind は使わない)。

#### Prepare JS (`jsPrepareTemplate`)

```javascript
(() => {
  const scripts = JSON.parse(<scriptsJSON>);
  for (const script of scripts) {
    try {
      new Function("document", script)(document);  // sync evaluation
    } catch (e) {
      console.debug("script error: " + e);
    }
  }
  const svg = document.querySelector("svg");
  if (!svg) throw new Error("no <svg> root in document");
  if (!svg.classList.contains("no-animations")) {
    svg.classList.add("no-animations");
    svg.dataset.metricsAnimationsToggled = "1";
  }
  return true;
})()
```

#### Settle (Go 側 `chromedp.Sleep(opts.SettleDelay)`)

JS 内では待機しない。`chromedp.Sleep(SettleDelay)` で外側から駆動。

#### Measure JS (`jsMeasureTemplate`)

```javascript
(() => {
  const padding = JSON.parse(<paddingJSON>);
  const svg = document.querySelector("svg");
  if (!svg) throw new Error("no <svg> root in document at measurement time");
  const endNode = svg.querySelector("#metrics-end");
  if (!endNode) throw new Error("missing #metrics-end measurement anchor");

  // height は #metrics-end の y を SVG ルートの y で減算して算出。
  // width は SVG ルートの bbox から取る (#metrics-end は空 <g> で
  // bbox 幅が 0 になる Blink 動作への対応)。
  const svgRect = svg.getBoundingClientRect();
  const endRect = endNode.getBoundingClientRect();
  let height = endRect.y - svgRect.y;
  let width  = svgRect.width;

  height = Math.max(1, Math.ceil(height * padding.height + padding.absoluteHeight));
  width  = Math.max(1, Math.ceil(width  * padding.width  + padding.absoluteWidth));
  if (svg.getAttribute("height") !== "auto") svg.setAttribute("height", String(height));
  if (svg.dataset.metricsAnimationsToggled === "1") {
    svg.classList.remove("no-animations");
    delete svg.dataset.metricsAnimationsToggled;
  }
  return JSON.stringify({
    resized: new XMLSerializer().serializeToString(svg),
    width: width,
    height: height,
  });
})()
```

返却値は **JSON 文字列**として受け取り、Go 側で `json.Unmarshal` する。これは Runtime.evaluate が object を返したとき Go 側に object のまま渡る (string の想定が崩れる) 挙動を避けるための workaround。

### 2.2 chromedp Tasks 構成

```go
// 擬似コード — 実装は internal/render/svg_resize.go
tabCtx, cancel, err := b.NewTab(ctx)
if err != nil { return ResizeResult{}, err }
defer cancel()

var prepareOK bool
var rawJSON string
err = chromedp.Run(tabCtx,
    chromedp.EmulateViewport(int64(opts.ViewportWidth), int64(opts.ViewportHeight)),
    chromedp.Navigate("about:blank"),
    setDocumentContent(in),               // CDP page.SetDocumentContent
    chromedp.Evaluate(prepareJS, &prepareOK),
    chromedp.Sleep(opts.SettleDelay),      // settle delay は Go 側で駆動
    chromedp.Evaluate(measureJS, &rawJSON),
)
if err != nil { return ResizeResult{}, xerrors.NewRetryableError(err) }

var parsed jsResult
if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
    return ResizeResult{}, xerrors.NewRetryableError(err)
}

if opts.Convert == "svg" {  // normalize は "" を "svg" に置換済み
    return ResizeResult{
        Body: []byte(parsed.Resized), Width: parsed.Width, Height: parsed.Height,
        MIME: "image/svg+xml",
    }, nil
}

// PNG/JPEG branch (§4)
```

### 2.3 SetDocumentContent

`page.SetDocumentContent(frameID, html)` で SVG を body 全体として注入する。chromedp の `chromedp.Navigate("data:text/html;charset=utf-8,...")` は URL 長制限があるため使わない。

## 3. padding パース

`opts.Padding` は string slice。`docs/design/13-appendix.md §G.1` の規則に従う。

```go
// internal/render/padding.go (擬似コード)
type padding struct {
    width, height           float64 // 1.0 + relative/100 (倍率)
    absoluteWidth, absoluteHeight float64
}

func parsePadding(in []string) padding {
    var w, h string
    switch len(in) {
    case 0:
        w, h = "", ""
    case 1:
        // comma-separated を split
        parts := strings.Split(in[0], ",")
        w = strings.TrimSpace(parts[0])
        if len(parts) > 1 { h = strings.TrimSpace(parts[1]) } else { h = w }
    default:
        w, h = strings.TrimSpace(in[0]), strings.TrimSpace(in[1])
    }

    return padding{
        width:  1 + relativeOf(w)/100,
        height: 1 + relativeOf(h)/100,
        absoluteWidth:  absoluteOf(w),
        absoluteHeight: absoluteOf(h),
    }
}

func relativeOf(s string) float64 {
    // /([+-]?[\d.]+)%$/ の末尾マッチ
}
func absoluteOf(s string) float64 {
    // relative を取り除いた後、/^([+-]?[\d.]+)/ の先頭マッチ
}
```

| 入力 | width | height | absoluteW | absoluteH |
|---|---|---|---|---|
| `[]` | 1.0 | 1.0 | 0 | 0 |
| `["0 + 6%"]` | 1.06 | 1.06 | 0 | 0 |
| `["0 + 6%", "8 + 11%"]` | 1.06 | 1.11 | 0 | 8 |
| `["8 + 11%, 0 + 6%"]` (カンマ単一) | 1.11 | 1.06 | 8 | 0 |
| `["+0.5", "1.5%"]` | 1.0 | 1.015 | 0.5 | 0 |
| `["bogus"]` | 1.0 | 1.0 | 0 | 0 (warning ログ) |

## 4. PNG / JPEG 変換

chromedp の `chromedp.CaptureScreenshot(&buf)` ではなく、CDP `page.CaptureScreenshotParams` を直接構築する (clip + format + omitBackground 指定のため)。

```go
// 擬似コード
var buf []byte
err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
    params := page.CaptureScreenshot().
        WithClip(&page.Viewport{
            X: 0, Y: 0,
            Width:  float64(result.Width),
            Height: float64(result.Height),
            Scale:  1,
        }).
        WithFormat(formatFor(opts.Convert)).
        WithOmitBackground(true)
    var encoded string
    return params.Do(ctx).PutInto(&encoded) // chromedp helper that base64-decodes
}))
```

| `opts.Convert` | `format` | MIME | 戻り値 `Body` |
|---|---|---|---|
| `"png"` | `page.CaptureScreenshotFormatPng` | `image/png` | PNG bytes (`\x89PNG\r\n\x1a\n` 始まり) |
| `"jpeg"` | `page.CaptureScreenshotFormatJpeg` | `image/jpeg` | JPEG bytes (`\xff\xd8\xff` 始まり) |

`omitBackground=true` は透明背景を抜くため、SVG が `<rect fill="white">` を持たない場合に背景透過 PNG を返す。

## 5. テスト契約

### 5.1 chromedp 非依存テスト (`go test ./...`)

- `internal/render/padding_test.go::TestParsePadding_Table` — §3 表の全パターン。
- `internal/render/svg_resize_test.go::TestResizeOptsNormalize` — `ResizeOpts.normalize()` の zero-fallback。
- `internal/engine/dispatch_test.go::TestDispatchSVG_UsesRenderer` — `FakeRenderer` で `Renderer.Resize` が呼ばれることを mock 検証。

### 5.2 chromedp 依存テスト (`//go:build chromedp`)

- `internal/render/svg_resize_test.go::TestResize_FixedSVG_HeightInRange` — 固定 SVG (#metrics-end at y=640) → height ∈ `[630, 650]`。
- `internal/render/svg_resize_test.go::TestResize_PNG_Decodable` — `image.Decode` 成功 + 期待 PNG magic header。
- `internal/render/svg_resize_test.go::TestResize_JPEG_Decodable` — 同 (jpeg)。
- `internal/render/chrome_test.go::TestBrowser_RecycleEvery` — N=3 で 4 回目の `NewTab` が新 allocCtx を返す。
- `tests/integration/render_chromedp_test.go::TestComputePNG_E2E` — classic + mocked GraphQL + FakeRenderer 経由ではなく **本物の Browser** で `engine.Compute(Format:"png")` → `image.Decode` 成功。

### 5.3 性能テスト

- `internal/render/svg_resize_bench_test.go::BenchmarkResize_FixedSVG` (chromedp tag) — 1 回 < 2.5 秒の wall time 上限 assert。
- SC-003 を満たしていることを CI ジョブログから確認。

## 6. 失敗時の挙動 (Edge Cases との整合)

| 状況 | 挙動 |
|---|---|
| chromedp 起動失敗 (chromium 不在) | `*RetryableError`、`ResizeResult{}` 空返却。engine 側は `Result.Errors` に追記、SVG 経路では装飾後 SVG を `Output` に乗せる、PNG/JPEG 経路では `Output == nil, MIME == ""` (R-013 で再考した方針)。 |
| `#metrics-end` が DOM に存在しない | JS が `TypeError` を投げる。chromedp は `*RetryableError` で wrap、装飾後 SVG を返す (height 書き換えなし)。 |
| `ctx` deadline 超過 | `Resize` は `ctx.Err()` を即返す。allocCtx は cancel しない (Browser は使い回し)。 |
| `omitBackground=true` で SVG が全透明 | PNG/JPEG bytes は valid だが全透明画像、`image.Decode` 成功するため SC-001 は緑。 |
| recycle 中の concurrent `Resize` 呼び出し | Browser の mutex でシリアライズされる。max 5 秒のブロック許容。 |
