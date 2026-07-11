# Upstream fixture cache

This directory stores the upstream lowlighter/metrics reference JSON
output by `internal/tools/sync-fixtures`.

## 21-plugin baseline (`octocat.json`)

As of M4 Polish (T092), this has not yet been generated. Generation steps:

1. `cd org_repo && npm install` (upstream's dev dependencies; about 1 GB)
2. Create `org_repo/tests/cases/octocat.yml`. Define a case that enables
   all 21 adopted slugs (P1 5 / P2 12 / P3 4 = topics / starlists +
   languages.recent / languages.indepth). Since `languages.recent` /
   `languages.indepth` are serialized upstream as submodes of
   `plugins.languages`, you need to specify `most-used,recently-used`
   for `plugin_languages_sections` and `plugin_languages_indepth: yes`
   respectively (they are not independent top-level plugin slugs).
3. Run `go run ./internal/tools/sync-fixtures --user octocat --full`
4. Vendor + commit the output `tests/fixtures/upstream/octocat.json`

While ungenerated:

- `internal/tools/sync-fixtures` exits 1 when `tests/cases/<user>.yml`
  is absent, but exits 2 for a soft pass in environments without
  `org_repo` itself
- The M4 compatibility tests in `tests/compatibility/json_test.go`
  `t.Skip` when the fixture is absent, so the build stays green

Until steps 1-4 above are completed manually, T092 / T093 are treated as
"deferred" per the spec. Once complete, SC-004 evidence (key/type diff = 0)
is reported in the PR body when manually reproduced.
