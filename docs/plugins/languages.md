<!-- AUTOGEN_START: title-and-description -->
# Plugin: languages

This plugin can display which languages you use across all repositories you contributed to.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![languages sample](../examples/plugin-languages.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_languages` | Enable languages plugin | `no` | no | boolean |
| `plugin_languages_ignored` | Ignored languages | `` | no | array |
| `plugin_languages_skipped` | Skipped repositories | `` | no | array |
| `plugin_languages_limit` | Display limit | `8` | no | number |
| `plugin_languages_threshold` | Display threshold (percentage) | `0%` | no | string |
| `plugin_languages_other` | Group unknown, ignored and over-limit languages into "Other" category | `no` | no | boolean |
| `plugin_languages_colors` | Custom languages colors | `github` | no | array |
| `plugin_languages_aliases` | Custom languages names | `` | no | string |
| `plugin_languages_sections` | Displayed sections | `most-used` | no | array |
| `plugin_languages_details` | Additional details | `` | no | array |
| `plugin_languages_indepth` | Indepth mode | `no` | no | boolean |
| `plugin_languages_indepth_custom` | Indepth mode - Custom repositories | `` | no | array |
| `plugin_languages_analysis_timeout` | Indepth mode - Analysis timeout | `15` | no | number |
| `plugin_languages_analysis_timeout_repositories` | Indepth mode - Analysis timeout (repositories) | `7.5` | no | number |
| `plugin_languages_categories` | Indepth mode - Displayed categories (most-used section) | `markup, programming` | no | array |
| `plugin_languages_recent_categories` | Indepth mode - Displayed categories (recently-used section) | `markup, programming` | no | array |
| `plugin_languages_recent_load` | Indepth mode - Events to load (recently-used section) | `300` | no | number |
| `plugin_languages_recent_days` | Indepth mode - Events maximum age (day, recently-used section) | `14` | no | number |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_languages: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_languages=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Public repositories with detectable source code.** The default `most-used` mode reads aggregated language data from owned / contributed repositories. The `recently-used` sub-mode (`plugin_languages_sections=recently-used`) additionally requires recent PushEvent activity and accessible commit files (resolved via the GitHub compare API). Accounts with only empty repositories will see a blank bar.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/languages/metadata.yml`](../../assets/plugins/languages/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: public_access
