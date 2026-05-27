# upstream ↔ Go実装 出力比較 (plugin parity)

各 plugin について、**左 = upstream `lowlighter/metrics` の公式出力**、**右 = 本リポジトリ Go 実装の出力** を並べて、レイアウト・DOM 構造のパリティを目視確認するためのページです。

- 左 (upstream): [`docs/original_examples/`](original_examples/) — `lowlighter/metrics` の `examples` ブランチ (commit `1dac69e`)。lowlighter 本人のプロフィールデータをレンダリングした正規サンプル。出所詳細は [original_examples/README.md](original_examples/README.md)。
- 右 (Go 実装): [`docs/examples/`](examples/) — 本リポジトリのサンプル出力。`make docs-samples` (= `scripts/gen-doc-samples.sh`) で `mjun0812` のデータからレンダリング。

## 凡例 / 注意

- ⚠ **データソースが異なるため数値・項目は一致しません**。確認対象は「枠・配色・要素の配置・DOM 構造」が upstream と同等か（= parity）です。
- 📁 **= ファイルサイズが大きく (>500KB) 埋め込むと GitHub 表示が重いためリンク**にしています。クリックで表示してください。
- 一部の SVG は `<foreignObject>` (HTML 埋め込み) を含み、`<img>` 経由だとブラウザ / GitHub 上で HTML 部分が描画されないことがあります。崩れて見える場合はファイルを直接開いて確認してください。
- 表示幅は `width="420"` で統一しています（原寸は各ファイル参照）。
- **空カードについて**: 右の Go 実装サンプルは `mjun0812` のデータを使うため、本人に該当データが無い plugin（topics / starlists / sponsors / projects 等）は枠だけの空表示になります。これは未実装ではなく「サンプルユーザーにデータが無い」ためです。

## variant（サブモード）の Go 実装対応状況

upstream に存在する各 plugin のサブモードについて、Go 実装の対応状況です。

| upstream variant                 | Go 側                   | 備考                                                                                                                |
| -------------------------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `languages.recent`               | ✅ 比較可               | `plugin-languages-recent.svg`                                                                                       |
| `languages.indepth`              | ✅ 比較可               | `plugin-languages-indepth.svg`                                                                                      |
| `languages.details`              | ✅ 比較可               | `plugin-languages-details.svg`（本対応で追加）                                                                      |
| `achievements.compact`           | ✅ 比較可               | `plugin-achievements-compact.svg`（本対応で追加）                                                                   |
| `isocalendar.fullyear`           | ✅ 比較可               | `plugin-isocalendar-fullyear.svg`（本対応で追加）                                                                   |
| `calendar.full`                  | ◐ 対応・差分なし        | `plugin_calendar_limit=0` を受け付けるが、このサンプルデータでは無印とバイト同一                                    |
| `repositories.pinned`            | ◐ 対応・差分なし        | `plugin_repositories_pinned` を受け付けるが、このサンプルデータでは無印とバイト同一                                 |
| `topics.icons`                   | ○ データ無し            | `plugin_topics_mode=icons` 対応。サンプルユーザーに topics 無しで空                                                 |
| `starlists.languages`            | ○ データ無し            | `plugin_starlists_languages` 対応。サンプルユーザーにデータ無しで空                                                 |
| `sponsors.full`                  | ○ データ無し            | `plugin_sponsors_sections` 対応。サンプルユーザーにデータ無しで空                                                   |
| `habits.facts` / `habits.charts` | ✅ 比較可               | `plugin_habits_facts` / `plugin_habits_charts` で個別トグル。`plugin-habits-facts.svg` / `plugin-habits-charts.svg` |
| `notable.indepth`                | ✅ 比較可               | `plugin_notable_indepth` 対応。`@owner/repo` 粒度チップ + 統計ゲージ。`plugin-notable-indepth.svg`                  |
| `contributors.contributions`     | ◐ 実装済み（repo 専用） | `plugin_contributors_contributions` 対応。repository template 専用のため user サンプルなし                          |
| `stargazers.graph`               | ✅ 比較可               | `plugin_stargazers_charts_type=graph` 対応。`plugin-stargazers-graph.svg`                                           |
| `stargazers.worldmap`            | ✗ 未対応                | backlog（Google Maps API、R-012 Skipped path）                                                                      |
| `stargazers.chartist`            | ◐ graph と同一          | `charts_type=chartist` は deprecated alias。`graph` とバイト同一出力                                                |
| `people.repository`              | ◐ 実装済み（repo 専用） | contributors/stargazers/watchers 対応。repository template 専用のため user サンプルなし                             |

✅ = 左右比較可 / ◐ = Go は対応するがサンプル差分なし・または repository 専用 / deprecated alias でサンプル省略 / ○ = Go は対応するがサンプルユーザーにデータ無し（空） / ✗ = Go 実装が未対応（要実装・backlog）

