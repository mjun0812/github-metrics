<!-- AUTOGEN_START: title-and-description -->
# Plugin: starlists

This plugin displays star lists.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![starlists sample](../examples/plugin-starlists.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_starlists` | Enable starlists plugin | `no` | no | boolean |
| `plugin_starlists_limit` | Display limit (star lists) | `2` | no | number |
| `plugin_starlists_limit_repositories` | Display limit (repositories per star list) | `2` | no | number |
| `plugin_starlists_languages` | Star lists languages statistics | `no` | no | boolean |
| `plugin_starlists_limit_languages` | Display limit (languages per star list) | `8` | no | number |
| `plugin_starlists_languages_ignored` | Ignored languages in star lists | `` | no | array |
| `plugin_starlists_languages_aliases` | Custom languages names in star lists | `` | no | string |
| `plugin_starlists_shuffle_repositories` | Shuffle data | `yes` | no | boolean |
| `plugin_starlists_ignored` | Skipped star lists | `` | no | array |
| `plugin_starlists_only` | Showcased star lists | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_starlists: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_starlists=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/starlists/metadata.yml`](../../assets/plugins/starlists/metadata.yml) — upstream metadata
- 対応アカウント種別: user
