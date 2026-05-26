<!-- AUTOGEN_START: title-and-description -->
# Plugin: stargazers

This plugin displays stargazers evolution across affiliated repositories.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![stargazers sample](../examples/plugin-stargazers.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_stargazers` | Enable stargazers plugin | `no` | no | boolean |
| `plugin_stargazers_days` | Time range | `14` | no | number |
| `plugin_stargazers_charts` | Charts | `yes` | no | boolean |
| `plugin_stargazers_charts_type` | Charts display type | `classic` | no | string |
| `plugin_stargazers_worldmap` | Stargazers worldmap | `no` | no | boolean |
| `plugin_stargazers_worldmap_token` | Stargazers worldmap token | `` | no | token |
| `plugin_stargazers_worldmap_sample` | Stargazers worldmap sample | `0` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_stargazers: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_stargazers=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Public repositories owned by the user that have stargazers.** For repository mode (`--account=repository`), a single repo with stargazers is enough. The `graph` mode (`plugin_stargazers_charts_type=graph`) renders the same data as a time-series line chart. The `worldmap` mode is currently backlog — it requires a third-party geocoding API (see #396 / #409).

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/stargazers/metadata.yml`](../../assets/plugins/stargazers/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: public_access
