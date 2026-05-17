# Upstream fixture cache

このディレクトリは `internal/tools/sync-fixtures` が出力する upstream
lowlighter/metrics の参照 JSON を保管します。

## 21-plugin baseline (`octocat.json`)

M4 Polish (T092) 時点では未生成です。生成手順:

1. `cd org_repo && npm install` (upstream の dev dependencies。約 1 GB)
2. `org_repo/tests/cases/octocat.yml` を作成 — 21 採用 slug (P1 5 / P2 12 /
   P3 4 = topics / starlists + languages.recent / languages.indepth)
   すべてを有効化した case を定義。`languages.recent` /
   `languages.indepth` は upstream では `plugins.languages` のサブモード
   として serialize されるので、`plugin_languages_sections` に
   `most-used,recently-used` を、`plugin_languages_indepth: yes` を
   それぞれ指定する必要がある (top-level の独立 plugin slug ではない)。
3. `go run ./internal/tools/sync-fixtures --user octocat --full` を実行
4. 出力された `tests/fixtures/upstream/octocat.json` を vendor + commit

未生成状態でも:

- `internal/tools/sync-fixtures` は `tests/cases/<user>.yml` 不在で
  exit 1、ただし `org_repo` 自体が無い環境では exit 2 で soft pass
- `tests/compatibility/json_test.go` の M4 互換テストは fixture 不在
  時に `t.Skip` するためビルドは緑のまま

上記 1-4 が手動完了するまで T092 / T093 は spec 上 "deferred" 扱い。
完了 SC-004 evidence (key/型 diff = 0) は手動再現時に PR body で報告。
