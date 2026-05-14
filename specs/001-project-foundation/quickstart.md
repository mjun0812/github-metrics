# Quickstart: プロジェクト土台 (M1 19 タスク)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md)

本書は新しい貢献者が **数分で土台動作を確認** できる手順を示す。実装完了後の動作確認手順としてだけでなく、実装途中で「どの user story まで動いているか」を peer 検証する際にも使う。

## 0. 前提

| 項目 | 要件 |
|---|---|
| Go | 1.23.x (`go.mod` の `go` directive と一致) |
| make | GNU make (BSD make でも可) |
| git | 2.30+ |
| OS | macOS / Linux (Windows は配布ビルドのみ確認、開発主環境は非対応) |

## 1. クローンとビルド (US1)

```sh
git clone https://github.com/mjun0812/github-metrics.git
cd github-metrics

# 開発ツール (golangci-lint / govulncheck) の取得
make tools

# 全 target を一発確認
make build test lint
```

期待結果:

- `bin/metrics-action` と `bin/metrics-cli` が生成される。
- `make test` が緑 (空テストでも OK)。
- `make lint` (golangci-lint + govulncheck) が緑。
- 所要時間 < 3 分 (SC-001)。

トラブルシュート:

- `golangci-lint: command not found` → `make tools` を再実行。`$(go env GOPATH)/bin` が PATH に入っていることを確認。
- `govulncheck` が脆弱性を検出 → 当該依存をアップデート。決して `--exclude` で隠さない (constitution Development Workflow PR ゲート)。

## 2. 設定ローダの動作確認 (US2)

```sh
# サンプル settings.json (// コメントキー含む) でローダを叩く
go run ./cmd/metrics-cli --help    # M1 段階では --help のみ
go test ./internal/config/...      # ローダの全テストを実行
```

期待結果:

- `internal/config` の unit test がすべて緑。
- `settings.json` 不在時に `Settings{Port: 3000}` が返ることを `TestLoadAbsent` が確認。
- `metadata.yml` のロードで採用 21 plugin + base/core + classic/repository のキーが揃うことを `TestMetadataLoad_AllAdopted` が確認 (SC-003)。

メタデータの差分監査 (SC-002):

```sh
make check-compat
```

`./org_repo/source/plugins/*/metadata.yml` と本実装の `assets/plugins/*/metadata.yml` の **キー集合差** を表示する。差分 0 件で 0 exit。

## 3. GitHub API クライアントの動作確認 (US3)

```sh
# unit test は外部通信不要 (httptest 経由)
go test ./internal/httpx/... ./internal/githubapi/...

# race detector ジョブ
make test-race
```

期待結果:

- `httpx.Client.Get` のリトライテストが緑。
- `githubapi.NewREST` の token 種別判定テスト全件緑 (`gh[pousr]_` 受理、`github_pat_` 拒否、`NOT_NEEDED` / `MOCKED_TOKEN` 認識)。
- `githubapi.Resources.Refresh` の並行アクセスが race detector clean。

`MOCKED_TOKEN` panic ガードの確認:

```sh
go test -run TestMockedToken_RealHostPanic ./internal/githubapi/...
```

このテストは `mockedGuardRoundTripper` に対し `api.github.com` への request を投げ、panic を期待する。SC-004 のセーフティネット。

## 4. プラグイン / テンプレート registry (US4)

```sh
go test ./internal/plugins/... ./internal/templates/...
```

期待結果:

- `plugins.Register` の二重登録 panic テストが緑。
- `core.RunPlugins` で 3 つの fake plugin (success / error / panic) がすべて完了し、`Data.Plugins[name]` に集約されることを確認。
- `templates.Get("classic")` が `*NotFoundError` (M1 段階では未登録) を返す。M2 で `classic` 登録後に同じテストが成功側に切り替わる想定。

## 5. End-to-End データ結線 (US5)

```sh
# integration test は mocked GitHub backend を使う
go test ./tests/integration/...
```

期待結果:

- `tests/integration/foundation_test.go` が mocked backend に対し `engine.Compute` を実行。
- `result.Data.User.Login == "octocat"` を assert。
- 250 件 (3 ページ) のリポジトリページングが完了し、`Computed.Repositories.Count == 250`。
- organization アカウントの fallback、bulk 失敗時の field 単位 fallback もテーブルケースで網羅。

ベンチ:

```sh
go test -bench BenchmarkCompute_Foundation_Mocked ./tests/integration/...
```

SC-006 の 2 秒以内を確認。

## 6. アセットの同期 (T-012)

```sh
./scripts/sync-assets.sh
```

`./org_repo/source/...` から `assets/plugins/**` と `assets/templates/{classic,repository}/**` を取り込む。スクリプトは:

- 上流の commit hash を `assets/.upstream.lock` に書き出す。
- 採用 21 plugin + base/core + 2 templates 以外を **取得しない** (constitution 原則 III)。
- checksum 検証を行い、ファイル改竄を検出する。

`./org_repo` 自体は `.gitignore` で git 履歴から除外されているため (constitution Development Workflow)、`scripts/sync-assets.sh` を介した取得のみが正当な経路となる。

## 7. CI 検証

PR を作成すると以下が走る:

| Job | 内容 | 必須 |
|---|---|---|
| `test` | `go test ./...` | ✅ |
| `vet` | `go vet ./...` | ✅ |
| `lint` | `golangci-lint run --timeout=10m` | ✅ |
| `vuln` | `govulncheck ./...` | ✅ |
| `test-race` | `go test -race ./...` | ✅ |
| `check-compat` | metadata キー差分 0 件 | ✅ |
| `smoke-macos` | `go build ./... + go test ./internal/config/...` | ✅ |

全 job が PR 上で 7 分以内に緑 (SC-001) を merge ゲートとする。

## 8. トラブルシューティング

| 症状 | 対処 |
|---|---|
| `make build` 失敗、`unknown directive: go 1.23` | Go 1.22 以下を使っている。`go version` を確認し 1.23.x を導入。 |
| `golangci-lint` 失敗、`exhaustruct` 等で false positive 多発 | `.golangci.yml` で disable されていることを確認。研究 R-008 の方針に従う。 |
| `genqlient` 生成失敗 | `assets/plugins/base/schema.graphql` が固定 commit のものか確認。`make gen` で再生成。 |
| `make test-race` で race 検出 | `internal/githubapi/rate.go` の `sync.RWMutex` か `internal/plugins/core/run_plugins.go` の errgroup を確認。R-012 参照。 |
| `MOCKED_TOKEN` panic で test failure | テストが実 URL を叩いている。fixture または `internal/githubapi/testhelper.go` の mock を経由するよう修正。 |
| `./org_repo` が `git status` に出る | `.gitignore` のローカル差分を確認。`org_repo/` 行が消えていないか。 |

## 9. 次のステップ

- 本 feature の `tasks.md` が `/speckit-tasks` で生成されたら、issue 化してから着手する。
- M2 以降の feature spec はそれぞれ `/speckit-specify` で独立に起こす (土台が完了するまで M2 spec は書かない方針)。
- 採用範囲の変更は constitution 改定 PR (`/speckit-constitution`) を先行させる。