---

## テンプレート / base

upstream の参考表示と Go 実装側の対応サンプルを並べています。`classic 総合` は upstream の `metrics.classic.svg` (= 実質 base ヘッダのみ) に対し、Go 側は採用 19 plugin のうち `mjun0812` で非空となる主要 12 plugin (`isocalendar` / `calendar` / `languages` / `activity` / `achievements` / `notable` / `repositories` / `habits` / `stars` / `reactions` / `stargazers` / `traffic`) を合成した overview を表示します。`repository 総合` は repository template 専用のため Go の user mode サンプルはありません。

| 種別               | upstream                                                         | Go 実装                                                       |
| ------------------ | ---------------------------------------------------------------- | ------------------------------------------------------------- |
| base (plugin なし) | <img src="original_examples/metrics.base.svg" width="420">       | <img src="examples/plugin-base.svg" width="420">              |
| classic 総合       | <img src="original_examples/metrics.classic.svg" width="420">    | <img src="examples/metrics-classic.svg" width="420">          |
| repository 総合    | <img src="original_examples/metrics.repository.svg" width="420"> | （repository template 専用、user mode 用サンプルなし）        |

> ✅ Go サンプル `metrics-classic.svg` / `.png` は `scripts/gen-doc-samples.sh` の独立セクションで生成。複数 plugin を 1 回の `render_one` で合成。データ無しで空表示になる plugin (`contributors` / `projects` / `sponsors` / `sponsorships` / `starlists` / `topics`) と巨大化する `people` は除外し、合成 SVG は ~370KB / 高さ ~3976px。

---

## plugin 比較

### languages

