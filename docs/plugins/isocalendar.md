<!-- AUTOGEN_START: title-and-description -->

# Plugin: isocalendar

This plugin displays an isometric view of a user commit calendar along with a few additional statistics like current streak and average number of commit per day.

<!-- AUTOGEN_END: title-and-description -->

## Sample

![isocalendar sample](../examples/plugin-isocalendar.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| Input                         | Description               | Default     | Required | Type    |
| ----------------------------- | ------------------------- | ----------- | -------- | ------- |
| `plugin_isocalendar`          | Enable isocalendar plugin | `no`        | no       | boolean |
| `plugin_isocalendar_duration` | Time range                | `half-year` | no       | string  |

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

Same data source as the `calendar` plugin (the user's public contribution calendar). Public contributions within the past year are required for a non-empty heatmap. The `full-year` mode renders 52 weeks of the same dataset.

## Notes

- 表示上は "Commits per day" ですが、データソースは GitHub GraphQL の
  `contributionsCollection.contributionCalendar` の日別 `contributionCount` であり、
  commit 専用カウントではありません (issue / PR / review 等も含む)。upstream
  `lowlighter/metrics` と同じ挙動です。
- private contribution は、ユーザーの GitHub 設定
  "Include private contributions on my profile" が有効な場合に GitHub 側で
  `contributionCount` へ折り込まれます。プラグイン側で public/private の
  フィルタリングは行いません。
- 集計期間は upstream parity です: `half-year` は now−180 日、`full-year` は
  now−1 年を、それぞれ直前の日曜 00:00 UTC へ丸めた日を起点とします。
- カレンダーは upstream と同じく 4 週間単位のチャンクで取得します。GitHub は
  ヒートマップ色 (`ContributionDay.color`) を**クエリ期間内の最大値**で正規化する
  ため、チャンク取得によって upstream と同じ色のグラデーションになります
  (1 年分を一括取得すると大半の日が最薄色に潰れる — #467)。
- GraphQL クライアントが利用できない場合や取得に失敗した場合は、共有の
  indepth カレンダー (過去 1 年) から末尾 26 週 / 53 週をスライスする
  degraded path にフォールバックします。

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/isocalendar/metadata.yml`](../../assets/plugins/isocalendar/metadata.yml) — upstream metadata
- Supported account types: user
- Required scopes: public_access
