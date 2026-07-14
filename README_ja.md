# github-metrics

[![CI](https://shieldcn.dev/github/mjun0812/github-metrics/ci.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)
[![Release](https://shieldcn.dev/github/mjun0812/github-metrics/release.svg)](https://github.com/mjun0812/github-metrics/releases)
[![Go version](https://shieldcn.dev/badge/go-1.26-blue.svg)](https://github.com/mjun0812/github-metrics/blob/main/go.mod)
[![License](https://shieldcn.dev/github/mjun0812/github-metrics/license.svg)](LICENSE)

本プロジェクトは [lowlighter/metrics](https://github.com/lowlighter/metrics) をもとに、Goで再実装したものです。
GitHub プロフィールまたはリポジトリのメトリクスを SVG / PNG / JPEG / JSON で生成する単一の Go バイナリで、
`lowlighter/metrics`で必要だった、NodeランタイムとChrome headlessは不要です。
**GitHub Action**、**Standalone CLI**、**Docker** のいずれの形態でも動作します。

既存の `lowlighter/metrics` ワークフローは `uses:` 行を差し替えるだけで動作します。
移植ガイドと未移植機能の一覧は [`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) を参照してください。

## プラグイン

<!-- AUTOGEN_START: plugins-gallery -->

|                                                                                                           |                                                                                        |                                                                                        |
| :-------------------------------------------------------------------------------------------------------: | :------------------------------------------------------------------------------------: | :------------------------------------------------------------------------------------: |
|          [![achievements](docs/examples/plugin-achievements.svg)](docs/plugins/achievements.md)           |       [![activity](docs/examples/plugin-activity.svg)](docs/plugins/activity.md)       |       [![calendar](docs/examples/plugin-calendar.svg)](docs/plugins/calendar.md)       |
|                              [`achievements`](docs/plugins/achievements.md)                               |                         [`activity`](docs/plugins/activity.md)                         |                         [`calendar`](docs/plugins/calendar.md)                         |
| [![contributors](docs/examples/plugin-contributors-repo-contributions.svg)](docs/plugins/contributors.md) |          [![habits](docs/examples/plugin-habits.svg)](docs/plugins/habits.md)          |          [![header](docs/examples/plugin-header.svg)](docs/plugins/header.md)          |
|                              [`contributors`](docs/plugins/contributors.md)                               |                           [`habits`](docs/plugins/habits.md)                           |                           [`header`](docs/plugins/header.md)                           |
|            [![isocalendar](docs/examples/plugin-isocalendar.svg)](docs/plugins/isocalendar.md)            |     [![languages](docs/examples/plugin-languages.svg)](docs/plugins/languages.md)      |        [![notable](docs/examples/plugin-notable.svg)](docs/plugins/notable.md)         |
|                               [`isocalendar`](docs/plugins/isocalendar.md)                                |                        [`languages`](docs/plugins/languages.md)                        |                          [`notable`](docs/plugins/notable.md)                          |
|                   [![people](docs/examples/plugin-people.svg)](docs/plugins/people.md)                    |     [![reactions](docs/examples/plugin-reactions.svg)](docs/plugins/reactions.md)      | [![repositories](docs/examples/plugin-repositories.svg)](docs/plugins/repositories.md) |
|                                    [`people`](docs/plugins/people.md)                                     |                        [`reactions`](docs/plugins/reactions.md)                        |                     [`repositories`](docs/plugins/repositories.md)                     |
|                [![sponsors](docs/examples/plugin-sponsors.svg)](docs/plugins/sponsors.md)                 | [![sponsorships](docs/examples/plugin-sponsorships.svg)](docs/plugins/sponsorships.md) |    [![stargazers](docs/examples/plugin-stargazers.svg)](docs/plugins/stargazers.md)    |
|                                  [`sponsors`](docs/plugins/sponsors.md)                                   |                     [`sponsorships`](docs/plugins/sponsorships.md)                     |                       [`stargazers`](docs/plugins/stargazers.md)                       |
|               [![starlists](docs/examples/plugin-starlists.svg)](docs/plugins/starlists.md)               |           [![stars](docs/examples/plugin-stars.svg)](docs/plugins/stars.md)            |          [![topics](docs/examples/plugin-topics.svg)](docs/plugins/topics.md)          |
|                                 [`starlists`](docs/plugins/starlists.md)                                  |                            [`stars`](docs/plugins/stars.md)                            |                           [`topics`](docs/plugins/topics.md)                           |
|                  [![traffic](docs/examples/plugin-traffic.svg)](docs/plugins/traffic.md)                  |                                                                                        |                                                                                        |
|                                   [`traffic`](docs/plugins/traffic.md)                                    |                                                                                        |                                                                                        |

<!-- AUTOGEN_END: plugins-gallery -->

すべての入力は [`action.yml`](action.yml) を参考にしてください。対応する `lowlighter/metrics` の入力と同一です。  
templateは現在`classic` と `repository` の 2 種類があり、`classic` はユーザープロフィールを`repository` はリポジトリの情報を生成します。

## Usage

### 認証

すべての実行に GitHub トークンが必要です。

- **GitHub Action**: `with.token` で渡します。完全なメトリクスを得るには通常 `${{ secrets.METRICS_TOKEN }}` (Personal Access Token) を、公開データのみで良い場合は `${{ github.token }}` を指定します。ランナーは `with:` キーを `INPUT_<UPPER>` 環境変数として公開し、統合パイプラインが自動で読み取ります。
- **CLI / Docker**: 環境変数 `GITHUB_TOKEN` を export します (例: `GITHUB_TOKEN=$(gh auth token)`)。バイナリはこれを自動で読み取ります。

トークンには最低でも `public_repo` (classic PAT) が必要です。fine-grained トークンの場合は、有効化した plugin が扱うデータ (issues、pulls、projects など) に対する read scope が必要です。

### GitHub Action

```yaml
# .github/workflows/metrics.yml
name: Metrics
on:
  schedule: [{ cron: "0 0 * * *" }]
  workflow_dispatch:

jobs:
  github-metrics:
    runs-on: ubuntu-latest
    steps:
      - uses: mjun0812/github-metrics@latest
        with:
          user: octocat
          token: ${{ secrets.METRICS_TOKEN }}
          template: classic
          combined: "yes"
          plugin_languages: "yes"
          plugin_languages_limit: "5"
          committer_branch: main
          output_action: commit
          output_condition: data-changed

      # Repository
      - uses: mjun0812/github-metrics@latest
        with:
          user: ${{ github.repository_owner }}
          repo: ${{ github.event.repository.name }}
          template: repository
          token: ${{ secrets.METRICS_TOKEN }}
          combined: "yes" # repository テンプレートは結合済み SVG を 1 枚レンダリングします
```

結合済みの 1 枚のSVGが欲しい場合は、ワークフローで `combined: 'yes'` を渡します。
完全なワークフロー例は [docs/examples/profile-readme.md](docs/examples/profile-readme.md) を参照してください。

### CLI

```sh
# GitHub Release からインストールします (linux/darwin × amd64/arm64)。
# 最新バージョンを GitHub API から取得し、環境変数 VERSION に代入します。
VERSION=$(curl -s https://api.github.com/repos/mjun0812/github-metrics/releases/latest | grep -oE '"tag_name": "[^"]+"' | cut -d'"' -f4)

# Linux (amd64)
curl -L -o metrics-cli \
  "https://github.com/mjun0812/github-metrics/releases/download/${VERSION}/metrics-cli_${VERSION}_linux_amd64"
chmod +x metrics-cli

# macOS (Apple Silicon / arm64)
curl -L -o metrics-cli \
  "https://github.com/mjun0812/github-metrics/releases/download/${VERSION}/metrics-cli_${VERSION}_darwin_arm64"
chmod +x metrics-cli

# あるいは go install 経由 (Go 1.26+ が必要):
go install github.com/mjun0812/github-metrics/cmd/metrics-cli@latest

# SVG を標準出力にレンダリングします。GITHUB_TOKEN をシェルで設定しておいてください。
GITHUB_TOKEN=$(gh auth token) metrics-cli --user octocat \
  --output svg --dryrun --filename -
```

### Docker

```sh
docker run --rm \
  -v "$PWD/out:/renders" \
  -w /renders \
  -e GITHUB_TOKEN \
  ghcr.io/mjun0812/github-metrics:latest \
  --user octocat --template classic \
  --output svg --filename github-metrics.svg
```

### 出力形式

| Format         | MIME               | Rendering pipeline                                     |
| -------------- | ------------------ | ------------------------------------------------------ |
| `svg`          | `image/svg+xml`    | Go テンプレート (高さは Go 側で算出)                   |
| `png` / `jpeg` | `image/png/jpeg`   | SVG → resvg ラスタライザ (jpeg は Go 側で再エンコード) |
| `json`         | `application/json` | `lowlighter/metrics` とバイト互換                      |

JSON 出力は採用済み plugin について `lowlighter/metrics` と **バイト互換** です。
SVG 出力は **DOM 構造が同等** です。要素 / 属性 / class の構造は `lowlighter/metrics` と一致しますが、
動的な文字列 (バージョンフッター、生成タイムスタンプ) は異なります。

### プロフィール README

デフォルトの出力モードでは、有効化した plugin ごとに 1 枚の SVG を `output_dir` (デフォルト `./metrics-renders/`) に書き出します。
プラグイン単位の SVG を埋め込むことでプロフィール README を作成できます。

```markdown
<img src="metrics-renders/header.svg" width="100%">

<img src="metrics-renders/languages.svg" align="left" width="48%">
<img src="metrics-renders/stars.svg" align="right" width="48%">

<br clear="both">

<img src="metrics-renders/activity.svg" width="100%">
```

## `lowlighter/metrics` からの移行

1. `uses:` 行を差し替えます。

   ```diff
   - uses: lowlighter/metrics@v3.34
   + uses: mjun0812/github-metrics@v4
   ```

2. ワークフローを再実行します。未移植 plugin のゲートは `with:` ブロックに残ったままでも問題ありません。都合の良いタイミングで削除します。

3. 出力を比較します。JSON はきれいに差分が取れるはずです。SVG は構造的に一致するはずです (差分を取る前に `xmllint --format` で整形します)。

[`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) には、採用済み plugin と未移植 plugin、テンプレート、出力形式、1 行ロールバック手順を含む完全なマトリクスを記載しています。

## リリースの検証

すべてのリリースは GitHub Actions issuer に対して cosign keyless OIDC で署名されます。イメージマニフェストを検証するには次を実行します。

```sh
cosign verify ghcr.io/mjun0812/github-metrics:v4.1.3 \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

GitHub Release に添付されるバイナリには、`SHA256SUMS` ファイルと成果物ごとの `.sig`、`.cert`、`.cosign.bundle` が付属します。
バイナリの検証には `cosign verify-blob` を使います。

```sh
cosign verify-blob metrics-cli_v4.1.3_linux_amd64 \
  --bundle metrics-cli_v4.1.3_linux_amd64.cosign.bundle \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Contribute

バグ報告と pull request を歓迎します。PR を開く前に [`CONTRIBUTING.md`](CONTRIBUTING.md) の開発ワークフロー (ツールチェーン、hooks、テストカテゴリ) とプロジェクトのスコープ規律を必ず読んでください。未移植機能の一覧は [`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) にあります。

## License

[MIT](LICENSE)。本プロジェクトは [lowlighter/metrics](https://github.com/lowlighter/metrics) (MIT ライセンス) から派生した Go 再実装です。MIT ライセンス条項に従い、`lowlighter/metrics` の帰属表示を保持しています。

GitHub Octicons のアセット (`assets/octicons/` 以下) は GitHub, Inc. による MIT ライセンス下のもので、`//go:embed` 経由で埋め込んでいます。
