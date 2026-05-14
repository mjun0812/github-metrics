# 10. テスト・ビルド・配布仕様

## 目次

- [1. テスト戦略](#1-テスト戦略)
- [2. mocks の設計](#2-mocks-の設計)
- [3. ベンチマークとパフォーマンス目標](#3-ベンチマークとパフォーマンス目標)
- [4. ビルド](#4-ビルド)
- [5. Docker](#5-docker)
- [6. リリース](#6-リリース)
- [7. CI/CD パイプライン](#7-cicd-パイプライン)
- [8. 開発ツール](#8-開発ツール)

---

## 1. テスト戦略

### 1.1 階層

| レイヤ | 対象 | ツール |
|--------|------|-------|
| unit | format / config / metadata / preset ローダ | `testing` + `testify/assert` |
| plugin unit | plugin ごとの Run (mock GraphQL/REST) | 同上 + `httptest`, faked GraphQL responses |
| engine integration | base + 主要プラグイン + classic template の SVG 生成 | golden file テスト |
| web e2e | `httptest.NewServer` で API を立ち上げ HTTP リクエスト | `testing` |
| action e2e | バイナリ起動 + INPUTS 注入 + dryrun | bash スクリプト + Go テスト |
| render e2e | chromedp 経由 SVG → PNG/PDF | docker-compose で headless chrome 起動 |

### 1.2 テスト構成

```
tests/
├── unit/
│   ├── config/...
│   ├── plugins/...
│   └── render/...
├── integration/
│   ├── compute_test.go        // base + plugins + template の生成
│   ├── server_test.go         // /:login パス全体
│   └── action_test.go         // Action 入力解釈
├── cases/                     // Node 版を踏襲した期待値ケース
│   ├── <case>.yml             // INPUTS と期待結果
│   └── ...
└── fixtures/                  // mocked API レスポンス
    ├── github/
    └── external/
```

### 1.3 Node 版テストの取り扱い

- 既存 `tests/cases/*.yml` (テストケース YAML) はそのまま流用する。Go の YAML パーサで読み、`engine.Compute` の入力に変換。
- 既存 `tests/metrics.test.js`, `tests/ci.test.js`, `tests/presets.test.js` は Go 側に同等の動作確認テストとして書き直す。
- 共通の golden assertion 戦略: 生成 SVG を XML 正規化して MD5 比較 (footer の動的部分は除外)。

## 2. mocks の設計

### 2.1 mocked API

- オリジナル実装 (Node 版 `tests/mocks/index.mjs`) では `Octokit` を差し替えて faker でランダムデータを返している。同等の機能を Go 側で再実装する。
- Go 版は `internal/testutil/mocks` で同等を実装:
  - `RESTMux`: `*go-github.Client.Transport` を `mock.NewRoundTripper(routes)` に置換。
  - `GraphQLMux`: `genqlient` の `Doer` を mock 実装に置換し、クエリ名で fixture を返す。
  - faker: `github.com/jaswdr/faker` で `Login`, `Repo` 等を生成。

### 2.2 mocked token

- `MOCKED_TOKEN` を受けた場合は mock RoundTripper を強制的に注入する。
- API 呼び出しが mock を経由しない経路で発生したら panic する (テスト網羅性確保のため)。

### 2.3 sandbox モード

- `settings.json` 読み込みをスキップし、すべての plugin を有効化、すべての token を mock 値で埋める。
- Web インスタンスを `--sandbox` でローカル起動するときは debug ログ + mocked + optimize=true を強制。

## 3. ベンチマークとパフォーマンス目標

| 指標 | 目標 |
|------|------|
| `engine.Compute` (classic, 主要 10 plugin, repositories=50) | < 5 秒 (chromedp 抜き) |
| `svg.Resize` (chromedp) | < 5 秒/レンダリング |
| Web `/:login` (cached hit) | < 50 ms |
| Web `/:login` (cold) | < 30 秒 |
| メモリ使用量 (idle) | < 80 MB (chromedp プロセス除く) |
| Docker イメージサイズ | < 600 MB (Chrome 同梱)、CLI のみ版は < 50 MB |

`testing.B` で `BenchmarkCompute_Classic` などを定期実行 (Go bench)。

## 4. ビルド

### 4.1 Go ビルドコマンド

```sh
go build -trimpath -ldflags="-s -w -X main.version=$(git describe --tags --dirty)" \
  -o bin/metrics-action ./cmd/metrics-action

go build -trimpath -ldflags="-s -w -X main.version=$(git describe --tags --dirty)" \
  -o bin/metrics-server ./cmd/metrics-server
```

クロスコンパイル例:

```sh
GOOS=linux   GOARCH=amd64 go build ...
GOOS=linux   GOARCH=arm64 go build ...
GOOS=darwin  GOARCH=arm64 go build ...
GOOS=windows GOARCH=amd64 go build ...
```

### 4.2 go generate

- `assets/octicons/data.json` の生成: 公式 `@primer/octicons` の npm tarball を fetch → 必要メタを抽出。
- `assets/twemoji/index.json` の生成: Twemoji リポジトリの commit hash 固定。
- `api/*.schema.json` の生成: metadata.yml + action.yml から自動生成。
- 各 generator を `go run ./internal/tools/<name>` として実装し、`go generate ./...` で一括実行。

### 4.3 `embed.FS`

- `assets/` 配下を `//go:embed assets/*` でバンドル。
- ビルド時に `git lfs` 等を要求しないよう、画像はサイズが小さいもののみ。
- 動的取得が必要な大型アセット (Twemoji 全絵文字) はオプションで起動時 fetch する余地を残す。

## 5. Docker

### 5.1 マルチステージビルド

```dockerfile
# build stage
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-action ./cmd/metrics-action \
 && go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-server ./cmd/metrics-server

# runtime stage (Chrome 同梱)
FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y \
    ca-certificates fonts-ipafont-gothic fonts-wqy-zenhei fonts-thai-tlwg \
    fonts-kacst fonts-freefont-ttf libxss1 libx11-xcb1 libxtst6 \
    chromium chromium-driver \
 && rm -rf /var/lib/apt/lists/*

ENV METRICS_CHROME_PATH=/usr/bin/chromium

COPY --from=build /out/metrics-action /metrics-action
COPY --from=build /out/metrics-server /metrics-server

WORKDIR /metrics
ENTRYPOINT ["/metrics-action"]
```

### 5.2 軽量イメージ (CLI 専用)

- Chrome を含まない `metrics-cli` イメージを別途配布。
- `chromedp` が必要なプラグイン (`topics`, `starlists`, `achievements`, `support`) と `svg.Resize`, `svg.PDF`, insights は **利用不可** と判定し、起動時に警告。
- `FROM gcr.io/distroless/cc-debian12` で最小化。

### 5.3 イメージタグ運用

| タグ | 用途 |
|------|------|
| `ghcr.io/lowlighter/metrics:latest` | 最新リリース (チャネル: 安定) |
| `ghcr.io/lowlighter/metrics:X.Y` | メジャー.マイナー |
| `ghcr.io/lowlighter/metrics:X.Y-beta` | beta |
| `ghcr.io/lowlighter/metrics:X.Y.Z` | 完全タグ |
| `ghcr.io/lowlighter/metrics:cli-X.Y` | CLI 軽量版 |

## 6. リリース

### 6.1 Semantic Versioning

- バージョン: `vMAJOR.MINOR.PATCH`。Node 版 (`3.35.0-beta`) からの継続として **`v4.0.0`** から開始する。
- メジャーアップ理由: ランタイムを node → go に変更するため。
- マイナー: 新 plugin 追加、互換維持の機能。
- パッチ: バグ修正。

### 6.2 リリース成果物

| 種別 | 内容 |
|------|------|
| GitHub Release | linux/amd64, linux/arm64, darwin/arm64, windows/amd64 各バイナリ |
| Docker | `ghcr.io/lowlighter/metrics:vX.Y.Z`, `:X.Y`, `:cli-X.Y` |
| GitHub Action | `lowlighter/metrics@vX.Y.Z`, `@vX` (デフォルトブランチ alias) |

### 6.3 リリースフロー

1. `release/vX.Y.Z` ブランチを切る。
2. `go test ./...` パスを確認。
3. `tag` を切る (`git tag -a vX.Y.Z`)。
4. CI が docker push、GitHub Release 作成、Action ブランチ alias 更新を自動化。

## 7. CI/CD パイプライン

### 7.1 既存 GitHub Workflows との対応

| ファイル | 役割 | Go 版での扱い |
|--------|------|---------------|
| `.github/workflows/test.yml` | jest テスト | `go test ./...` + bench |
| `.github/workflows/test.presets.yml` | presets テスト | `go test -run Presets ./...` |
| `.github/workflows/ci.yml` | metrics 自身の生成 | `metrics-action --dryrun` |
| `.github/workflows/branches.yml` | ブランチクリーンアップ | そのまま流用 |
| `.github/workflows/clean.yml` | ワークフロー run 掃除 | そのまま流用 |
| `.github/workflows/examples.yml` / `examples.presets.yml` | examples 再生成 | そのまま流用 (`metrics-action` を呼ぶ) |
| `.github/workflows/label.yml` | issue ラベル付け | 流用 |
| `.github/workflows/spelling.yml` | spell check | 流用 |
| `.github/workflows/stale.yml` | stale issue | 流用 |

### 7.2 追加ジョブ

- `lint`: `staticcheck ./...`, `golangci-lint run` (`gosec`, `govet`, `revive`)。
- `vuln`: `govulncheck ./...`。
- `bench`: 主要ベンチ (`go test -bench . -benchmem -count=5`)。
- `docker-build`: PR 毎に `metrics:pr-<num>` ビルド (push しない)。

### 7.3 リリースワークフロー

```yaml
on:
  push:
    tags: ["v*"]
jobs:
  release:
    permissions: { contents: write, packages: write }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test ./...
      - run: ./scripts/release.sh        # クロスコンパイル + checksum
      - uses: docker/build-push-action@v6 # multi-arch image
      - uses: softprops/action-gh-release@v2
```

## 8. 開発ツール

| ツール | 用途 |
|------|------|
| `go test ./...` | 単体テスト |
| `go test -race ./...` | レースコンディション検知 |
| `go test -bench .` | ベンチ |
| `goimports -w .` | フォーマット |
| `golangci-lint run` | リンター集約 |
| `staticcheck` | 高度な静的解析 |
| `govulncheck ./...` | 既知脆弱性チェック |
| `air` / `reflex` | ホットリロード (`metrics-server` 開発時) |
| `mockgen` | Mocks 生成 (必要な場合) |
| `genqlient` | GraphQL クエリの型生成 |

### 8.1 Makefile (推奨)

```
make build       # 両バイナリビルド
make test        # ユニットテスト
make e2e         # docker-compose 起動 + シナリオテスト
make docker      # docker image
make lint
make bench
make gen         # go generate ./...
```

### 8.2 開発用 docker-compose

```yaml
services:
  metrics-dev:
    build: .
    env_file: .env
    ports: ["3000:3000"]
    volumes:
      - .:/src
    command: ["air", "-c", ".air.toml"]
```

`.air.toml` で `cmd/metrics-server/main.go` を監視・再起動する。
