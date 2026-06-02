<!-- AUTOGEN_START: title-and-description -->
# Plugin: contributors

This plugin display repositories contributors from a commit range along with additional stats.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![contributors sample](../examples/plugin-contributors-repo-contributions.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `plugin_contributors` | Enable contributors plugin | `no` | no | boolean |
| `plugin_contributors_base` | Base reference | `` | no | string |
| `plugin_contributors_head` | Head reference | `master` | no | string |
| `plugin_contributors_ignored` | Ignored users | `` | no | array |
| `plugin_contributors_contributions` | Contributions count | `no` | no | boolean |
| `plugin_contributors_sections` | Displayed sections | `contributors` | no | array |
| `plugin_contributors_categories` | Contribution categories | `{
  "📚 Documentation": ["README.md", "docs/**"],
  "💻 Code": ["source/**", "src/**"],
  "#️⃣ Others": ["*"]
}
` | no | json |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    plugin_contributors: yes
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin plugin_contributors=yes
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Repository context required.** Run with `--account=repository` and a target repository (`--repo owner/name`). The repo must have multiple contributors for a meaningful list. The `contributions` display mode (`plugin_contributors_contributions=yes`) additionally fetches per-contributor statistics via GitHub's stats API (`GET /repos/{owner}/{repo}/stats/contributors`).

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/contributors/metadata.yml`](../../assets/plugins/contributors/metadata.yml) — upstream metadata
- 対応アカウント種別: repository
- 必要スコープ: public_access
