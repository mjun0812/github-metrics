# Contract: Result dispatch by Request.Format (M2)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `engine.Compute` が `Request.Format` を元に出力形式を選び、`Result.Output` / `Result.MIME` を populate するアルゴリズムを定義する。

## 1. 入力: `Request.Format`

| 値 | 意味 | M2 の挙動 |
|---|---|---|
| `""` (空) | テンプレートの既定 | `Template.Metadata().Formats[0]` を使用。Template が nil なら `"json"`。 |
| `"json"` | JSON 出力 | `engine.Marshal(data)` を呼ぶ。 |
| `"svg"` | SVG 出力 | `Template.Run(...)` を呼ぶ。 |
| `"png"` | PNG 出力 | M3 で chromedp 変換。M2 は SVG を返し warn ログを出す。 |
| `"jpeg"` | JPEG 出力 | 同上 (warn ログ + SVG bytes)。 |
| 上記以外 | 未対応 | `*errors.UnsupportedFormatError` を返す。 |

## 2. 出力: `Result.Output` / `Result.MIME`

| Format | `Output` | `MIME` |
|---|---|---|
| `json` | `engine.Marshal(data)` の戻り値 | `application/json` |
| `svg` | `Template.Run(...)` を `[]byte` 化 | `image/svg+xml` |
| `png` | M2 では SVG bytes (warn ログ済) | `image/png` |
| `jpeg` | 同上 | `image/jpeg` |

## 3. ディスパッチ位置

`engine.Compute` の **Stage 4 (template.Run)** を拡張する:

```go
// 既存の M1 実装:
//   if tmpl != nil {
//     if _, err := tmpl.Run(ctx, pcPartial); err != nil { ... }
//   }
//
// M2 で拡張:
output, mime, err := dispatchOutput(ctx, req, deps, tmpl, data, pcPartial)
if err != nil {
    return nil, fmt.Errorf("engine: dispatch %q: %w", req.Format, err)
}
res.Output = output
res.MIME = mime
```

`dispatchOutput` は以下の擬似コード:

```go
func dispatchOutput(...) ([]byte, string, error) {
    format := req.Format
    if format == "" {
        if tmpl != nil && len(tmpl.Metadata().Formats) > 0 {
            format = tmpl.Metadata().Formats[0]
        } else {
            format = "json"
        }
    }

    switch format {
    case "json":
        b, err := Marshal(data)
        return b, "application/json", err
    case "svg":
        if tmpl == nil {
            return nil, "", xerrors.NewInputError("template",
                errors.New("svg output requires a registered template"))
        }
        out, err := tmpl.Run(ctx, pcPartial)
        return []byte(out), "image/svg+xml", err
    case "png", "jpeg":
        deps.Logger.Warn("png/jpeg output: M2 returns SVG bytes; chromedp conversion lands in M3",
            "format", format)
        out, err := tmpl.Run(ctx, pcPartial)
        mime := "image/png"
        if format == "jpeg" {
            mime = "image/jpeg"
        }
        return []byte(out), mime, err
    default:
        return nil, "", xerrors.NewUnsupportedFormatError(format, nil)
    }
}
```

## 4. デフォルト解決ロジック (FR-014)

| 状態 | 結果 |
|---|---|
| `Format=""`, `Template="classic"` registered | classic の `metadata.yml.formats[0]` = `"svg"` |
| `Format=""`, `Template="noop"` | `"json"` (noop は metadata を持たないため fallback) |
| `Format=""`, `Template=""` (Template が nil) | `"json"` |
| `Format="svg"`, `Template=""` | エラー (`InputError{Field: "template"}`) |

## 5. テスト契約

- `tests/integration/output_test.go::TestComputeJSON_DefaultFromTemplate`
- `tests/integration/output_test.go::TestComputeJSON_NoTemplate`
- `tests/integration/output_test.go::TestComputeSVG_Classic`
- `tests/integration/output_test.go::TestComputePNG_M2WarnsAndReturnsSVG`
- `tests/integration/output_test.go::TestComputeUnknownFormat_Error`

各ケースで `Result.MIME` および `Result.Output` の先頭 bytes / `json.Valid` を assert。
