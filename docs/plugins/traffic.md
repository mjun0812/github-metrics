<!-- AUTOGEN_START: title-and-description -->
# Plugin: traffic

This plugin displays the number of page views across affiliated repositories.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![traffic sample](../examples/plugin-traffic.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_traffic` | Enable traffic plugin | `no` | no | boolean |
| `plugin_traffic_skipped` | Skipped repositories | `` | no | array |
| `plugin_traffic_hide_empty` | Hide repositories with zero views from the per-repo breakdown | `yes` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_traffic: yes
```

### CLI

```sh
metrics-action --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_traffic=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## 既知の制約 / 注意点

- **Repository admin 権限が必要**: GitHub の traffic endpoints (`/repos/{owner}/{repo}/traffic/views`, `/repos/{owner}/{repo}/traffic/clones`) は repository administrator のみアクセス可能です。non-admin token では 403 になります。`--account=repository` + `--repo owner/name` での実行を推奨します。
- **Per-repo 行の区切り**: 各リポジトリ行は `<span class="repo">owner/name</span>: <count> views (<uniques> unique)` の形で出力されます。`</span>` 直後にコロン + 半角スペースの区切りが入るため、長いリポジトリ名が view 数と視覚的に潰れません (issue #412 対応)。
- **0-view リポジトリの非表示**: `plugin_traffic_hide_empty` は既定で `yes` (true)。`v.Count == 0` のリポジトリは per-repo 一覧から除外されます。aggregate (合計) 行は常に出力されます。レガシー挙動 (0-view 行も表示) に戻したい場合は `plugin_traffic_hide_empty: no` を指定してください。
- **Token scope の要件**: `repo` scope が必要です。scope が無いと plugin は Skipped となり、section 自体がレンダリングされません。
- **403 の取り扱い**: 個別リポジトリで 403 (collaborator 権限不足など) が返った場合はそのリポジトリのみドロップし、aggregation は継続します。
## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/traffic/metadata.yml`](../../assets/plugins/traffic/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: repo
