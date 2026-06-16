# upstream ↔ Go実装 出力比較 (plugin parity)

各 plugin について、**左 = upstream `lowlighter/metrics` の公式出力 (lowlighter データ)**、**中 = upstream を `mjun0812` データで実行した参照出力**、**右 = 本リポジトリ Go 実装の出力 (`mjun0812` データ)** を 3 列で並べて、レイアウト・DOM 構造のパリティを目視確認するためのページです。

- 左 (upstream / lowlighter): [`docs/original_examples/`](original_examples/) — `lowlighter/metrics` の `examples` ブランチ (commit `1dac69e`)。lowlighter 本人のプロフィールデータをレンダリングした正規サンプル。出所詳細は [original_examples/README.md](original_examples/README.md)。
- 中 (upstream / mjun0812): [`docs/reference_examples/`](reference_examples/) — upstream `lowlighter/metrics@v3.34` を **`mjun0812` 本人のデータ**で実行した参照出力。右の Go 実装と**同一データ**のため、データを揃えた状態での見た目・DOM 差分 (apples-to-apples) を確認できます。生成方法・欠番の理由は [reference_examples/README.md](reference_examples/README.md)。
- 右 (Go 実装 / mjun0812): [`docs/examples/`](examples/) — 本リポジトリのサンプル出力。`make docs-samples` (= `scripts/gen-doc-samples.sh`) で `mjun0812` のデータからレンダリング。

## 凡例

- ⚠ 左列だけはデータソースが異なるため数値・項目は一致しません（lowlighter vs mjun0812）。中列と右列は同一データなので項目レベルで一致するはずです。確認対象は「枠・配色・要素の配置・DOM 構造」が upstream と同等か (= parity) です。
- 🖼️ すべてのサンプルはサイズに関わらず `<img width="420">` で埋め込んでいます（原寸は各ファイル参照）。
- 一部の SVG は `<foreignObject>` を含み、`<img>` 経由ではブラウザ / GitHub 上で HTML 部分が描画されないことがあります。崩れて見える場合はファイルを直接開いてください。
- 中列と右列はともに `mjun0812` のデータを使うため、本人に該当データが無い plugin（topics / starlists / sponsors 等）は枠だけの空表示になります。
- 中列の `— 該当なし` は upstream v3.34 のエラー・API 廃止・既知バグで生成できなかったものです（詳細は [reference_examples/README.md](reference_examples/README.md) 参照）。

## variant（サブモード）の Go 実装対応状況

| upstream variant                 | Go 側                  | 備考                                                                                                                                           |
| -------------------------------- | ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `languages.recent`               | ✅ 比較可              | `plugin-languages-recent.svg`                                                                                                                  |
| `languages.indepth`              | ✅ 比較可              | `plugin-languages-indepth.svg`                                                                                                                 |
| `languages.details`              | ✅ 比較可              | `plugin-languages-details.svg`                                                                                                                 |
| `achievements.compact`           | ✅ 比較可              | `plugin-achievements-compact.svg`                                                                                                              |
| `isocalendar.fullyear`           | ✅ 比較可              | `plugin-isocalendar-fullyear.svg`                                                                                                              |
| `calendar.full`                  | ✅ 比較可              | `plugin-calendar-full.svg` (`plugin_calendar_limit=0` で全期間)                                                                                |
| `repositories.pinned`            | ✅ 配線済              | `plugin_repositories_pinned=yes` で `viewer.pinnedItems` を取得し Featured の後ろに重複除外して描画                                            |
| `topics.icons`                   | ○ データ無し           | `plugin_topics_mode=icons` 対応。サンプルユーザーに topics 無しで空                                                                            |
| `starlists.languages`            | ○ データ無し           | `plugin_starlists_languages` 対応。サンプルユーザーにデータ無しで空                                                                            |
| `sponsors.full`                  | ○ データ無し           | `plugin_sponsors_sections` 対応。サンプルユーザーにデータ無しで空                                                                              |
| `habits.facts` / `habits.charts` | ✅ 比較可              | `plugin_habits_facts` / `plugin_habits_charts` で個別トグル                                                                                    |
| `notable.indepth`                | ✅ 比較可              | `plugin_notable_indepth` で `@owner/repo` 粒度チップ + 統計ゲージ                                                                              |
| `contributors.contributions`     | ✅ 比較可（repo mode） | `plugin_contributors_contributions` 対応 (adds/dels 列付き)                                                                                    |
| `stargazers.graph`               | ✅ 比較可              | `plugin_stargazers_charts_type=graph` 対応                                                                                                     |
| `stargazers.chartist`            | ◐ graph と同一         | deprecated alias。`graph` とバイト同一出力                                                                                                     |
| `stargazers.worldmap`            | ✗ 未対応               | backlog (Google Maps API)                                                                                                                      |
| `people.repository`              | ✅ 比較可（repo mode） | contributors / stargazers / watchers 対応                                                                                                      |

