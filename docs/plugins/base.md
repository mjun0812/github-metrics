<!-- AUTOGEN_START: title-and-description -->
# Plugin: base

`base` is the foundational plugin that runs before every other plugin and populates the shared `data.User` / `data.Organization` / `data.Computed` fields downstream plugins depend on. It also owns the user/org header card (avatar, login, follower/sponsor counts, two-week commit calendar) that every other plugin's output sits on top of.
<!-- AUTOGEN_END: title-and-description -->

## サンプル出力

![base sample](../examples/plugin-base.svg)

> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。

## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->

<!-- AUTOGEN_START: config-table -->
## 設定 (inputs)

| Input | 説明 | デフォルト | 必須 | 型 |
|-------|------|------------|------|----|
| `base` | Base content | `header, activity, community, repositories, metadata` | no | array |
| `base_indepth` | Indepth mode | `no` | no | boolean |
| `base_hireable` | Show `Available for hire!` in header section | `no` | no | boolean |
| `base_skip` | Skip base content | `no` | no | boolean |
| `repositories` | Fetched repositories | `100` | no | number |
| `repositories_batch` | Fetched repositories per query | `100` | no | number |
| `repositories_forks` | Include forks | `no` | no | boolean |
| `repositories_affiliations` | Repositories affiliations | `owner` | no | array |
| `repositories_skipped` | Default skipped repositories | `` | no | array |
| `users_ignored` | Default ignored users | `github-actions[bot], dependabot[bot], dependabot-preview[bot], actions-user, action@github.com` | no | array |
| `commits_authoring` | Identifiers that has been used for authoring commits | `.user.login` | no | array |
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
## 使い方

### GitHub Action

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: <your-login>
    token: ${{ secrets.METRICS_TOKEN }}
    base: header, activity, community, repositories, metadata
```

### CLI

```sh
metrics-cli --user <your-login> --token-env GITHUB_TOKEN \
  --output svg --filename - \
  --plugin 'base=header, activity, community, repositories, metadata'
```
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**A valid GitHub username (or organization login) and a token with at minimum `read:user` + `public_repo`.** The base plugin queries the GraphQL `user(login:)` / `organization(login:)` endpoint to populate the header card and walks the repositories connection (paged, with batch-halving on transient 5xx) to seed `Computed.RepositoryList` for every downstream plugin. Setting `base: ""` disables every base section but **does not** skip the GraphQL fetch — to fully skip base data fetching, use `base_skip: yes` and pair it with a plugin that supports `token: NOT_NEEDED`.

## 既知の制約 / 注意点

<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->

## 参照

- [`action.yml`](../../action.yml) — canonical input schema
- [`assets/plugins/base/metadata.yml`](../../assets/plugins/base/metadata.yml) — upstream metadata
- 対応アカウント種別: user, organization, repository
- 必要スコープ: public_access
