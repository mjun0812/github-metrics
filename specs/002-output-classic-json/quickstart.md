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
go test ./tests/integration/... -run TestComputeJSON_OctocatGolden -v
```

期待:

- `TestMarshal_TopLevelShape` が `account / user / config / computed / plugins / errors` の 6 トップレベルキーを返す
- `TestMarshal_TokenNeverLeaks` で `config.Token` が `"(provided)"` にマスクされる
- `TestMarshal_SelfReference` / `TestMarshal_MutualReference` / `TestMarshal_SliceCycle` で `"[Circular]"` を含む出力 (panic しない)
- `TestComputeJSON_OctocatGolden` が `tests/golden/json/octocat.json` と一致

## 3. classic SVG 出力の確認 (US2)

```sh
go test ./internal/templates/classic/... -v
go test ./tests/integration/... -run TestComputeSVG_ClassicOctocatGolden -v
```

期待:

- `TestClassic_Check_RepositoryRejected` が `repository` account に対し `*InputError`
- `TestClassic_Check_PDFUnsupported` が `pdf` 形式に対し `*UnsupportedFormatError`
- `TestBaseHeader_PopulatedEscapesName` が `<Octo & cat>` を `&lt;Octo &amp; cat&gt;` にエスケープ
- `TestBaseRepositories_RendersCounts` が `250 repositories / 1.5k stargazers / 13 forks` を含む
- `TestComputeSVG_ClassicOctocatGolden` が `tests/golden/classic/octocat.svg` と XML 正規化後 MD5 一致

## 4. engine 全体経路 (US3)

```sh
go test ./tests/integration/... -run "TestComputeJSON_Default|TestComputeSVG_Classic$|TestComputePNG|TestComputeUnknownFormat|TestComputeSVG_NoTemplate" -v
```

期待 (`contracts/result-dispatch.md` §4 のトラスステーブルと完全対応):

- `TestComputeJSON_DefaultFromTemplate` (T026): `Format=""` + Template 未登録 → `application/json`
- `TestComputeJSON_DefaultWhenNoopTemplate` (T026): `Template="noop"` → `application/json` (noop は metadata を持たないため fallback)
- `TestComputeJSON_DefaultFromClassicMetadata` (T026): `Template="classic"` + `Format=""` → `image/svg+xml` (classic metadata.formats[0]="svg")
- `TestComputeSVG_Classic` (T027): 明示的な `svg` 指定で `<svg` で始まる Output + `image/svg+xml`
- `TestComputePNG_M2WarnsAndReturnsSVG` (T028): `format="png"` で `image/png` + SVG bytes + level=WARN ログ
- `TestComputeUnknownFormat_Error` (T029): `format="bogus"` → `*UnsupportedFormatError`
- `TestComputeSVG_NoTemplate_Errors` (T030): `format="svg"` + Template 未登録 → `*InputError{Field:"template"}`

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