✅ = 左右比較可 / ◐ = サンプル差分なし or deprecated alias / ○ = Go は対応するがサンプルユーザーにデータ無し / ✗ = 未対応 (backlog)

---

## テンプレート / base

`classic 総合` は upstream `metrics.classic.svg` に合わせ、plugin トグルなしの classic/base 出力です。`repository 総合` は `--template repository --repo mjun0812/flash-attention-prebuild-wheels` の **base 出力**で、upstream `metrics.repository.svg` と apples-to-apples になるよう plugin トグルなしの base chrome のみを描画します。

| 種別               | upstream (lowlighter)                                            | upstream (mjun0812)                                               | Go 実装 (mjun0812)                                      |
| ------------------ | ---------------------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------- |
| base (plugin なし) | <img src="original_examples/metrics.base.svg" width="420">       | <img src="reference_examples/metrics.base.svg" width="420">       | <img src="examples/plugin-base.svg" width="420">        |
| classic 総合       | <img src="original_examples/metrics.classic.svg" width="420">    | <img src="reference_examples/metrics.classic.svg" width="420">    | <img src="examples/metrics-classic.svg" width="420">    |
| repository 総合    | <img src="original_examples/metrics.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.svg" width="420"> | <img src="examples/metrics-repository.svg" width="420"> |

### base partial parity (`base.header` / `base.repositories`)

`base.header.ejs` / `base.repositories.ejs` で upstream が描画する各フィールドの本リポジトリ実装状況です。両 partial は全サンプル SVG が embed する基盤のため、ここのカバレッジが上がると全サンプルの見た目が改善します。

| partial             | フィールド                           | 状態         | データソース                                                           |
| ------------------- | ------------------------------------ | ------------ | ---------------------------------------------------------------------- |
| `base.header`       | Avatar + display name                | ✅           | `user.{avatarUrl,name,login}`                                          |
| `base.header`       | Joined GitHub `<age>`                | ✅           | `user.createdAt`                                                       |
| `base.header`       | Followed by N users                  | ✅           | `user.followers.totalCount`                                            |
| `base.header`       | Following N users                    | ✅           | `user.following.totalCount`                                            |
| `base.header`       | Contributed to N repositories        | ✅           | `user.repositoriesContributedTo.totalCount`                            |
| `base.header`       | Contribution calendar 11×7 mini grid | ✅           | `user.contributionsCollection.contributionCalendar.weeks` (末尾 11 週) |
| `base.repositories` | N repositories                       | ✅           | `Computed.Repositories.Count`                                          |
| `base.repositories` | N stargazers                         | ✅           | `Computed.Repositories.Stargazers`                                     |
| `base.repositories` | N forks                              | ✅           | `Computed.Repositories.Forks`                                          |
| `base.repositories` | Watching N repositories              | ✅           | `user.watching.totalCount`                                             |
| `base.repositories` | N sponsors                           | ✅           | `user.sponsorshipsAsMaintainer.totalCount`                             |
| `base.repositories` | N releases                           | ✅           | `repository.releases.totalCount` の合算                                |
| `base.repositories` | N packages                           | ✅           | `repository.packages.totalCount` の合算                                |
| `base.repositories` | `<disk-usage>` used                  | ✅           | `repository.diskUsage` (KB) の合算                                     |
| `base.repositories` | License preference (top 3)           | ✅           | `repository.licenseInfo` の集計 + 上位 N                               |
| n/a (out of scope)  | `+N added` / `-N removed`            | ✗ 永久対象外 | M8 不採用の `lines` plugin                                             |

---

## plugin 比較

### languages

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-languages.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.svg" width="420"> | <img src="examples/plugin-languages.svg" width="420"> |

**variant: details** — upstream `languages.details` ↔ Go `plugin-languages-details.svg`

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go 実装                                                       |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="examples/plugin-languages-details.svg" width="420"> |

### languages.recent

| upstream (lowlighter)                                                         | upstream (mjun0812)                 | Go 実装 (`plugin-languages-recent.svg`)                      |
| ----------------------------------------------------------------------------- | ----------------------------------- | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.languages.recent.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-languages-recent.svg" width="420"> |

