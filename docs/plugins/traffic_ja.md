<!-- AUTOGEN_START: title-and-description -->

# Plugin: traffic

このプラグインは、関連リポジトリ全体のページビュー数を表示する。

<!-- AUTOGEN_END: title-and-description -->

## Sample

![traffic sample](../examples/plugin-traffic.svg)

> `--user mjun0812` のデータで、このプラグインのみを有効にしてレンダリングした。`make docs-examples` で再生成できる。

<!-- AUTOGEN_START: config-table -->

## Configuration (inputs)

| 入力                        | 説明                                                         | デフォルト | 必須 | 型      |
| --------------------------- | ------------------------------------------------------------ | ---------- | ---- | ------- |
| `plugin_traffic`            | traffic プラグインを有効にする                               | `no`       | no   | boolean |
| `plugin_traffic_skipped`    | スキップするリポジトリ                                       | ``         | no   | array   |
| `plugin_traffic_hide_empty` | per-repo breakdown 内でビュー数 0 のリポジトリを非表示にする | `yes`      | no   | boolean |

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
# export GITHUB_TOKEN=$(gh auth token)
metrics-cli --user <your-login> \
  --output svg --filename - \
  --plugin plugin_traffic=yes
```

<!-- AUTOGEN_END: usage-snippet -->

## Notes

- **表示位置**: traffic 単体の partial は空文字列を返すスタブ実装で、SVG 出力には独自セクションを持たない。views 合計 (`TotalViews`) は `plugin_base.RepositoriesPartial` の右列下端に `<N> view[s] in last two weeks` として inline される (v2.0.2 / #638)。`plugin-traffic` サンプルは `chrome_repositories=yes` + `plugin_traffic=yes` の組合せで生成され、repositories パネル末尾に views 行が乗る (v3.0 / #649 以降、`plugin_base_repositories` などの v2 alias は不要である)。
- **Per-repo の breakdown は JSON output のみ**: `Result.Views` map は GraphQL/JSON 出力経由でのみ参照可能である。SVG 上で per-repo 行を見せたい場合は将来 `traffic.Partial` の実装が必要になる (今は intentional stub)。
- **0-view リポジトリの非表示**: `plugin_traffic_hide_empty` は既定で `yes` (true)。`v.Count == 0` のリポジトリは `Result.Views` map から除外される (JSON 出力に影響)。SVG 表示は views 合計のみなので、この入力は SVG モードでは観測できない。
- **Repository admin 権限が必要**: GitHub の traffic endpoints (`/repos/{owner}/{repo}/traffic/views`, `/repos/{owner}/{repo}/traffic/clones`) は repository administrator のみアクセス可能である。non-admin token では 403 になる。`--template=repository --repo owner/name` での実行を推奨する。
- **Token scope の要件**: `repo` scope が必要である。scope が無いと plugin は Skipped (`Result.Skipped = true`、`SkippedReason = "missing repo scope"`) となり、views 行も rendered されない。
- **403 の取り扱い**: 個別リポジトリで 403 (collaborator 権限不足など) が返った場合はそのリポジトリのみドロップし、aggregation は継続する。

## References

- [`action.yml`](../../action.yml): 入力スキーマの正本
- [`assets/plugins/traffic/metadata.yml`](../../assets/plugins/traffic/metadata.yml): upstream のメタデータ
- 対応アカウント種別: user, organization, repository
- 必要スコープ: repo
