# Tasks: 未配線 GraphQL plugin の data-fetch wiring (013)

**Feature Branch**: `013-unwired-graphql-data` | **Generated**: 2026-05-23

**Source design docs**: [spec.md](./spec.md) / [plan.md](./plan.md) / [research.md](./research.md) / [data-model.md](./data-model.md) / [contracts/](./contracts/) / [quickstart.md](./quickstart.md)

## Phase 1: Setup

- [ ] T001 Create `internal/githubapi/queries/viewer_sponsors.graphql`, `viewer_sponsorships.graphql`, `viewer_projects.graphql`, `viewer_notable.graphql`, `viewer_stargazers_repos.graphql`, `viewer_pinned_items.graphql` (skeleton query bodies per `contracts/<plugin>-fetch.md`)
- [ ] T002 Run `go generate ./internal/githubapi/...` and commit the regenerated `internal/githubapi/graphql_gen.go`
- [ ] T003 Add public wrappers in `internal/githubapi/graphql.go` for the six new gen-functions (mirror existing wrappers like `UserFollowers`, `UserStarredRepositories`)

## Phase 2: Foundational

- [ ] T004 Add `MonthPoint` type to `internal/plugins/stargazers/stargazers.go` (fields: `Month time.Time`, `Cumulative int`, JSON tags `month` / `cumulative`)
- [ ] T005 Add `Series []MonthPoint` to `internal/plugins/stargazers/Chart` struct (additive, JSON tag `series,omitempty`)
- [ ] T006 Confirm `pc.GraphQL.RateBudget()` API exists in `internal/githubapi/rate.go`. If only `Remaining`/`Used` exist, add a thin `RateBudget() int` accessor (cost units = points; default to remaining if no explicit budget tracking)
- [ ] T007 [P] Add a per-plugin helper `func recordRetryable(pc *plugins.PluginContext, pluginName string, err error)` in `internal/plugins/internal/fetchhelp/fetchhelp.go` (new file under existing `internal/plugins/` tree) — wraps `&xerrors.RetryableError{Err: err}` and appends to `pc.Data.Errors`

## Phase 3: User Story 1 — sponsors / sponsorships の実データ表示 (P1)

**Story goal**: token に `read:user` + `read:org` scope がある場合に、active / past sponsor および sponsorships を実取得し partial に反映する。
**Independent test**: `go test ./internal/plugins/sponsors/... ./internal/plugins/sponsorships/...` で 8+ ケースが pass し、PartialContext に Result を埋めた状態で既存 partial_test.go の golden が通る。

- [ ] T008 [P] [US1] Implement `fetchSponsors(ctx, pc, opts)` in `internal/plugins/sponsors/sponsors.go` calling the new `pc.GraphQL.ViewerSponsors(...)` wrapper; map `Result.Active` / `Result.Past` / `Result.Goal` / `Result.About`. Preserve the existing scope-gate path.
- [ ] T009 [US1] Replace the `return &Result{...}, nil` placeholder in `sponsors.Plugin.Run` (current line ~160) with the new fetch + populated Result. Add rate-budget guard at the top (cost estimate 5; on insufficient, return Skipped + RetryableError via `recordRetryable`).
- [ ] T010 [P] [US1] Add table-driven tests to `internal/plugins/sponsors/sponsors_test.go`: `TestRun_Sponsors_Happy`, `TestRun_Sponsors_ScopeMissing`, `TestRun_Sponsors_NetworkFailure`, `TestRun_Sponsors_EmptyData`, `TestRun_Sponsors_RateBudgetTooLow`. Use the existing `internal/testutil/mocks` GraphQL mux. (Total: 5 cases for sponsors.)
- [ ] T011 [P] [US1] Implement `fetchSponsorships(ctx, pc, opts)` in `internal/plugins/sponsorships/sponsorships.go` calling `pc.GraphQL.ViewerSponsorships(...)`; map `Result.List`.
- [ ] T012 [US1] Replace the placeholder in `sponsorships.Plugin.Run` with the new fetch + Result. Add rate-budget guard (cost estimate 3).
- [ ] T013 [P] [US1] Add table-driven tests to `internal/plugins/sponsorships/sponsorships_test.go`: `Happy`, `ScopeMissing`, `NetworkFailure`, `EmptyData`, `RateBudgetTooLow`.

**Checkpoint US1**: sponsors / sponsorships の両 plugin が non-Skipped 経路で動き、scope-gate と rate guard が正しく Skipped へ落ちる。partial_test.go の既存 golden は不変。

