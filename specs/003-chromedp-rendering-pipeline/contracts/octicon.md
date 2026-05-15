# Contract: octicon プレースホルダ置換 (M3)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `internal/render.ReplaceOcticons` の I/O、`:octicon-<name>(-<size>)?:` のマッチ規則、`assets/octicons/data.json` のスキーマと生成手順を契約として固定する。

## 1. シグネチャと不変条件

```go
// internal/render/octicon.go
func ReplaceOcticons(in string) (string, error)
```

| 項目 | 規則 |
|---|---|
| 入力 `""` | 戻り値 `("", nil)` |
| 既知 octicon (例 `:octicon-star-16:`) | `<svg class="octicon" width="16" height="16" viewBox="...">...</svg>` で置換 |
| size 省略 (例 `:octicon-star:`) | 16px を採用 |
| 未知 name (例 `:octicon-doesnotexist:`) | プレースホルダ素通し (置換せず) |
| 未知 size (例 `:octicon-star-32:`) | プレースホルダ素通し (16/24 のみサポート) |
| `data.json` がロードできない | 戻り値 `(in, *InputError)` (passthrough + error 返却) |

## 2. 正規表現

```go
var octiconRe = regexp.MustCompile(`:octicon-([a-z0-9-]+?)(?:-(16|24))?:`)
```

- name 部分は `[a-z0-9-]+?` (lazy match) でハイフン込みの octicon 名 (例 `chevron-down`) に対応。
- size 部分は `16` または `24` のみ (primer/octicons の規約)。
- 末尾の `:` までを 1 トークンとして消費。

### マッチ例

| 入力 | match | 置換結果 |
|---|---|---|
| `:octicon-star-16:` | name=`star`, size=`16` | `<svg class="octicon" width="16"...>` |
| `:octicon-star:` | name=`star`, size=`` (→ 16) | 上と同じ |
| `:octicon-chevron-down-24:` | name=`chevron-down`, size=`24` | 24px svg |
| `:octicon-unknown-16:` | name=`unknown`, size=`16` | 素通し |
| `:octicon-star-32:` | (`-32` は `(?:-(16|24))?` にマッチしない) | name=`star-32`, size=`` → `star-32` で lookup → 素通し |
| `Octicon-star-16` (`:` 抜け) | マッチなし | 素通し |
| `<text>:octicon-star-16:</text>` | テキストノード内でも match | `<text><svg class="octicon"...></svg></text>` (SVG 仕様上 valid) |

## 3. 置換 SVG fragment 構造

`data.json` から得た raw SVG fragment に `class="octicon"` を追加する:

```text
data.json[name][size] = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><path d="..."/></svg>'

ReplaceOcticons 出力 = '<svg xmlns="http://www.w3.org/2000/svg" class="octicon" width="16" height="16" viewBox="0 0 16 16"><path d="..."/></svg>'
```

`class` 属性が既に存在する場合は **追加 (`octicon` を空白区切りで append)**。

## 4. `assets/octicons/data.json` 生成

### 4.1 ツール `internal/tools/gen-octicons/main.go`

```text
Usage: go run ./internal/tools/gen-octicons -in <path-to-primer-octicons-build-data.json> -out assets/octicons/data.json

  -in   primer/octicons の build/data.json (npm install @primer/octicons の出力)
  -out  生成先 (既定 assets/octicons/data.json)
```

### 4.2 アルゴリズム

1. primer/octicons の `build/data.json` を読み込む (スキーマ: `{ [name]: { heights: { "16": { path, options }, "24": ... }, ... } }`)。
2. 各 `(name, size)` について `<svg xmlns="..." width="{w}" height="{h}" viewBox="{vb}">{path}</svg>` を組み立てる。
3. 結果を本 spec data-model.md E-006 のスキーマに従って整形し、`_meta.source` と `_meta.generated_at` を埋め、出力。
4. `oxfmt` でフォーマット (M1 で確立した markdown formatter は JSON にも uniform に走るとは限らないので、JSON 出力は `json.MarshalIndent("", "  ")` で 2 sp インデント済みにしておく)。

### 4.3 冪等性

- 同じ input から同じ output (タイムスタンプ `_meta.generated_at` を除く)。
- CI で `make gen-octicons && git diff --exit-code assets/octicons/data.json` 緑であること (将来 PR で octicon の追加忘れを検知)。

## 5. テスト契約

`internal/render/octicon_test.go`:

| テスト名 | ケース |
|---|---|
| `TestReplace_Known16` | `:octicon-star-16:` → `<svg class="octicon"...>` |
| `TestReplace_DefaultSize` | `:octicon-star:` → 16px svg |
| `TestReplace_ChevronDown24` | `:octicon-chevron-down-24:` → 24px svg |
| `TestReplace_UnknownPasses` | `:octicon-doesnotexist:` → 素通し |
| `TestReplace_InvalidSize` | `:octicon-star-32:` → 素通し |
| `TestReplace_MultipleInOneString` | `"A :octicon-star: B :octicon-repo-24: C"` → 2 件置換 |
| `TestReplace_ExistingClass` | input `:octicon-star-16:` の data.json fragment が `class="x"` を持っていたら `class="x octicon"` |
| `TestReplace_Idempotent` | `ReplaceOcticons(ReplaceOcticons(in)) == ReplaceOcticons(in)` |
| `TestReplace_EmptyInput` | `("", nil)` |

`tests/golden/render/octicon_replaced.svg`:

- 入力: `tests/fixtures/render/input_with_octicon.svg` (`:octicon-star-16:` + `:octicon-repo-24:` + 未知 1 件)
- 期待: 既知 2 件置換、未知 1 件素通し、その他 DOM 変化なし。

## 6. パフォーマンス予算

- 1 SVG (最大 500KB, 50 octicon プレースホルダ) に対し `ReplaceOcticons` 単体 < 10 ms (`BenchmarkReplaceOcticons_LargeSVG`)。
- `data.json` (200KB JSON) は `init()` 内で 1 回 unmarshal して `map[string]map[string]string` にキャッシュ。
- 正規表現は `regexp.MustCompile` で package level 1 回コンパイル、再利用。
