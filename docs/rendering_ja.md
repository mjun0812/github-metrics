# レンダリング

出力は **SVG / PNG / JPEG / JSON** の 4 形式。v4.0.0 (#409) 以降、レンダリングは完全に browser-free で、Chromium / chromedp への実行時依存を持たない。SVG は Go 側で高さまで確定し、ラスタライズ (PNG / JPEG) だけを外部 `resvg` バイナリに委譲する。

## 目次

- [1. パイプライン概要](#1-パイプライン概要)
- [2. partial と高さの確定](#2-partial-と高さの確定)
- [3. 装飾パイプライン (`render.Apply`)](#3-装飾パイプライン-renderapply)
- [4. resvg ラスタライズ](#4-resvg-ラスタライズ)
- [5. padding](#5-padding)
- [6. JSON 出力](#6-json-出力)
- [7. SVG ハッシュ (data-changed 判定)](#7-svg-ハッシュ-data-changed-判定)

---

## 1. パイプライン概要

`engine.Compute` はプラグイン実行後、`engine/dispatch.go` の `dispatchOutput` で `format` に応じて出力する。

- **json**: `MarshalWithProvider` で `data` をシリアライズ ([§6](#6-json-出力))。
- **svg / png / jpeg**:
  1. `template.Run(ctx, pc)` が partial を縦に積んで 1 枚の SVG 文字列を生成する ([§2](#2-partial-と高さの確定))。
  2. `render.Apply` の装飾ステージ (octicon 置換 → 画像インライン → 任意の CSS purge / XML 整形) を順に通す ([§3](#3-装飾パイプライン-renderapply))。
  3. **svg**: 高さは生成時に確定済みなので、そのまま返す (ラスタライザを呼ばない)。任意で padding を適用する ([§5](#5-padding))。
  4. **png / jpeg**: 確定済み SVG を `render.Renderer.Resize` (既定は resvg) でラスタライズする ([§4](#4-resvg-ラスタライズ))。

テンプレートは EJS ではなく Go コードで実装されている。`lowlighter/metrics` の `.ejs` ファイル名がコメントに残る箇所があるが、これは移植元の追跡用であり、実行時に EJS エンジンは存在しない。

## 2. partial と高さの確定

`lowlighter/metrics` は headless Chromium で描画後の高さを実測し、`foreignObject` 内の HTML を測っていた。本移植ではブラウザを使わず、**各 partial が自分の消費高さを申告する**:

```go
// internal/templates/template.go
type PartialFunc func(ctx context.Context, pc *PartialContext) (markup string, height int, err error)
```

- `height > 0`: その partial が消費する正確な px 数 (native SVG)。
- `height == 0`: 高さ未申告。markup を出しつつ `height <= 0` の section はスタック時に警告付きで skip される。

`internal/templates/chrome` の `StackSections` が各 partial の申告高さを積算し、`<g transform="translate(0,y)">` で縦に並べ、テンプレートがルート `<svg height>` に確定値を直接書き込む (`height="99999"` プレースホルダも計測パスも存在しない)。

SVG プリミティブ (`WrapSection` / `SVGText` / `SVGField` 等) は `internal/templates/chrome/svg.go` にあり、新しい partial はこれを再利用する。テキスト幅は `internal/render/fontmetrics` (Liberation Sans 埋め込み) で計測する。閲覧ブラウザの fallback フォントは最大 ~13.5% 幅広なので、幅を固定する要素には ~14% のヘッドルームを取る。実装上の注意点は CLAUDE.md の "Rendering architecture" を参照。

## 3. 装飾パイプライン (`render.Apply`)

`render.Apply(stages, svg)` は `PipelineStage` を順に適用する。各ステージはエラー時も入力を次段へ渡す best-effort チェーンで、収集したエラーは呼び出し側が致命かどうかを判断する (`internal/render/pipeline.go`)。

| ステージ          | 内容                                                                                   | ファイル          |
| ----------------- | -------------------------------------------------------------------------------------- | ----------------- |
| octicon           | `:octicon-<name>-<size>:` を埋め込み SVG に置換 (データは `assets/octicons/data.json`) | `octicon.go`      |
| image inline      | リモート / ローカル画像を取得して `data:` URI にインライン化                           | `image_inline.go` |
| CSS 最適化 (任意) | HTML に出現しないセレクタを purge し、`tdewolff/minify` で minify                      | `css.go`          |
| XML 整形 (任意)   | `<svg>` を再インデント整形                                                             | `xml_format.go`   |

絵文字は octicon のみ対応する。SVGO 相当の SVG 最適化、twemoji / gemoji 置換は不採用 ([`scope.md`](scope_ja.md#33-その他の不採用))。

## 4. resvg ラスタライズ

PNG / JPEG は確定済み SVG を `resvg` バイナリでラスタライズする (`internal/render/resvg.go`, `render.NewResvg`)。

- **PNG**: `resvg` サブプロセスに SVG を stdin で流し込み、PNG を stdout で受け取る (`rasterizePNG`)。生成 SVG は画像を base64 でインライン済みなので resources-dir は不要。
- **解像度**: PNG/JPEG は `render.RasterScale` (= 2) 倍でラスタライズする (`--zoom 2`)。SVG 座標系 480px 幅 → 出力 960px 幅。高 DPI (Retina) ディスプレイでのぼやけを防ぐためで、カード原寸で埋め込む場合は `<img width="480">` のように表示幅を指定する。
- **JPEG**: resvg は JPEG を出力しないため、一旦 PNG にして Go 標準の `image/jpeg` で再エンコードする。
- **SVG**: `Resize` は padding 適用のみ (ラスタライズしない)。

### 4.1 バイナリ解決とフォント

- 解決順: `ResvgOpts.ExecPath` → 環境変数 `METRICS_RESVG_PATH` → `PATH` 上の `resvg`。構築時に stat し、見つからなければ即エラー (`*InputError`) にして無言劣化を防ぐ。
- **フォントフラグは必須**。生成 SVG のフォントスタックは CSS 総称ファミリ (`sans-serif` / `monospace`) で終わるため、`--sans-serif-family "Liberation Sans"` 等で Liberation ファミリにマップする。マップしないと resvg は `<text>` を全て無言でスキップする。
- resvg は `var()` / `:root` / `:not` 等を解釈しない。色は literal な `fill` / `stroke` 属性で出力する (`@keyframes` は無視され inline 最終値で描かれるので、アニメ CSS は残してよい)。

resvg のプレビルドバイナリは Docker イメージに同梱される。ローカルでのテストは `make test-resvg` (要 `METRICS_RESVG_PATH`)。

## 5. padding

`config_padding` は `lowlighter/metrics` 互換の `"<絶対> + <相対>%"` 形式をサポートする (`internal/render/padding.go`)。元はブラウザ計測誤差の吸収用だったが、計測が無くなった現在の既定は実質 no-op。非自明な padding が指定された場合のみ、ルート `<svg>` の width / height / viewBox を算術で書き換える。

## 6. JSON 出力

`data` をシリアライズして返す (MIME `application/json`)。プラグイン結果の JSON キーは `lowlighter/metrics` (`data.plugins.<name>`) と一致するよう各プラグインの struct タグで固定してある。

## 7. SVG ハッシュ (data-changed 判定)

`output_condition=data-changed` のコミット可否判定に使う (`internal/render/svg_hash.go`, `Hash`)。

- footer (`<g data-section="metadata">`) を除去した SVG の outer HTML を MD5 (hex 32 文字) でハッシュする。
- timezone / version / generated time といった footer のみの差分は同一ハッシュになる。
- 実装は pure Go (`goquery` + `crypto/md5`)。ラスタライザ依存はない。
