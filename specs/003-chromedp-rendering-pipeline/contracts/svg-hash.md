# Contract: SVG Hash (M3 / 上流 svg.hash 互換)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/render.Hash` の I/O とアルゴリズムを契約として固定する。`output_condition=data-changed` (M6 T-114) の比較に使われる。

## 1. シグネチャと不変条件

```go
// internal/render/svg_hash.go
func Hash(rendered string) (string, error)
```

| 項目 | 規則 |
|---|---|
| 入力 `""` または白文字のみ | 戻り値 `("", nil)`。エラーは返さない (上流 §H と一致) |
| `<svg>` ルートが存在しない | 戻り値 `("", *errors.InputError)` |
| `<footer>` ノードが 0 件 | 戻り値は `<svg>` outerHTML の MD5 (footer 削除 step は no-op) |
| `<footer>` ノードが 1 件 | 削除してから `<svg>` outerHTML を取り、MD5 |
| `<footer>` ノードが 2 件以上 | **最初の 1 件のみ** 削除 (M2 classic は 1 件のみ生成、防御コードとして残す) |
| 戻り値の MD5 | 32 文字小文字 hex (`hex.EncodeToString`) |

## 2. アルゴリズム (pseudo)

```go
func Hash(rendered string) (string, error) {
    if strings.TrimSpace(rendered) == "" {
        return "", nil
    }

    doc, err := goquery.NewDocumentFromReader(strings.NewReader(rendered))
    if err != nil {
        return "", &xerrors.InputError{Field: "rendered", Err: err}
    }

    svgSel := doc.Find("svg").First()
    if svgSel.Length() == 0 {
        return "", &xerrors.InputError{Field: "rendered", Err: errors.New("no <svg> root")}
    }

    footerSel := svgSel.Find("footer").First()
    if footerSel.Length() > 0 {
        footerSel.Remove()
    }

    outer, err := goquery.OuterHtml(svgSel)
    if err != nil {
        return "", err
    }

    sum := md5.Sum([]byte(outer))
    return hex.EncodeToString(sum[:]), nil
}
```

## 3. 入出力の例

| 入力 SVG (概略) | 出力 |
|---|---|
| `<svg><g></g></svg>` | `md5("<svg><g></g></svg>")` (footer なし) |
| `<svg><g/><footer>generated 2026-01-01</footer></svg>` | `md5("<svg><g></g></svg>")` (footer 除去後) |
| `<svg><g/><footer>v1.2.3</footer></svg>` | 上と **一致** (footer 内容は無視) |
| `<svg><g class="x"/><footer/></svg>` | `md5("<svg><g class=\"x\"></g></svg>")` |
| `""` | `""` |
| `"<html><body>no svg</body></html>"` | `("", *InputError{Field:"rendered"})` |

## 4. テスト契約

`internal/render/svg_hash_test.go`:

- `TestHash_EmptyString` — `("", nil)` を返す。
- `TestHash_NoSVGRoot` — `*InputError`、空文字。
- `TestHash_FooterRemoved` — footer のみ違う 2 本で hash が一致。SC-004 の合否判定。
- `TestHash_DOMDifference` — `<g class="x"/>` vs `<g class="y"/>` で hash 不一致。
- `TestHash_MultipleFooters` — 2 件目の `<footer>` は残ること (`Remove().First()` の境界)。
- `TestHash_GoldenOctocat` — `tests/golden/render/octocat_svg.txt` の固定 SVG に対する hash `"<32 hex chars>"` の golden。`-update` フラグ更新対応。

## 5. 上流互換性検証

`tests/compatibility/svg_hash_test.go` (本 spec で新規追加):

- 上流 `lowlighter/metrics` の `metrics.utils.svg.hash` を Node で実行した結果と、本実装の `render.Hash` の結果が **同じ MD5** を返すことを 3 ケース以上で確認する。
- 上流結果は `tests/fixtures/upstream/svg_hash/<case>.{svg,hash}` に静的 vendor (sync-fixtures 不要、頻繁には変わらないため)。
- 不一致が出たら SC-001 の前提 (DOM 単位互換性) が崩れているサインなので CI で fail させる。

## 6. パフォーマンス予算

- 入力 SVG 500 KB (classic + 主要 10 plugin の最大ケース) に対し `Hash` 単体で **5 ms 以内** に完走 (`BenchmarkHash_LargeSVG`)。
- goquery の DOM パース + outerHTML 生成が支配的。MD5 自体は < 1 ms。
- chromedp 不要 (pure Go) であることを `internal/render/svg_hash.go` の import に chromedp が含まれないテストで担保 (`grep -L chromedp svg_hash.go` を `make test-static-imports` 系の lint で回す案、将来検討)。
