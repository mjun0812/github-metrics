<!-- AUTOGEN_START: title-and-description -->
# Plugin: topics

This plugin displays [starred topics](https://github.com/stars?filter=topics).
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![topics sample](../examples/plugin-topics.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_topics` | Enable topics plugin | `no` | no | boolean |
| `plugin_topics_mode` | Display mode | `starred` | no | string |
| `plugin_topics_sort` | Sorting method | `stars` | no | string |
| `plugin_topics_limit` | Display limit | `15` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_topics: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_topics=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**At least one starred topic.** Topics can be starred from any topic page (e.g. `https://github.com/topics/go` → "Star" button). Data is fetched by HTTP-scraping `https://github.com/stars/{user}/topics` (public SSR page) — no token is required for public topic stars.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/topics/metadata.yml`](../../assets/plugins/topics/metadata.yml) — upstream metadata
- 対応アカウント種別: user