| upstream                                                               | Go 実装 (`plugin-languages.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.svg" width="420"> | <img src="examples/plugin-languages.svg" width="420"> |

**variant: details** — upstream `languages.details` ↔ Go `plugin-languages-details.svg`

| upstream                                                                       | Go 実装                                                       |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="examples/plugin-languages-details.svg" width="420"> |

### languages.recent

| upstream                                                                      | Go 実装 (`plugin-languages-recent.svg`)                      |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.languages.recent.svg" width="420"> | <img src="examples/plugin-languages-recent.svg" width="420"> |

### languages.indepth

| upstream                                                                       | Go 実装 (`plugin-languages-indepth.svg`)                      |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="examples/plugin-languages-indepth.svg" width="420"> |

### activity

| upstream                                                              | Go 実装 (`plugin-activity.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.activity.svg" width="420"> | <img src="examples/plugin-activity.svg" width="420"> |

### achievements

| upstream                                                                  | Go 実装                                                  |
| ------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.svg" width="420"> | <img src="examples/plugin-achievements.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.achievements.compact.svg" width="420"> (compact)

> ✅ Go 側は `plugin_achievements_display=compact` に対応。Go サンプルは `scripts/gen-doc-samples.sh` で `plugin-achievements-compact.svg` / `.png` として生成。

### repositories

| upstream                                                                  | Go 実装 (`plugin-repositories.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.svg" width="420"> | <img src="examples/plugin-repositories.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.repositories.pinned.svg" width="420"> (pinned)

> ◐ Go 側は `plugin_repositories_pinned` を受け付けるが、サンプルデータでは無印と同一出力のため別サンプルなし。

### isocalendar

| upstream                                                                 | Go 実装 (`plugin-isocalendar.svg`)                      |
| ------------------------------------------------------------------------ | ------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="examples/plugin-isocalendar.svg" width="420"> |

**variant: fullyear** — upstream `isocalendar.fullyear` ↔ Go `plugin-isocalendar-fullyear.svg`

| upstream                                                                          | Go 実装                                                          |
| --------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="examples/plugin-isocalendar-fullyear.svg" width="420"> |

### calendar

| upstream                                                              | Go 実装 (`plugin-calendar.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.svg" width="420"> | <img src="examples/plugin-calendar.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.calendar.full.svg" width="420"> (full)

> ◐ Go 側は `plugin_calendar_limit=0` を受け付けるが、サンプルデータでは無印と同一出力のため別サンプルなし。

### habits

| upstream (charts)                                                          | Go 実装 (`plugin-habits.svg`)                      |
| -------------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | <img src="examples/plugin-habits.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.habits.facts.svg" width="420"> (facts)

> ✅ Go 側は `plugin_habits_facts` / `plugin_habits_charts` で facts / charts を個別トグル。Go サンプル: `plugin-habits-facts.svg`（facts のみ）/ `plugin-habits-charts.svg`（charts のみ）。

### stars

| upstream                                                           | Go 実装 (`plugin-stars.svg`)                      |
| ------------------------------------------------------------------ | ------------------------------------------------- |
| <img src="original_examples/metrics.plugin.stars.svg" width="420"> | <img src="examples/plugin-stars.svg" width="420"> |

### topics

| upstream                                                            | Go 実装 (`plugin-topics.svg`)                      |
| ------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.topics.svg" width="420"> | <img src="examples/plugin-topics.svg" width="420"> |

他バリアント (upstream): 📁 [topics.icons (1.1 MB)](original_examples/metrics.plugin.topics.icons.svg)

> ○ Go 側は `plugin_topics_mode=icons` 対応だが、サンプルユーザーに topics 無しで空表示。
> 実装メモ: upstream は puppeteer で `/stars/<user>/topics` をスクレイピングするが、Go 実装は公開 SSR ページを HTTP + goquery で取得する (chromium 不要)。

### starlists

| upstream                                                               | Go 実装 (`plugin-starlists.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.svg" width="420"> | <img src="examples/plugin-starlists.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.starlists.languages.svg" width="420"> (languages)

> ○ Go 側は `plugin_starlists_languages` 対応だが、サンプルユーザーにデータ無しで空表示。
> 実装メモ: upstream は puppeteer で `/stars/<user>/lists` をスクレイピングするが、Go 実装は GitHub GraphQL の `user.lists` を 1 クエリで取得する (chromium 不要)。

### people

Go 実装の `plugin-people.svg` は 5.8MB のため埋め込まずリンクにしています。

| upstream (followers)                                                          | Go 実装                                                     |
| ----------------------------------------------------------------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.followers.svg" width="420"> | 📁 [plugin-people.svg (5.8 MB)](examples/plugin-people.svg) |

他バリアント (upstream): 📁 [people.repository (4.0 MB)](original_examples/metrics.plugin.people.repository.svg)

> ◐ Go 側は user mode (followers/following) に加え repository context types (contributors/stargazers/watchers) を実装済み。repository template 専用のため user モードの plugin 単体サンプルは生成していない。

### notable

| upstream                                                             | Go 実装 (`plugin-notable.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.svg" width="420"> | <img src="examples/plugin-notable.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.notable.indepth.svg" width="420"> (indepth)

> ✅ Go 側は `plugin_notable_indepth` 対応済み。indepth は基本モードの組織単位 (`@org`) ではなくリポジトリ単位 (`@org/repo`) のチップを描画し、commits / stars / issues / pulls の統計ゲージを付与する（upstream `notable.ejs` と同一 DOM）。Go サンプル: `plugin-notable-indepth.svg`。

### contributors

upstream の出力は両バリアントとも巨大 (9.9 / 8.7 MB) のためリンクにしています。

| upstream                                                                                            | Go 実装 (`plugin-contributors.svg`)                      |
| --------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| 📁 [contributors.categories (9.9 MB)](original_examples/metrics.plugin.contributors.categories.svg) | <img src="examples/plugin-contributors.svg" width="420"> |

他バリアント (upstream): 📁 [contributors.contributions (8.7 MB)](original_examples/metrics.plugin.contributors.contributions.svg)

> ◐ Go 側は `plugin_contributors_contributions` 対応済み（per-contributor commits / additions / deletions）。repository template 専用のため user モードの plugin 単体サンプルは生成していない。

### reactions

| upstream                                                               | Go 実装 (`plugin-reactions.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.reactions.svg" width="420"> | <img src="examples/plugin-reactions.svg" width="420"> |

### projects

| upstream                                                              | Go 実装 (`plugin-projects.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.projects.svg" width="420"> | <img src="examples/plugin-projects.svg" width="420"> |

### sponsors

| upstream                                                              | Go 実装 (`plugin-sponsors.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="examples/plugin-sponsors.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.sponsors.full.svg" width="420"> (full)

> ○ Go 側は `plugin_sponsors_sections=goal,about,list` で full 相当を表現可能だが、サンプルユーザーにデータ無しで空表示。

### sponsorships

| upstream                                                                  | Go 実装 (`plugin-sponsorships.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="examples/plugin-sponsorships.svg" width="420"> |

### stargazers

| upstream                                                                | Go 実装 (`plugin-stargazers.svg`)                      |
| ----------------------------------------------------------------------- | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="examples/plugin-stargazers.svg" width="420"> |

他バリアント (upstream): <img src="original_examples/metrics.plugin.stargazers.graph.svg" width="420"> (graph) / <img src="original_examples/metrics.plugin.stargazers.chartist.svg" width="420"> (chartist) / 📁 [stargazers.worldmap (1.5 MB)](original_examples/metrics.plugin.stargazers.worldmap.svg)

> ✅ Go 側は `plugin_stargazers_charts_type=graph` 対応（`chartist` は graph の deprecated alias でバイト同一）。Go サンプル: `plugin-stargazers-graph.svg`。worldmap は Google Maps API 必須の backlog（Skipped path、未対応）。

### traffic

| upstream                                                             | Go 実装 (`plugin-traffic.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.traffic.svg" width="420"> | <img src="examples/plugin-traffic.svg" width="420"> |
