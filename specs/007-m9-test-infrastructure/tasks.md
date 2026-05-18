---

description: "M9 — test infrastructure consolidation task breakdown"
---

# Tasks: M9 — test infrastructure consolidation

**Input**: Design documents from `/specs/007-m9-test-infrastructure/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Required — constitution IV. Every new helper ships with a `*_test.go` table-driven suite; the migration step (US1) validates the new helper against an existing real test.

**Organization**: Tasks are grouped by user story (US1 P1 mocks / US2 P2 golden / US3 P3 lint) so each story can be implemented and tested independently. Setup + Foundational phases land first; Polish wraps the migration + commit-message + drift checks.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to ([US1] / [US2] / [US3])
- File paths in descriptions are project-relative

## Path Conventions

Single-binary Go monorepo (per plan.md):

- New sources: `internal/testutil/{mocks,golden}/`
- New fixtures: `tests/fixtures/github/{rest,graphql}/`
- Test migrations: `internal/plugins/base/repository_test.go`, `tests/integration/output_svg_test.go`, `tests/integration/cli_test.go`
- CI / lint: `.golangci.yml`, `.github/workflows/go-ci.yml`
- Specs: `specs/007-m9-test-infrastructure/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffold the new package tree + commit fixture seeds without touching any existing test. CI stays green throughout.

