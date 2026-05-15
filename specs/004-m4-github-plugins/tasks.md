---

description: "Task list for m4-github-plugins (採用 21 plugin + base 拡張 + classic partial)"
---

# Tasks: GitHub プラグイン群 (M4)

**Input**: Design documents from `/specs/004-m4-github-plugins/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: 必須 (constitution 原則 IV + FR-014/015/016)。各 plugin に最低 5 ケースのテーブルテスト + JSON shape golden + SVG partial golden。chromedp / heavy 依存テストは build tag (`chromedp` / `heavy`) で隔離。

**Organization**: 6 フェーズ — Setup / Foundational / US1 (P1 MVP) / US2 (P2 GraphQL+REST) / US3 (P3 chromedp+heavy) / Polish。各 user story は独立にテスト可能。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 並列実行可能 (別ファイル、未完了依存なし)
- **[Story]**: US1〜US3 のうちどのストーリーに属するか
- 各タスク行に対象ファイルの相対パスを明記

## Path Conventions

- 単一 Go プロジェクトレイアウト: `internal/<pkg>/`, `assets/`, `tests/`
- 詳細は [plan.md §Project Structure](./plan.md)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 新規依存 (go-enry / go-git) を `go.mod` に追加し、Makefile / CI / sync-fixtures 拡張 / classic partial ディレクトリ整備。後続フェーズ全体が依存する。

- [ ] T001 Add new module dependencies to `go.mod`: `github.com/go-enry/go-enry/v2@latest`, `github.com/go-git/go-git/v5@latest`. **PR body MUST** list each dependency with the 「代替検討と棄却理由」 lines copied from `research.md` R-002 / R-003 (constitution 原則 V). `go mod tidy` runs lazily — each lib re-enters `go.mod`/`go.sum` automatically when its first importing task lands (go-enry → T065, go-git → T067).
- [ ] T002 [P] Add `test-heavy` target to `Makefile` per `quickstart.md §3`: `test-heavy: \n\tgo test -tags=heavy ./...`. Verify it runs `0 tests` cleanly until US3 task lands.
- [ ] T003 [P] Add a `test-heavy` job to `.github/workflows/go-ci.yml` in parallel to existing `test` and `test-chromedp` (M3) jobs. Job uses the same checkout + Go setup steps and runs `make test-heavy`.
- [ ] T004 [P] Extend `internal/tools/sync-fixtures/main.go` with a `--full` flag that causes the upstream `npm test` invocation to run with all 21 採用 plugin enabled (`docs/design/15-selection-answer.md` §6 のリスト)。Output remains `tests/fixtures/upstream/<login>.json`. Add unit test `internal/tools/sync-fixtures/main_test.go::TestFullFlag_TogglesPluginInputs` covering the flag plumbing.
- [ ] T005 [P] Create the partial template directory skeleton `internal/templates/classic/partials/` with a placeholder `internal/templates/classic/partials/doc.go` that documents the file-naming convention (`<plugin-slug>.svg.tmpl`) per `contracts/partial-classic-m4.md §1`. No `.svg.tmpl` files yet — those land per-plugin in US1/US2/US3.
- [ ] T006 Extend `internal/templates/classic/classic.go` with a partial dispatcher: given `pc.Data.Plugins` and `pc.Inputs`, iterate over the 21 採用 plugin slugs and call the partial template (`partials/<slug>.svg.tmpl`) only when `pc.Inputs["plugin_<slug>"]==true`. Skip if `result.Skipped==true` per `contracts/partial-classic-m4.md §6`. Wrap each partial output in `<g class="plugin-<slug>" data-plugin="<slug>" transform="translate(0, <offset>)">...</g>`. Plumb the offset accumulator. Update existing `classic_test.go` to assert the wrapper structure on a 0-plugin run (no partials emitted).

**Checkpoint (Phase 1)**: `go test ./...` still green with new deps unimported; `make test-heavy` runs (empty); CI workflow has 3 parallel test jobs; `internal/templates/classic/partials/` exists with doc.go; classic dispatcher is wired but emits no partials yet.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: base plugin の完成形拡張 (organization / indepth / paging) と `REST.Scopes()` helper。これらが揃わないと US1 以降の plugin はテスト fixture を組み立てることすらできない。

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### REST scopes helper

- [ ] T007 [P] Add `internal/githubapi/scopes.go` per `data-model.md E-050`: `(r *REST) Scopes() ([]string, error)` — first call issues `GET /` and parses `X-OAuth-Scopes` header (comma-separated), caches the slice; subsequent calls return the cached value. Add a `scopes_test.go` with 4 cases: (a) authenticated token with `repo,read:user`, (b) unauthenticated (no header) returning `[]`, (c) HTTP error returning error, (d) cache reuse — 2nd call must not issue a 2nd HTTP request (mock counter).

### base plugin organization branch (T-018)

- [ ] T008 [P] Add `assets/plugins/base/schema.graphql` extension covering `organization(login)` with `membersWithRole(first:N) { nodes { login name avatarUrl } pageInfo {...} }` and `repositories(first:N, orderBy:...)`. Run `make gen-graphql` (or equivalent genqlient regen) — extend Makefile target if missing. Commit regenerated `internal/githubapi/graphql_gen.go`. **Note**: the schema delta must be 1:1 with upstream `./org_repo/source/app/metrics/queries/organization.graphql` (constitution 原則 I).
- [ ] T009 [P] Add `internal/plugins/base/organization.go` per `contracts/plugin-base-extension.md §1` defining `runOrganization(ctx, pc, login) (any, error)`. Wires the new GraphQL query, populates `pc.Data.Organization` (Members + Repositories), and triggers paging via `repositories.go` helpers (T011). Wires into `base.Run` dispatcher when `pc.Data.Account == AccountOrganization`.
- [ ] T010 [P] Add `internal/plugins/base/base_test.go::TestRun_Organization` (new test in existing file) covering: account kind detection (`organization(login).login` exists), `Data.Organization.Members` populated with 100 fixture members, `Data.Organization.Repositories` populated via paging mock returning 250 repos across 3 pages. Mocked GraphQL via `internal/testutil/mocks.GraphQLMux`.

### base plugin repositories paging (T-020)

- [ ] T011 Rewrite `internal/plugins/base/repositories.go` per `contracts/plugin-base-extension.md §3`: introduce `pagingState` struct holding `batchSize, cursor, acc`. Implement `fetchRepositories(ctx, pc, initialBatch int) ([]Repository, error)` with batch-halving on 502/timeout (`batch = max(1, batch/2)`, retry same cursor). Used by both `runUser` and `runOrganization`. Replace M1 single-shot fetch.
- [ ] T012 [P] Add `internal/plugins/base/repositories_test.go::TestPaging_BatchHalving`: mocked GraphQL returns 502 on first call, then 200 on second (with `batch=50` confirmed via captured request body), continues to third 200 (`batch=50`). Assert final `Repositories` length == sum of nodes and `endCursor` properly threaded.
- [ ] T013 [P] Add `TestPaging_PartialFailure`: mock returns 5xx persistently. Assert (a) `Repositories` contains the accumulator at point of failure, (b) `Result.Errors` (via the `*Result` plumbing) carries a `*RetryableError` with `"batch=1"` in message, (c) `Run` returns `(nil, nil)` (best-effort completion). **Note**: `*Result` plumbing — current M2 `engine.Compute` passes `*Result` into dispatch (M3 T025) but **not** into plugins; base writes errors via `pc.Data.Errors` (new field added in T020 below) which core collects into `Result.Errors`.

### base plugin indepth query (T-019)

- [ ] T014 [P] Add `internal/plugins/base/indepth.go` per `contracts/plugin-base-extension.md §2`: `runIndepth(ctx, pc) error` issuing the additional GraphQL query (commits totalCount, issues totalCount, pullRequests totalCount, contributionsCollection.contributionCalendar). Populates `pc.Data.Computed` fields. Trigger condition matches §2.1 (any of the listed plugin inputs being truthy). Indepth GraphQL schema delta lands in T008.
- [ ] T015 [P] Add `internal/plugins/base/indepth_test.go::TestIndepth_TriggerMatrix`: 5 cases — (a) no indepth flags → skip, (b) `plugin_repositories_pinned=true` → trigger, (c) `plugin_isocalendar=true` → trigger, (d) `plugin_habits=true` → trigger, (e) `plugin_notable_indepth=true` + `plugin_notable=true` → trigger.
- [ ] T016 [P] Add `TestIndepth_DegradedPath` in the same file: indepth GraphQL returns 5xx. Assert (a) base standard query results are preserved in `Data.Computed`, (b) indepth-only fields (e.g. `TotalCommits`) are zero-value, (c) error appended to `Data.Errors`.

### base plugin: data plumbing

- [ ] T017 Extend `internal/plugins/base/base.go::Run` to call (in order): account kind decision → `runUser` or `runOrganization` → `fetchRepositories` (paging) → `runIndepth` (if triggered). Preserve M1 behavior on user branch (regression-safe). Update doc comment.
- [ ] T018 [P] Update `internal/plugins/base/base_test.go::TestRun_User` (M1 existing) to additionally assert `Data.Computed.Repositories` is populated post-paging. Existing M1 assertions remain green.

### `pc.Data.Plugins` / `pc.Data.Errors` plumbing

- [ ] T019 [P] In `internal/plugins/plugin.go`, ensure `PluginContext` exposes `pc.Data.Plugins map[string]any` (added in M2 if not already). No type change — just confirm. If `pc.Data.Errors []error` does NOT exist, add it with mutex-protected setter `(d *Data) AppendError(err error)` for plugins to record `*RetryableError` while `core.RunPlugins` collects into `Result.Errors`.
- [ ] T020 Extend `internal/plugins/core/run_plugins.go` to drain `pc.Data.Errors` after each plugin's `Run` and append to `*Result.Errors`. Keep the M2 mutex semantics intact. Update `run_plugins_test.go` with a stub plugin that appends to `Data.Errors` to assert the drain path.

**Checkpoint (Phase 2)**: `base` plugin is complete (user + organization + indepth + batch-halving paging); `REST.Scopes()` returns cached scopes; `Data.Errors` plumbing flows into `Result.Errors`. All existing M1/M2/M3 tests still green. No 21-plugin work has started.

---

## Phase 3: User Story 1 — P1 MVP プラグイン 5 個 (Priority: P1) 🎯 MVP

**Goal**: classic SVG に languages (標準) / activity / achievements / repositories / isocalendar の 5 plugin が出力され、README に意味あるデータが乗る MVP 状態を達成する。

**Independent Test**: mocked GraphQL/REST + classic + `engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})` を呼ぶ。`Result.Output` に 5 plugin の必須 DOM 痕跡 (`contracts/partial-classic-m4.md §5`) がすべて含まれ、`Result.Errors` 空、`data.Plugins` に 5 キー non-null。

### plugin: languages 標準モード (T-041)

- [ ] T021 [P] [US1] Add `internal/plugins/languages/languages_test.go` with 5 table cases per `contracts/plugin-p1-mvp.md §1.5`: (a) octocat 正常系 — `Favorites` 上位 3 件確認, (b) `_limit=3` で 4 件目以降 `Other` 集約, (c) `_aliases="TypeScript:JavaScript"` で言語合流, (d) `Computed.Repositories` 空 → `Skipped=true`, (e) `_ignored="Markdown"` で除外。Fixture vendor: `tests/fixtures/plugins/languages/{octocat_repos.json, empty.json, single.json}`.
- [ ] T022 [P] [US1] Implement `internal/plugins/languages/languages.go` per `contracts/plugin-p1-mvp.md §1`: `Plugin` interface実装、`init()` で `plugins.Register(Plugin)`。Result 構造体は [data-model.md E-010](./data-model.md#E-010)。`Run` は `pc.Imports.Get("base")` から `Computed.Repositories` を読み、languages.edges 集計 → sort → Favorites / Other 分割 → `Result.Mostly` 確定。Make T021 pass.
- [ ] T023 [P] [US1] Add `internal/templates/classic/partials/languages.svg.tmpl` per `contracts/partial-classic-m4.md §5` (`<g class="languages-progress">` + `<rect class="language-bar">`)。Add `tests/golden/classic/m4/languages.svg` via `go test ./internal/templates/classic/... -run TestPartial_Languages_Golden -update`. Add `tests/golden/json/m4/languages.json` via `go test ./internal/plugins/languages/... -run TestRun_GoldenShape -update`.

### plugin: activity (T-044)

- [ ] T024 [P] [US1] Add `internal/plugins/activity/activity_test.go` with 5 cases per `contracts/plugin-p1-mvp.md §2.5`: 正常系 / `_limit=1` / 空 events / 5xx → `*RetryableError` / `_visibility="private"` filter。Fixture: `tests/fixtures/plugins/activity/{octocat_events.json, empty_events.json, error_500.json}`.
- [ ] T025 [P] [US1] Implement `internal/plugins/activity/activity.go`: REST `/users/X/events?per_page=100` を `plugin_activity_load` (default 300) 件まで paging、type filter + days filter + skipped/ignored 適用、Date 降順で limit 件に切り詰め。Result は [data-model.md E-013](./data-model.md#E-013)。Make T024 pass.
- [ ] T026 [P] [US1] Add `internal/templates/classic/partials/activity.svg.tmpl` (`<text class="activity-event">` + octicon プレースホルダ `:octicon-<event-type>-16:`)。Golden generation: `tests/golden/classic/m4/activity.svg` + `tests/golden/json/m4/activity.json`.

### plugin: achievements (T-045)

- [ ] T027 [P] [US1] Add `internal/plugins/achievements/achievements_test.go` with 5 cases per `contracts/plugin-p1-mvp.md §3.5`: 正常系 (`_threshold="C"` で 5 件以上 List)、`_threshold="S"` で 1 件未満許容、`_only="commits,stars"` で限定、`_ignored="organizations"` で除外、`Computed` 空 → `Skipped=true`。Fixture: `tests/fixtures/plugins/achievements/{octocat_computed.json, empty_computed.json}`.
- [ ] T028 [P] [US1] Implement `internal/plugins/achievements/achievements.go`: rank テーブル (`docs/design/06-plugins-detail.md §2.1`) を package-level constant でハードコード、`base.Computed` の各統計値を rank 関数で `"S"/"A"/"B"/"C"/"X"` に変換、threshold 以上の achievement を List 構築、`only` / `ignored` フィルタ適用。Result は [data-model.md E-014](./data-model.md#E-014)。Make T027 pass.
- [ ] T029 [P] [US1] Add `internal/templates/classic/partials/achievements.svg.tmpl` (`<svg class="achievement">` または `data-achievement="<rank>"`)。Golden: `tests/golden/classic/m4/achievements.svg` + `tests/golden/json/m4/achievements.json`.

### plugin: repositories (T-046)

- [ ] T030 [P] [US1] Add `internal/plugins/repositories/repositories_test.go` with 5 cases per `contracts/plugin-p1-mvp.md §4.5`: Featured 上位 N 件、`_pinned=true` で pinnedItems 追加 fetch、`_starred=true` で starred 追加、`_forks=false` で fork 除外、`_random=true` で fisher-yates 順序固定 (seed 注入)。Fixture: `tests/fixtures/plugins/repositories/{octocat_pinned.json, octocat_starred.json}`.
- [ ] T031 [P] [US1] Implement `internal/plugins/repositories/repositories.go`: `base.Computed.Repositories` から `affiliations` / `forks` / `skipped` でフィルタしつつ `_order` で sort → `Featured` 構築。`_pinned/_starred/_random` フラグごとに並列 (errgroup) で追加 GraphQL/集計を実行。Result は [data-model.md E-015](./data-model.md#E-015)。Make T030 pass.
- [ ] T032 [P] [US1] Add `internal/templates/classic/partials/repositories.svg.tmpl` (`<a class="repository">` × ≥ 1)。Golden: `tests/golden/classic/m4/repositories.svg` + `tests/golden/json/m4/repositories.json`.

### plugin: isocalendar (T-049)

- [ ] T033 [P] [US1] Add `internal/plugins/isocalendar/isocalendar_test.go` with 5 cases per `contracts/plugin-p1-mvp.md §5.5`: 正常系 half-year で 26 週、full-year で 53 週、`Streak.Max` greedy 算出 (連続 N 日 fixture)、`Streak.Current` (末尾連続)、organization アカウント → `Skipped=true`。Fixture: `tests/fixtures/plugins/isocalendar/{octocat_halfyear.json, octocat_fullyear.json, streak_5days.json}`.
- [ ] T034 [P] [US1] Implement `internal/plugins/isocalendar/isocalendar.go`: GraphQL `user.contributionsCollection(from, to) { contributionCalendar }` を duration に応じて 26/53 週分、weeks 配列を `[7]int` に整形、Streak greedy 計算、Sum/Average 算出。Result は [data-model.md E-016](./data-model.md#E-016)。Make T033 pass.
- [ ] T035 [P] [US1] Add `internal/templates/classic/partials/isocalendar.svg.tmpl` (`<g class="calendar">` + `<rect class="calendar-day">`)。Golden: `tests/golden/classic/m4/isocalendar.svg` + `tests/golden/json/m4/isocalendar.json`.

### US1 integration test

- [ ] T036 [US1] Add `tests/integration/plugins_p1_test.go::TestComputeSVG_P1AllPlugins`: mocked GraphQL/REST で octocat 相当 fixture を組み立て、`pc.Inputs` で 5 P1 plugin をすべて有効化、`engine.Compute(Request{Login:"octocat", Template:"classic", Format:"svg"})`。Assert `Result.MIME == "image/svg+xml"`、`Result.Errors == nil`、SVG に 5 plugin の必須 DOM 痕跡 (`contracts/partial-classic-m4.md §5` table) がすべて含まれること。
- [ ] T037 [US1] Add `tests/integration/plugins_p1_test.go::TestComputeJSON_P1AllPlugins`: 同 fixture で `Format:"json"`、`data.Plugins` に 5 キー (languages, activity, achievements, repositories, isocalendar) が non-null で含まれ、各構造体が data-model.md と一致することを assert。

**Checkpoint (US1)**: P1 5 plugin が動き、SVG/JSON に上流互換構造で出力される。M3 完了時の「空白に近い SVG」状態から脱出 = README に意味あるデータが乗る MVP 完成。

---

## Phase 4: User Story 2 — P2 GraphQL+REST 単体プラグイン 12 個 (Priority: P2)

**Goal**: P1 の 5 plugin に加え、GraphQL/REST だけで完結する 12 plugin (habits / calendar / stars / people / notable / contributors / reactions / projects / sponsors / sponsorships / stargazers / traffic) を実装し、採用 21 中 17 plugin (81%) が動く状態にする。

**Independent Test**: 各 plugin を `tests/golden/json/m4/<name>.json` の shape と一致させる + `tests/integration/plugins_p2_test.go` で 4 plugin ずつのバンドル e2e を 3 回実行 (合計 12 plugin)。

### plugin: calendar (T-050)

- [ ] T038 [P] [US2] Add `internal/plugins/calendar/calendar_test.go` (5 cases) + fixture `tests/fixtures/plugins/calendar/octocat_5years.json`.
- [ ] T039 [P] [US2] Implement `internal/plugins/calendar/calendar.go` per [contracts/plugin-p2-graphql.md §1](./contracts/plugin-p2-graphql.md#1-calendar-t-050)。Result は [data-model.md E-020](./data-model.md#E-020)。Make T038 pass.
- [ ] T040 [P] [US2] Add `internal/templates/classic/partials/calendar.svg.tmpl` (`<g class="calendar-year">` × ≥ 1) + golden file 生成。

### plugin: habits (T-051)

- [ ] T041 [P] [US2] Add `internal/plugins/habits/habits_test.go` (5 cases) + fixtures `tests/fixtures/plugins/habits/{octocat_commits.json, indent_tabs.json, no_commits.json}`.
- [ ] T042 [P] [US2] Implement `internal/plugins/habits/habits.go` per [contracts/plugin-p2-graphql.md §2](./contracts/plugin-p2-graphql.md#2-habits-t-051)。PushEvent から最新 N コミット → 各 commit diff (mocked REST `/repos/X/commits/Y`) → indent / lines / Days/Hours 集計。Result は [data-model.md E-021](./data-model.md#E-021)。Make T041 pass.
- [ ] T043 [P] [US2] Add `internal/templates/classic/partials/habits.svg.tmpl` (`<g class="habit-chart">` charts=true 時) + golden。

### plugin: stars (T-052)

- [ ] T044 [P] [US2] Add `internal/plugins/stars/stars_test.go` (5 cases) + fixture `tests/fixtures/plugins/stars/octocat_starred.json`.
- [ ] T045 [P] [US2] Implement `internal/plugins/stars/stars.go` per [contracts/plugin-p2-graphql.md §3](./contracts/plugin-p2-graphql.md#3-stars-t-052)。GraphQL `user.starredRepositories(orderBy:STARRED_AT, last:limit)`。Result は [data-model.md E-022](./data-model.md#E-022)。Make T044 pass.
- [ ] T046 [P] [US2] Add `internal/templates/classic/partials/stars.svg.tmpl` (`<a class="starred-repo">` × ≥ 1) + golden。

### plugin: people (T-055)

- [ ] T047 [P] [US2] Add `internal/plugins/people/people_test.go` (5 cases) + fixtures `tests/fixtures/plugins/people/{followers.json, following.json, contributors.json, unknown_type.json}`.
- [ ] T048 [P] [US2] Implement `internal/plugins/people/people.go` per [contracts/plugin-p2-graphql.md §4](./contracts/plugin-p2-graphql.md#4-people-t-055)。`_types` ごとに別 GraphQL を並列発行 (errgroup)、未知 type は WARN ログ 1 行で無視、`_shuffle=true` で fisher-yates。Result は [data-model.md E-025](./data-model.md#E-025)。Make T047 pass.
- [ ] T049 [P] [US2] Add `internal/templates/classic/partials/people.svg.tmpl` (`<g class="person">` + `<image class="avatar">`) + golden。

### plugin: notable (T-056)

- [ ] T050 [P] [US2] Add `internal/plugins/notable/notable_test.go` (5 cases) + fixture `tests/fixtures/plugins/notable/octocat_contribs.json`.
- [ ] T051 [P] [US2] Implement `internal/plugins/notable/notable.go` per [contracts/plugin-p2-graphql.md §5](./contracts/plugin-p2-graphql.md#5-notable-t-056)。`_indepth=true` の追加クエリ条件分岐込み。Result は [data-model.md E-026](./data-model.md#E-026)。Make T050 pass.
- [ ] T052 [P] [US2] Add `internal/templates/classic/partials/notable.svg.tmpl` (`<g class="notable-contrib">`) + golden。

### plugin: contributors (T-059, M4 では account 検証のみ)

- [ ] T053 [P] [US2] Add `internal/plugins/contributors/contributors_test.go` (3 cases — user account → skipped, organization → skipped, repository account → would-be normal but M4 doesn't have repository templates yet so also `Skipped=true, SkippedReason="repository template not yet available"`).
- [ ] T054 [P] [US2] Implement `internal/plugins/contributors/contributors.go` per [contracts/plugin-p2-graphql.md §6](./contracts/plugin-p2-graphql.md#6-contributors-t-059)。M4 段階では account kind 検証 + skipped path のみ実装、実 GraphQL/REST 呼び出しは M7 (repository template) 完成後の N-task で活性化。Result は [data-model.md E-027](./data-model.md#E-027)。Make T053 pass.
- [ ] T055 [P] [US2] Add `internal/templates/classic/partials/contributors.svg.tmpl` — skipped=true 時は空文字列を返すガードのみ。golden は `tests/golden/classic/m4/contributors.svg` を空 SVG fragment で commit (ファイル存在の整合性確保)。

### plugin: reactions (T-062)

- [ ] T056 [P] [US2] Add `internal/plugins/reactions/reactions_test.go` (5 cases) + fixtures `tests/fixtures/plugins/reactions/{issues.json, comments.json, discussions.json, mixed.json, empty.json}`.
- [ ] T057 [P] [US2] Implement `internal/plugins/reactions/reactions.go` per [contracts/plugin-p2-graphql.md §7](./contracts/plugin-p2-graphql.md#7-reactions-t-062)。`user.issues / issueComments / discussionComments` の reactions を集計、`_details=true` で `Details map[emoji]int` を埋める。Result は [data-model.md E-028](./data-model.md#E-028)。Make T056 pass.
- [ ] T058 [P] [US2] Add `internal/templates/classic/partials/reactions.svg.tmpl` (`<text class="reaction-count">`) + golden。

### plugin: projects (T-063, scope 検証付き)

- [ ] T059 [P] [US2] Add `internal/plugins/projects/projects_test.go` (5 cases) + fixtures `tests/fixtures/plugins/projects/{has_scope.json, no_scope.json, user_projects.json, repo_projects.json}`. `no_scope.json` ケースで `_test.go` 内で `REST.Scopes()` を `[]string{}` (空) に注入し `Skipped=true, SkippedReason="missing read:project scope"` を assert。
- [ ] T060 [P] [US2] Implement `internal/plugins/projects/projects.go` per [contracts/plugin-p2-graphql.md §8](./contracts/plugin-p2-graphql.md#8-projects-t-063)。`pc.REST.Scopes()` で `read:project` 不在を検出 → `Skipped=true`。Result は [data-model.md E-029](./data-model.md#E-029)。Make T059 pass.
- [ ] T061 [P] [US2] Add `internal/templates/classic/partials/projects.svg.tmpl` (`<g class="project">` skipped=false 時のみ) + golden。

### plugin: sponsors (T-064, scope 検証付き)

- [ ] T062 [P] [US2] Add `internal/plugins/sponsors/sponsors_test.go` (5 cases) + fixtures `tests/fixtures/plugins/sponsors/{active.json, past_included.json, no_scope.json}`. `no_scope.json` で `read:user` + `read:org` 両方不在 → `Skipped=true`。
- [ ] T063 [P] [US2] Implement `internal/plugins/sponsors/sponsors.go` per [contracts/plugin-p2-graphql.md §9](./contracts/plugin-p2-graphql.md#9-sponsors-t-064)。`_past=true` で `Past` フィールド埋め。Result は [data-model.md E-030](./data-model.md#E-030)。Make T062 pass.
- [ ] T064 [P] [US2] Add `internal/templates/classic/partials/sponsors.svg.tmpl` (`<g class="sponsor">`) + golden。

### plugin: sponsorships (T-065)

- [ ] T065 [P] [US2] Add `internal/plugins/sponsorships/sponsorships_test.go` (5 cases) + fixture `tests/fixtures/plugins/sponsorships/viewer_sponsoring.json`.
- [ ] T066 [P] [US2] Implement `internal/plugins/sponsorships/sponsorships.go` per [contracts/plugin-p2-graphql.md §10](./contracts/plugin-p2-graphql.md#10-sponsorships-t-065)。`viewer.sponsorshipsAsSponsor(activeOnly:false)` を呼び `Active` / `Past` に振り分け。Result は [data-model.md E-031](./data-model.md#E-031)。Make T065 pass.
- [ ] T067 [P] [US2] Add `internal/templates/classic/partials/sponsorships.svg.tmpl` (`<g class="sponsored">`) + golden。

### plugin: stargazers (T-066, worldmap は nil)

- [ ] T068 [P] [US2] Add `internal/plugins/stargazers/stargazers_test.go` (5 cases) + fixtures `tests/fixtures/plugins/stargazers/{repo_stargazers.json, user_account.json, worldmap_input.json}`. `worldmap_input.json` で `_worldmap=true` 指定でも `Result.Worldmap == nil` + WARN ログを assert。
- [ ] T069 [P] [US2] Implement `internal/plugins/stargazers/stargazers.go` per [contracts/plugin-p2-graphql.md §11](./contracts/plugin-p2-graphql.md#11-stargazers-t-066-worldmap-は-nil)。GraphQL `repository.stargazers(orderBy:STARRED_AT, first:N)` paging で `List` 構築、`Charts` を時系列に整形。`Worldmap` は **常に nil** (M4 では実装しない、R-012)。Result は [data-model.md E-032](./data-model.md#E-032)。Make T068 pass.
- [ ] T070 [P] [US2] Add `internal/templates/classic/partials/stargazers.svg.tmpl` (`<g class="stargazers-charts">`、worldmap セクションは省略) + golden。

### plugin: traffic (T-068, scope 検証付き)

- [ ] T071 [P] [US2] Add `internal/plugins/traffic/traffic_test.go` (5 cases) + fixtures `tests/fixtures/plugins/traffic/{has_repo_scope.json, no_repo_scope.json, partial_403.json}`. `partial_403.json` で 1 repo が 403 を返しても他 repo の集計が継続することを assert。
- [ ] T072 [P] [US2] Implement `internal/plugins/traffic/traffic.go` per [contracts/plugin-p2-graphql.md §12](./contracts/plugin-p2-graphql.md#12-traffic-t-068)。`pc.REST.Scopes()` で `repo` 不在 → `Skipped=true`。`base.Computed.Repositories` を並列 GET、403 repo は除外 continue。Result は [data-model.md E-033](./data-model.md#E-033)。Make T071 pass.
- [ ] T073 [P] [US2] Add `internal/templates/classic/partials/traffic.svg.tmpl` (`<text class="traffic-count">` skipped=false 時のみ) + golden。

### US2 integration tests

- [ ] T074 [US2] Add `tests/integration/plugins_p2_test.go::TestComputeSVG_P2Bundle_A` covering 4 plugin (habits / calendar / stars / people)、`tests/integration/plugins_p2_test.go::TestComputeSVG_P2Bundle_B` (notable / contributors / reactions / projects)、`tests/integration/plugins_p2_test.go::TestComputeSVG_P2Bundle_C` (sponsors / sponsorships / stargazers / traffic)。各バンドルで mocked deps を組み立て classic SVG 経由で必須 DOM 痕跡を assert。
- [ ] T075 [US2] Add `tests/integration/plugins_p2_test.go::TestComputeJSON_P2AllPlugins`: 12 plugin すべてを有効化した状態で `Format:"json"`、`data.Plugins` に 12 キー non-null で含まれることを assert (P2 単独、P1 とは独立にテスト可能)。

**Checkpoint (US2)**: 採用 21 plugin 中 17 (P1+P2) が動く。GraphQL/REST 単体経路は完成。chromedp / heavy 依存 plugin は未着手。

---

## Phase 5: User Story 3 — P3 chromedp + heavy 依存プラグイン 4 個 (Priority: P3)

**Goal**: 採用 21 plugin の最後の 4 個 (languages.recent / languages.indepth / topics / starlists) を実装し、上流互換性 21/21 を達成する。chromedp 経路と heavy 経路は build tag で隔離。

**Independent Test**: `make test-chromedp` (topics / starlists) + `make test-heavy` (languages.recent / indepth) の 2 ジョブが個別に green。通常 `go test ./...` は P3 plugin のテストを skip した状態で green を維持。

### plugin: languages.recent (T-042, heavy)

- [ ] T076 [P] [US3] Add `internal/plugins/languages/recent_heavy_test.go` (build tag `//go:build heavy`) with 5 cases per [contracts/plugin-p3-heavy.md §1.5](./contracts/plugin-p3-heavy.md#16-エッジケース): 正常系 (3 commits, JS+Go)、`metrics.run.linguist=false` extras で skipped、PushEvent 0 件、`_days=1` で 1 日以内のみ、`_categories="programming"` で go-enry filter。Fixture: `tests/fixtures/plugins/languages/recent/{octocat_push.json, no_push.json, mixed_langs.json}`.
- [ ] T077 [P] [US3] Implement `internal/plugins/languages/recent.go` per [contracts/plugin-p3-heavy.md §1](./contracts/plugin-p3-heavy.md#1-languagesrecent-t-042)。REST `/users/X/events` PushEvent → 各 commit `/repos/X/commits/Y` の files → `go-enry.GetLanguage(filename, nil)` で言語判定 → T-041 と同じ集計ロジック (`_aliases` 等再利用可)。Result は [data-model.md E-011](./data-model.md#E-011)。Make T076 pass.
- [ ] T078 [P] [US3] Add classic partial の `recently-used` セクション分岐: `internal/templates/classic/partials/languages.svg.tmpl` (US1 T023 既存) に `{{if has "recently-used" .Inputs.plugin_languages_sections}}` ブロックを追加して `<g class="languages-recent">` を出力。partial 自体は 1 ファイルなので [P] 不可。golden 更新は T023 の path を re-update。

### plugin: languages.indepth (T-043, heavy)

- [ ] T079 [P] [US3] Add `internal/plugins/languages/indepth_heavy_test.go` (build tag `//go:build heavy`) with 5 cases per [contracts/plugin-p3-heavy.md §2.6](./contracts/plugin-p3-heavy.md#26-エッジケース): 正常系 (3 repos × tree walk)、extras 不在で skipped、個別 repo timeout、全体 timeout、disk full 模擬。Fixture: `t.TempDir()` 配下に最小 `.git` ディレクトリを 3 つ組み立てる helper を `_heavy_test.go` 内で定義。実 GitHub clone は使わない。
- [ ] T080 [P] [US3] Implement `internal/plugins/languages/indepth.go` per [contracts/plugin-p3-heavy.md §2](./contracts/plugin-p3-heavy.md#2-languagesindepth-t-043)。`base.Computed.Repositories` を errgroup で concurrency=4 並列、各 repo を `git.PlainClone(t.TempDir, Depth:1, URL)` → tree walk → go-enry 判定 → `LanguageBytes.Bytes` に集計。Timeout は context cancel で実現。Result は [data-model.md E-012](./data-model.md#E-012)。Make T079 pass.
- [ ] T081 [P] [US3] Add classic partial の `indepth` セクション分岐: T078 と同じく `languages.svg.tmpl` に `{{if .Result.Indepth}}` で `<g class="languages-indepth">` を出力するブロックを追加。

### plugin: topics (T-053, chromedp)

- [ ] T082 [P] [US3] Add `internal/plugins/topics/topics_test.go` (no build tag) covering 3 non-chromedp cases: `Deps.Render == nil` → skipped、extras `metrics.run.puppeteer.scrapping=false` → skipped、normal path は chromedp テストで網羅 (skip in pure-Go test)。
- [ ] T083 [P] [US3] Add `internal/plugins/topics/topics_chromedp_test.go` (`//go:build chromedp`) with 2 cases: 実 chromedp で `https://github.com/stars/<login>/topics` (mocked HTTP server で fake page を返す) を navigate、5 topic カードを抽出 / chromedp タイムアウト時 `*RetryableError`. Fixture: HTML page in `tests/fixtures/plugins/topics/stars_topics.html`.
- [ ] T084 [P] [US3] Implement `internal/plugins/topics/topics.go` per [contracts/plugin-p3-heavy.md §3](./contracts/plugin-p3-heavy.md#3-topics-t-053-chromedp)。`pc.Deps.Render` を `*render.Browser` に型 assertion (failed → skipped)、`Browser.NewTab(ctx)` でタブ取得、JS で DOM 抽出、JSON で Go へ受け渡し。Result は [data-model.md E-023](./data-model.md#E-023)。Make T082 + T083 pass.
- [ ] T085 [P] [US3] Add `internal/templates/classic/partials/topics.svg.tmpl` (`<g class="topic">` + `<text class="topic-name">`) + golden。

### plugin: starlists (T-054, chromedp)

- [ ] T086 [P] [US3] Add `internal/plugins/starlists/starlists_test.go` (no build tag) covering 3 non-chromedp cases (skipped paths).
- [ ] T087 [P] [US3] Add `internal/plugins/starlists/starlists_chromedp_test.go` (`//go:build chromedp`) with 3 cases: list 一覧抽出、`_languages=true` で各 list 内 repo の言語集計 (T-041 ロジック再利用)、chromedp タイムアウト。Fixture: `tests/fixtures/plugins/starlists/stars_lists.html` + 各 list 詳細 HTML。
- [ ] T088 [P] [US3] Implement `internal/plugins/starlists/starlists.go` per [contracts/plugin-p3-heavy.md §4](./contracts/plugin-p3-heavy.md#4-starlists-t-054-chromedp)。chromedp で starlist 一覧ページ → 各 list 詳細ページ navigate → repos 抽出 → (option) 言語集計。Result は [data-model.md E-024](./data-model.md#E-024)。Make T086 + T087 pass.
- [ ] T089 [P] [US3] Add `internal/templates/classic/partials/starlists.svg.tmpl` (`<g class="starlist">` + `<text class="starlist-name">`) + golden (build tag `chromedp` テスト経由で生成)。

### US3 integration tests

- [ ] T090 [US3] Add `tests/integration/plugins_p3_chromedp_test.go` (`//go:build chromedp`) covering topics + starlists の 2 plugin を同時に有効化した classic SVG e2e。chromedp 起動 + 必須 DOM 痕跡 assert。
- [ ] T091 [US3] Add `tests/integration/plugins_p3_heavy_test.go` (`//go:build heavy`) covering languages.recent + languages.indepth の 2 plugin を同時に有効化した classic SVG / JSON e2e。fixture `.git` ディレクトリ組み立て + tree walk 経路 assert。

**Checkpoint (US3)**: 採用 21 plugin が完全に動く。通常 CI ジョブは P3 を skip して < 60s で green、`test-chromedp` / `test-heavy` ジョブが個別に green。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 上流互換性 fixture 再生成、性能ベンチ、compliance test 拡張、ドキュメント更新、classic 全体 golden の再生成。

- [ ] T092 [P] Regenerate `tests/fixtures/upstream/octocat.json` with `go run ./internal/tools/sync-fixtures --user octocat --full` (T004 で追加した `--full` フラグを使用)。Vendor the resulting 21-plugin complete JSON. Inspect diff and commit.
- [ ] T093 [P] Update `tests/compatibility/json_test.go` to compare against the new 21-plugin upstream fixture (T092)。Assert key/型 diff = 0 across all 21 plugin entries. SC-004 evidence。
- [ ] T094 [P] Add `internal/engine/bench_full_test.go::BenchmarkCompute_Full_21Plugins` running `engine.Compute(Format:"svg")` with all 21 plugin enabled on mocked deps。Assert p95 wall time < 5s (SC-003)。Record actual on PR body。
- [ ] T095 [P] Add `internal/engine/bench_full_test.go::BenchmarkCompute_MemoryPeak` using `runtime.MemStats` + multiple iterations。Assert peak memory < 800 MB (SC-009)。Record actual on PR body。
- [ ] T096 [P] Extend `tests/compliance/compliance_test.go` (M2 existing) with a new test `TestCompliance_M4_AdoptedPlugins`: scan `internal/plugins/` for sub-directories, assert exactly the 21 採用 plugin slugs present (no unadopted plugins from FR-018: code/discussions/followup/gists/introduction/licenses/lines/skyline/support/anilist/leetcode/music/pagespeed/posts/rss/stackoverflow/steam/tweets/wakatime)。SC-007 evidence。
- [ ] T097 [P] Regenerate `tests/golden/classic/octocat.svg` (M2 baseline) via `go test ./tests/integration/... -run TestComputeSVG_ClassicOctocatGolden -update`。21 plugin 分の DOM が増えるが、constitution 原則 II (DOM 単位互換) を満たす限り受け入れ可能。Inspect diff, confirm only the new plugin sections appear, and commit。
- [ ] T098 [P] Update `README.md` "Status" line and feature list: 21 plugin all live, P1 5 (MVP), P2 12 (graphql/rest), P3 4 (chromedp/heavy)。Add `make test-heavy` to the contributor quickstart alongside `make test-chromedp`。
- [ ] T099 [P] Update `internal/plugins/base/base.go` doc comments: remove the M1 note that says "full upstream behavior ... lands incrementally with the M4 plugin work" since M4 is now landing — replace with a reference to `specs/004-m4-github-plugins/contracts/plugin-base-extension.md`。
- [ ] T100 Run [quickstart.md](./quickstart.md) end-to-end on a clean checkout, walking each numbered step。Capture pass status in the PR description; flag any step that requires `npm install` of upstream deps and document the skip behavior. Verify `make test`, `make test-chromedp`, `make test-heavy` all green on the maintainer environment。

---

## Dependencies

### Phase ordering

- **Phase 1 Setup** (T001-T006) → Phase 2 Foundational (T007-T020) → User Story phases (T021..T091) → Phase 6 Polish (T092-T100)
- User Story phases can run partially in parallel after Phase 2 completes, but T036-T037 (US1 integration) requires all P1 plugin tasks complete; T074-T075 (US2 integration) requires all P2 plugin tasks complete; T090-T091 (US3 integration) requires all P3 plugin tasks complete.

### Cross-phase blockers

- **T011 (paging rewrite)** blocks all plugins that consume `base.Computed.Repositories` (i.e., almost all of US1/US2/US3 except achievements which only reads computed scalars).
- **T007 (Scopes helper)** blocks T059/T060 (projects), T062/T063 (sponsors), T071/T072 (traffic) — only those 3 scope-gated plugins.
- **T006 (classic partial dispatcher)** blocks every `partials/*.svg.tmpl` task (T023, T026, T029, T032, T035, T040, T043, T046, T049, T052, T055, T058, T061, T064, T067, T070, T073, T085, T089).
- **T078 / T081** (languages partial extension) requires T023 (US1 languages partial) to land first — they extend the same file.

### Parallel opportunities within phases

- **Phase 1 (Setup)**: T002, T003, T004, T005 are all [P] (different files). T001 must complete first.
- **Phase 2 (Foundational)**:
  - T007 (scopes) is [P] with T008-T018 (base extensions).
  - T008, T009 are [P] (different files).
  - T010, T012, T013, T015, T016, T018 are all test files in different focus areas — all [P] amongst themselves.
- **Phase 3 (US1)**: 5 plugins × 3 tasks each = 15 plugin tasks, all [P] amongst the 5 plugins (each plugin owns 3 tasks that are sequential within itself: test → impl → partial). Integration tests T036-T037 are NOT [P] — sequential at the end.
- **Phase 4 (US2)**: 12 plugins × 3 tasks each = 36 plugin tasks, all [P] amongst the 12 plugins. Integration tests T074-T075 sequential.
- **Phase 5 (US3)**: 4 plugins × 3 tasks each = 12 plugin tasks, all [P] amongst the 4 plugins. Integration tests T090-T091 sequential.
- **Phase 6 (Polish)**: T092, T093, T094, T095, T096, T097, T098, T099 are all [P] (different files). T100 sequential (full quickstart walk).

### Suggested execution flow (1 developer)

```text
T001
 └─ T002 ∥ T003 ∥ T004 ∥ T005    (Phase 1 parallel batch)
     └─ T006                       (classic dispatcher)
         ├─ T007                   (scopes helper)
         └─ T008 → T009 → T011 → T014 → T017 → T020   (base extension serial chain)
             └─ T010 ∥ T012 ∥ T013 ∥ T015 ∥ T016 ∥ T018 ∥ T019  (base tests parallel)
                 └─ Phase 3 (US1) plugins in parallel: T021-T035, then T036, T037
                     └─ Phase 4 (US2) plugins in parallel: T038-T073, then T074, T075
                         └─ Phase 5 (US3) plugins in parallel: T076-T089, then T090, T091
                             └─ Phase 6 polish T092-T100
```

### Suggested execution flow (2 developers parallel)

```text
After Phase 2 (T007-T020) completes:

Dev A:
  - Phase 3 US1 plugins (T021-T035, T036-T037)
  - then half of Phase 4 US2 plugins (e.g., habits / calendar / stars / people / notable / contributors)
  - then Polish T092 / T093 / T097 / T100

Dev B:
  - Other half of Phase 4 US2 plugins (reactions / projects / sponsors / sponsorships / stargazers / traffic)
  - then Phase 5 US3 plugins (T076-T089, T090-T091)
  - then Polish T094 / T095 / T096 / T098 / T099
```

---

## Implementation Strategy

- **MVP first**: P1 (US1) を完走させて MVP リリース可能状態を作る。M3 完了時の「空白に近い SVG」状態から「他人に見せたくなる SVG」状態 = README に意味あるデータが乗る変化点。
- **Incremental delivery**: US1 → US2 → US3 の順で merge する 3 PR 分割。M3 のような 1 PR 巨大化を避ける。
- **Foundational gating**: T011 (paging rewrite) は base 全体の挙動を変えるので、M1/M2 既存テストへの regression を必ず確認してから次の plugin タスクに進む。
- **Build tag 隔離**: chromedp + heavy のテストが通常 CI ジョブを壊さないこと (= `go test ./...` が常に green) を Phase 5 着手前に必ず確認。
- **Golden file 戦略**: 各 plugin に 2 つの golden (`tests/golden/json/m4/<name>.json` + `tests/golden/classic/m4/<name>.svg`) を 1 タスクで commit する。`-update` フラグ生成 → 目視確認 → commit のループ。byte-identical な golden は文字列リテラル差分 (生成時刻等) で fragile になりがちなので、上流互換性は T093 の key/型 diff で担保し、partial golden は structural assert (regex / DOM presence) で補完してよい (M3 R-014 の経験則に従う)。

## 完了基準

T001..T100 すべて green、Quickstart 7 ステップ pass、CI 通常ジョブ + chromedp ジョブ + heavy ジョブの 3 並列が緑 (SC-008)、上流互換性 fixture diff 0 (SC-004)、compliance test 21/21 + 0/19 採用判定 (SC-007)、ベンチ p95 < 5s + メモリピーク < 800MB (SC-003 / SC-009)。
