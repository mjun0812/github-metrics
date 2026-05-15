# Contract: 装飾パイプラインのディスパッチ (M3)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `engine.Compute` の Stage 4 (template.Run) 直後に挿入される装飾パイプラインの段順序、入力解釈、失敗時の挙動を契約として固定する。

## 1. パイプライン段一覧

| # | 段 | 関数 | 条件 | 役割 |
|---|---|---|---|---|
| 1 | `Template.Run` | M2 既存 | Template registered & `Format ∈ {svg, png, jpeg}` | partial 順に DOM を組み立て、`<svg>` を生成 |
| 2 | `octicon` | `render.ReplaceOcticons` | 常時 enable | `:octicon-<name>(-<size>)?:` を SVG fragment に置換 |
| 3 | `css` | `render.OptimizeCSS` | `inputs["svg.optimize.css"] == true` | `<style data-optimizable="true">` の未使用セレクタを削除 + minify |
| 4 | `xml` | `render.FormatXML` | `inputs["svg.optimize.xml"] == true` | 2 sp インデント + `collapseContent` |
| 5 | `resize` | `render.Renderer.Resize` | `Format ∈ {svg, png, jpeg}` | chromedp で計測 + `<svg height>` 書き換え + 必要なら PNG/JPEG |

JSON 経路 (`Format == "json"`) は段 1〜5 を **すべてスキップ** する (M2 で確立した dispatch のまま)。

## 2. dispatch アルゴリズム

```go
// internal/engine/dispatch.go (改修後)
func dispatchOutput(
    ctx context.Context,
    req Request,
    deps Deps,
    tmpl templates.Template,
    data *plugins.Data,
    pcPartial *templates.PartialContext,
    res *Result, // *Result 経由で Errors を追記できるよう変更
) ([]byte, string, error) {
    format := resolveFormat(req, tmpl) // M2 の既存ロジック

    if format == "json" {
        body, err := Marshal(data)
        if err != nil { return nil, "", fmt.Errorf("engine: marshal json: %w", err) }
        return body, "application/json", nil
    }

    if tmpl == nil {
        return nil, "", xerrors.NewInputError("template",
            fmt.Errorf("format %q requires a registered template", format))
    }

    rendered, err := tmpl.Run(ctx, pcPartial)
    if err != nil {
        return nil, "", fmt.Errorf("engine: template %q run: %w", req.Template, err)
    }

    // Stages 2-4: best-effort decoration
    stages := buildPipelineStages(req.Inputs)
    rendered, stageErrs := render.Apply(stages, rendered)
    for _, e := range stageErrs {
        res.Errors = append(res.Errors, e)
        deps.Logger.Warn("engine: pipeline stage error", "err", e)
    }

    // Stage 5: resize / convert
    renderer := deps.Render
    var ownedBrowser *render.Browser
    if renderer == nil {
        b, err := render.New(render.BrowserOpts{Logger: deps.Logger})
        if err != nil {
            // PNG/JPEG: 失敗を Errors に追記し、空 Output で返す。
            // SVG: 装飾後 rendered を passthrough で返す。
            res.Errors = append(res.Errors, &xerrors.RetryableError{Err: err})
            if format == "svg" {
                return []byte(rendered), "image/svg+xml", nil
            }
            return nil, "", nil
        }
        renderer = b
        ownedBrowser = b
        defer ownedBrowser.Close()
    }

    out, err := renderer.Resize(ctx, rendered, render.ResizeOpts{
        Convert: format,                              // "svg" / "png" / "jpeg"
        Padding: inputsAsStringSlice(req.Inputs, "config.padding"),
        Scripts: inputsAsStringSlice(req.Inputs, "extras.js"),
    })
    if err != nil {
        res.Errors = append(res.Errors, fmt.Errorf("engine: resize: %w", err))
        if format == "svg" {
            return []byte(rendered), "image/svg+xml", nil
        }
        return nil, "", nil
    }
    return out.Body, out.MIME, nil
}

func buildPipelineStages(inputs map[string]any) []render.PipelineStage {
    out := []render.PipelineStage{
        {Name: "octicon", Run: render.ReplaceOcticons},
    }
    if asBool(inputs, "svg.optimize.css") {
        out = append(out, render.PipelineStage{Name: "css", Run: render.OptimizeCSS})
    }
    if asBool(inputs, "svg.optimize.xml") {
        out = append(out, render.PipelineStage{Name: "xml", Run: render.FormatXML})
    }
    return out
}
```

