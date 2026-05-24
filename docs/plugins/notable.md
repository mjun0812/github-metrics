<!-- AUTOGEN_START: title-and-description -->
# Plugin: notable

This plugin displays badges for notable contributions on repositories.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![notable sample](../examples/plugin-notable.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_notable` | Enable notable plugin | `no` | no | boolean |
| `plugin_notable_filter` | Query filter | `` | no | string |
| `plugin_notable_skipped` | Skipped repositories | `` | no | array |
| `plugin_notable_from` | Repository owner account type filter | `organization` | no | string |
| `plugin_notable_repositories` | Repository name | `no` | no | boolean |
| `plugin_notable_indepth` | Indepth mode | `no` | no | boolean |
| `plugin_notable_types` | Contribution types filter | `commit` | no | array |
| `plugin_notable_self` | Include own repositories | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_notable: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_notable=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/notable/metadata.yml`](../../assets/plugins/notable/metadata.yml) — upstream metadata
- 対応アカウント種別: user
- 必要スコープ: public_access
