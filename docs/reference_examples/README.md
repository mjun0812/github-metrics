# Reference examples (upstream, 実データ)

このディレクトリは upstream [`lowlighter/metrics`](https://github.com/lowlighter/metrics)
**本体**で生成した参照カードです。`mjun0812` 本人の GitHub データと、リポジトリ
[`mjun0812/flash-attention-prebuild-wheels`](https://github.com/mjun0812/flash-attention-prebuild-wheels)
を対象にしています。

## なぜ別ディレクトリなのか

- [`docs/org_examples`](../org_examples)（= `original_examples`）は upstream の
  `examples` ブランチをそのまま取り込んだもので、**データ主体が lowlighter 本人**です。
  構造比較には使えますが、データが違うため「同じ入力で upstream と Go 実装がどう違うか」
  という apples-to-apples 比較はできません。
- ここは **同じデータ（mjun0812 / flash-attention-prebuild-wheels）** を upstream に
  流し込んだ出力です。[`docs/examples`](../examples)（本プロジェクトの Go 実装出力）と
  並べることで、データを揃えた状態での見た目・DOM の差分を確認できます。

## 生成条件

| 項目 | 値 |
| --- | --- |
| ツール | `ghcr.io/lowlighter/metrics:v3.34`（upstream 本体・Docker イメージ） |
| 実行方法 | ローカル Docker（`docker run --env-file ... :/renders`、action.yml と同一方式） |
| 対象 user | `mjun0812` |
| 対象 repository | `mjun0812/flash-attention-prebuild-wheels`（`template: repository`） |
| 対象プラグイン | **本プロジェクトが Go 実装済みのプラグインのみ**（採用外 plugin は生成しない） |
| `output_action` | `none`（レンダリングのみ。commit はしない） |
| `config_timezone` | `Asia/Tokyo`（一部カード） |
| 生成日 | 2026-05-30 |

> ⚠️ これらは正規化していない **生の** upstream 出力です。フッターの生成タイムスタンプや
> `Metrics` バージョン文字列など動的な部分は再生成ごとに変わります（`docs/examples`
> 側は `normalize-svg-stream` でマスク済みなので、その点だけ差分が出ます）。

## ファイル一覧

### User カード（`user: mjun0812`）

- `metrics.base.svg` / `metrics.classic.svg`
- `metrics.plugin.achievements.svg` / `metrics.plugin.achievements.compact.svg`
- `metrics.plugin.activity.svg`
- `metrics.plugin.calendar.svg` / `metrics.plugin.calendar.full.svg`
- `metrics.plugin.habits.facts.svg` / `metrics.plugin.habits.charts.svg`
- `metrics.plugin.isocalendar.svg` / `metrics.plugin.isocalendar.fullyear.svg`
- `metrics.plugin.languages.svg` / `.details.svg` / `.recent.svg` / `.indepth.svg`
- `metrics.plugin.notable.svg` / `metrics.plugin.notable.indepth.svg`
- `metrics.plugin.people.svg`
- `metrics.plugin.projects.svg`
- `metrics.plugin.reactions.svg`
- `metrics.plugin.repositories.svg` / `metrics.plugin.repositories.pinned.svg`
- `metrics.plugin.sponsors.svg` / `metrics.plugin.sponsorships.svg`
- `metrics.plugin.stargazers.svg` / `metrics.plugin.stargazers.graph.svg`
- `metrics.plugin.stars.svg`
- `metrics.plugin.starlists.svg`
- `metrics.plugin.topics.svg` / `metrics.plugin.topics.icons.svg`
- `metrics.plugin.traffic.svg`

### Repository カード（`repo: flash-attention-prebuild-wheels`, `template: repository`）

- `metrics.repository.svg`
- `metrics.repository.plugin.languages.svg`
- `metrics.repository.plugin.contributors.svg`
- `metrics.repository.plugin.activity.svg`
- `metrics.repository.plugin.people.svg`
- `metrics.repository.plugin.stargazers.svg`
- `metrics.repository.plugin.traffic.svg`

## 再生成

`GITHUB_TOKEN`（`repo` / `read:user` 程度のスコープを持つ PAT）を環境変数に設定し、
upstream イメージへ `INPUT_*` を渡して実行します。例:

```sh
docker run --init --rm -v "$PWD/docs/reference_examples:/renders" \
  -e INPUT_TOKEN="$GITHUB_TOKEN" \
  -e INPUT_OUTPUT_ACTION=none \
  -e INPUT_USER=mjun0812 \
  -e INPUT_BASE= \
  -e INPUT_PLUGIN_LANGUAGES=yes \
  -e INPUT_FILENAME=metrics.plugin.languages.svg \
  ghcr.io/lowlighter/metrics:v3.34
```
