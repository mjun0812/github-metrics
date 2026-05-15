# Quickstart: classic + JSON 出力 (M2)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md)

本書は M2 feature の動作を 5 分で確認するための開発者向け手順。実装完了後の sanity check として使う。

## 0. 前提

- M1 が完走している (`origin/main` に PR #64〜#71 がマージ済み)
- ローカル toolchain がインストール済み (`make tools`、`make hooks-install`)

## 1. ブランチに切り替え

```sh
git fetch origin
git switch 002-output-classic-json
```

## 2. JSON 出力の確認 (US1)

```sh
go test ./internal/engine/... -run TestMarshal -v
```

期待:

- `TestMarshal_OctocatGolden` が `tests/golden/json/octocat.json` と一致
- `TestMarshal_CircularPayload` が `"[Circular]"` を含む出力を返す
- `TestMarshal_Errors` がトップレベル `errors` 配列を含む

## 3. classic SVG 出力の確認 (US2)

```sh
go test ./internal/templates/classic/... -v
```

期待:

- `TestClassic_Run_OctocatGolden` が `tests/golden/classic/octocat.svg` と XML 正規化後 MD5 一致
- `TestBaseHeader_Render` `TestIntroduction_StubsEmpty` `TestBaseActivityCommunity_Render` `TestBaseRepositories_Render` がそれぞれ partial の golden fragment と一致
- `TestClassic_Check_AccountUnsupported` で `repository` account が `*InputError`

## 4. engine 全体経路 (US3)

```sh
go test ./tests/integration/... -run TestCompute -v
```

期待:

- `TestComputeJSON_DefaultFromTemplate` が `application/json` MIME を返す
- `TestComputeSVG_Classic` が `image/svg+xml` MIME と `<svg` で始まる Output を返す
- `TestComputePNG_M2WarnsAndReturnsSVG` が PNG MIME + SVG bytes + warn ログを返す

## 5. 上流互換性の確認 (SC-001)

```sh
make sync-fixtures              # upstream octocat.json を取得 (./org_repo 必須)
go test ./tests/compatibility/...
```

期待: `TestUpstreamKeysetCompatibility` が pass。上流キー集合のうち本実装に **存在しない** キーが 1 件もない (採用 plugin 由来のキーは追加で良い)。

## 6. パフォーマンス確認 (SC-003)

```sh
go test -bench=BenchmarkCompute_JSON -run=^$ ./tests/integration/...
go test -bench=BenchmarkCompute_SVG  -run=^$ ./tests/integration/...
```

両者とも 1 op あたり < 2 秒。

## 7. pre-commit / CI

```sh
make hooks-run          # local: lefthook で gofumpt / go vet / tidy / golangci-lint
```

PR を作成すると CI 7 jobs (test / vet / lint / vuln / test-race / smoke-macos / compliance) が走り、本 feature の new tests (golden + compatibility + integration) もすべて緑になるはず。

## 8. トラブルシューティング

| 症状 | 対処 |
|---|---|
| `tests/golden/json/octocat.json: file does not exist` | `go test ./... -update -run TestMarshal_OctocatGolden` で golden 初期化 |
| `tests/golden/classic/octocat.svg` の MD5 不一致 | XML 正規化アルゴリズム (`tests/integration/svg_normalize.go`) を疑う前に diff を確認。動的部分 mask が漏れていないか? |
| `make sync-fixtures` が失敗 | `./org_repo` が clone されていることを確認。`scripts/sync-fixtures.sh --user octocat` で個別実行可能 |
| classic 描画で `data.User == nil` panic | partial の `data.User == nil` ガード漏れ。`internal/templates/classic/partials/base_header.go` の冒頭に nil 早期 return を追加 |
| `golangci-lint` の `revive: exported` で stutter | classic.Template の export 名を確認。stutter 警告は `.golangci.yml` で disable されている |

## 9. 次のステップ

M2 完了後の次マイルストーン:

- **M3 (chromedp / rendering pipeline)**: SVG → PNG/JPEG 変換、CSS purge、twemoji / gemoji / octicon 置換
- **M4 (21 plugins)**: 各 plugin が `Data.Plugins[name]` に payload を追加する。classic partial も次第に hydrate される
