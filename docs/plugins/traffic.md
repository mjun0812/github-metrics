<!-- AUTOGEN_START: title-and-description -->
# Plugin: traffic

This plugin displays the number of page views across affiliated repositories.
<!-- AUTOGEN_END: title-and-description -->

## Sample

![traffic sample](../examples/plugin-traffic.svg)

> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.

<!-- AUTOGEN_START: config-table -->
## Configuration (inputs)

| Input | Description | Default | Required | Type |
| ----- | ----------- | ------- | -------- | ---- |
| `plugin_traffic` | Enable traffic plugin | `no` | no | boolean |
| `plugin_traffic_skipped` | Skipped repositories | `` | no | array |
| `plugin_traffic_hide_empty` | Hide repositories with zero views from the per-repo breakdown | `yes` | no | boolean |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## Usage

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
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_traffic=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Notes

- **表示位置**: classic template では traffic 単体 partial は空文字列を返し、views 合計は `base.repositories` の右列に統合表示されます。traffic サンプルも `base=repositories` と組み合わせて生成します。
- **Repository admin 権限が必要**: GitHub の traffic endpoints (`/repos/{owner}/{repo}/traffic/views`, `/repos/{owner}/{repo}/traffic/clones`) は repository administrator のみアクセス可能です。non-admin token では 403 になります。`--account=repository` + `--repo owner/name` での実行を推奨します。
- **Per-repo 行の区切り**: 各リポジトリ行は `<span class="repo">owner/name</span>: <count> views (<uniques> unique)` の形で出力されます。`</span>` 直後にコロン + 半角スペースの区切りが入るため、長いリポジトリ名が view 数と視覚的に潰れません (issue #412 対応)。
- **0-view リポジトリの非表示**: `plugin_traffic_hide_empty` は既定で `yes` (true)。`v.Count == 0` のリポジトリは per-repo 一覧から除外されます。aggregate (合計) 行は常に出力されます。レガシー挙動 (0-view 行も表示) に戻したい場合は `plugin_traffic_hide_empty: no` を指定してください。
- **Token scope の要件**: `repo` scope が必要です。scope が無いと plugin は Skipped となり、section 自体がレンダリングされません。
- **403 の取り扱い**: 個別リポジトリで 403 (collaborator 権限不足など) が返った場合はそのリポジトリのみドロップし、aggregation は継続します。

## References

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/traffic/metadata.yml`](../../assets/plugins/traffic/metadata.yml) — upstream metadata
- Supported account types: user, organization, repository
- Required scopes: repo
