<!-- AUTOGEN_START: title-and-description -->
# Plugin: achievements

This plugin displays several highlights about what an account has achieved on GitHub.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![achievements sample](../examples/plugin-achievements.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_achievements` | Enable achievements plugin | `no` | no | boolean |
| `plugin_achievements_threshold` | Rank threshold filter | `C` | no | string |
| `plugin_achievements_secrets` | Secrets achievements | `yes` | no | boolean |
| `plugin_achievements_display` | Display style | `detailed` | no | string |
| `plugin_achievements_limit` | Display limit | `0` | no | number |
| `plugin_achievements_ignored` | Ignored achievements | `` | no | array |
| `plugin_achievements_only` | Showcased achievements | `` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_achievements: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_achievements=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

An account with **public activity history** — commits, repositories, issues, stars, followers, etc. The rank computation needs measurable metrics; accounts with no commits or repositories will see most achievement cards marked as "locked".

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/achievements/metadata.yml`](../../assets/plugins/achievements/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization
- 必要スコープ: public_access
