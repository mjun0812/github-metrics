<!-- AUTOGEN_START: title-and-description -->
# Plugin: reactions

This plugin displays overall user reactions on recent issues, comments and discussions.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![reactions sample](../examples/plugin-reactions.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_reactions` | Enable reactions plugin | `no` | no | boolean |
| `plugin_reactions_limit` | Display limit (issues and pull requests comments) | `200` | no | number |
| `plugin_reactions_limit_issues` | Display limit (issues and pull requests, first comment) | `100` | no | number |
| `plugin_reactions_limit_discussions` | Display limit (discussions, first comment) | `100` | no | number |
| `plugin_reactions_limit_discussions_comments` | Display limit (discussions comments) | `100` | no | number |
| `plugin_reactions_days` | Comments maximum age | `0` | no | number |
| `plugin_reactions_display` | Display mode | `absolute` | no | string |
| `plugin_reactions_details` | Additional details | `` | no | array |
| `plugin_reactions_ignored` | Ignored users | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_reactions: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_reactions=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Public comments / issues / pull requests authored by the user that received reactions** (👍 😄 🎉 ❤️ 🚀 👀). Requires public activity within `plugin_reactions_days` (default `14`). Accounts with no reactions in that window produce an empty card.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/reactions/metadata.yml`](../../assets/plugins/reactions/metadata.yml) — upstream metadata
- 対応アカウント種別: user
- 必要スコープ: public_access
