<!-- AUTOGEN_START: title-and-description -->
# Plugin: people

This plugin can display relationships with users, such as followers, sponsors, contributors, stargazers, watchers, members, etc.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![people sample](../examples/plugin-people.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_people` | Enable people plugin | `no` | no | boolean |
| `plugin_people_limit` | Display limit | `24` | no | number |
| `plugin_people_identicons` | Force identicons pictures | `no` | no | boolean |
| `plugin_people_identicons_hide` | Hide identicons pictures | `no` | no | boolean |
| `plugin_people_size` | Profile picture display size | `28` | no | number |
| `plugin_people_types` | Displayed sections | `followers, following` | no | array |
| `plugin_people_thanks` | Special thanks | `` | no | array |
| `plugin_people_sponsors_custom` | Custom sponsors | `` | no | array |
| `plugin_people_shuffle` | Shuffle data | `no` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_people: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_people=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/people/metadata.yml`](../../assets/plugins/people/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: public_access
