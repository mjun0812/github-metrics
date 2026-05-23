# Quickstart: 未配線 GraphQL plugin の data-fetch wiring (013)

## 1. Branch + Dependencies

```sh
git checkout 013-unwired-graphql-data
go mod tidy
```

## 2. GraphQL クエリ生成

新規クエリ追加後:

```sh
go generate ./internal/githubapi/...
# → internal/githubapi/graphql_gen.go が再生成される
```

## 3. テスト

```sh
# 単体 (plugin ごと)
go test ./internal/plugins/sponsors/...
go test ./internal/plugins/sponsorships/...
go test ./internal/plugins/projects/...
go test ./internal/plugins/notable/...
go test ./internal/plugins/stargazers/...
go test ./internal/plugins/repositories/...

# integration
go test ./tests/integration/... -run PluginsP2

# 全件
go test ./... -race
```

## 4. 実 GitHub データでの render 検証

`scripts/capture-mjun-references.sh` を流用 (011 で導入済の手順)。
事前準備: `INPUT_TOKEN` env に `read:user` / `read:org` / `read:project` scope 付き PAT、Chromium 利用可能な環境。

```sh
# 6 plugin それぞれを enable した SVG を render
./scripts/capture-mjun-references.sh \
  --plugins "sponsors,sponsorships,projects,notable,stargazers,repositories" \
  --output specs/013-unwired-graphql-data/plugins/screenshots/

# 各 SVG を Chromium で PNG 化
./scripts/capture-plugin-screenshot.sh \
  --input specs/013-unwired-graphql-data/plugins/screenshots/*.svg
```

成果物:

```
specs/013-unwired-graphql-data/plugins/screenshots/
├── mjun-sponsors-013.png
├── mjun-sponsorships-013.png
├── mjun-projects-013.png
├── mjun-notable-013.png
├── mjun-stargazers-013.png
└── mjun-repositories-pinned-013.png
```

PR 説明に上記 6 件への direct リンクを含める (本 spec の SC-005)。

## 5. データ取得 confirm (CLI 直叩き)

```sh
go run ./cmd/metrics-cli \
  --plugin "plugin_sponsors=yes" \
  --plugin "plugin_sponsorships=yes" \
  --plugin "plugin_projects=yes" \
  --plugin "plugin_notable=yes" \
  --plugin "plugin_stargazers=yes" \
  --plugin "plugin_repositories_pinned=yes" \
  --output /tmp/metrics-013.svg
```

JSON 出力で各 plugin が `Skipped=false` であることを確認:

```sh
go run ./cmd/metrics-cli ... --format json | jq '
  .data.plugins
  | { sponsors: .sponsors.skipped // false,
      sponsorships: .sponsorships.skipped // false,
      projects: .projects.skipped // false,
      notable: .notable.skipped // false,
      stargazers: .stargazers.skipped // false,
      repositories_pinned: (.repositories.pinned | length) }
'
```

期待値:

```json
{
  "sponsors": false,
  "sponsorships": false,
  "projects": false,
  "notable": false,
  "stargazers": false,
  "repositories_pinned": 6   // (pin 数。0 なら 0)
}
```

## 6. compliance gate

```sh
go test ./tests/compliance/...
```

採用 19 plugin リストや不採用 plugin 名の混入チェックは破ってはならない。

## 7. PR 提出時のチェックリスト

- [ ] 6 plugin × 4 テストケース pass (24+ cases)
- [ ] `go test ./...` all green
- [ ] golang-ci-lint / vet / vuln clean
- [ ] compliance test pass
- [ ] PR 説明に screenshots へのリンク 6 件
- [ ] PR 説明に rate cost の実測値 (`X-RateLimit-Used` の delta) 記載
- [ ] Constitution Check (5 原則) を PR 説明に表で表示