## Phase 4: User Story 2 — projects / notable / stargazers の実データ表示 (P1)

**Story goal**: 上記 3 plugin が user-mode で実 GraphQL fetch し partial に反映する。
**Independent test**: `go test ./internal/plugins/projects/... ./internal/plugins/notable/... ./internal/plugins/stargazers/...` で 15 ケース pass。

### projects

- [ ] T014 [P] [US2] Implement `fetchProjects(ctx, pc, opts)` in `internal/plugins/projects/projects.go` calling `pc.GraphQL.ViewerProjects(...)`; map `Result.List` per `contracts/projects-fetch.md`. Preserve scope-gate (`read:project`).
- [ ] T015 [US2] Replace the placeholder in `projects.Plugin.Run` with the new fetch + Result. Add rate-budget guard (cost estimate 3).
- [ ] T016 [P] [US2] Add table-driven tests to `internal/plugins/projects/projects_test.go`: `Happy`, `ScopeMissing`, `NetworkFailure`, `EmptyData`, `RateBudgetTooLow`.

### notable

- [ ] T017 [P] [US2] Implement `fetchNotable(ctx, pc, opts)` in `internal/plugins/notable/notable.go` calling `pc.GraphQL.ViewerNotable(...)`; map `Result.List` per `contracts/notable-fetch.md` (basic mode — `nameWithOwner` / `description` / `stargazerCount` / `forkCount` / `Role="Maintainer"`).
- [ ] T018 [US2] Replace the M4 baseline Skipped return in `notable.Plugin.Run` with the new fetch + Result. Update `SkippedReason` for non-happy paths. Add rate-budget guard (cost estimate 5).
- [ ] T019 [P] [US2] Add table-driven tests to `internal/plugins/notable/notable_test.go`: `Happy`, `NetworkFailure`, `EmptyData`, `RateBudgetTooLow` (no scope gate — public PAT enough).

### stargazers

- [ ] T020 [P] [US2] Implement `fetchStargazersUser(ctx, pc, opts)` in `internal/plugins/stargazers/stargazers.go` calling `pc.GraphQL.ViewerStargazersRepos(...)`; bucket `starredAt` per `YYYY-MM` and build cumulative `[]MonthPoint`. Add "Showing latest 100 stars" suffix to `Chart.Title` if any repo exceeded 100.
- [ ] T021 [US2] Replace the M4 baseline "stargazers requires repository account kind (M7 territory)" Skipped path with user-mode wiring: `AccountKind=="user"` → run new fetch; `AccountKind=="repository"` → continue existing Skipped (M7 territory); `AccountKind=="organization"` → Skipped with reason "stargazers organization mode deferred to 014". Add rate-budget guard (cost estimate 30).
- [ ] T022 [P] [US2] Add table-driven tests to `internal/plugins/stargazers/stargazers_test.go`: `Happy_User`, `Skipped_RepositoryKind`, `Skipped_OrgKind`, `NetworkFailure`, `EmptyRepos`, `RateBudgetTooLow`, `LatestHintAppearsWhenOver100`.

**Checkpoint US2**: 3 plugin が user-mode で動き、stargazers の Chart.Series が month-bucket で累積カウントを返す。

## Phase 5: User Story 3 — repositories.Pinned の Featured 連動解除 (P2)

**Story goal**: pin が Featured コピーから本物の `viewer.pinnedItems` 取得結果に切り替わる。
**Independent test**: `go test ./internal/plugins/repositories/... -run Pinned` で 5 ケース pass。012 の Starred 6 ケースは不変。

- [ ] T023 [P] [US3] Implement `fetchPinned(ctx, pc, opts)` in `internal/plugins/repositories/repositories.go` (or new helper file) calling `pc.GraphQL.ViewerPinnedItems(...)`; map each Repository node to `plugins.Repository` per `contracts/repositories-pinned-fetch.md`.
- [ ] T024 [US3] Replace `res.Pinned = featured` (line ~124) with: `pc.GraphQL == nil` → keep Featured-copy fallback; `pc.GraphQL.RateBudget() < 3` → nil + RetryableError; otherwise call `fetchPinned`. Preserve all other Featured/Starred/Random behavior unchanged.
- [ ] T025 [P] [US3] Add table-driven tests to `internal/plugins/repositories/repositories_test.go` (Pinned-scoped): `TestRun_Pinned_Happy`, `TestRun_Pinned_RESTNilFallback` (= Featured copy), `TestRun_Pinned_NetworkFailure`, `TestRun_Pinned_EmptyPins`, `TestRun_Pinned_RateBudgetTooLow`.