## 3. 入力解釈

| input key | 型 | 既定値 | 適用段 | 出典 |
|---|---|---|---|---|
| `svg.optimize.css` | bool | `false` | 段 3 | `assets/plugins/core/metadata.yml` (M1 既存) |
| `svg.optimize.xml` | bool | `false` | 段 4 | 同上 |
| `config.padding` | string / []string | `""` | 段 5 | 同上 |
| `extras.js` | []string | `nil` | 段 5 | 同上 (extras flag で gated) |
| `convert` | string | "" | 段 5 (Format に matched) | M1 既存 — Request.Format に解決済み |

新規 input は **追加しない**。constitution 原則 I に準拠。

## 4. エラー伝播

| シナリオ | `Result.Output` | `Result.MIME` | `Result.Errors` | Compute 戻り値 |
|---|---|---|---|---|
| 全段成功 | `out.Body` | `out.MIME` | 既存 plugin エラーのみ | `(*Result, nil)` |
| octicon 段失敗 (data.json 破損等) | 装飾前 SVG → 後段に passthrough | `out.MIME` | `[..., stage "octicon": err]` | `(*Result, nil)` |
| css 段失敗 (CSS パースエラー) | 直前段 (octicon 後) の SVG | `out.MIME` | `[..., stage "css": err]` | `(*Result, nil)` |
| chromedp 起動失敗、`Format=svg` | 装飾後 SVG (resize 前) | `image/svg+xml` | `[..., RetryableError]` | `(*Result, nil)` |
| chromedp 起動失敗、`Format=png/jpeg` | `nil` | `""` | `[..., RetryableError]` | `(*Result, nil)` |
| `Die == true` で plugin error 1 件以上 | — | — | — | `(nil, err)` (M2 既存挙動) |
| Template 未登録 + Format=svg | `nil` | `""` | — | `(nil, InputError)` (M2 既存挙動) |

## 5. テスト契約

- `internal/engine/dispatch_test.go::TestDispatch_SVG_PipelineApplied` — FakeRenderer 注入で octicon + css + xml の各段が呼ばれることを mock 検証。
- `internal/engine/dispatch_test.go::TestDispatch_PNG_BrowserError_NilOutput` — Renderer が error を返した場合、PNG 経路で `Output == nil && MIME == ""` を assert、`Errors` に RetryableError が含まれる。
- `internal/engine/dispatch_test.go::TestDispatch_SVG_BrowserError_PassthroughSVG` — 同条件で SVG 経路では `Output` が非空、`MIME == "image/svg+xml"`。
- `tests/integration/render_pipeline_test.go::TestComputeSVG_OcticonReplaced` — FakeRenderer 経由で octicon プレースホルダが SVG に置換されることを正規表現で assert。
- `tests/integration/render_pipeline_test.go::TestComputeSVG_OptimizeCSSEnabled` — `inputs["svg.optimize.css"]=true` で `<style>` から未使用 selector が消えることを assert。

## 6. 段の冪等性

すべての段は **冪等** (`Run(Run(in)) == Run(in)`) であること:

- `ReplaceOcticons`: 置換後の `<svg class="octicon">` には `:octicon-...:` 形式が含まれないため再実行で no-op。
- `OptimizeCSS`: 全 selector が使用中になった後の再実行で no-op。
- `FormatXML`: 整形済 XML を再整形しても同じ出力。
- `Resize`: 既に `height` が確定済の SVG に対しても再計測で同じ結果 (allocCtx は新規 tab を作るため副作用なし)。

冪等性は契約として `internal/render/<stage>_test.go::Test<Stage>_Idempotent` で各段に対し assert する。
