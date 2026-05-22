# Quickstart: REST Data-Fetch Wiring Verification

**Feature**: `012-rest-data-fetch` | **Date**: 2026-05-22

実装完了後、本 feature が正しく動くことを確認するための quickstart シナリオ。各 User Story (US1 / US2 / US3) を独立に検証できる。

## 前提

- リポジトリ作業ツリーが clean、branch=`012-rest-data-fetch`
- `GITHUB_TOKEN` が env に設定済
- Docker image `github-metrics:local` を事前ビルド (`docker build -f deploy/Dockerfile -t github-metrics:local .`)
- 検証対象アカウント (例: `mjun0812`) に対し動作させる

## US1 Verification: Traffic plugin

### Step 1: token に `repo` scope 付きで実行

```bash
docker run --rm -e GITHUB_TOKEN \
  -v "$(pwd)/specs/012-rest-data-fetch/plugins:/out" \
  github-metrics:local \
  --user mjun0812 \
  --token-env GITHUB_TOKEN \
  --template classic \
  --output svg \
  --filename /out/traffic_with_scope.svg \
  --dryrun \
  --plugin "base=" \
  --plugin "plugin_traffic=yes"
```

**Expect**:
- SVG に `<section data-section="traffic">` が含まれる
- 「N views (M unique)」aggregate 行が non-zero
- 各 repository ごとの per-repo row が含まれる

### Step 2: token から `repo` scope を外して実行 → Skipped 確認

```bash
GITHUB_TOKEN=$PUBLIC_ONLY_TOKEN docker run ... 同上
```

**Expect**:
- SVG 内に traffic section が **存在しない** (Result.Skipped=true → partial 抑制)
- engine log に `missing repo scope` warning が 1 件

### Step 3: 一部 repo 404 でも他 repo は集計される

```bash
# 一時的に削除 / 改名された repo がある場合の挙動を fixture テストで確認
go test -run TestTraffic_PartialFailure ./internal/plugins/traffic/...
```

**Expect**: TC-002 pass

---

## US2 Verification: Contributors plugin (user/org mode)

```bash
docker run --rm -e GITHUB_TOKEN \
  -v "$(pwd)/specs/012-rest-data-fetch/plugins:/out" \
  github-metrics:local \
  --user mjun0812 \
  --token-env GITHUB_TOKEN \
  --template classic \
  --output svg \
  --filename /out/contributors.svg \
  --dryrun \
  --plugin "base=" \
  --plugin "plugin_contributors=yes" \
  --plugin "plugin_contributors_limit=10"
```

**Expect**:
- SVG に `<section data-section="contributors">` が含まれる
- 「N Contributor(s)」 header
- ≥1 contributor row (login + avatar + commit count)
- 自分自身 (mjun0812) が常にエントリに含まれる (Featured repo のコミット主)

### Verification: Ignore filter

```bash
# bot を除外
docker run ... --plugin "plugin_contributors_ignored=dependabot[bot],github-actions[bot]"
```

**Expect**: List に bot login が含まれない

---

## US3 Part 1 Verification: repositories.Starred

```bash
docker run --rm -e GITHUB_TOKEN \
  -v "$(pwd)/specs/012-rest-data-fetch/plugins:/out" \
  github-metrics:local \
  --user mjun0812 \
  --token-env GITHUB_TOKEN \
  --template classic \
  --output json \
  --filename /out/repositories.json \
  --dryrun \
  --plugin "base=" \
  --plugin "plugin_repositories=yes" \
  --plugin "plugin_repositories_starred=yes" \
  --plugin "plugin_repositories_limit=4"

jq '.plugins.repositories.starred | length' specs/012-rest-data-fetch/plugins/repositories.json
```

**Expect**: 上記コマンドが `4` を出力 (limit-truncated)

```bash
jq '.plugins.repositories.starred[0] | {nameWithOwner, stars, forks}' specs/012-rest-data-fetch/plugins/repositories.json
```

**Expect**: 最近 star した repo の name + counts

---

## US3 Part 2 Verification: repositories.Random (determinism)

```bash
# 同 seed で 2 回実行
for i in 1 2; do
  docker run --rm -e GITHUB_TOKEN \
    -v "$(pwd)/specs/012-rest-data-fetch/plugins:/out" \
    github-metrics:local \
    --user mjun0812 \
    --token-env GITHUB_TOKEN \
    --template classic \
    --output json \
    --filename "/out/random_run${i}.json" \
    --dryrun \
    --plugin "base=" \
    --plugin "plugin_repositories=yes" \
    --plugin "plugin_repositories_random=3" \
    --plugin "plugin_repositories_random_seed=42"
done

diff <(jq -S '.plugins.repositories.random' specs/012-rest-data-fetch/plugins/random_run1.json) \
     <(jq -S '.plugins.repositories.random' specs/012-rest-data-fetch/plugins/random_run2.json)
```

**Expect**: `diff` の出力が空 (両 run の Random フィールドが byte-identical)

---

## Full regression check

```bash
go test ./...
```

**Expect**: 全 test green (`tests/golden/json/m4/{traffic,contributors,repositories}.json` の golden 更新を含む)

```bash
go test -run TestComputeJSON_OctocatGolden ./tests/integration/...
```

**Expect**: octocat golden が `repositories.starred / random` フィールド付きで再ベースライン済

---

## CI verification

self-hosted runner (`hestia-x64-hestia`) で:

- `lint` / `vet` / `vuln` / `compliance` jobs all green (現在の active set)
- 新規追加された `tests/integration/plugins_012_test.go` が `compliance` の adopted-plugin check に違反しない

quota 復活時に `test` / `test-race` / `test-heavy` を re-enable → 本 feature の REST mock 経路でも green を確認。
