<!-- AUTOGEN_START: title-and-description -->
# プラグイン: activity

このプラグインは GitHub 上での最近の活動を表示します。
<!-- AUTOGEN_END: title-and-description -->

## サンプル

![activity sample](../examples/plugin-activity.svg)

> `--user mjun0812` のデータでこのプラグインのみを有効にしてレンダリングしたものです。`make docs-examples` で再生成できます。

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| 入力 | 説明 | 既定値 | 必須 | 型 |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_activity` | `activity` プラグインを有効化します。 | `no` | no | boolean |
| `plugin_activity_limit` | 表示する件数の上限。 | `5` | no | number |
| `plugin_activity_load` | 読み込むイベント数。 | `300` | no | number |
| `plugin_activity_days` | 対象とするイベントの最大日数。 | `14` | no | number |
| `plugin_activity_visibility` | 表示するイベントの公開範囲。 | `all` | no | string |
| `plugin_activity_timestamps` | イベントごとのタイムスタンプ表示。 | `no` | no | boolean |
| `plugin_activity_skipped` | 集計から除外するリポジトリ。 | `` | no | array |
| `plugin_activity_ignored` | 集計から除外するユーザー。 | `` | no | array |
| `plugin_activity_filter` | 対象とするイベント種別。 | `all` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_activity: yes
```

### CLI

```sh
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_activity=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 参考

- [`action.yml`](../../action.yml) — 入力スキーマのリファレンス
- [`assets/plugins/activity/metadata.yml`](../../assets/plugins/activity/metadata.yml) — upstream 由来の metadata
- 対応アカウント種別: user, organization, repository
- 必要なスコープ: public_access
