# Quickstart: M6 — Action / CLI 開発手順

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md)

M6 を開発する人 (= 本リポジトリで `internal/action/*.go` / `cmd/metrics-action/main.go` / `action.yml` / `Dockerfile` を書く担当) 向けの初期セットアップ + 開発フロー + ローカル動作確認手順。M1-M4 の quickstart (`specs/001-...`, `specs/004-...`) を踏まえている前提で、M6 固有の差分のみを記載する。

## 1. 必要なツール

### 1.1 通常開発に必要

M1-M4 から変更なし:

- Go 1.26+ (`go.mod` の `go` directive と一致)
- `make`
- `git`
- `gofumpt` / `golangci-lint` (M1 で導入済)

### 1.2 Action のローカル動作確認に必要

- `docker` (Docker Engine 20+ 推奨) — `metrics-action` Docker image を build / run するため
- `act` (https://github.com/nektos/act) — workflow を local で実行して action の動作確認 (任意、必須ではない)

### 1.3 chromedp 経路に必要

M3 から変更なし — `METRICS_CHROME_PATH` で chromium binary を指す。Docker image 内では chromium が同梱されている。

## 2. 開発の進め方 (action / CLI 機能追加 5 ステップ)

`internal/action/*.go` 配下に新規ファイルを追加する手順:

### Step 1: contracts/ で挙動を確認

`specs/005-m6-action-cli/contracts/` の該当 contract (例: `committer.md`, `retry-policy.md`, `output-actions.md`, `token-validation.md`) を読み、入力 / state transitions / error handling を把握する。

### Step 2: テストファースト (table test)

`internal/action/<file>_test.go` を作り、対応 contract の Tests Strategy 節に列挙されたケースをテーブルテストで実装する。例:

```go
package action

func TestRetryPolicy_Do(t *testing.T) {
    cases := []struct {
        name     string
        retries  int
        fnErrs   []error  // 各回 fn が返す error
        wantErr  bool
        wantCalls int
    }{
        {"first_success", 3, []error{nil}, false, 1},
        {"retryable_then_success", 3, []error{wrapRetryable(io.ErrUnexpectedEOF), nil}, false, 2},
        {"non_retryable_fail_fast", 3, []error{errors.New("plain")}, true, 1},
        // ...
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### Step 3: 実装

`internal/action/<file>.go` で対応するロジックを書く。Step 2 のテストが緑になることを確認。

### Step 4: integration test

`tests/integration/action_test.go` または `cli_test.go` に end-to-end ケースを追加。mocked GitHub backend (`internal/githubapi` の MockTransport) + FakeRenderer (M3) で完結させる。

### Step 5: action.yml / contract の更新

新規 plugin input / CLI flag / output_action 値を追加した場合、対応する `assets/plugins/<slug>/metadata.yml` を更新し、`make gen-action-yml` で action.yml を再生成 + commit。

## 3. Makefile ターゲット

M3 / M4 から拡張:

```makefile
# M6 で新規追加 (予定)
build-action:
    go build -o bin/metrics-action ./cmd/metrics-action

gen-action-yml:
    go run ./internal/tools/gen-action-yml --output ./action.yml

docker-build:
    docker build -t ghcr.io/mjun0812/github-metrics:dev .

docker-run-cli:
    docker run --rm -e INPUT_USER=octocat -e USE_MOCKED_DATA=true \
        ghcr.io/mjun0812/github-metrics:dev --dryrun --filename - --output svg

# M3 既存
test:
    go test ./...
test-chromedp:
    METRICS_CHROME_PATH=... go test -tags=chromedp ./...
test-heavy:
    go test -tags=heavy ./...
```

## 4. ローカル動作確認 (CLI モード)

```sh
# Step 1: build
go build -o bin/metrics-action ./cmd/metrics-action

# Step 2: dryrun + mocked
./bin/metrics-action --user octocat --template classic --output svg \
    --plugin use_mocked_data=true --dryrun --filename -

# 期待: 標準出力に <svg ...>...</svg> が流れる
```

```sh
# Step 3: real GitHub token (octocat 自身ではなく自分のアカウント等で)
./bin/metrics-action --user $YOUR_LOGIN --template classic --output svg \
    --token-env GITHUB_TOKEN --filename github-metrics.svg

# 期待: github-metrics.svg がカレントディレクトリに生成
```

```sh
# Step 4: YAML config 経由 (action.yml と同じ inputs)
cat > metrics-inputs.yaml <<EOF
user: $YOUR_LOGIN
template: classic
output: svg
filename: github-metrics.*
plugins:
  languages: true
  languages_limit: 5
  activity: true
config:
  timezone: Asia/Tokyo
EOF

./bin/metrics-action --config metrics-inputs.yaml --token-env GITHUB_TOKEN --dryrun
```

## 5. ローカル動作確認 (Action モード)

Docker image を build してから `INPUT_*` env で起動 = GitHub Actions runner と等価:

```sh
# Step 1: image build
docker build -t ghcr.io/mjun0812/github-metrics:dev .

# Step 2: action mode 起動 (mocked data で safe)
docker run --rm \
    -e GITHUB_ACTIONS=true \
    -e GITHUB_REPOSITORY=mjun0812/test-repo \
    -e GITHUB_RUN_ID=12345 \
    -e GITHUB_OUTPUT=/tmp/output \
    -e INPUT_USER=octocat \
    -e INPUT_TEMPLATE=classic \
    -e INPUT_USE_MOCKED_DATA=true \
    -e INPUT_OUTPUT_ACTION=none \
    -e INPUT_DRYRUN=yes \
    -v /tmp:/tmp \
    ghcr.io/mjun0812/github-metrics:dev

# 期待: stderr に banner + slog ログ、/renders/github-metrics.svg が生成
```

## 6. trouble shooting

### 6.1 `metrics-action` 起動時に "token required" で exit 1

- `--token-env GITHUB_TOKEN` を渡し忘れている、または `$GITHUB_TOKEN` env が空
- もしくは `--dryrun` と `--plugin use_mocked_data=true` を両方渡すと token なしで動かせる

### 6.2 Docker image build が遅い

- M3 の chromium 同梱で image が ~500 MB ある (research R-001)
- 初回は数分かかる。`docker build --cache-from ghcr.io/mjun0812/github-metrics:latest` で前回 image を cache に使うと数十秒に短縮
- 開発中は `--target builder` で builder stage だけ build して Go binary 単体動作確認も可

### 6.3 GitHub Actions ローカル実行 (`act`) で action がうまく動かない

- `act` は GitHub Actions runner を完全には再現しない (cgroup / Docker-in-Docker など)
- 推奨: `act` ではなく `docker run` で env を手動セットアップする方式 (本書 §5) で動作確認

### 6.4 `INPUTS` JSON が action.yml で受け取れない

- `runs.using: docker` の場合、`INPUTS` env は自動 set されない (composite action 専用)
- M6 では `INPUT_<UPPER>` のみ使う想定 (FR-001 の "INPUTS JSON preferred, INPUT_<UPPER> fallback" は CLI モード経由の YAML 統合のための design)

## 7. PR セルフレビュー基準 (M6 action / CLI 機能単体)

PR を出す前に以下を確認:

- [ ] 対応する contract (`specs/005-m6-action-cli/contracts/<name>.md`) の Tests Strategy 節の全ケースが緑
- [ ] `internal/action/<file>_test.go` で **table test** + edge case を網羅
- [ ] `tests/integration/{action,cli}_test.go` で end-to-end ケースを 1 つ以上追加
- [ ] action.yml を変更した場合は `make gen-action-yml` を実行 + `git diff --quiet action.yml` で drift がないこと
- [ ] 不採用 plugin slug / 未対応 output_action 値が production code に混入していないこと (`go test ./tests/compliance/...` 緑)
- [ ] banner / log / error message が **英語** (clarification Q5 / FR-003)
- [ ] token が含まれる可能性のあるログ経路で **mask されている** (config.Token の Stringer 経由)
- [ ] `internal/errors.RetryableError` を新規 raise した場合は、対応する RetryPolicy.Do の経路を試験 (errors.As で検出される)
- [ ] 新規依存ライブラリを追加していない (constitution 原則 V; 採用根拠を PR 本文に記述する代わりにゼロを維持)
