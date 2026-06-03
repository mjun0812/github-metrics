<!-- AUTOGEN_START: title-and-description -->
# Plugin: stars

This plugin displays recently starred repositories.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![stars sample](../examples/plugin-stars.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_stars` | Enable stars plugin | `no` | no | boolean |
| `plugin_stars_limit` | Display limit | `4` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_stars: yes
```

### CLI

```sh
metrics-action --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_stars=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**At least one starred repository.** Reads starred repositories via GraphQL (`user.starredRepositories`). Accounts that never starred anything see an empty card.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/stars/metadata.yml`](../../assets/plugins/stars/metadata.yml) — upstream metadata
- 対応アカウント種別: user
- 必要スコープ: public_access
