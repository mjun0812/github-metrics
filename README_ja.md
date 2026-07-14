# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)
[![Release](https://img.shields.io/github/v/release/mjun0812/github-metrics?sort=semver)](https://github.com/mjun0812/github-metrics/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/mjun0812/github-metrics)](https://github.com/mjun0812/github-metrics/blob/main/go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

GitHub プロフィールまたは単一リポジトリのメトリクスを SVG / PNG / JPEG / JSON で生成する。**GitHub Action**、**スタンドアロン CLI**、**Docker コンテナ** のいずれの形態でも動作する。単一の Go バイナリで、Node ランタイムは不要である。

本プロジェクトは [lowlighter/metrics](https://github.com/lowlighter/metrics) の subset を Go で再実装したものである。採用済みの入力は drop-in 互換であり、既存の upstream ワークフローは `uses:` 行を差し替えるだけで動作する。移植ガイドと未移植機能の一覧は [`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) を参照。

---

## ハイライト

- **Drop-in Action 差し替え** — `uses: mjun0812/github-metrics@v1` は、採用済みの全機能について `lowlighter/metrics@v3` と同じ `with:` 入力を読み取る。未移植機能の入力 (`plugin_anilist: yes` など) は暗黙的に no-op となるため、ワークフローファイルは編集なしで移行できる。
- **21 プラグイン** を 2 種類のテンプレートで提供。詳細は下の [プラグイン](#プラグイン) を参照。
- **4 種の出力形式** — SVG、PNG、JPEG、JSON (upstream とバイト互換)。
- **マルチアーキテクチャコンテナ** — `ghcr.io/mjun0812/github-metrics` に `linux/amd64` + `linux/arm64` を配置し、Sigstore cosign keyless OIDC (transparency log) で署名済み。
- **クロスコンパイル済みバイナリ** — Linux / macOS × amd64 / arm64 を全リリースに添付し、`SHA256SUMS` と成果物ごとの cosign sign-blob bundle を付属する。
- **ハードニング済みランタイム** — 非 root ユーザー (uid 10001)、resvg ラスタライザ + Noto フォントを同梱し、Node ツールチェーンも headless ブラウザも不要である。

## プロフィール README

デフォルトの出力モードでは、有効化した plugin ごとに 1 枚の SVG を `output_dir` (デフォルト `./metrics-renders/`) に書き出す。
プラグイン単位の SVG を埋め込むことでプロフィール README を組み立てる。

```markdown
<img src="metrics-renders/header.svg" width="100%">

<img src="metrics-renders/languages.svg" align="left" width="48%">
<img src="metrics-renders/stars.svg" align="right" width="48%">

<br clear="both">

<img src="metrics-renders/activity.svg" width="100%">
```

完全なワークフロー例は [docs/examples/profile-readme.md](docs/examples/profile-readme.md) を参照。

結合済みの 1 枚 SVG (旧来の動作) が欲しい場合は、ワークフローで `combined: 'yes'` を渡す。

## クイックスタート

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
      - uses: mjun0812/github-metrics@v1
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
```

> `combined: 'yes'` は単一 SVG committer 経路にオプトインするための指定である。新しいデフォルト (`combined: 'no'`) では plugin ごとの SVG を `output_dir` に出力し、`output_action: commit` とは併用できない。README への埋め込み形式に合うワークフローを選ぶこと。

`@v1` を推奨する pin である。これは最新の `v1.x.y` リリースに解決されるため、利用者は編集なしにバグ修正や機能追加を自動で受け取れる。バイト単位で不変な pin が欲しい場合は `@v1.0.0` (または任意の `vX.Y.Z`) を使う。`@latest` も公開してはいるが、本番ワークフローには推奨しない。

### リポジトリ用テンプレート

`template: repository` は、ユーザープロフィールではなく単一リポジトリを中心に SVG を再構成する。

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: ${{ github.repository_owner }}
    repo: ${{ github.event.repository.name }}
    template: repository
    token: ${{ secrets.METRICS_TOKEN }}
    combined: "yes" # repository テンプレートは結合済み SVG を 1 枚レンダリングする
```

JSON 出力は既存の `data.user` の隣に `data.repo` フィールドを追加する。SVG にはリポジトリのアバター、説明、コミュニティヘルス、直近のアクティビティが含まれる。

### CLI

```sh
# GitHub Release からインストール (linux/darwin × amd64/arm64)。
# `linux_amd64` は実行環境のプラットフォームタグに置き換えること。
curl -L -o metrics-cli \
  https://github.com/mjun0812/github-metrics/releases/download/v1.0.0/metrics-cli_v1.0.0_linux_amd64
chmod +x metrics-cli

# あるいは go install 経由 (Go 1.26+ が必要):
go install github.com/mjun0812/github-metrics/cmd/metrics-cli@v1.0.0

# SVG を標準出力にレンダリングする。GITHUB_TOKEN をシェルで設定しておくこと。
GITHUB_TOKEN=$(gh auth token) metrics-cli --user octocat \
  --output svg --dryrun --filename -
```

### Docker

```sh
docker run --rm \
  -v "$PWD/out:/renders" \
  -w /renders \
  -e GITHUB_TOKEN \
  ghcr.io/mjun0812/github-metrics:v1.0.0 \
  --user octocat --template classic \
  --output svg --filename github-metrics.svg
```

`-e GITHUB_TOKEN` はホストの環境変数をコンテナに引き継ぐ指定である。統合パイプラインはトークンフォールバックチェーン (`INPUT_TOKEN` → `GITHUB_TOKEN`) 経由でこれを読み取る。事前にホストシェルで `GITHUB_TOKEN` を設定しておくこと (例: `export GITHUB_TOKEN=$(gh auth token)`)。

このイメージはマルチアーキテクチャに対応する。`docker pull` はホストのアーキテクチャに自動で解決する。

## 認証

すべての実行に GitHub トークンが必要である。バイナリは統合された単一の入力パイプライン (v3.0、[#646](https://github.com/mjun0812/github-metrics/issues/646)) を備える。必ず先に INPUT\_<UPPER> / INPUTS 環境変数を読み、その上に CLI フラグを重ねる (競合時はフラグが勝つ)。

- **GitHub Action**: `with.token` で渡す。完全なメトリクスを得るには通常 `${{ secrets.METRICS_TOKEN }}` (Personal Access Token) を、公開データのみで良い場合は `${{ github.token }}` を指定する。ランナーは `with:` キーを `INPUT_<UPPER>` 環境変数として公開し、統合パイプラインが自動で読み取る。
- **CLI / Docker**: 環境変数 `GITHUB_TOKEN` を export する (例: `GITHUB_TOKEN=$(gh auth token)`)。バイナリはこれを自動で読み取る。`--token` / `--token-env` フラグは v3.0 で削除した。
- **ハイブリッド**: ワークフローで `with:` 入力の上に CLI 上書きを重ねることもできる。`INPUT_TOKEN=$SECRET metrics-cli --debug --user octocat` はトークンを env レイヤから、残りをフラグから解決する。ローカルデバッグ時に、シェルから紛れ込む `INPUT_FOO=bar` の干渉を抑えたいときは `--no-env` を渡して INPUT\_/INPUTS の env レイヤをまるごと無効化する。

トークンには最低でも `public_repo` (classic PAT) が必要である。fine-grained トークンの場合は、有効化した plugin が扱うデータ (issues、pulls、projects など) に対する read scope が必要である。共通の dataprovider は常に対象ユーザーのプロフィールを取得するため、トークンが欠けている場合は即座に失敗する。

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

上記の 20 個のユーザー向け plugin は常に利用可能である。`plugin_<slug>: yes` で個別に有効化し、タイルをクリックすると plugin ごとのドキュメントページに遷移する。もう 1 つの内部 plugin (`core`) はメタデータパイプラインの基盤であり、自動で実行される。

`languages` plugin は `plugin_languages_sections` を介して `recent` および `indepth` のサブモードを提供する。`topics` と `starlists` は Action / Docker ランタイムを必要とし、スタンドアロン Go バイナリでは警告とともにスキップされる。

すべての入力は [`action.yml`](action.yml) に文書化されており、対応する upstream の入力と同一である。未移植 plugin をゲートする入力 (`plugin_anilist`、`plugin_leetcode` など) は受理されるが暗黙的に無視される。既存ワークフローの移行作業は不要である。

## 出力形式

| Format         | MIME               | Rendering pipeline                                     |
| -------------- | ------------------ | ------------------------------------------------------ |
| `svg`          | `image/svg+xml`    | Go テンプレート (高さは Go 側で算出)                   |
| `png` / `jpeg` | `image/png/jpeg`   | SVG → resvg ラスタライザ (jpeg は Go 側で再エンコード) |
| `json`         | `application/json` | upstream とバイト互換のエンベロープ                    |

JSON 出力は採用済み plugin について upstream `lowlighter/metrics` と **バイト互換** である。JSON エンベロープを消費する下流ツールはそのまま動作する。SVG 出力は **DOM 構造が同等** である。要素 / 属性 / class の構造は upstream と一致するが、動的な文字列 (バージョンフッター、生成タイムスタンプ) は異なる。

## `lowlighter/metrics` からの移行

1. `uses:` 行を差し替える。

   ```diff
   - uses: lowlighter/metrics@v3.34
   + uses: mjun0812/github-metrics@v1
   ```

   `@v1` は自動的に最新の `v1.x.y` リリースに解決される。バイト単位で不変な pin を選ぶなら `@v1.0.0` (または任意の `vX.Y.Z`) を使う。

2. ワークフローを再実行する。未移植 plugin のゲートは `with:` ブロックに残ったままでも問題ない。都合の良いタイミングで削除する。

3. 出力を比較する。JSON はきれいに差分が取れるはずである。SVG は構造的に一致するはずである (差分を取る前に `xmllint --format` で整形する)。

[`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) には、採用済み plugin と未移植 plugin、テンプレート、出力形式、1 行ロールバック手順を含む完全なマトリクスを記載している。

## リリースの検証

すべてのリリースは GitHub Actions issuer に対して cosign keyless OIDC で署名される。イメージマニフェストを検証するには次を実行する。

```sh
cosign verify ghcr.io/mjun0812/github-metrics:v1.0.0 \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

GitHub Release に添付されるバイナリには、`SHA256SUMS` ファイルと成果物ごとの `.sig`、`.cert`、`.cosign.bundle` が付属する。バイナリの検証には `cosign verify-blob` を使う。

```sh
cosign verify-blob metrics-cli_v1.0.0_linux_amd64 \
  --bundle metrics-cli_v1.0.0_linux_amd64.cosign.bundle \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

ヘルパースクリプト [`scripts/release-verify.sh`](scripts/release-verify.sh) は、マニフェスト / チェックサム / 署名 / `action.yml` の参照チェックを 1 コマンドにまとめており、メンテナがリリース直後の検証に利用する。

## コントリビュート

バグ報告と pull request を歓迎する。PR を開く前に [`CONTRIBUTING.md`](CONTRIBUTING.md) の開発ワークフロー (ツールチェーン、hooks、テストカテゴリ) とプロジェクトのスコープ規律を必ず読むこと。未移植機能の一覧は [`docs/migration-to-go_ja.md`](docs/migration-to-go_ja.md) §3 にある。スコープを拡張する提案はまず discussion issue で歓迎する。

## ライセンス

[MIT](LICENSE)。本プロジェクトは [lowlighter/metrics](https://github.com/lowlighter/metrics) (MIT ライセンス) から派生した Go 再実装である。MIT ライセンス条項に従い、upstream の帰属表示を保持している。

GitHub Octicons のアセット (`assets/octicons/` 以下) は GitHub, Inc. による MIT ライセンス下のもので、`//go:embed` 経由で埋め込んでいる。
