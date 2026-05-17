---

description: "M7 — repository template task breakdown"
---

# Tasks: M7 — repository template

**Input**: Design documents from `/specs/006-m7-repository-template/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Required — constitution IV mandates table + golden file tests for every rendering / formatting change. Test tasks are integrated into each user story phase.

**Organization**: Tasks are grouped by user story (US1 P1 MVP / US2 P2 multi-format / US3 P3 validation) so each story can be implemented and tested independently. Setup + Foundational phases land first; Polish wraps cross-cutting concerns.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to ([US1] / [US2] / [US3])
- File paths in descriptions are absolute project-relative

## Path Conventions

Single-binary Go monorepo (per plan.md):

- Sources: `internal/`, `cmd/metrics-action/`
- Tests: `internal/<pkg>/*_test.go` (unit), `tests/integration/`, `tests/compliance/`, `tests/golden/`
- Assets: `assets/templates/repository/` (already present, untouched)
- Specs: `specs/006-m7-repository-template/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Wire the new GraphQL query, template registration, and input flow before US1 work begins.

- [X] T001 [P] Add `repo` field to `cmd/metrics-action/main.go` env passthrough (no logic change yet — confirm `INPUT_REPO` lands in `inv.Inputs["repo"]` after `ParseInputs`). Add `cmd/metrics-action/main_test.go::TestEnvPassthrough_Repo`.
- [X] T002 [P] Create the new GraphQL query file `internal/githubapi/queries/repository.graphql` per [contracts/base-repository-query.md §1](./contracts/base-repository-query.md#1-operation). Schema: `query Repository($login: String!, $repo: String!) { repository(owner: $login, name: $repo) { ... } }` covering databaseId / name / nameWithOwner / description / stargazerCount / forkCount / isArchived / primaryLanguage / licenseInfo / defaultBranchRef / owner / issues(states:OPEN) / pullRequests(states:OPEN).
- [X] T003 Regenerate genqlient client by running `make gen-graphql` (or equivalent `go generate ./...`). Verify the new file `internal/githubapi/graphql_repository.go` is created with `Repository(ctx, client, login, repo) (*RepositoryResponse, error)` and that the `lefthook.yml` `gen-graphql-drift` hook (or `go-mod-tidy` if no dedicated hook) stays green. Commit both files together.
- [X] T004 [P] Extend `internal/tools/gen-action-yml/main.go` to emit the new top-level `repo` input per [contracts/repo-input.md §1](./contracts/repo-input.md#1-actionyml-entry). The entry MUST land alphabetically alongside `user` / `template`. Run `make gen-action-yml` and commit the regenerated `action.yml`. Update `internal/tools/gen-action-yml/main_test.go::TestGenerate_HasRequiredSections` to assert `\n  repo:\n` is present.

**Checkpoint**: GraphQL query + action.yml input + env passthrough ready. The shape exists but is not yet consumed by any production code path.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the data model + base plugin extension that every user story depends on.

**⚠️ CRITICAL**: No user-story work can begin until this phase is complete.

- [X] T005 [P] Create `internal/plugins/repo.go` defining `type Repo struct` and `type RepoActivity struct` per [data-model.md E-003](./data-model.md#e-003-pluginsrepo-new-entity). Include `Owner / OwnerAvatar / Name / Description / Stargazers / Forks / Contributors / PrimaryLanguage / LicenseName / DefaultBranch / Activity / SponsorshipsAsMaintainer`. Add Goose docstring. Unit test: `internal/plugins/repo_test.go::TestRepo_ZeroValueIsNilSafe`.
- [X] T006 [P] Extend `internal/plugins/data.go` per [data-model.md E-004](./data-model.md#e-004-pluginsdata-field-extension): add `Repo *Repo` field on `Data` + helper `func (d *Data) RepoRef() *Repo` (RLock for read). Update `NewData()` doc to note the field stays nil for non-repository runs. Add `internal/plugins/data_repo_test.go::TestData_RepoRef_NilByDefault` + `TestData_RepoRef_Concurrent`.
- [X] T007 [US-shared] Add `Repo string` field to `engine.Request` in `internal/engine/engine.go` per [data-model.md E-002](./data-model.md#e-002-enginerequest-field-extension). Document that it MUST be empty when `Account != AccountRepository`; populated from `inv.Inputs["repo"]` by `internal/action::newInvocation`. Update `internal/engine/engine_test.go::TestRequest_RepoFieldShape`.
- [X] T008 [US-shared] Extend `internal/action/action.go::newInvocation` to populate `engine.Request.Repo` from `stringInput(inputs, "repo", "")`, and set `Account = AccountRepository` when `Template == "repository"`. Existing flows (`Account = AccountUser` default) untouched for other templates. Add `internal/action/action_test.go::TestNewInvocation_RepoTemplate_SetsAccount` + `TestNewInvocation_ClassicTemplate_LeavesRepoEmpty`.
- [X] T009 [US-shared] Implement `internal/plugins/base/repository.go::FetchRepo(ctx, login, repo, rest, graphql) (*plugins.Repo, error)` per [contracts/base-repository-query.md §3](./contracts/base-repository-query.md#3-invocation-site). Sequence: GraphQL `Repository` query → REST `listContributors` (count) → REST `listCommits?per_page=100&since=<30 days ago>` (count). Use existing `internal/githubapi.REST.Get` / GraphQL client; do NOT add new HTTP code. Wrap 5xx as `*xerrors.RetryableError`; 404 as `*xerrors.InputError` on `repo`. Add `internal/plugins/base/repository_test.go` with 4 cases (happy path / 404 / 5xx retryable / REST contributors 5xx → best-effort).
- [X] T010 [US-shared] Wire `FetchRepo` into `internal/plugins/base/base.go::Compute`: when `data.Account == AccountRepository` AND `inv.Inputs["repo"] != ""` AND `data.Repo == nil` → call `FetchRepo`, populate `data.Repo`. The existing user-fetch path runs first (so `Data.User` is set), then the repo-fetch path runs. Add `internal/plugins/base/base_test.go::TestCompute_RepoMode_PopulatesDataRepo` + `TestCompute_UserMode_LeavesDataRepoNil`.

**Checkpoint**: Foundation ready. `data.Repo` is populated for any run that selects `template == "repository"` + has a non-empty `repo` input. Plugins can now switch on `data.Repo != nil` (US1).

---

> **Status (initial chained run, commit `<HEAD>`)**: T001-T015 + T035 landed in
> this first M7 PR. Remaining T016-T034 + T036-T040 are intentionally
> deferred to focused follow-up PRs:
>
> - **T016-T017** repository plugin partial re-exports + per-partial table tests (low risk, single-package change)
> - **T018-T024** 7-plugin repo-mode refactors + paired tests (single biggest scope item; each plugin is independent)
> - **T025-T026** template integration tests (depend on T018-T024 to assert non-empty plugin sections)
> - **T027-T031** US2 multi-format output + golden files
> - **T032-T034** US3 validation-guard regression tests
> - **T036-T040** Polish (compliance extensions for plugins, README update, quickstart sweep, action.yml drift gate)

## Phase 3: User Story 1 — P1 MVP repository template SVG (Priority: P1) 🎯

**Goal**: A repository maintainer can render their repo's metrics SVG via `--template repository --user owner --repo name --output svg`. The output contains the repo identity, community health, and recent activity — populated from `data.Repo`.

**Independent Test**: Run `bin/metrics-action --user octocat --repo hello-world --template repository --token-env GITHUB_TOKEN --output svg --dryrun --filename -` against mocked deps → assert `<svg ... </svg>` on stdout containing the repo name + at least one of (contributors / stargazers / activity) panes. Under 30s (SC-001).

### Validation guards

- [X] T011 [P] [US1] Add the repo-input fail-fast check to `internal/action/action.go::runWith` AND `runCLIWith` per [contracts/repo-input.md §6](./contracts/repo-input.md#6-validation-timing): after `newInvocation`, before `defaultBuildDeps`, when `inv.Template == "repository"` && `stringInput(inv.Inputs, "repo", "") == ""` → return a typed `*xerrors.InputError("repo", ...)`. Add `internal/action/action_test.go::TestRun_RepoTemplate_MissingRepo_FailFast` asserting exit before any HTTP call.
- [X] T012 [P] [US1] Add `--repo <name>` flag handling per [contracts/repo-input.md §2](./contracts/repo-input.md#2-cli-flag) to `internal/action/cli.go`: extend `CLIFlags` with `Repo string`, register the flag in `ParseFlags`, wire into `ToInvocation` (`inputs["repo"] = cf.Repo` when non-empty). Handle the `owner/name` form (warn + take suffix). Add `internal/action/cli_test.go::TestParseFlags_Repo` + `TestParseFlags_RepoWithSlash_Warns` + `TestToInvocation_RepoFlagBeatsConfig`.

### Template registration

- [X] T013 [US1] Create `internal/templates/repository/repository.go` per [data-model.md E-001](./data-model.md#e-001-templatestemplate-instance-repository) + [contracts/repository-template.md §1-§2](./contracts/repository-template.md#1-partial-dispatch-order-svg). Implement `templates.Template` interface (`Name() / Check() / Run() / Metadata() / SupportedFormats()`). `Check()` rejects `Account != AccountRepository` and `inputs["repo"] == ""` with `*xerrors.InputError`. `Run()` loads partial ordering from `assets/templates/repository/partials/_.json` via the M2 `templates.LoadPartialOrder` helper (extract one if missing). Register via `init() { templates.Register(&t) }`.
- [X] T014 [US1] Add `cmd/metrics-action/plugins.go` side-effect import: `_ "github.com/mjun0812/github-metrics/internal/templates/repository"`. Rebuild and verify `bin/metrics-action --template repository ...` resolves the template (no "template not found" error).

### Per-template partials (4 new partials owned by repository template)

- [X] T015 [P] [US1] Create `internal/templates/repository/partials/partials.go` with the 4 repository-specific partials: `BaseHeader` (owner avatar + repo nameWithOwner + description), `Introduction` (description prose / about), `BaseCommunity` (contributors count + stargazers + forks + license), `BaseActivity` (recent commits / open issues / open PRs). All MUST be nil-safe (empty when `pc.Data.Repo == nil`). Mirror the `internal/templates/classic/partials/partials.go` layout (PartialFunc signature, `Register` calls in `init`).
- [ ] T016 [P] [US1] Create `internal/templates/repository/partials/plugins.go` mirroring `internal/templates/classic/partials/plugins.go`: re-exports the M4 plugin partials into the repository template's partial registry by name (`languages`, `projects`, `stargazers`, `people`, `activity`, `contributors`, `sponsors`). Each registered entry calls the plugin's existing partial function — the plugin Result struct already carries the data; only the registry routing changes.
- [ ] T017 [US1] Add per-partial table tests in `internal/templates/repository/partials/partials_test.go`: `TestBaseHeader_RepoNameWithOwner` / `TestBaseHeader_NilRepoSafe` / `TestIntroduction_DescriptionRendered` / `TestIntroduction_EmptyDescriptionSafe` / `TestBaseCommunity_AllFieldsPopulated` / `TestBaseCommunity_ZeroStargazersHidden` / `TestBaseActivity_RecentCommits` / `TestBaseActivity_ZeroActivityHidden`. 8 cases, 1 file.

### Repo-mode for the 7 reused plugins

(Per [contracts/repo-mode-plugin.md §3](./contracts/repo-mode-plugin.md#3-per-plugin-internal-refactor). Each task adds a `repo_mode.go` to the plugin package with `computeRepoMode` + refactors the existing `Compute` body into `computeUserMode`. Result struct gains a `Mode string` field. Tests pair each existing user-mode case with a new repo-mode case.)

- [ ] T018 [P] [US1] `internal/plugins/contributors/`: split `Compute` → `computeUserMode` / `computeRepoMode`. Repo-mode reads `pc.Data.Repo` (contributors count + REST list). Add `repo_mode.go` + `repo_mode_test.go::TestComputeRepoMode_HappyPath` + `TestComputeRepoMode_NilRepoFallsBackToUserMode`. Update existing user-mode tests to assert `result.Mode == "user"`.
- [ ] T019 [P] [US1] `internal/plugins/languages/`: same split. Repo-mode reads `pc.Data.Repo.PrimaryLanguage`. Add `repo_mode.go` + 2 paired tests.
- [ ] T020 [P] [US1] `internal/plugins/activity/`: same split. Repo-mode reads `pc.Data.Repo.Activity` (RecentCommits, OpenIssues, OpenPullRequests). Add `repo_mode.go` + 2 paired tests.
- [ ] T021 [P] [US1] `internal/plugins/stargazers/`: same split. Repo-mode reads `pc.Data.Repo.Stargazers` + REST stargazers time-series. Add `repo_mode.go` + 2 paired tests.
- [ ] T022 [P] [US1] `internal/plugins/people/`: same split. Repo-mode reads `pc.Data.Repo` collaborators (REST). Add `repo_mode.go` + 2 paired tests.
- [ ] T023 [P] [US1] `internal/plugins/projects/`: same split. Repo-mode reads `pc.Data.Repo` pinned items (REST). Add `repo_mode.go` + 2 paired tests.
- [ ] T024 [P] [US1] `internal/plugins/sponsors/`: same split. Repo-mode reads `pc.Data.Repo.SponsorshipsAsMaintainer`. Add `repo_mode.go` + 2 paired tests.

### Template integration test

- [ ] T025 [US1] Add `internal/templates/repository/repository_test.go::TestRun_PartialOrder_MatchesUnderscoreJsonIntersection` per [contracts/repository-template.md §6](./contracts/repository-template.md#6-test-plan). Mock `data.Repo` populated + plugin Result entries set; assert the rendered SVG contains DOM landmarks in the order declared by `_.json` ∩ registered partials. Plus `TestRun_Check_RejectsNonRepositoryAccount` + `TestRun_Check_RejectsEmptyRepoInput`.
- [ ] T026 [US1] Add `tests/integration/output_test.go::TestRepositoryTemplate_OctocatHelloWorld_SVG` per spec SC-001. Use the M4 `buildTestDeps` pattern with httptest-backed GraphQL mock that responds to `operationName: "Repository"` with the canned fixture. Assert valid `<svg ... </svg>`, `<svg ... aria-label="GitHub metrics for octocat/hello-world"`, contains "hello-world" string. Time budget < 30s.

**Checkpoint (US1)**: Spec SC-001 + SC-003 + SC-005 satisfied. Repository template renders valid SVG via Action + CLI mode with mocked deps.

---

## Phase 4: User Story 2 — P2 multi-format support (Priority: P2)

**Goal**: PNG / JPEG / JSON output formats work for the repository template the same way they work for classic.

**Independent Test**: 4 CLI invocations (one per format) all produce non-empty, format-valid output within 30s each. PNG/JPEG header bytes match magic numbers.

- [ ] T027 [P] [US2] Add `engine.MarshalJSON` extension in `internal/engine/marshal.go` (or wherever the M2 JSON marshaller lives) to include the new `data.repo` field per [contracts/repository-template.md §3](./contracts/repository-template.md#3-json-output-shape). The field is only emitted when `data.Repo != nil`. Snake_case keys per upstream convention. Add `internal/engine/marshal_test.go::TestMarshal_DataRepo_Emitted` + `TestMarshal_NoRepo_FieldOmitted`.
- [ ] T028 [P] [US2] Add `tests/integration/output_test.go::TestRepositoryTemplate_OctocatHelloWorld_JSON_Golden` per [contracts/repository-template.md §6](./contracts/repository-template.md#6-test-plan). Diff against `tests/golden/repository/octocat_hello-world.json` (seed via `-update`). Use the M2 byte-compare pattern.
- [ ] T029 [P] [US2] Add `tests/integration/output_test.go::TestRepositoryTemplate_OctocatHelloWorld_SVG_Golden`. Diff against `tests/golden/repository/octocat_hello-world.svg` using the M2 `NormalizeSVG` helper. Seed via `-update`.
- [ ] T030 [US2] Add `tests/integration/output_test.go::TestRepositoryTemplate_PNG_MagicNumber` + `TestRepositoryTemplate_JPEG_MagicNumber`. Both use the M3 chromedp render path. Assert first 8 bytes (PNG) / first 3 bytes (JPEG SOI+APP0 / SOI+APP1) match the expected magic numbers per SC-002. Gate under the existing `chromedp` build tag (file naming: `*_chromedp_test.go`).
- [ ] T031 [US2] Extend `tests/integration/cli_test.go::TestCLI_ConfigYAML_Equivalence` with a `--repo hello-world` ↔ `repo: hello-world` paired case. Mocked deps; assert byte equivalence between the CLI-flags and YAML-config invocations.

**Checkpoint (US2)**: Spec SC-002 satisfied. All 4 formats validated against golden files.

---

## Phase 5: User Story 3 — P3 validation guard rails (Priority: P3)

**Goal**: Mismatched template / input combinations fail fast with clear errors.

**Independent Test**: `--template repository --user octocat` (no `--repo`) → exit 1 + stderr contains "repository template requires" + no API call. Under 5s (SC-003).

- [ ] T032 [P] [US3] Add `internal/action/action_test.go::TestRun_RepoTemplate_MissingRepo_ExitOneNoAPI` (extends T011's coverage). Set up an httptest server with a counter; assert the counter stays at 0 after the failed run.
- [ ] T033 [P] [US3] Add `internal/action/action_test.go::TestRun_ClassicTemplate_WithRepoInput_IgnoredAndLogged` per FR-007. Inject `--template classic --repo hello-world`; assert classic-shape output is produced + `slog.Debug` log mentions ignored repo input.
- [ ] T034 [US3] Add `tests/integration/cli_test.go::TestCLI_RepoTemplate_MissingRepo_FailFast` exercising the binary end-to-end. Assert exit code 1 + stderr text + completion under 5s. Strip `GITHUB_ACTIONS` env (M6 pattern).

**Checkpoint (US3)**: Spec SC-003 + FR-007 satisfied. M2 compat preserved.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Compliance gate + README update + final regression sweep.

- [X] T035 [P] Add `tests/compliance/compliance_test.go::TestCompliance_M7_TemplateInvariant` per [research.md R-008](./research.md#r-008-compliance-test-extension). Enumerate `internal/templates/` subdirectories (exclude `partials/`) and assert the set equals `{classic, repository}`. Pair with `TestCompliance_M4_AdoptedPlugins` for the dual-invariant defense.
- [ ] T036 [P] Add `tests/compliance/compliance_test.go::TestCompliance_M7_NoNewPluginSlugs` (extends `TestCompliance_M6_NoNewPlugins`) — confirm `internal/plugins/` still contains exactly the 21 adopted slugs + `base/` + `core/`. Constitution III invariant.
- [ ] T037 Add `tests/compliance/compliance_test.go::TestCompliance_M7_NonAffectedPluginsAreRepoMode_Inert` per [contracts/repo-mode-plugin.md §6](./contracts/repo-mode-plugin.md#6-non-affected-plugins-guard-test). Iterate the 14 non-affected plugin slugs; for each, compute results with `Data.Repo == nil` and `Data.Repo != nil` and assert byte-identical output. Locks the contract that only the 7 listed plugins branch on `data.Repo`.
- [ ] T038 [P] Update `README.md` Status line: `M7 (repository template) complete.` Add a `## Repository template` mini-section linking to the M7 quickstart. Sample example: `metrics-action --user octocat --repo hello-world --template repository --output svg --dryrun --filename -`.
- [ ] T039 [P] Run the full quickstart end-to-end per [quickstart.md §3-§5](./quickstart.md). Verify `make test` / `make test-chromedp` / `make test-heavy` / `make lint` all green on the maintainer environment (macOS + Apple M5 + system Chrome). Capture pass/fail per quickstart step in the PR description.
- [ ] T040 Run `make gen-action-yml` once more to confirm the committed `action.yml` matches the regenerated output (lefthook `action-yml-drift` gate). If drift detected, regenerate + commit before merging.

**Checkpoint (Phase 6)**: All 6 SCs satisfied. 4 compliance invariants (M4 21 plugins + M6 no-new-plugins + M7 templates + M7 non-affected-plugin-inertness) hold. README points users to the M7 quickstart.

---

## Dependencies

### Phase ordering

- **Phase 1 (Setup)** → **Phase 2 (Foundational)** → **Phase 3 (US1)** → **Phase 4 (US2)** → **Phase 5 (US3)** → **Phase 6 (Polish)**
- T003 (genqlient regen) must complete after T002 (query file added) and before any task that imports `internal/githubapi/graphql_repository.go` (T009 onwards).
- T013-T014 (template registration) blocks T025 (template integration test), T026 (US1 integration test), T029-T030 (US2 golden + PNG/JPEG tests).
- T018-T024 (7 plugins repo-mode) all block T025/T026 because partials registered via T016 reference these plugin Result structs.

### Cross-phase blockers

- **T005-T006** (Repo entity + Data.Repo) block T009 (FetchRepo) + T010 (base.Compute wire-up) + T011 (validator) + T013 (template Check) + T018-T024 (plugin repo-mode).
- **T007-T008** (engine.Request.Repo + newInvocation wiring) block T011 (validator), T013 (template registration consumes Account), and T026 (integration test).
- **T009-T010** (FetchRepo + base wire-up) block T018-T024 (plugin repo-mode tests need `data.Repo` populated) and T025-T026.
- **T013-T016** (template + partials) block T025 (template integration test) and US2 tasks (T028-T030).

### Parallel opportunities within phases

- **Phase 1**: T001 / T002 / T004 are [P] amongst themselves. T003 sequential after T002.
- **Phase 2**: T005 / T006 are [P] (different files). T007-T010 sequential (they touch shared types incrementally — engine.Request → action.newInvocation → base.Compute).
- **Phase 3**: T011 / T012 are [P]. T013-T016 sequential against same `internal/templates/repository/` directory. T017 sequential after T015-T016. T018-T024 are all [P] (7 different plugin packages). T025 sequential after T013-T016 + T018-T024. T026 sequential after T013-T014.
- **Phase 4**: T027 / T028 / T029 are [P] (different test files). T030 / T031 sequential after T013-T016.
- **Phase 5**: T032 / T033 are [P] (different test functions in same file — Go testing handles concurrent t.Run subtests with t.Parallel). T034 sequential after T011.
- **Phase 6**: T035 / T036 / T038 / T039 are [P] (different files / suites). T037 sequential after T018-T024. T040 final sequential gate.

---

## Implementation Strategy

### MVP scope (just US1 → ship)

US1 alone delivers spec SC-001 + SC-003 + SC-005. Skip US2 (golden files for PNG/JPEG/JSON) and US3 (FR-007 backward-compat tests) only if time pressure forces incremental delivery — but bundle them into the same PR for review economy (1-task spec, 40-task implementation, single review pass per existing project cadence).

### Recommended order (full M7)

1. **Phase 1 (T001-T004)**: Setup — under 30 min.
2. **Phase 2 (T005-T010)**: Foundational — under 2 hours. Verify `data.Repo` populates correctly via T010 test.
3. **Phase 3 (T011-T026)**: US1 — half a day, with T018-T024 (7 plugin refactors) parallelizable.
4. **Phase 4 (T027-T031)**: US2 — under 2 hours, with golden files seeded via `-update`.
5. **Phase 5 (T032-T034)**: US3 — 1 hour, validation guards.
6. **Phase 6 (T035-T040)**: Polish — 1-2 hours, including end-to-end quickstart run.

Total estimate: 1-1.5 days for a single contributor; 0.5 day if T018-T024 parallelize across reviewers.

### Test ordering within each story

Per constitution IV (Table + Golden File), tests land alongside the implementation file (not before — this project does not enforce strict TDD). Each plugin refactor PR includes both the implementation and the paired user-mode + repo-mode tests in a single commit so the constitution gate `golangci-lint run` passes.
