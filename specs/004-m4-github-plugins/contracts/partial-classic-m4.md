# Contract: classic テンプレート partial の M4 拡張

**Date**: 2026-05-16 (revised during T005/T006 implementation) | **Plan**: [../plan.md](../plan.md)

本書は classic テンプレートに採用 21 plugin の partial を組み込むための DOM 階層と命名規約を確定する。

> **Implementation note (2026-05-16)**: M2 で classic テンプレートは **Go 関数ベースの partial** (`internal/templates/classic/partials/partials.go` 内に `BaseHeader` 等の `templates.PartialFunc` を定義し `partials.Lookup(name)` で取得) を採用済。本 spec の初版は `.svg.tmpl` text/template を想定していたが、M2 既存パターンに合わせて **Go 関数 + `_.json` 順序定義** で再構成する (constitution 原則 V「コードは Go」と整合)。

## 1. ファイル配置

```text
internal/templates/classic/
├── classic.go                              # 既存。M4 で plugin partial dispatcher を追加。
├── classic_test.go                         # 既存。M4 dispatcher 用 case を追加。
└── partials/                               # 既存パッケージ。M2 で base.* partial を提供済。
    ├── partials.go                         # 既存。BaseHeader / Introduction / BaseActivityCommunity / BaseRepositories。
    ├── plugins.go                          # 新規 (T005)。pluginPartialOrder スライス + 21 個の PluginXxx 関数の skeleton を定義。
    ├── text.go                             # 既存。EscapeXML / FormatCount 等の helper。
    └── *_test.go                           # 既存 + M4 plugin 単位の追加テスト。

assets/templates/classic/partials/
└── _.json                                  # 既存。M2 の base.* slug を持つ。M4 で `plugin.<slug>` 21 個を追記。
```

## 2. partial の DOM 規約

classic テンプレートは `<foreignObject>` 配下に HTML フローで partial を並べる (M2 既存)。M4 plugin partial も同方針で、各 partial の出力は **1 つの `<div>` ブロック** にラップされる:

```html
<div class="plugin-{{slug}}" data-plugin="{{slug}}">
  <!-- plugin 固有の HTML / SVG 子要素 -->
</div>
```

- `class="plugin-<slug>"` は全 plugin 共通命名。CSS purge (M3) の対象クラス。
- `data-plugin="<slug>"` 属性は data-changed mode (M6) で plugin 単位の DOM diff を取るためのアンカー。
- レイアウト座標は不要。HTML block flow が縦方向に積み上げる。`transform="translate(0, ...)"` は classic では使わない MUST (foreignObject 配下なので SVG 座標系は要らない)。

## 3. classic.go の組み立て規約

`classic.Run` の処理順:

```text
1. <svg> + <foreignObject> + <div class="items-wrapper"> を開く
2. _.json の base.* partial を順に Lookup + 呼び出し (M2 既存)
3. **M4 追加**: pluginPartialOrder 配列を順に走査:
   for each slug in pluginPartialOrder:
     a. pc.Inputs["plugin_<slug>"] が truthy でなければ skip
     b. pc.Data.Plugins[slug] が nil / 取得不能なら skip
     c. type assertion で result.Skipped が true なら skip
     d. partials.Lookup("plugin." + slug) で関数取得
        - 未実装の slug (US1〜US3 で順次 land) は skip (warn ログ なし)
     e. 関数を呼び出して fragment string を取得
     f. fragment が空文字なら skip (二重ガード)
     g. <div class="plugin-<slug>" data-plugin="<slug>"> で wrap して書き込み
4. <div id="metrics-end"></div> + footer (M2 既存)
5. </div></foreignObject></svg> で閉じる
```

`pluginPartialOrder` は M4 で導入する `internal/templates/classic/partials/plugins.go` に定数として配置:

```go
// pluginPartialOrder defines the M4 plugin partial render order.
// Mirrors the upstream lowlighter/metrics classic template order.
var pluginPartialOrder = []string{
    "languages", "activity", "achievements", "repositories", "isocalendar",
    "calendar", "habits", "stars", "topics", "starlists", "people",
    "notable", "contributors", "reactions", "projects", "sponsors",
    "sponsorships", "stargazers", "traffic",
}
```

順序は upstream に合わせる (P1 5 個 → P2 + P3 16 個)。

## 4. partial の入力 / Lookup 規約

各 plugin partial は `templates.PartialFunc` を満たす:

