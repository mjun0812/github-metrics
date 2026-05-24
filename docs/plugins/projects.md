<!-- AUTOGEN_START: title-and-description -->
# Plugin: projects

This plugin displays progress of profile and repository projects.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![projects sample](../examples/plugin-projects.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_projects` | Enable projects plugin | `no` | no | boolean |
| `plugin_projects_limit` | Display limit | `4` | no | number |
| `plugin_projects_repositories` | Featured repositories projects | `` | no | array |
| `plugin_projects_descriptions` | Projects descriptions | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_projects: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_projects=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/projects/metadata.yml`](../../assets/plugins/projects/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: public_access, public_repo, read:project
