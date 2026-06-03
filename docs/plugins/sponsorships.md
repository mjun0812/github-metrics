<!-- AUTOGEN_START: title-and-description -->
# Plugin: sponsorships

This plugin displays sponsorships funded through [GitHub sponsors](https://github.com/sponsors/).
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![sponsorships sample](../examples/plugin-sponsorships.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_sponsorships` | Enable sponsorships plugin | `no` | no | boolean |
| `plugin_sponsorships_sections` | Displayed sections | `amount, sponsorships` | no | array |
| `plugin_sponsorships_size` | Profile picture display size | `24` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_sponsorships: yes
```

### CLI

```sh
metrics-action --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_sponsorships=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

The user must **sponsor others via GitHub Sponsors** (outgoing sponsorships). Users who do not sponsor anyone see an empty card.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/sponsorships/metadata.yml`](../../assets/plugins/sponsorships/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization
- 必要スコープ: read:user, read:org
