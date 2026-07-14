# Reference examples (`lowlighter/metrics`, 実データ)

このディレクトリは [`lowlighter/metrics`](https://github.com/lowlighter/metrics)
**本体**で生成した参照カードである。`mjun0812` 本人の GitHub データと、リポジトリ
[`mjun0812/flash-attention-prebuild-wheels`](https://github.com/mjun0812/flash-attention-prebuild-wheels)
を対象にしている。

## なぜ別ディレクトリなのか

- [`docs/org_examples`](../org_examples) (= `original_examples`) は `lowlighter/metrics` の
  `examples` ブランチをそのまま取り込んだもので、**データ主体が lowlighter 本人**である。
  構造比較には使えるが、データが違うため「同じ入力で `lowlighter/metrics` と Go 実装がどう違うか」
  という apples-to-apples 比較はできない。
- ここは **同じデータ (mjun0812 / flash-attention-prebuild-wheels)** を `lowlighter/metrics` に
  流し込んだ出力である。[`docs/examples`](../examples) (本プロジェクトの Go 実装出力) と
  並べることで、データを揃えた状態での見た目 / DOM の差分を確認できる。

## 生成条件

| 項目              | 値                                                                                                                                            |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| ツール            | `lowlighter/metrics@v3.34` (`lowlighter/metrics` 本体 / GitHub Action)                                                                        |
| 実行方法          | 一時的な GitHub Actions ワークフローを GitHub-hosted runner (`ubuntu-latest`) で1度実行し、`github-actions[bot]` がこのディレクトリへコミット |
| 対象 user         | `mjun0812`                                                                                                                                    |
| 対象 repository   | `mjun0812/flash-attention-prebuild-wheels` (`template: repository`)                                                                           |
| 対象プラグイン    | **本プロジェクトが Go 実装済みのプラグインのみ** (採用外 plugin は生成しない)                                                                 |
| `output_action`   | `none` (レンダリングのみ。コミットはワークフロー最終 step が別途実施)                                                                         |
| `config_timezone` | `Asia/Tokyo` (一部カード)                                                                                                                     |
| 生成日            | 2026-05-31                                                                                                                                    |

> 追補: `metrics.plugin.people.svg` は `docs/examples` の再生成で follower 数が 60
> に更新されたため、2026-06-05 に `ghcr.io/lowlighter/metrics:v3.34` のローカル
> Docker 実行で単独再生成した。

> ⚠️ これらは正規化していない **生の** `lowlighter/metrics` 出力である。フッターの生成タイムスタンプや
> `Metrics` バージョン文字列など動的な部分は再生成ごとに変わる。`docs/examples`
> 側 (本プロジェクトの Go 実装出力) も同様に生のタイムスタンプを保持しているため、
> 比較時はこれらの動的部分を読み飛ばす必要がある。

## 正しく描画できなかったカード (削除済み)

以下のカードは生成時に **`lowlighter/metrics` v3.34 側のエラー**でカード本体に `Unexpected error`
が描画される、または対象データが取得できず空になったため、**このディレクトリから削除**
した。いずれも本プロジェクトの Go 実装の不具合ではなく、`lowlighter/metrics` ツールのコードバグ /
GitHub API 仕様変更 / 権限/データ起因である。

| 削除したカード                            | 原因                                                                                                                                                                                                                                       |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `metrics.plugin.achievements.svg`         | GitHub が **Projects (classic) API を廃止** ([sunset 2024-05](https://github.blog/changelog/2024-05-23-sunset-notice-projects-classic/))。achievements は内部で classic projects を参照するため GraphQL `NOT_FOUND` → `Unexpected error`。 |
| `metrics.plugin.achievements.compact.svg` | 同上 (achievements の compact 表示)。                                                                                                                                                                                                      |
| `metrics.plugin.projects.svg`             | 同上。`projects` プラグインが Projects (classic) API を直接参照するため `NOT_FOUND` → `Unexpected error`。                                                                                                                                 |
| `metrics.plugin.activity.svg`             | `lowlighter/metrics` のコードバグ。`TypeError: Cannot read properties of undefined (reading 'filter')` (`source/plugins/activity`) で `Unexpected error`。                                                                                 |
| `metrics.repository.plugin.activity.svg`  | 同上 (repository テンプレートの activity)。                                                                                                                                                                                                |
| `metrics.plugin.habits.facts.svg`         | `lowlighter/metrics` のコードバグ。`TypeError: Cannot destructure property 'author' of 'undefined'` (`source/plugins/habits/index.mjs:51`) で `Unexpected error`。                                                                         |
| `metrics.plugin.habits.charts.svg`        | 同上 (habits の charts 表示)。                                                                                                                                                                                                             |
| `metrics.repository.plugin.traffic.svg`   | `traffic` API はリポジトリ push/admin 権限が必要で、データ取得できずセクションごとスキップ。コンテンツ無し (高さ ~8px) の空カードになるため削除。                                                                                          |

### 未生成 (最初から対象外)

| カード                                | 原因                                                                                                                                                                                                                                                          |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `metrics.plugin.languages.recent.svg` | `lowlighter/metrics` v3.34 の `source/plugins/languages/analyzer/recent.mjs:70` が `mjun0812` のデータで例外 (`Array.filter`) を投げて決定的に失敗。`plugin_languages_sections: recently-used` 指定時のみ発生するため、ワークフローの生成対象に含めていない。 |

> 補足:
>
> - `metrics.plugin.stargazers.svg` / `.graph.svg` はエラー無し / 実データ描画済みで**正常**である。
> - `metrics.plugin.traffic.svg` (user テンプレート) は traffic セクションこそデータ無しだが、
>   カード自体は他要素を含めて描画されており保持している。
> - `languages` プラグイン本体 / `.details` / `.indepth` の各カードは正常である。
> - `metrics.plugin.reactions.svg` などテキスト量の少ないカードはエラーではなく、対象データが
>   少ないだけで正常である。

## ファイル一覧

### User カード (`user: mjun0812`)

- `metrics.base.svg` / `metrics.classic.svg`
- `metrics.plugin.calendar.svg` / `metrics.plugin.calendar.full.svg`
- `metrics.plugin.isocalendar.svg` / `metrics.plugin.isocalendar.fullyear.svg`
- `metrics.plugin.languages.svg` / `.details.svg` / `.indepth.svg`
- `metrics.plugin.notable.svg` / `metrics.plugin.notable.indepth.svg`
- `metrics.plugin.people.svg`
- `metrics.plugin.reactions.svg`
- `metrics.plugin.repositories.svg` / `metrics.plugin.repositories.pinned.svg`
- `metrics.plugin.sponsors.svg` / `metrics.plugin.sponsorships.svg`
- `metrics.plugin.stargazers.svg` / `metrics.plugin.stargazers.graph.svg`
- `metrics.plugin.stars.svg`
- `metrics.plugin.starlists.svg`
- `metrics.plugin.topics.svg` / `metrics.plugin.topics.icons.svg`
- `metrics.plugin.traffic.svg`

> achievements / achievements.compact / activity / habits.facts / habits.charts /
> projects は `lowlighter/metrics` v3.34 のエラーで描画できず削除した (上記「正しく描画できなかった
> カード」参照)。

### Repository カード (`repo: flash-attention-prebuild-wheels`, `template: repository`)

- `metrics.repository.svg`
- `metrics.repository.plugin.languages.svg`
- `metrics.repository.plugin.contributors.svg`
- `metrics.repository.plugin.people.svg`
- `metrics.repository.plugin.stargazers.svg`

> activity / traffic は `lowlighter/metrics` v3.34 のエラー / データ不足で描画できず削除した
> (上記「正しく描画できなかったカード」参照)。

## 再生成

参照カードは一時的な GitHub Actions ワークフローで生成した。リポジトリ secret
`METRICS_TOKEN` (classic PAT) を設定し、対象ブランチへワークフローを push して
GitHub-hosted runner で1度実行 → `github-actions[bot]` がこのディレクトリへコミット、
という流れである。`github.token` ではリポジトリ外データが取得できずカードが空になるため、
PAT secret が必須である。

> ⚠️ self-hosted runner では動かない。`lowlighter/metrics` action が `--volume $GITHUB_EVENT_PATH`
> をマウントするが host 側に実体が無く、コンテナ内で `@actions/github` が `EISDIR`
> (`event.json` をディレクトリとして読む) で即死し、カードが1枚も生成されない。必ず
> GitHub-hosted (`ubuntu-latest`) を使う必要がある。

### ローカル Docker での単発再生成

1枚だけ手元で再生成する場合は `lowlighter/metrics` イメージを直接実行できる。`GITHUB_TOKEN`
(`repo` / `read:user` 程度のスコープを持つ PAT) を環境変数に設定し、`lowlighter/metrics` イメージへ
`INPUT_*` を渡す。値は `jq @uri` で url-encode して渡す点に注意する必要がある (`lowlighter/metrics` の
`metadata.mjs` が `decodeURIComponent` で復号する)。例:

```sh
docker run --init --rm -v "$PWD/docs/reference_examples:/renders" \
  -e INPUT_TOKEN="$GITHUB_TOKEN" \
  -e INPUT_OUTPUT_ACTION=none \
  -e INPUT_USER=mjun0812 \
  -e INPUT_BASE= \
  -e INPUT_PLUGIN_LANGUAGES=yes \
  -e INPUT_FILENAME=metrics.plugin.languages.svg \
  ghcr.io/lowlighter/metrics:v3.34
```
