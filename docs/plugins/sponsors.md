<!-- AUTOGEN_START: title-and-description -->

# Plugin: sponsors

This plugin displays sponsors and introduction text from [GitHub sponsors](https://github.com/sponsors/).

<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![sponsors sample](../examples/plugin-sponsors.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->

## 設定 (inputs)

| Input                      | 説明                         | デフォルト          | 必須 | 型      |
| -------------------------- | ---------------------------- | ------------------- | ---- | ------- |
| `plugin_sponsors`          | Enable sponsors plugin       | `no`                | no   | boolean |
| `plugin_sponsors_sections` | Displayed sections           | `goal, list, about` | no   | array   |
| `plugin_sponsors_past`     | Past sponsorships            | `no`                | no   | boolean |
| `plugin_sponsors_size`     | Profile picture display size | `24`                | no   | number  |
| `plugin_sponsors_title`    | Title caption                | `Sponsor Me!`       | no   | string  |

<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->

## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_sponsors: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_sponsors=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Requirements

The user must be **sponsored via GitHub Sponsors** (incoming sponsorships) for a non-empty card. Non-sponsored accounts produce an empty card; this is normal.

## 既知の制約 / 注意点

- The `about` section uses GitHub Sponsors `fullDescription` when available and renders it under an `About Me` heading.

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/sponsors/metadata.yml`](../../assets/plugins/sponsors/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: read:user, read:org