```go
type PartialFunc func(ctx context.Context, pc *templates.PartialContext) (string, error)
```

- `pc.Data.Plugins[<slug>]` から自身の Result を type assertion で取り出す (例: `result := pc.Data.Plugins["languages"].(*languages.Result)`)
- `result.Skipped` または truthy gate に該当しないとき空文字を返す (nil-safe / empty-safe)
- `pc.Inputs` から自分の `plugin_<slug>_<opt>` を読み込んで表示に反映する
- 文字列は `partials.EscapeXML` を必ず通す (M2 規約継承)
- 数値は `partials.FormatCount` を必ず通す (M2 規約継承)

partial 関数の命名: パスカルケースの slug + `Plugin` プレフィックスなし (M2 と一貫). 例: `Languages`, `Activity`, `Achievements`. Lookup キーは `"plugin." + slug` (例 `"plugin.languages"`)。

assets 側の `_.json` には M4 plugin slug は **入れない**。理由: M2 base partial は常に呼ばれるので `_.json` で順序定義する価値があるが、M4 plugin partial は inputs ゲート + 条件付き呼び出しなので、Go 側の `pluginPartialOrder` 定数で順序を持つ方が型安全で一致性も保ちやすい。

## 5. 各 partial の DOM 期待値 (テスト assertion 対象)

| Plugin           | 必須 DOM 痕跡 (golden file で検証)                              |
| ---------------- | --------------------------------------------------------------- |
| languages        | `<g class="languages-progress">`, `<rect class="language-bar">` |
| activity         | `<text class="activity-event">`, `<svg class="octicon">`        |
| achievements     | `<svg class="achievement">` または `data-achievement="<rank>"`  |
| repositories     | `<a class="repository">` × ≥ 1                                  |
| isocalendar      | `<g class="calendar">`, `<rect class="calendar-day">`           |
| calendar         | `<g class="calendar-year">` × ≥ 1                               |
| habits           | `<g class="habit-chart">` (charts=true 時)                      |
| stars            | `<a class="starred-repo">` × ≥ 1                                |
| topics           | `<g class="topic">`, `<text class="topic-name">`                |
| starlists        | `<g class="starlist">`, `<text class="starlist-name">`          |
| people           | `<g class="person">`, `<image class="avatar">`                  |
| notable          | `<g class="notable-contrib">`                                   |
| contributors     | `<g class="contributor">` (repository テンプレ用、M4 では空)    |
| reactions        | `<text class="reaction-count">`                                 |
| projects         | `<g class="project">` (skipped=false 時のみ)                    |
| sponsors         | `<g class="sponsor">`                                           |
| sponsorships     | `<g class="sponsored">`                                         |
| stargazers       | `<g class="stargazers-charts">` (worldmap は M4 出さない)       |
| traffic          | `<text class="traffic-count">` (skipped=false 時のみ)           |

## 6. Skipped 時の DOM 出力

`Skipped=true` の plugin に対して partial は **何も出力しない** (空文字列を返す)。`classic.go` 側で `if result.Skipped { return "" }` ガードを早期 return する。これにより SVG に `<g class="plugin-X">` 要素自体が現れない — 上流挙動と一致。

## 7. CSS / octicon 統合

各 partial は M3 で実装済の `:octicon-<name>-<size>:` プレースホルダ記法を使ってよい。`<style data-optimizable="true">` の中に partial 固有 CSS を入れた場合、`svg.optimize.css=true` のとき M3 CSS purge が selector を消す。`<style>` 内のクラス名は partial 内で必ず使う MUST (purge 防止のため自動で `data-plugin` 属性に対応する CSS は予約クラスとして残す)。

## 8. テスト規約

- 各 partial に対し `tests/golden/classic/m4/<name>.svg` を 1 つ commit
- partial 単体テストは `internal/templates/classic/partials_test.go` で `RenderPartial(name, data) -> string` の golden 比較
- partial 内のロジック (条件分岐や loop) は最小限とし、複雑な計算は plugin に寄せる MUST

## 9. classic 全体 golden の再生成

`tests/golden/classic/octocat.svg` (M2 既存) を M4 完了時に再生成する (`go test ./tests/integration/... -run TestComputeSVG_ClassicOctocatGolden -update`)。21 plugin 分の DOM が増えるが、上流互換性 (DOM 単位の同等性、constitution 原則 II) を維持していれば diff は受け入れ可能。
