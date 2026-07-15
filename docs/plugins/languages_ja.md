<!-- AUTOGEN_START: title-and-description -->
# プラグイン: languages

このプラグインは、あなたがコントリビュートした全リポジトリで使用されている言語を表示します。
<!-- AUTOGEN_END: title-and-description -->

## サンプル

![languages sample](../examples/plugin-languages.svg)

> `--user mjun0812` のデータでこのプラグインのみを有効にしてレンダリングしたものです。`make docs-examples` で再生成できます。

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| 入力 | 説明 | 既定値 | 必須 | 型 |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_languages` | `languages` プラグインを有効化します。 | `no` | no | boolean |
| `plugin_languages_ignored` | 集計から除外する言語。 | `` | no | array |
| `plugin_languages_skipped` | 集計から除外するリポジトリ。 | `` | no | array |
| `plugin_languages_limit` | 表示する言語数の上限。 | `8` | no | number |
| `plugin_languages_threshold` | 表示する割合の下限 (パーセント指定)。 | `0%` | no | string |
| `plugin_languages_other` | 不明・除外対象・上限超過の言語を "Other" カテゴリにまとめます。 | `no` | no | boolean |
| `plugin_languages_colors` | 言語ごとに使用する色のカスタマイズ。 | `github` | no | array |
| `plugin_languages_aliases` | 言語表示名のカスタマイズ。 | `` | no | string |
| `plugin_languages_sections` | 表示するセクション。 | `most-used` | no | array |
| `plugin_languages_details` | 表示する追加情報。 | `` | no | array |
| `plugin_languages_indepth` | Indepth モードを有効化します。 | `no` | no | boolean |
| `plugin_languages_indepth_custom` | Indepth モード — 追加で解析するリポジトリ。 | `` | no | array |
| `plugin_languages_analysis_timeout` | Indepth モード — 解析のタイムアウト。 | `15` | no | number |
| `plugin_languages_analysis_timeout_repositories` | Indepth モード — 解析のタイムアウト (リポジトリ単位)。 | `7.5` | no | number |
| `plugin_languages_categories` | Indepth モード — `most-used` セクションで表示するカテゴリ。 | `markup, programming` | no | array |
| `plugin_languages_recent_categories` | Indepth モード — `recently-used` セクションで表示するカテゴリ。 | `markup, programming` | no | array |
| `plugin_languages_recent_load` | Indepth モード — `recently-used` セクションで読み込むイベント数。 | `300` | no | number |
| `plugin_languages_recent_days` | Indepth モード — `recently-used` セクションでイベントを遡る最大日数。 | `14` | no | number |
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
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_languages=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 参考

- [`action.yml`](../../action.yml) — 入力スキーマの正本
- [`assets/plugins/languages/metadata.yml`](../../assets/plugins/languages/metadata.yml) — upstream の metadata
- 対応アカウント種別: user, organization, repository
- 必要なスコープ: public_access