**Checkpoint US3**: Pinned が本物の pinned items に切り替わり、Featured/Starred/Random は不変。

## Phase 6: Integration tests + visual verification

- [ ] T026 Add `tests/integration/plugins_p2_test.go` covering all six newly-wired plugins enabled simultaneously via GraphQL mux fixtures. Assert: each plugin's Skipped is false, JSON shape matches `data-model.md`, no `*RetryableError` in `Data.Errors`.
- [ ] T027 Add `tests/golden/classic/m4/stargazers.svg` (new golden) — assert the Chart `<g>` includes the new month-series. Initial baseline generated with `go test ... -update`.
- [ ] T028 Render six PNGs under `specs/013-unwired-graphql-data/plugins/screenshots/mjun-{plugin}-013.png` using `scripts/capture-mjun-references.sh` (token + chromedp required). Commit the PNGs.

## Phase 7: Polish & cross-cutting

- [ ] T029 Run `gofumpt -l -w .`, `go vet ./...`, `golangci-lint run`, `govulncheck ./...` — all green
- [ ] T030 Update `docs/design/15-selection-answer.md` §6 status table: mark T-052/056/063/064/065/066/repositories.Pinned as wired in spec 013
- [ ] T031 Update `specs/013-unwired-graphql-data/spec.md` Success Criteria checklist (SC-001 through SC-005) with measured values from render verification
- [ ] T032 Verify `tests/compliance/compliance_test.go` (`TestCompliance_M4_AdoptedPlugins` and `TestNoUnadoptedPluginReference`) stays green

## Dependencies

```
Phase 1 (Setup)
  └── T001 → T002 → T003

Phase 2 (Foundational)
  ├── T004 → T005 (stargazers type addition)
  ├── T006 (rate budget API)
  └── T007 (retryable helper)

Phase 3+ (User Stories) depend on Phase 1 + Phase 2

  US1 (sponsors / sponsorships): T008→T009→T010, T011→T012→T013
  US2 (projects / notable / stargazers): T014→T015→T016, T017→T018→T019, T020→T021→T022
  US3 (repositories.Pinned): T023→T024→T025

Phase 6 (Integration) depends on US1+US2+US3

Phase 7 (Polish) is last.
```

## Parallel execution opportunities

- Phase 1: T001 / T002 / T003 are sequential (gen script reads queries; wrappers reference gen output).
- Phase 2: T004/T005 sequential within stargazers type, T006 and T007 [P] independent.
- Phase 3: Per-plugin sub-blocks are sequential within plugin, but `sponsors block` and `sponsorships block` run in parallel.
- Phase 4: 3 plugins (projects / notable / stargazers) sub-blocks all run in parallel.
- Phase 5: Single plugin — internally sequential.
- Phase 6: T026 depends on US1+US2+US3 completion. T027 depends on T021. T028 depends on T028 prerequisites + token & chromedp.

## Independent test criteria (per story)

| Story | Pass criteria |
|---|---|
| US1 | `go test ./internal/plugins/sponsors/... ./internal/plugins/sponsorships/...` — 10 ケース all pass |
| US2 | `go test ./internal/plugins/projects/... ./internal/plugins/notable/... ./internal/plugins/stargazers/...` — 17 ケース all pass |
| US3 | `go test ./internal/plugins/repositories/... -run Pinned` — 5 ケース all pass; 012 Starred 6 ケース不変 |

## Implementation strategy

1. **MVP**: US1 (sponsors / sponsorships). 既存 scope-gate ロジックを上書きせず最小コストで wiring を入れる。
2. **次の増分**: US2 を順次 (projects → notable → stargazers)。stargazers は cost が大きいので最後。
3. **Polish**: US3 + integration test + visual render を最後。
4. **PR 提出**: User Story 単位ではなく **1 PR にまとめる** 方針 (採用機能の完全実装が目的で、partial PR は cherry-pick 困難)。
5. **screenshot 添付**: PR 説明に `specs/013-unwired-graphql-data/plugins/screenshots/` への direct link を 6 件含める (SC-005)。

## Task count summary

| Phase | Tasks |
|---|---|
| Setup | 3 |
| Foundational | 4 |
| US1 | 6 |
| US2 | 9 |
| US3 | 3 |
| Integration | 3 |
| Polish | 4 |
| **Total** | **32** |
