---

description: "Task list for 012 — wire repositories.Starred to /users/{login}/starred"
---

# Tasks: Wire `repositories.Starred` to `/users/{login}/starred`

**Input**: Design documents from `/specs/012-rest-data-fetch/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (required), [contracts/plugin-repositories-starred.md](./contracts/plugin-repositories-starred.md)

**Tests**: ENABLED. Constitution principle IV (テーブルテスト + Golden File) requires table tests + golden tests for any plugin Run change.

**Scope**: 1 User Story only (US1 — Starred fetch). See [spec.md §History](./spec.md#history) for why the original 4-plugin scope was reduced.

## Format

`- [ ] [ID] [P?] [Story?] Description with file path`

---

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Verify branch `012-rest-data-fetch` checked out and `git status` clean. Run `go build ./...` to confirm baseline compiles. *(Already verified during initial scope.)*
- [X] T002 Confirm `(*REST).Get` + `(*REST).Scopes` helpers exist. *(Verified: rest.go:157 + scopes.go:28.)*

## Phase 2: Foundational

*(Initial scope had a `restutil/parallel.go` helper; with the scope narrowed to a single-endpoint Starred fetch, the helper is no longer required for this PR. It is left in place as a future-use utility — see [research.md §R-005](./research.md).)*

- [X] T003 [P] Existing `internal/testutil/mocks/rest.go::RESTMux` already supports per-path handlers. *(NO-OP.)*
- [X] T004 [P] `(*REST).Get` + `Scopes` helpers already present. *(NO-OP.)*
- [X] T005 [P] `internal/plugins/restutil/parallel.go` — initially implemented + tested, then **removed** after scope reduction. The compliance gate (`TestCompliance_M4_AdoptedPlugins`) restricts `internal/plugins/` to adopted plugin slugs only, and the helper is unused by the Starred-only path. Will be re-added under `internal/restutil/` (outside `plugins/`) in PR 013 when GraphQL parallel fetching needs it.
- [X] T006 [P] `internal/plugins/restutil/parallel_test.go` — covered by the T005 deletion.

---

## Phase 3: User Story 1 - Starred fetch (Priority: P1) 🎯 MVP

**Goal**: Replace the `res.Starred = featured` placeholder at `internal/plugins/repositories/repositories.go:127` with a real fetch from `/users/{login}/starred`.

**Independent Test**: Per [spec.md §US1](./spec.md#user-story-1---starred-repositories-list-reflects-users-actual-stars-priority-p1--mvp) Acceptance Scenarios 1-5.

### Tests for User Story 1 (write FIRST)

- [X] T007 [P] [US1] Added 6 table-test cases (`TestRun_Starred_*`) to `internal/plugins/repositories/repositories_test.go`: HappyPath / MappingCorrectness / NetworkFailure / EmptyLogin / NotEnabled / RESTNilFallback.

### Implementation for User Story 1

- [X] T008 [US1] Modify `internal/plugins/repositories/repositories.go`:
  - Change `Run(_ context.Context, ...)` → `Run(ctx context.Context, ...)` so the REST call receives a real context.
  - Add a new helper `fetchStarred(ctx, rest, login string, limit int) ([]plugins.Repository, error)` that calls `rest.Get(ctx, "/users/<login>/starred?per_page=100&sort=created&direction=desc", nil)`, decodes the response, maps each repo entry to `plugins.Repository` per [data-model.md §E-012-C](./data-model.md#e-012-c-repositoriesresult-既存-starred--random-フィールド-populate), truncates to limit, and returns the slice.
  - Replace `if in.starred { res.Starred = featured }` with:
    ```go
    if in.starred {
        if pc.REST != nil && pc.Data.User != nil && pc.Data.User.Login != "" {
            starred, err := fetchStarred(ctx, pc.REST, pc.Data.User.Login, in.limit)
            if err != nil {
                pc.Data.AppendError(xerrors.NewRetryableError(fmt.Errorf("repositories.starred: %w", err)))
                // leave res.Starred nil
            } else {
                res.Starred = starred
            }
        } else if pc.REST == nil && pc.Data.User != nil && pc.Data.User.Login != "" {
            // M4 baseline fallback (FR-006)
            res.Starred = featured
        }
        // else: Result.Starred stays nil (FR-005)
    }
    ```
  - Add per-request timeout via `context.WithTimeout(ctx, 30*time.Second)`.
  - ~80 LOC including helper + decoder struct.

- [X] T009 [US1] `tests/golden/json/m4/repositories.json` UNCHANGED — the default M4 octocat case in `TestRun_GoldenShape_Repositories` does not enable `plugin_repositories_starred`, so the golden delta is empty as predicted.

- [X] T010 [US1] `go test ./internal/plugins/repositories/...` — 13/13 GREEN (including 6 new Starred TCs).

**Checkpoint**: Starred uses real `/users/{login}/starred` data. The remaining 3 originally-planned stories (traffic / contributors user-org / random) were either already implemented (traffic / random) or intentionally Skipped per upstream contract (contributors user-org); see [spec.md §History](./spec.md#history).

---

## Phase 4: Polish & Cross-Cutting Concerns

- [X] T011 `go test ./...` — all green across all packages (after removing the speculatively-added `internal/plugins/restutil/` directory that triggered the M4 adopted-plugin gate).
- [X] T012 [P] `TestNoUnadoptedPluginReference` allow-list — UNCHANGED. The new code adds the literal `"starred"` (= adopted plugin slug, not unadopted) and `"created"` / `"direction"` (URL query parameters, not slugs).
- [X] T013 Constitution gate `go test ./tests/compliance/...` — green.

---

## Dependencies & Execution Order

- T001-T002: completed during exploration.
- T003-T006: completed in earlier Foundational phase (kept for future PRs).
- T007 must FAIL before T008 (TDD per constitution IV).
- T008 → T009 → T010 → T011 → T012/T013 sequential.

## Implementation Strategy

Single-user-story PR — sequential implementation:

1. T007 (tests fail) → T008 (real fetch) → T009 (golden update if any) → T010 (tests green).
2. T011 broad regression.
3. T012/T013 final compliance check.

Total estimated ~2 hours for the implementation + tests.

## Notes

- The original 27-task plan was scope-reduced after the initial implementation pass discovered that 3 of the 4 originally-targeted plugins were either already wired (traffic / random) or intentionally Skipped (contributors user-org).
- `cmd/render-fixture/` (added in 011) is not relevant to this PR — Starred is fetched live via REST, no fixture-render path needed.
- GraphQL plugins (sponsors / sponsorships / projects / repositories.Pinned 等) remain on the PR 013 roadmap; chromedp plugins (topics) on 014.
