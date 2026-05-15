# Contract: classic テンプレート partial の M4 拡張

**Date**: 2026-05-16 | **Plan**: [../plan.md](../plan.md)

本書は classic テンプレートに採用 21 plugin の partial を組み込むための DOM 階層と命名規約を確定する。

## 1. ファイル配置

```text
internal/templates/classic/
├── classic.go                   # 既存。21 plugin 分の partial 呼び出しを追加。
├── classic.svg.tmpl             # 既存。トップレベル <svg> + <foreignObject> 構造。
└── partials/                    # 新規ディレクトリ
    ├── languages.svg.tmpl       # plugin 1 個 = partial 1 個 (21 partial)
    ├── activity.svg.tmpl
    ├── achievements.svg.tmpl
    ├── repositories.svg.tmpl
    ├── isocalendar.svg.tmpl
    ├── calendar.svg.tmpl
    ├── habits.svg.tmpl
    ├── stars.svg.tmpl
    ├── topics.svg.tmpl
    ├── starlists.svg.tmpl
    ├── people.svg.tmpl
    ├── notable.svg.tmpl
    ├── contributors.svg.tmpl
    ├── reactions.svg.tmpl
    ├── projects.svg.tmpl
    ├── sponsors.svg.tmpl
    ├── sponsorships.svg.tmpl
    ├── stargazers.svg.tmpl
    └── traffic.svg.tmpl
```

## 2. partial の DOM 規約

各 partial の output は 1 つの最上位 `<g>` 要素にラップされる:

```svg
<g class="plugin-{{name}}" data-plugin="{{name}}" transform="translate(0, {{offset}})">
  <!-- plugin 固有の DOM -->
</g>
```

- `class` は `plugin-<slug>` で全 plugin 共通の命名。CSS purge の対象クラス。
- `data-plugin` 属性は SVG 出力後に DOM 解析 (data-changed mode、M6) で plugin 単位の diff を取るためのアンカー。
- `transform` の `offset` は classic.go 側で `accum_height` から動的注入する (各 partial の Y 位置 = 前 partial の終端)。partial 内では相対座標で記述。

## 3. classic.go の組み立て規約

```text
1. base のトップレベル要素 (avatar / login / name / bio) を出力
2. pc.Inputs を順に評価:
   - plugin_languages    → partial languages を呼ぶ
   - plugin_activity     → partial activity を呼ぶ
   - ...
3. 各 partial の戻り値 (string) を <g class="plugin-X" transform="translate(0, accum_y)"> で wrap
4. accum_y += partial の bounding box height
5. 全 partial 結合後、トップレベル <svg height="..."> を chromedp 計測 (M3 既存) で確定
```

## 4. partial の入力 (template data)

各 partial は `text/template` の data として以下を受け取る:

```go
type partialData struct {
    Result    any              // plugin 固有 Result (e.g. *languages.Result)
    Inputs    map[string]any   // pc.Inputs 全体 (色設定など UI 入力を参照するため)
    Settings  *config.Settings // settings.json 全体
    Metadata  *config.PluginMetadata // metadata.yml 由来の入力スキーマ
}
```

partial 内では `{{.Result.Favorites}}` のように構造体フィールドを直接参照する。template-side で複雑な計算をしない MUST — 計算は plugin の `Run` で完了させ、partial は表示のみ。

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