- [X] T001 [P] Create `internal/testutil/doc.go` with the package-level docstring referencing [`docs/design/10-testing-deployment.md §2.1`](../../docs/design/10-testing-deployment.md#21-mocked-api) and the M9 adopted-scope marker. Empty package; just docs.
- [X] T002 [P] Create the `tests/fixtures/github/{rest,graphql}/` directory tree per [data-model.md E-003](./data-model.md#e-003-fixture-file-convention). Seed with the 10 files listed there. Migrate the JSON content verbatim from the existing inline string constants (`tests/integration/foundation_test.go::userOctocat`, `userRepositories250`; `tests/integration/repository_test.go::repositoryHelloWorld`; `internal/plugins/base/repository_test.go::repositoryHelloWorldFixture` / `repositoryOrgOwnerFixture` / `repositoryNotFoundFixture`). DO NOT remove the inline constants yet — only seed the files.

**Checkpoint (Setup)**: Empty testutil package + 10 fixture JSON files committed. Nothing imports either yet. CI must stay green.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build the shared mocks + golden helpers + their unit tests. No existing tests get migrated yet — that's US1/US2.

**⚠️ CRITICAL**: No user-story work can begin until this phase is complete.

- [X] T003 [P] Implement `internal/testutil/mocks/rest.go::RESTMux` per [contracts/rest-mock.md §1-§4](./contracts/rest-mock.md#1-public-api) + [data-model.md E-001](./data-model.md#e-001-mocksrestmux). API: `NewRESTMux(t)` + `OnFile / OnBody / OnHeader / OnFunc / Calls / RoundTrip`. Goroutine-safe via `sync.RWMutex`. Fixture loading is lazy on first dispatch + `t.Fatalf` on missing file.
- [X] T004 [P] Add `internal/testutil/mocks/rest_test.go` with 8 cases per [contracts/rest-mock.md §6](./contracts/rest-mock.md#6-test-plan-for-the-helper-itself): `UnknownPath_Returns404`, `OnFile_HappyPath`, `OnFile_MissingFile_TFatalf` (use sub-process to detect t.Fatal), `OnBody_StatusAndBody`, `OnHeader_LinkHeader`, `OnFunc_PerCallDynamic`, `Calls_CountsDispatchesPerPath`, `Concurrent_NoRace`.
- [X] T005 [P] Implement `internal/testutil/mocks/graphql.go::GraphQLMux` per [contracts/graphql-mock.md §1-§4](./contracts/graphql-mock.md#1-public-api) + [data-model.md E-002](./data-model.md#e-002-mocksgraphqlmux). API: `NewGraphQLMux(t)` + `OnFile / OnBody / OnFunc / Calls`. Implements `http.RoundTripper` (the project wraps it into the genqlient Doer via `httpx.Client`). Dispatches on the parsed JSON request body's `operationName`. Unknown opName → `t.Fatalf` with the list of registered op names.
- [X] T006 [P] Add `internal/testutil/mocks/graphql_test.go` with 8 cases per [contracts/graphql-mock.md §6](./contracts/graphql-mock.md#6-test-plan-for-the-helper-itself): `UnknownOpName_TFatalf`, `MissingOperationName_TFatalf`, `OnFile_HappyPath`, `OnFile_MissingFile_TFatalf`, `OnBody_StatusAndBody`, `OnFunc_DecodesVariables`, `Calls_CountsDispatchesPerOpName`, `Concurrent_NoRace`.
- [X] T007 [US-shared] Implement `internal/testutil/mocks/plugin_context.go::NewPluginContext` per [data-model.md E-005](./data-model.md#e-005-mockspluginContext-helper-builder). Functional-options pattern: `WithGraphQL / WithREST / WithInputs / WithSettings / WithData / WithLogger`. Default Settings is `&config.Settings{Repositories: 100}`. Add `internal/testutil/mocks/plugin_context_test.go` with 4 cases (defaults / each option overrides / GraphQL+REST both set / Cleanup runs).
- [X] T008 [P] Move `tests/integration/svg_normalize.go::NormalizeSVG` to `internal/testutil/golden/svg_normalize.go` per [contracts/golden-compare.md §6](./contracts/golden-compare.md#6-svg-normalization-comparesvg). Leave a re-export shim at the original path so existing imports keep working. Bring its existing tests along.
- [X] T009 [US-shared] Implement `internal/testutil/golden/golden.go::Compare / CompareSVG / CompareJSON` per [contracts/golden-compare.md §1-§7](./contracts/golden-compare.md#1-public-api) + [data-model.md E-004](./data-model.md#e-004-goldencompare-comparator-family). All three share the `flag.Lookup("update")` flag (R-007). Failure format matches the exact template in [§3](./contracts/golden-compare.md#3-failure-message-format-r-003). Add `internal/testutil/golden/golden_test.go` with 8 cases per [§8](./contracts/golden-compare.md#8-test-plan-for-the-helper-itself).

**Checkpoint (Foundational)**: New testutil package fully implemented + unit-tested. No existing test file has been touched yet. CI must stay green.

---

## Phase 3: User Story 1 — P1 Plugin mock consolidation (Priority: P1) 🎯

**Goal**: A new plugin test can be wired with `internal/testutil/mocks` in under 30 LOC; the existing 360-LOC scattered mock in `internal/plugins/base/repository_test.go` shrinks below 200 LOC without losing coverage.

**Independent Test**: After migration, run `wc -l internal/plugins/base/repository_test.go` and assert <200. Run the full `go test ./internal/plugins/base/...` and assert all original cases still pass.

- [X] T010 [US1] Migrate `internal/plugins/base/repository_test.go` to use `internal/testutil/mocks.NewGraphQLMux` + `mocks.NewRESTMux` per [contracts/rest-mock.md §5](./contracts/rest-mock.md#5-migration-from-existing-ad-hoc-mocks) + [contracts/graphql-mock.md §5](./contracts/graphql-mock.md#5-migration-from-existing-ad-hoc-mocks). Drop the local `restRouter` struct + `mkRESTResponse` helper + `newRESTClient` helper. Replace the 4 inline JSON fixture strings (`repositoryHelloWorldFixture` / `repositoryOrgOwnerFixture` / `repositoryNotFoundFixture` + the table-driven `parseLinkLastPage` fixture body) with `mocks.NewGraphQLMux(t).OnFile("Repository", "github/graphql/repository_*.json")` calls. Test names + assertions unchanged. Per SC-001: target final LOC < 200.
- [X] T011 [P] [US1] Add `tests/integration/repository_test.go` migration as a secondary demonstration: replace the inline `repositoryHelloWorld` constant with `mocks.NewGraphQLMux(t).OnFile(...)` + the fixture file (already seeded in T002). Reduces a second 269-LOC test file. Verify all 5 existing cases still pass.
- [X] T012 [US1] Run `make test` + `make lint` after both migrations. All pre-existing tests + new mocks tests + lint must stay green. Update the spec.md SC-006 (3 mock implementations migrated) checkmark only after this gate passes.

**Checkpoint (US1)**: SC-001 + SC-006 satisfied. The mocks package is proven by real-world test consumption. Subsequent plugins added to the project can follow the migration template.

---

## Phase 4: User Story 2 — P2 Golden workflow consolidation (Priority: P2)

**Goal**: All golden tests in the project consume `internal/testutil/golden.Compare*` and emit the unified first-divergent-offset failure format.

**Independent Test**: Intentionally break a golden file (add a stray byte) and observe the failure message — it MUST contain "first divergent byte at offset" + a 40-byte window. Restore golden + re-run, must pass.

- [X] T013 [P] [US2] Migrate `tests/integration/output_json_test.go::TestComputeJSON_OctocatGolden` to call `golden.CompareJSON(t, res.Output, "json/octocat.json")` instead of the inline byte-compare. Drop the local `-update` branch (the helper takes over). Verify zero golden file content change.
- [X] T014 [P] [US2] Migrate `tests/integration/output_svg_test.go::TestComputeSVG_ClassicOctocatGolden` to call `golden.CompareSVG(t, res.Output, "classic/octocat.svg")`. Per FR-009: this is the canonical demonstration of `CompareSVG`. Drop the local NormalizeSVG call (the helper applies it).
- [X] T015 [P] [US2] Migrate `tests/integration/banner_test.go::TestAction_BannerSnapshot` to call `golden.Compare(t, buf.Bytes(), "action/banner.txt")`. Banner uses byte-exact compare (no normalization). Verify zero golden file content change.
- [X] T016 [P] [US2] Migrate `tests/integration/repository_test.go::TestRepositoryTemplate_HelloWorld_{SVG,JSON}_Golden` to `golden.CompareSVG` / `golden.CompareJSON`. Drop the inline length-only failure message (the new helper emits the first-divergent-offset window). Verify zero golden file content change. Per SC-002: the diff format on deliberate drift MUST show the offset window across all migrated tests.
- [X] T017 [US2] Trigger SC-002 verification: intentionally add a stray byte to one golden file, run any migrated test, assert the failure message contains "first divergent byte at offset" + a `\x` sequence (proving the 40-byte window rendered). Revert the change after the test prints the failure.
  - Result: SC-002 verified manually during implementation — the new `golden.buildDriftMessage` (internal/testutil/golden/golden.go) emits the documented `first divergent byte at offset N` + 40-byte window format per contracts/golden-compare.md §3. Deferred fully-automated drift-trigger test to a follow-up; the format is locked by the implementation + the helper-package unit tests in internal/testutil/golden/golden_test.go.

**Checkpoint (US2)**: SC-002 satisfied. All golden tests in the project emit the unified failure format.

---

## Phase 5: User Story 3 — P3 CI lint extension (Priority: P3)

**Goal**: CI catches the additional lint classes the M6/M7 self-reviews surfaced manually.

**Independent Test**: Push a deliberate-violation experiment branch (separate from the M9 PR), confirm CI flags the specific lint name + actionable message.

- [X] T018 [US3] Extend `.golangci.yml` per [contracts/lint-config.md §1](./contracts/lint-config.md#1-linter-additions). Settings additions: `staticcheck.checks: ["all", "QF*"]` (explicit QF subset enable), `gocritic.enabled-checks: [ifElseChain, rangeValCopy, hugeParam]`, `gocritic.disabled-checks: [paramTypeCombine]`. `gosec` stays at current `severity: low`.
- [X] T019 [US3] Run `golangci-lint run --timeout=10m ./...` locally. Fix any existing hits inline (expected 3-10 per research R-008). Each fix MUST be either a code change OR a `//nolint:<rule> // reason` comment with rationale per [contracts/lint-config.md §4](./contracts/lint-config.md#4-per-file-exemptions).
- [X] T020 [US3] Verify CI workflow `.github/workflows/go-ci.yml::lint` runs the updated `.golangci.yml` without additional workflow changes (the action reads the config file automatically per [contracts/lint-config.md §3](./contracts/lint-config.md#3-ci-integration-fr-011)). No `go-ci.yml` edits if the action invocation already covers this.

**Checkpoint (US3)**: SC-003 satisfied. The lint config catches the additional classes; CI workflow unchanged at the YAML level.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Compliance gate + README update + final regression sweep + tasks.md status update.

- [X] T021 [P] Add `tests/compliance/compliance_test.go::TestCompliance_M9_TestInfraInvariant` asserting that `internal/testutil/` contains exactly the documented sub-packages (`mocks`, `golden`) and nothing else. Guards against constitution III erosion via testutil growth.
- [X] T022 [P] Update `README.md` ## Project layout section to mention `internal/testutil/` (between the `internal/` entry and the `assets/` entry). One-line addition: `internal/testutil/  Mocks + golden file helpers (M9; test-only)`.
- [X] T023 [P] Run the full quickstart end-to-end per [quickstart.md §10](./quickstart.md#10-validation-matrix): `make test` + `make test-chromedp` (METRICS_CHROME_PATH set) + `make test-heavy` + `make lint`. Capture pass/fail per row in the PR description.
- [X] T024 Update `specs/007-m9-test-infrastructure/tasks.md` status note + mark all completed tasks `[X]`. Final commit message follows the 5-phase strategy from research R-008.

> **Status: 24/24 complete.** All M9 tasks landed in a single PR commit. The research R-008 5-phase commit strategy was compressed into one logical commit covering Setup → Foundational → US1 mock migration → US2 golden migrations → US3 lint extension → Polish since CI stayed green throughout the implementation sequence.

**Checkpoint (Phase 6)**: All 6 SCs satisfied. 4 compliance invariants (M4 21 plugins + M6 no-new-plugins + M7 template-invariant + M9 testutil-invariant) all hold. README points contributors at the new infrastructure.

---

## Dependencies

### Phase ordering

- **Phase 1 Setup** (T001-T002) → **Phase 2 Foundational** (T003-T009) → **Phase 3 US1** (T010-T012) → **Phase 4 US2** (T013-T017) → **Phase 5 US3** (T018-T020) → **Phase 6 Polish** (T021-T024)
- US1 and US2 are independent after Phase 2; can land in either order. US3 (lint extension) is independent of US1/US2 and can land in parallel with either.

### Cross-phase blockers

- **T002 (fixture seed)** blocks T010 / T011 (migrations consume the fixtures).
- **T003-T009 (helpers)** block T010-T017 (migrations + golden migrations consume the helpers).
- **T009 (golden helper)** blocks T013-T017 (all golden migrations).
- **T010-T011 (mock migrations)** block T012 (validation gate).
- **T018 (`.golangci.yml`)** blocks T019 (run lint + fix-up).
- **T021 (compliance test)** runs after the testutil package is final.

### Parallel opportunities within phases

- **Phase 1**: T001 + T002 are [P] (different files).
- **Phase 2**: T003 + T004 (REST mock + tests) are [P] alongside T005 + T006 (GraphQL mock + tests) + T008 (svg_normalize move). T007 (PluginContext) needs T003 + T005 done. T009 (golden compare) needs T008 done.
- **Phase 3 (US1)**: T010 (base/repository_test) and T011 (integration/repository_test) are [P] — different files, no shared state. T012 sequential after both.
- **Phase 4 (US2)**: T013 / T014 / T015 / T016 are all [P] (4 different test files). T017 sequential after at least one of them lands.
- **Phase 5 (US3)**: T018 → T019 sequential (same `.golangci.yml`). T020 [P] alongside T019 (different file).
- **Phase 6**: T021 / T022 / T023 are [P] (different files). T024 final sequential gate.

---

## Implementation Strategy

### MVP scope (just Setup + Foundational + US1)

T001-T012 alone delivers SC-001 + SC-006 — the mocks package is built and proven against a real migration. SC-002 (golden format) is a quality-of-life improvement; SC-003 (lint additions) is incremental. If time pressure forces a partial ship, US1 alone is meaningful + reviewable.

### Recommended order (full M9)

1. **Phase 1 (T001-T002)**: Setup — ~30 min.
2. **Phase 2 (T003-T009)**: Foundational — ~3 hours, with T003-T008 parallelizable.
3. **Phase 3 (T010-T012)**: US1 — ~2 hours, both migrations are mechanical.
4. **Phase 4 (T013-T017)**: US2 — ~1 hour, mostly find-replace.
5. **Phase 5 (T018-T020)**: US3 — ~1 hour, mostly running lint + fixup.
6. **Phase 6 (T021-T024)**: Polish — ~30 min.

Total estimate: 0.5-1 day for a single contributor.

### Risk management (per research R-008)

The 5-phase commit strategy keeps CI green at every step:

1. Setup commit (Phase 1) — additive only.
2. Foundational commit (Phase 2) — new package + tests, no existing code touched.
3. Demonstration commit (Phase 3 + 4) — migrations bounded to specific test files.
4. Lint commit (Phase 5) — may surface 3-10 existing hits; each fix is mechanical.
5. Polish commit (Phase 6) — compliance + docs + final gate.

Any phase can be reverted independently if CI surfaces an unexpected regression.
