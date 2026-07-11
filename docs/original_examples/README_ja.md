# original_examples: upstream `lowlighter/metrics` の出力リファレンス

本ディレクトリは、Go 再実装の参照用に **upstream の公式レンダリング済み SVG** を保存したものである。
本プロジェクトが採用している plugin / テンプレート ([docs/scope.md](../scope_ja.md) §2) に対応する出力だけを抜粋している。

## 出所 (provenance)

- リポジトリ: `lowlighter/metrics` (`org_repo/` の origin)
- ブランチ: `examples`
- コミット: `1dac69e` (`chore: update examples`, 2025-07-03)
- 生成主体: upstream が CI で lowlighter 本人のプロフィールデータをレンダリングした成果物
  (= 自分でトークンを使って再実行したものではなく、upstream が公開している正規サンプル)

取得方法 (ネットワーク不要 / ローカルの fetch 済みブランチから抽出):

```bash
cd org_repo
git show "origin/examples:metrics.plugin.languages.svg" > ../docs/original_examples/metrics.plugin.languages.svg
```

## テンプレート / base

| ファイル                 | 内容                                            |
| ------------------------ | ----------------------------------------------- |
| `metrics.base.svg`       | base/core のみ (プラグインなし) の classic 出力 |
| `metrics.classic.svg`    | classic テンプレートの総合サンプル              |
| `metrics.repository.svg` | repository テンプレートの総合サンプル           |

## 採用 plugin (19 plugin dir) の出力

| plugin         | ファイル                                                                                                                |
| -------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `languages`    | `metrics.plugin.languages.svg` / `.details.svg` / `.recent.svg` (languages.recent) / `.indepth.svg` (languages.indepth) |
| `activity`     | `metrics.plugin.activity.svg`                                                                                           |
| `achievements` | `metrics.plugin.achievements.svg` / `.compact.svg`                                                                      |
| `repositories` | `metrics.plugin.repositories.svg` / `.pinned.svg`                                                                       |
| `isocalendar`  | `metrics.plugin.isocalendar.svg` / `.fullyear.svg`                                                                      |
| `calendar`     | `metrics.plugin.calendar.svg` / `.full.svg`                                                                             |
| `habits`       | `metrics.plugin.habits.charts.svg` / `.facts.svg`                                                                       |
| `stars`        | `metrics.plugin.stars.svg`                                                                                              |
| `topics`       | `metrics.plugin.topics.svg` / `.icons.svg`                                                                              |
| `starlists`    | `metrics.plugin.starlists.svg` / `.languages.svg`                                                                       |
| `people`       | `metrics.plugin.people.followers.svg` / `.repository.svg`                                                               |
| `notable`      | `metrics.plugin.notable.svg` / `.indepth.svg`                                                                           |
| `contributors` | `metrics.plugin.contributors.categories.svg` / `.contributions.svg`                                                     |
| `reactions`    | `metrics.plugin.reactions.svg`                                                                                          |
| `projects`     | `metrics.plugin.projects.svg`                                                                                           |
| `sponsors`     | `metrics.plugin.sponsors.svg` / `.full.svg`                                                                             |
| `sponsorships` | `metrics.plugin.sponsorships.svg`                                                                                       |
| `stargazers`   | `metrics.plugin.stargazers.svg` / `.graph.svg` / `.worldmap.svg` / `.chartist.svg`                                      |
| `traffic`      | `metrics.plugin.traffic.svg`                                                                                            |

## 注意

- **不採用 plugin は含めていない**: `lines` / `gists` / `code` / `introduction` / `followup` /
  `discussions` / `skyline` / `licenses` / `support` および外部 API / community 系 (`wakatime` /
  `anilist` / `chess` 等) は upstream の examples ブランチには存在するが、採用外のため抽出していない。
- **Go 実装では backlog 扱いのバリアント** も参照用に含めている:
  - `metrics.plugin.stargazers.worldmap.svg` … 世界地図は Google Maps API key 必須 (現状 Skipped path)
  - `metrics.plugin.stargazers.chartist.svg` … 別 chart レンダラ
  - organization mode の総合サンプル (`metrics.organization.svg`) は user-mode のみ実装のため未抽出
- これらは **見た目 / レイアウトの参照 (パリティ確認) 用** である。データは lowlighter 本人のもので、
  自分の出力と一致させる用途には使えない。
