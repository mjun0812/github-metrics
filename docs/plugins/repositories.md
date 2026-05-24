<!-- AUTOGEN_START: title-and-description -->
# Plugin: repositories

This plugin displays a list of chosen featured repositories.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![repositories sample](../examples/plugin-repositories.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_repositories` | Enable repositories plugin | `no` | no | boolean |
| `plugin_repositories_featured` | Featured repositories | `` | no | array |
| `plugin_repositories_pinned` | Pinned repositories | `0` | no | number |
| `plugin_repositories_starred` | Featured most starred repositories | `0` | no | number |
| `plugin_repositories_random` | Featured random repositories | `0` | no | number |
| `plugin_repositories_order` | Featured repositories display order | `featured, pinned, starred, random` | no | array |
| `plugin_repositories_forks` | Include repositories forks | `no` | no | boolean |
| `plugin_repositories_affiliations` | Repositories affiliations | `owner` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_repositories: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_repositories=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/repositories/metadata.yml`](../../assets/plugins/repositories/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization
- 必要スコープ: public_access
