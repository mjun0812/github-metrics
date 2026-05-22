---

description: "Task list for 012 REST data-fetch wiring (traffic / contributors / repositories.{Starred,Random})"
---

# Tasks: REST Data-Fetch Wiring for 4 Plugins

**Input**: Design documents from `/specs/012-rest-data-fetch/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (required), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: ENABLED. The Constitution principle IV (テーブルテスト + Golden File) makes table tests + golden file tests mandatory for any plugin Run / render logic change. Test tasks are interleaved per user story below.

**Organization**: Tasks are grouped by user story (US1=traffic / US2=contributors / US3=repositories.Starred+Random) to enable independent implementation and verification.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependencies)
- **[Story]**: US1 / US2 / US3 — mapping to spec.md user story
- File paths are absolute under repo root

## Path Conventions

Per [plan.md §Project Structure](./plan.md#project-structure): `internal/plugins/<slug>/<slug>.go` + `<slug>_test.go` pattern (Single-project Go layout). New REST endpoint registration lives in existing `internal/testutil/mocks/`. Integration tests live in `tests/integration/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Working-tree readiness. The project is already initialized — these are sanity checks.

- [ ] T001 Verify branch `012-rest-data-fetch` checked out and `git status` clean. Run `go build ./...` to confirm the M4 baseline still compiles (`internal/plugins/{traffic,contributors,repositories}` unchanged on disk).
- [ ] T002 Confirm `internal/githubapi/rest.go` exposes the `Scopes(ctx)` + `Get(ctx, path)` helpers documented in [research.md §R-007](./research.md). If a helper is missing, this is a NO-OP (T-004 will add it under Foundational).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared helpers + REST mock plumbing reused by all 3 user stories. **No user story implementation can begin until this phase is complete.**

- [ ] T003 [P] Extend `internal/testutil/mocks/rest_mux.go` (or equivalent — currently embedded in test files per `tests/integration/plugins_p1_test.go::restEventsMux`) to support a Map-based path → response body lookup table. Pattern: take `map[string]string` keyed on URL path prefix, return the matching body via `http.RoundTripper`. Keep the existing `restEventsMux` working (no breaking change). ~80 LOC.
- [ ] T004 [P] Audit `internal/githubapi/rest.go` for a generic `GET(ctx, path)` returning `[]byte` + status. If missing, add it as a thin wrapper around the existing transport — DO NOT introduce a new client. Add `Scopes(ctx)` if not already present (mirror `internal/plugins/sponsors/sponsors.go::Run` usage). ~40 LOC.
- [ ] T005 [P] Add `internal/plugins/restutil/parallel.go`: a reusable `RunPerRepo(ctx, repos, concurrency, perRequestTimeout, fn)` helper that wraps the `errgroup + SetLimit + context.WithTimeout` pattern documented in [research.md §R-005](./research.md). Returns `(mu-protected results map, []error)`. ~60 LOC. *(If the abstraction proves premature later, inline the loop into each plugin — but extract first for testability.)*
- [ ] T006 [P] Table-test `restutil/parallel.go` in `internal/plugins/restutil/parallel_test.go`: cover happy-path (5 repos all success), partial-failure (3 succeed / 2 timeout), full-failure (all timeout), respect of concurrency cap. ~80 LOC.

**Checkpoint**: REST mock can serve arbitrary paths; the shared parallel-fetch helper is implemented + tested. All 3 user stories can now proceed independently.

---

## Phase 3: User Story 1 - Traffic plugin (Priority: P1) 🎯 MVP

**Goal**: Wire `/repos/{repo}/traffic/views` aggregation into `traffic.Run` so the partial (already shipped in 011) renders real data.

