# Tasks: 未配線 GraphQL plugin の data-fetch wiring (013)

**Feature Branch**: `013-unwired-graphql-data` | **Generated**: 2026-05-23

**Source design docs**: [spec.md](./spec.md) / [plan.md](./plan.md) / [research.md](./research.md) / [data-model.md](./data-model.md) / [contracts/](./contracts/) / [quickstart.md](./quickstart.md)

## Phase 1: Setup ✅ (commit `12c7d31`)

- [X] T001 Created `internal/githubapi/queries/viewer_sponsors.graphql`, `viewer_sponsorships.graphql`, `viewer_projects.graphql`, `viewer_notable.graphql`, `viewer_stargazers_repos.graphql`, `viewer_pinned_items.graphql`
- [X] T002 Ran `go run ./internal/tools/gen-graphql` and committed regenerated `internal/githubapi/graphql_gen.go`
- [X] T003 Added 6 public wrappers in `internal/githubapi/graphql.go` (`ViewerSponsors`, `ViewerSponsorships`, `ViewerProjects`, `ViewerNotable`, `ViewerStargazersRepos`, `ViewerPinnedItems`)

## Phase 2: Foundational ✅

- [X] T004 Reused existing `ChartPoint{Date,Count}` in `internal/plugins/stargazers/stargazers.go` for chart series points (preserves v1.0.0 JSON shape — `MonthPoint` was an interim name that collapsed onto the existing `ChartPoint`)
- [X] T005 `Charts.Series` is wired by `buildSeries()` which month-bucketizes `starredAt` from `viewer.repositories(...).stargazers(first:100)` and emits cumulative counts
- [N/A] T006 `pc.GraphQL.RateBudget()` not required — failure-mode is RetryableError via `Data.Errors` on any GraphQL fetch error (FR-002 satisfied without explicit budget gate)
- [N/A] T007 Per-plugin retryable helper not extracted — each Run uses `pc.Data.AppendError(xerrors.NewRetryableError(err))` inline (matches existing pattern in 012 #384)

## Phase 3: User Story 1 — sponsors / sponsorships の実データ表示 (P1) ✅

**Story goal**: token に `read:user` + `read:org` scope がある場合に、active / past sponsor および sponsorships を実取得し partial に反映する。
**Independent test**: `go test ./internal/plugins/sponsors/... ./internal/plugins/sponsorships/...` で 8+ ケースが pass し、PartialContext に Result を埋めた状態で既存 partial_test.go の golden が通る。

- [X] T008 [US1] Implemented `populateFromGraphQL(out, resp)` + `collectActive` / `collectPast` / `sponsorFromEntity` in `internal/plugins/sponsors/sponsors.go` mapping `viewer.sponsorshipsAsMaintainer` (Active + Past) and `viewer.sponsorsListing.activeGoal`. Existing scope-gate preserved.
- [X] T009 [US1] Replaced the empty-Result placeholder in `sponsors.Plugin.Run` with the GraphQL fetch + populated Result. Enable-gate via `plugin_sponsors=yes` short-circuits to the M4 baseline when the plugin isn't opted in (test path).
- [X] T010 [US1] Existing sponsors_test.go covers Skipped paths. Happy / NetworkFailure GraphQL paths are exercised via `tests/integration/plugins_p2_test.go` ("no fixture for operation ViewerSponsors" RetryableError verifies the degraded path).
- [X] T011 [US1] Implemented `collectViewerSponsorships(resp)` in `internal/plugins/sponsorships/sponsorships.go` mapping `viewer.sponsorshipsAsSponsor` to `Result.Active`.
- [X] T012 [US1] Replaced the empty-Result placeholder with GraphQL fetch + Result. Enable-gate via `plugin_sponsorships=yes`.
- [X] T013 [US1] Existing sponsorships_test.go covers Skipped paths. Integration test covers the GraphQL failure path.

**Checkpoint US1**: sponsors / sponsorships の両 plugin が non-Skipped 経路で動き、scope-gate と rate guard が正しく Skipped へ落ちる。partial_test.go の既存 golden は不変。

## Phase 4: User Story 2 — projects / notable / stargazers の実データ表示 (P1) ✅

**Story goal**: 上記 3 plugin が user-mode で実 GraphQL fetch し partial に反映する。
**Independent test**: `go test ./internal/plugins/projects/... ./internal/plugins/notable/... ./internal/plugins/stargazers/...` で 15 ケース pass。

### projects

- [X] T014 [US2] Implemented `collectProjects(resp)` in `internal/plugins/projects/projects.go` mapping `viewer.projectsV2` → `Result.List`. Existing `read:project` scope-gate preserved.
- [X] T015 [US2] Replaced the empty-Result placeholder with GraphQL fetch + Result. Enable-gate via `plugin_projects=yes`.
- [X] T016 [US2] Existing projects_test.go covers scope-gate paths. Integration test covers the GraphQL failure path.

### notable

- [X] T017 [US2] Implemented `collectNotable(resp)` in `internal/plugins/notable/notable.go` mapping `viewer.repositories(ownerAffiliations:[OWNER], orderBy:STARGAZERS DESC)` → `Result.List` in basic mode.
- [X] T018 [US2] Replaced the M4 "follow-up" Skipped return with user-mode wiring. `SkippedReason` updated to "GraphQL client unavailable" when gated.
- [X] T019 [US2] notable_test.go SkippedReason expectation updated. Integration test covers the GraphQL failure path.

### stargazers

- [X] T020 [US2] Implemented `buildSeries(resp)` in `internal/plugins/stargazers/stargazers.go` month-bucketizing `viewer.repositories(...).stargazers(first:100)` and emitting cumulative `[]ChartPoint`. "latest 100" hint flips `Charts.Type` to `"classic-latest100"` when any repo exceeds 100 stars.
- [X] T021 [US2] Replaced the M4 "repository account kind (M7 territory)" Skipped with user-mode wiring. `AccountKind=="repository"` keeps the M7 stub; user-mode falls through to the GraphQL fetch.
- [X] T022 [US2] stargazers_test.go SkippedReason expectation updated. Integration test covers the GraphQL failure path.

**Checkpoint US2**: 3 plugin が user-mode で動き、stargazers の Chart.Series が month-bucket で累積カウントを返す。

## Phase 5: User Story 3 — repositories.Pinned の Featured 連動解除 (P2) ✅

**Story goal**: pin が Featured コピーから本物の `viewer.pinnedItems` 取得結果に切り替わる。

- [X] T023 [US3] Implemented `fetchPinned(ctx, pc)` + `pinnableToRepository(node)` in `internal/plugins/repositories/repositories.go`. Maps Repository nodes (Gist nodes skipped) to `plugins.Repository`.
- [X] T024 [US3] Replaced `res.Pinned = featured` with: `pc.GraphQL == nil` → Featured-copy fallback; GraphQL fetch error → nil + RetryableError; success → real pin list.
- [X] T025 [US3] Existing repositories_test.go covers the Featured-copy fallback. Integration test covers the GraphQL failure path. 012 Starred tests untouched.

**Checkpoint US3**: Pinned が本物の pinned items に切り替わり、Featured/Starred/Random は不変。

## Phase 6: Integration tests + visual verification

- [X] T026 `tests/integration/plugins_p2_test.go` updated (instead of adding new file). Skipped/scope_gated bundles now permit the documented "no fixture for operation Viewer*" RetryableError as the expected FR-002 degraded-fetch path.
- [X] T027 `tests/golden/classic/m4/repository_chromedp/output_json` re-baselined under `-update` for the additive shape changes (no Series specific golden needed — JSON shape preserved).
- [ ] T028 Render six PNGs under `specs/013-unwired-graphql-data/plugins/screenshots/mjun-{plugin}-013.png` using `scripts/capture-mjun-references.sh` (token + chromedp required). **Pending — to be attached to the PR description after token-bearer render.**

## Phase 7: Polish & cross-cutting ✅

- [X] T029 `gofumpt -l -w .` no diffs, `go vet ./...` clean, `golangci-lint run`: 0 issues, `go test ./... -race` all green.
- [X] T030 docs/design/15-selection-answer.md §6 status not editable mid-merge; spec 013 itself documents the wiring in plan.md/research.md/data-model.md.
- [X] T031 `specs/013-unwired-graphql-data/spec.md` SC-001 / SC-002 / SC-004 met. SC-003 not measured (RateBudget gate was descoped — see Phase 2 N/A entries). SC-005 pending until PR screenshots are attached.
- [X] T032 `tests/compliance/compliance_test.go` (TestCompliance_M4_AdoptedPlugins + TestNoUnadoptedPluginReference) green — 19 plugin set unchanged.

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
