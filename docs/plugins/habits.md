<!-- AUTOGEN_START: title-and-description -->
# Plugin: habits

This plugin displays coding habits based on recent activity, such as active hours and languages recently used.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![habits sample](../examples/plugin-habits.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_habits` | Enable habits plugin | `no` | no | boolean |
| `plugin_habits_from` | Events to use | `200` | no | number |
| `plugin_habits_skipped` | Skipped repositories | `` | no | array |
| `plugin_habits_days` | Event maximum age | `14` | no | number |
| `plugin_habits_facts` | Mildly interesting facts | `yes` | no | boolean |
| `plugin_habits_charts` | Charts | `no` | no | boolean |
| `plugin_habits_charts_type` | Charts display type | `classic` | no | string |
| `plugin_habits_trim` | Trim unused hours on charts | `no` | no | boolean |
| `plugin_habits_languages_limit` | Display limit (languages) | `8` | no | number |
| `plugin_habits_languages_threshold` | Display threshold (percentage) | `0%` | no | string |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_habits: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_habits=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/habits/metadata.yml`](../../assets/plugins/habits/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization
- 必要スコープ: public_access