### languages.indepth

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go 実装 (`plugin-languages-indepth.svg`)                      |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="examples/plugin-languages-indepth.svg" width="420"> |

### activity

| upstream (lowlighter)                                                 | upstream (mjun0812)                 | Go 実装 (`plugin-activity.svg`)                      |
| --------------------------------------------------------------------- | ----------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.activity.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-activity.svg" width="420"> |

### achievements

| upstream (lowlighter)                                                     | upstream (mjun0812)                     | Go 実装                                                  |
| ------------------------------------------------------------------------- | --------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-achievements.svg" width="420"> |

**variant: compact** — upstream `achievements.compact` ↔ Go `plugin-achievements-compact.svg`

| upstream (lowlighter)                                                             | upstream (mjun0812)                     | Go 実装                                                          |
| --------------------------------------------------------------------------------- | --------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.compact.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-achievements-compact.svg" width="420"> |

### repositories

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装 (`plugin-repositories.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.svg" width="420"> | <img src="examples/plugin-repositories.svg" width="420"> |

**variant: pinned** — upstream `repositories.pinned` ↔ Go `plugin-repositories-pinned.svg`

| upstream (lowlighter)                                                            | upstream (mjun0812)                                                               | Go 実装                                                         |
| -------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.pinned.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.pinned.svg" width="420"> | <img src="examples/plugin-repositories-pinned.svg" width="420"> |

### isocalendar

| upstream (lowlighter)                                                    | upstream (mjun0812)                                                       | Go 実装 (`plugin-isocalendar.svg`)                      |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------- | ------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="examples/plugin-isocalendar.svg" width="420"> |

**variant: fullyear** — upstream `isocalendar.fullyear` ↔ Go `plugin-isocalendar-fullyear.svg`

| upstream (lowlighter)                                                             | upstream (mjun0812)                                                                | Go 実装                                                          |
| --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="examples/plugin-isocalendar-fullyear.svg" width="420"> |

### calendar

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go 実装 (`plugin-calendar.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.svg" width="420"> | <img src="examples/plugin-calendar.svg" width="420"> |

**variant: full** — upstream `calendar.full` ↔ Go `plugin-calendar-full.svg`

| upstream (lowlighter)                                                      | upstream (mjun0812)                                                         | Go 実装                                                   |
| -------------------------------------------------------------------------- | --------------------------------------------------------------------------- | --------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.full.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.full.svg" width="420"> | <img src="examples/plugin-calendar-full.svg" width="420"> |

### habits

| upstream (lowlighter, charts)                                              | upstream (mjun0812)                 | Go 実装 (`plugin-habits.svg`)                      |
| -------------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits.svg" width="420"> |

**variant: facts / charts** — upstream `habits.facts` / `habits.charts` ↔ Go `plugin-habits-facts.svg` / `plugin-habits-charts.svg`

| upstream (lowlighter, facts)                                              | upstream (mjun0812)                 | Go 実装 (facts)                                          |
| ------------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.facts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits-facts.svg" width="420"> |

| upstream (lowlighter, charts)                                              | upstream (mjun0812)                 | Go 実装 (charts)                                          |
| -------------------------------------------------------------------------- | ----------------------------------- | --------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits-charts.svg" width="420"> |

### stars

| upstream (lowlighter)                                              | upstream (mjun0812)                                                 | Go 実装 (`plugin-stars.svg`)                      |
| ------------------------------------------------------------------ | ------------------------------------------------------------------- | ------------------------------------------------- |
| <img src="original_examples/metrics.plugin.stars.svg" width="420"> | <img src="reference_examples/metrics.plugin.stars.svg" width="420"> | <img src="examples/plugin-stars.svg" width="420"> |

### topics

| upstream (lowlighter)                                               | upstream (mjun0812)                                                  | Go 実装 (`plugin-topics.svg`)                      |
| ------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.topics.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.svg" width="420"> | <img src="examples/plugin-topics.svg" width="420"> |

**variant: icons** — upstream `topics.icons`

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装                                                |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.topics.icons.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.icons.svg" width="420"> | — 別サンプルなし（サンプルユーザーに topics 無しで空） |

### starlists

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-starlists.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.svg" width="420"> | <img src="reference_examples/metrics.plugin.starlists.svg" width="420"> | <img src="examples/plugin-starlists.svg" width="420"> |

**variant: languages** — upstream `starlists.languages`

| upstream (lowlighter)                                                            | upstream (mjun0812) | Go 実装                                                     |
| -------------------------------------------------------------------------------- | ------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.languages.svg" width="420"> | — 該当なし          | — 別サンプルなし（サンプルユーザーに starlists データ無し） |

### people

| upstream (lowlighter, followers)                                              | upstream (mjun0812)                                                  | Go 実装 (`plugin-people.svg`)                      |
| ----------------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.followers.svg" width="420"> | <img src="reference_examples/metrics.plugin.people.svg" width="420"> | <img src="examples/plugin-people.svg" width="420"> |

**variant: repository** — upstream `people.repository` ↔ Go `plugin-people-repo-types.svg`

| upstream (lowlighter, repository)                                              | upstream (mjun0812, repo)                                                       | Go 実装 (repo mode)                                           |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420"> | <img src="examples/plugin-people-repo-types.svg" width="420"> |

### notable

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go 実装 (`plugin-notable.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.svg" width="420"> | <img src="examples/plugin-notable.svg" width="420"> |

**variant: indepth** — upstream `notable.indepth` ↔ Go `plugin-notable-indepth.svg`

| upstream (lowlighter)                                                        | upstream (mjun0812)                                                           | Go 実装                                                     |
| ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="examples/plugin-notable-indepth.svg" width="420"> |

### contributors

`contributors` は **repository mode 専用** plugin です (user mode では mode gate により Skipped)。Go 実装列は repo mode サンプル `plugin-contributors-repo-contributions.svg` を代表サンプルとして掲載しています。

| upstream (lowlighter, categories)                                                    | upstream (mjun0812, repo)                                                             | Go 実装 (repo mode)                                                         |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.categories.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

**variant: contributions** — upstream `contributors.contributions` ↔ Go `plugin-contributors-repo-contributions.svg`

| upstream (lowlighter, contributions)                                                    | upstream (mjun0812, repo)                                                             | Go 実装 (repo mode)                                                         |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.contributions.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

### reactions

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-reactions.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.reactions.svg" width="420"> | <img src="reference_examples/metrics.plugin.reactions.svg" width="420"> | <img src="examples/plugin-reactions.svg" width="420"> |

### projects

| upstream (lowlighter)                                                 | upstream (mjun0812)                     | Go 実装 (`plugin-projects.svg`)                      |
| --------------------------------------------------------------------- | --------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.projects.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-projects.svg" width="420"> |

### sponsors

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go 実装 (`plugin-sponsors.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="examples/plugin-sponsors.svg" width="420"> |

**variant: full** — upstream `sponsors.full`

| upstream (lowlighter)                                                      | upstream (mjun0812) | Go 実装                                                    |
| -------------------------------------------------------------------------- | ------------------- | ---------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.full.svg" width="420"> | — 該当なし          | — 別サンプルなし（サンプルユーザーに sponsors データ無し） |

### sponsorships

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装 (`plugin-sponsorships.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="examples/plugin-sponsorships.svg" width="420"> |

### stargazers

| upstream (lowlighter)                                                   | upstream (mjun0812)                                                      | Go 実装 (`plugin-stargazers.svg`)                      |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="examples/plugin-stargazers.svg" width="420"> |

**variant: graph** — upstream `stargazers.graph` ↔ Go `plugin-stargazers-graph.svg`

| upstream (lowlighter)                                                         | upstream (mjun0812)                                                            | Go 実装                                                      |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="examples/plugin-stargazers-graph.svg" width="420"> |

**variant: chartist / worldmap** — upstream `stargazers.chartist` / `stargazers.worldmap`

| variant  | upstream (lowlighter)                                                            | upstream (mjun0812) | Go 実装                                                      |
| -------- | -------------------------------------------------------------------------------- | ------------------- | ------------------------------------------------------------ |
| chartist | <img src="original_examples/metrics.plugin.stargazers.chartist.svg" width="420"> | — 該当なし          | — 別サンプルなし（`graph` の deprecated alias でバイト同一） |
| worldmap | <img src="original_examples/metrics.plugin.stargazers.worldmap.svg" width="420"> | — 該当なし          | — 未対応（Google Maps API 必須の backlog）                   |

### traffic

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go 実装 (`plugin-traffic.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.traffic.svg" width="420"> | <img src="reference_examples/metrics.plugin.traffic.svg" width="420"> | <img src="examples/plugin-traffic.svg" width="420"> |

Go 側は user mode で token owner が admin の全 repo に対して `/traffic/views` を並列取得し、`base.repositories` の右列に合計 views を統合表示します。traffic plugin 単体 partial は空文字列を返すため、`plugin-traffic.svg` は `base=repositories` と組み合わせて生成します。

---

## repository mode サンプル一覧

`--template repository --user mjun0812 --repo flash-attention-prebuild-wheels` で生成した repository-mode サンプル一覧です。`make docs-samples` で `scripts/gen-doc-samples.sh` が自動再生成します。

`--template repository` は `assets/templates/repository/partials/_.json` の固定 partial 順 (`base.header` → `introduction` → `followup` → `languages` → `projects` → `pagespeed` → `stargazers` → `people` → `activity` → `posts` → `rss` → `screenshot` → `stock` → `crypto` → `contributors` → `sponsors` → `licenses`) でレンダリングします。base 系セクション (`base.header` / `introduction` / `activity` / `contributors`) は `base.runRepository` が常に値を populate するため、`--plugin <slug>=yes` を渡さなくても chrome として表示されます。`languages` は `plugin_languages=yes` を明示しない限り `Run()` 冒頭の gate で Skipped となります (upstream parity)。

`--plugin <slug>=yes` トグルが追加の効果を持つ plugin は **`mjun0812/flash-attention-prebuild-wheels` で実測した限り** 以下に限定されます:

| 効果あり (repo mode で出力が変わる)                                                                                                                     | 効果なし (chrome と byte 同一になる)                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `people`<br>新規 `<section data-section="people">` を追加<br><br>`contributors_contributions`<br>chrome の contributors セクションに adds/dels 列を追加 | `achievements` / `activity` / `calendar` / `habits`<br>`isocalendar` / `notable` / `reactions` / `repositories`<br>`sponsorships` / `starlists` / `stars` / `topics`<br><br>`contributors`: chrome 側ですでに描画<br>`languages`: `plugin_languages=yes` 明示が必要<br>`projects` / `sponsors`: データ無し<br>`stargazers`: M7 MVP は totals のみで partial は空文字列<br>`traffic`: partial 未登録 |

「効果なし」側の plugin は `plugin-<slug>-repo.svg` を生成しても `plugin-base-repo.svg` と byte 同一になるため `docs/examples/` には残していません。

user mode 専用 plugin を `--template repository` で起動すると、各 plugin の `Run()` は `plugins.RequireUserMode()` で descriptive な reason をログに出して即 Skipped を返します。逆向きの `contributors` plugin（repository mode 専用）は `RequireRepoMode()` で同様の WARN を出します。

### 単体サンプル

| plugin | sample                            | 備考                                                                   |
| ------ | --------------------------------- | ---------------------------------------------------------------------- |
| base   | `examples/plugin-base-repo.svg`   | repository template chrome のみ（plugin toggle なし）                  |
| people | `examples/plugin-people-repo.svg` | `<section data-section="people">` 追加（既定 = stargazers + watchers） |

### 追加バリアント

| variant                             | sample                                                | 内容                                                   |
| ----------------------------------- | ----------------------------------------------------- | ------------------------------------------------------ |
| `contributors.contributions` (repo) | `examples/plugin-contributors-repo-contributions.svg` | `/stats/contributors` 経由で adds/dels 列を表示        |
| `people.types` (repo)               | `examples/plugin-people-repo-types.svg`               | stargazers + watchers + contributors を 1 カードで併記 |

### 総合サンプル

| sample                            | 内容                                                                                                                                                                                                  |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `examples/metrics-repository.svg` | repository template の base 出力。plugin トグルなしで upstream `metrics.repository.svg` と同じ base chrome のみ (repository 名 + `Created` / `Deployed` / disk-usage / 貢献カレンダー / `Environments`) |

### repository mode の upstream(mjun0812) ↔ Go 実装 比較

`mjun0812/flash-attention-prebuild-wheels` を `--template repository` で実行した、中列 (upstream / mjun0812) と右列 (Go 実装) の比較です。

| 種別            | upstream (mjun0812)                                                                   | Go 実装                                                                     |
| --------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| repository 総合 | <img src="reference_examples/metrics.repository.svg" width="420">                     | <img src="examples/metrics-repository.svg" width="420">                     |
| languages       | <img src="reference_examples/metrics.repository.plugin.languages.svg" width="420">    | — 別サンプルなし（`plugin_languages=yes` 明示が必要）                       |
| contributors    | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |
| people          | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420">       | <img src="examples/plugin-people-repo-types.svg" width="420">               |
| stargazers      | <img src="reference_examples/metrics.repository.plugin.stargazers.svg" width="420">   | <img src="examples/plugin-base-repo.svg" width="420">                       |
