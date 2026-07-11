<!-- AUTOGEN_START: title-and-description -->

# Plugin: isocalendar

このプラグインは、ユーザーの commit calendar を isometric 表示し、current streak や 1 日あたりの平均 commit 数などの追加統計をあわせて表示する。

<!-- AUTOGEN_END: title-and-description -->

## Sample

![isocalendar sample](../examples/plugin-isocalendar.svg)

> `--user mjun0812` のデータを、このプラグインのみ有効にしてレンダリングしたもの。`make docs-examples` で再生成できる。

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                         | Description                     | Default     | Required | Type    |
| ----------------------------- | ------------------------------- | ----------- | -------- | ------- |
| `plugin_isocalendar`          | isocalendar plugin を有効にする | `no`        | no       | boolean |
| `plugin_isocalendar_duration` | 集計期間                        | `half-year` | no       | string  |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## Usage

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_isocalendar: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_isocalendar=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Requirements

`calendar` プラグインと同じデータソース (ユーザーの public contribution calendar) を使う。ヒートマップを空にしないためには、過去 1 年以内の public contribution が必要である。`full-year` モードは同じデータセットの 52 週分を描画する。

## Notes

- 表示上は "Commits per day" だが、データソースは GitHub GraphQL の
  `contributionsCollection.contributionCalendar` の日別 `contributionCount` であり、
  commit 専用カウントではない (issue / PR / review 等も含む)。upstream
  `lowlighter/metrics` と同じ挙動である。
- private contribution は、ユーザーの GitHub 設定
  "Include private contributions on my profile" が有効な場合に GitHub 側で
  `contributionCount` へ折り込まれる。プラグイン側で public/private の
  フィルタリングは行わない。
- 集計期間は upstream parity である: `half-year` は now−180 日、`full-year` は
  now−1 年を、それぞれ直前の日曜 00:00 UTC へ丸めた日を起点とする。
- カレンダーは upstream と同じく 4 週間単位のチャンクで取得する。GitHub は
  ヒートマップ色 (`ContributionDay.color`) を**クエリ期間内の最大値**で正規化する
  ため、チャンク取得によって upstream と同じ色のグラデーションになる。1 年分を
  一括取得すると、大半の日が最薄色に潰れてしまう (#467)。
- GraphQL クライアントが利用できない場合や取得に失敗した場合は、共有の
  indepth カレンダー (過去 1 年) から末尾 26 週 / 53 週をスライスする
  degraded path にフォールバックする。

## References

- [`action.yml`](../../action.yml): 正式な input schema
- [`assets/plugins/isocalendar/metadata.yml`](../../assets/plugins/isocalendar/metadata.yml): upstream のメタデータ
- サポートされるアカウント種別: user
- 必要なスコープ: public_access