**Independent Test**: Per [quickstart.md §US1](./quickstart.md#us1-verification-traffic-plugin) — docker render against `mjun0812` with `repo`-scoped `GITHUB_TOKEN` produces a Traffic section with non-zero `N views (M unique)` + per-repo rows.

### Tests for User Story 1 (write FIRST, ensure FAIL)

- [ ] T007 [P] [US1] Add table-test cases TC-001..TC-005 from [contracts/plugin-traffic.md §6](./contracts/plugin-traffic.md#6-test-cases) to `internal/plugins/traffic/traffic_test.go`. Each case sets up a RESTMux + Computed.RepositoryList, calls `Run`, asserts on `Result.Views` / `Result.Total` / `Result.Skipped` / `len(pc.Data.Errors)`. The tests MUST fail at this stage (Run still returns the M4 stub Skipped path).

### Implementation for User Story 1

- [ ] T008 [US1] Modify `internal/plugins/traffic/traffic.go::Run` to:
  - keep the existing `plugin_traffic` gate;
  - call `pc.REST.Scopes(ctx)` and return Skipped (`"missing repo scope"`) when `repo` not present (FR-003);
  - iterate `pc.Data.Computed.RepositoryList`, skipping entries in `plugin_traffic_skipped` CSV;
  - parallelize per-repo via `restutil.RunPerRepo(ctx, repos, 4, 30*time.Second, fetchOne)` (FR-004);
  - in `fetchOne(ctx, repo)`: `pc.REST.Get(ctx, "/repos/<NameWithOwner>/traffic/views")` → JSON decode `{count int; uniques int}` → return `TrafficView`;
  - on per-repo error: `pc.Data.AppendError(xerrors.NewRetryableError(...))`, do not abort (FR-011);
  - aggregate into `Result.Views` map + `Result.Total`.
  Preserve the M4 Skipped path for `pc.REST == nil` / empty RepositoryList / plugin disabled. ~140 LOC.
- [ ] T009 [US1] Re-baseline `tests/golden/json/m4/traffic.json` (or add it if absent) via `go test ./internal/plugins/traffic/... -update` so the JSON shape with populated Views/Total lands. Confirm the diff matches the expected schema in [data-model.md §E-012-A](./data-model.md#e-012-a-trafficresult-既存-フィールド-populate-のみ).
- [ ] T010 [US1] Run `go test ./internal/plugins/traffic/...` — all TC-001..TC-005 GREEN. Run `go test ./...` to confirm no cross-package regression (`tests/integration/` may need a golden bump in Phase 6).

**Checkpoint**: traffic plugin produces real data end-to-end. US1 is independently verifiable per [quickstart.md §US1](./quickstart.md#us1-verification-traffic-plugin).

---

## Phase 4: User Story 2 - Contributors plugin user/org mode (Priority: P2)

**Goal**: Wire `/repos/{repo}/contributors` aggregation in user-and-org mode (repo-mode UNCHANGED).

**Independent Test**: Per [quickstart.md §US2](./quickstart.md#us2-verification-contributors-plugin-user-org-mode) — docker render against `mjun0812` with `plugin_contributors=yes` produces a Contributors section with ≥1 row and aggregated commit counts.

### Tests for User Story 2 (write FIRST, ensure FAIL)

- [ ] T011 [P] [US2] Add table-test cases TC-001..TC-006 from [contracts/plugin-contributors.md §6](./contracts/plugin-contributors.md#6-test-cases) to `internal/plugins/contributors/contributors_test.go`. Particularly: TC-001 (aggregation across 3 repos), TC-002 (sort tiebreak), TC-003 (limit truncation), TC-004 (ignored login). TC-005 (repo-mode unchanged) MUST pass before AND after T012.

### Implementation for User Story 2

- [ ] T012 [US2] Modify `internal/plugins/contributors/contributors.go::Run`:
  - keep the existing `plugin_contributors` gate;
  - branch on `pc.Data.Computed.Mode`: `"repository"` → existing repo-mode code path (UNCHANGED, FR-007); `"user"` / `"organization"` → new REST-aggregation path;
  - in the new path: iterate `pc.Data.Computed.RepositoryList`, parallel via `restutil.RunPerRepo`, fetch `/repos/<NameWithOwner>/contributors?per_page=100&anon=false`, decode `[]{login, avatar_url, contributions}`, filter by `plugin_contributors_ignored` CSV, accumulate into `map[login]*Contributor{Commits: sum, AvatarURL: first}` (FR-005);
  - sort `Commits` desc, `Login` asc tiebreak, truncate to `plugin_contributors_limit` (default 14, FR-006);
  - per-repo HTTP errors → `*RetryableError` on `Data.Errors` (FR-011), do not abort. ~120 LOC.
- [ ] T013 [US2] Re-baseline `tests/golden/json/m4/contributors.json` via `go test ./internal/plugins/contributors/... -update`. Inspect diff: existing repo-mode golden UNCHANGED, new user-mode golden contains the aggregated List.
- [ ] T014 [US2] Run `go test ./internal/plugins/contributors/...` — all TCs GREEN, including TC-005 (repo-mode regression guard).

**Checkpoint**: Contributors user/org mode produces real data. US2 independently verifiable.

---

## Phase 5: User Story 3 - Repositories.Starred + Random (Priority: P3)

**Goal**: Populate `Result.Starred` from `/users/{login}/starred` (REST) and `Result.Random` from a deterministic shuffle of `Result.Featured` (pure Go).

**Independent Test**: Per [quickstart.md §US3-part1](./quickstart.md#us3-part-1-verification-repositoriesstarred) and [§US3-part2](./quickstart.md#us3-part-2-verification-repositoriesrandom-determinism) — docker render produces non-empty `data.plugins.repositories.starred` and deterministic `data.plugins.repositories.random`.

### Tests for User Story 3 (write FIRST, ensure FAIL)

- [ ] T015 [P] [US3] Add table-test cases TC-001..TC-005 from [contracts/plugin-repositories-starred.md §6](./contracts/plugin-repositories-starred.md#6-test-cases) to `internal/plugins/repositories/repositories_test.go`. Cover the REST happy-path, mapping correctness (`private` → Visibility / `fork` → IsFork / `language` → Language.Name), network failure (502), gate-off, and missing login.
- [ ] T016 [P] [US3] Add table-test cases TC-001..TC-006 from [contracts/plugin-repositories-random.md §6](./contracts/plugin-repositories-random.md#6-test-cases) to `internal/plugins/repositories/repositories_test.go`. Cover determinism (same seed → same output), seed variance, n>len clamp, empty Featured, seed=0 fallback, random=0 (機能 off).

### Implementation for User Story 3

- [ ] T017 [US3] Extend `internal/plugins/repositories/repositories.go::Run` to handle Starred:
  - read inputs: `plugin_repositories_starred` (truthy), `plugin_repositories_limit` (default 8);
  - require `pc.Data.User.Login != ""` (else `Result.Starred = nil`);
  - `pc.REST.Get(ctx, "/users/<login>/starred?per_page=100&sort=created&direction=desc")` with 30s timeout;
  - decode response array; map each entry to `plugins.Repository` per the table in [data-model.md §E-012-C](./data-model.md#e-012-c-repositoriesresult-既存-starred--random-フィールド-populate);
  - truncate to limit; set `Result.Starred`. On error: `*RetryableError`, leave `Starred=nil`. ~80 LOC.
- [ ] T018 [US3] Add `internal/plugins/repositories/random.go` with `deterministicShuffle(in []Repository, seed int64, n int) []Repository` implementing the algorithm from [contracts/plugin-repositories-random.md §3](./contracts/plugin-repositories-random.md#3-execution). Use `math/rand/v2.NewPCG`. ~40 LOC.
- [ ] T019 [US3] In `repositories.go::Run`, after Featured is populated, call `deterministicShuffle(Featured, seed, n)` when `plugin_repositories_random > 0`, store in `Result.Random`. ~20 LOC.
- [ ] T020 [US3] Re-baseline `tests/golden/json/m4/repositories.json` via `go test ./internal/plugins/repositories/... -update`. Inspect diff: existing Featured UNCHANGED, new Starred + Random fields populated (Random uses a fixed test seed for deterministic golden).
- [ ] T021 [US3] Run `go test ./internal/plugins/repositories/...` — all TCs GREEN. Run TC-001 (determinism) twice in sequence to confirm reproducibility (`go test -count=2`).

**Checkpoint**: Repositories surface Starred + Random. US3 independently verifiable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end integration, golden coverage, documentation, regression validation.

- [ ] T022 Add `tests/integration/plugins_012_test.go`: drive `engine.Compute` through the classic template with all 4 wired plugins enabled (traffic / contributors user-mode / repositories with Starred+Random). Use the new RESTMux to serve `/repos/{repo}/traffic/views`, `/repos/{repo}/contributors`, `/users/{login}/starred`. Assert: rendered SVG contains `class="traffic-count"` (or new traffic markers), `class="contributor"`, `data-plugin="repositories"` with `Starred` + `Random` present in the JSON dump. Pattern: mirror `tests/integration/plugins_p1_test.go::TestComputeSVG_P1AllPlugins`. ~200 LOC.
- [ ] T023 [P] Update `tests/golden/json/octocat.json` if the M4 baseline golden now picks up Random + Starred fields (likely unchanged because the test fixtures don't enable those inputs — but confirm with `go test -update` + diff inspection).
- [ ] T024 [P] Update `tests/compliance/compliance_test.go::TestNoUnadoptedPluginReference` allow-list if any new file references a Go identifier that whole-word-matches an unadopted slug (e.g., a contributor login `chesspros` would false-flag the unadopted `chess` slug — add an allow-list entry with a 2-line justification, matching the existing `languages.go` / `partial.go` pattern).
- [ ] T025 [P] Document the 4 new behaviours in `specs/012-rest-data-fetch/README.md` (a short summary aimed at PR reviewers — links back to spec / plan / quickstart). ~40 lines.
- [ ] T026 Run `go test ./...` — all green. Run `gofumpt -l -extra .` + `golangci-lint run ./...` — clean. Verify constitution gate via `go test ./tests/compliance/...` — green.
- [ ] T027 Execute the [quickstart.md](./quickstart.md) end-to-end (US1 + US2 + US3 sections) against a real `mjun0812` token with `repo` scope. Capture before/after SVG diffs into `specs/012-rest-data-fetch/plugins/screenshots/` for the PR description's "Visual proof" section.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. T001 → T002 sequential (sanity checks).
- **Phase 2 (Foundational)**: After T001-T002. T003 / T004 / T005 [P] run in parallel; T006 depends on T005.
- **Phase 3 (US1)**: After Phase 2. T007 → T008 → T009 → T010 sequential within story.
- **Phase 4 (US2)**: After Phase 2. T011 → T012 → T013 → T014 sequential within story.
- **Phase 5 (US3)**: After Phase 2. T015 + T016 [P], then T017 / T018 / T019 sequential, then T020 → T021.
- **Phase 6 (Polish)**: After Phases 3–5 all complete. T022 must run after the 3 plugins are wired; T023 / T024 / T025 [P] can run in parallel; T026 / T027 sequential at the end.

### User Story Dependencies

- **US1 (P1, traffic)**: independent of US2 / US3 — touches only `internal/plugins/traffic/`.
- **US2 (P2, contributors)**: independent of US1 / US3 — touches only `internal/plugins/contributors/`.
- **US3 (P3, repositories)**: independent of US1 / US2 — touches only `internal/plugins/repositories/`.

All 3 stories can be assigned to different contributors **after Phase 2 completes**, with no cross-story merge conflicts.

### Within Each User Story

- Test tasks (T007 / T011 / T015 / T016) MUST be written FIRST and confirmed FAILING before implementation (constitution principle IV: golden tests are mandatory; TDD applies to behaviour changes).
- Implementation tasks must keep the M4 Skipped path intact as a fallback for missing inputs.
- Golden re-baseline tasks (T009 / T013 / T020) come AFTER the implementation lands and tests pass.

### Parallel Opportunities

- Phase 2: **3 tasks in parallel** (T003 mock infra / T004 REST helper audit / T005 parallel helper).
- Phase 3 ↔ Phase 4 ↔ Phase 5: **3 user stories in parallel** if 3 contributors available.
- Within Phase 5: T015 (Starred tests) ↔ T016 (Random tests) ↔ T018 (random.go).
- Phase 6: T023 / T024 / T025 [P] can run in parallel after T022 lands.

---

## Parallel Example: Phase 5 (User Story 3)

```bash
# Tests can land in parallel because they edit the same file but
# different test cases — easy merge:
Task: "Add Starred TC-001..TC-005 to internal/plugins/repositories/repositories_test.go"
Task: "Add Random  TC-001..TC-006 to internal/plugins/repositories/repositories_test.go"

# Implementation tasks have a partial order:
Task: "Implement Starred fetch in repositories.go::Run" (T017)
Task: "Add random.go with deterministicShuffle"          (T018) — independent
Task: "Wire Random into Run"                             (T019) — needs T018 done
```

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1 (Setup) — minutes.
2. Complete Phase 2 (Foundational) — ~half a day. **Blocks all stories.**
3. Complete Phase 3 (US1 / traffic) — ~half a day.
4. **STOP and VALIDATE**: Run [quickstart.md §US1](./quickstart.md#us1-verification-traffic-plugin) end-to-end. Demo to maintainers.
5. If approved, ship MVP. Pause if user feedback requests scope adjustment.

### Incremental Delivery

1. Setup + Foundational landed — branch ready for any of the 3 stories.
2. US1 (traffic) → test → quickstart → review.
3. US2 (contributors) → test → quickstart → review.
4. US3 (repositories Starred + Random) → test → quickstart → review.
5. Phase 6 polish → integration test → golden updates → quickstart full run.
6. Open PR with all 3 stories landed.

### Solo (likely path for this project)

Sequential: T001 → T002 → (T003 / T004 / T005 / T006 in quick parallel iteration) → US1 → US2 → US3 → Polish. Estimated 2–3 working days end-to-end.

---

## Notes

- All tasks include exact file paths.
- `[P]` markers indicate parallelism within a phase; the 3 user stories themselves are inter-phase parallelisable.
- Constitution gates (principles I–V) are re-checked in T026; the plan-phase Gate result documented in [plan.md §Constitution Check](./plan.md#constitution-check) holds for the duration unless a task discovers a hidden violation.
- No new external Go dependencies. All work is stdlib (`math/rand/v2`, `context`, `net/http`) + existing internal packages (`internal/githubapi`, `internal/plugins/restutil` if T005 lands).
- PR scope is bounded to these 4 wirings — GraphQL (013) and chromedp (014) are explicitly out.
