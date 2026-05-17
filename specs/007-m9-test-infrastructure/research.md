# Research: M9 — test infrastructure consolidation

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

This document captures the technical decisions taken before Phase 1
design. The M9 spec has zero `NEEDS CLARIFICATION` markers because
the source-of-truth docs already pinned the design surface; this
research expands those references into actionable design
constraints.

## R-001: Mock interface choice — extend existing or wrap

**Decision**: Build new mocks from scratch in `internal/testutil/mocks/`,
mirroring the API shape of the existing per-package `graphQLMux` in
`internal/plugins/base/testhelper_test.go` (which is the most
feature-complete existing implementation). The new mocks expose
`OnFile` / `OnBody` / `OnFunc` / `Calls` per the spec FR-001..FR-004,
plus `t.Cleanup` registration per FR-005.

**Rationale**: The `testhelper_test.go::graphQLMux` already implements
the contract correctly (sync.Mutex + per-operationName handler slice
+ atomic.Int32 counter); migrating it as-is would require exporting
test-helper code from a `_test.go` file (Go forbids that). Building
fresh in a new package keeps the helper non-test-tagged so other
packages can import it. The API shape is small enough (3 constructors
+ `Calls` + the Doer/RoundTripper method) that re-implementation is
cheaper than refactoring around Go's `*_test.go` access rules.

**Alternatives considered**:
- *Move `graphQLMux` to a non-test file in `internal/plugins/base`* —
  rejected because exposing test-only helpers from a production
  package is against Go convention and would force base to import
  `testing` from non-test code.
- *Use `httptest.NewServer` for everything* — rejected because the
  GraphQL mock dispatches on `operationName` (request body inspection),
  not URL path; httptest.Server is just sugar around RoundTripper
  with no operationName awareness. The shared mocks package can layer
  on top of httptest for tests that want it but doesn't depend on it.
- *Use a third-party library (`vektra/mockery`, `golang/mock`)* —
  rejected because the mocked interface (`graphql.Doer`) is one
  method and the dispatch logic is project-specific (operationName +
  fixture-file convention). A library adds dependency churn without
  saving meaningful code.

## R-002: Fixture file path convention

**Decision**: Fixture files live under
`tests/fixtures/github/{rest,graphql}/<name>.json`. The mock helpers
accept a path **relative to the repo root** (e.g.,
`"github/graphql/user_octocat.json"`), then prepend the discovered
repo-root path. Fixture path resolution uses the same `mustRepoRoot`
walker that `tests/compliance/compliance_test.go::mustRepoRoot`
already implements — extracted to `internal/testutil/internal/repoutil.go`
(or directly in `internal/testutil/mocks`).

**Rationale**: Mirrors the existing M2-M7 fixture convention
(`tests/fixtures/plugins/`, `tests/fixtures/upstream/`, etc.) so
contributors don't learn a new path layout. Relative-to-repo-root
paths are stable across the various test working directories Go
selects per package. The mustRepoRoot walker is a known-good helper
that all tests in the project consume.

**Alternatives considered**:
- *Relative to the test file's package directory* — rejected because
  fixtures shared across packages (e.g., `user_octocat.json` used by
  the action package AND the plugins/base package) would need
  duplicate copies per package directory.
- *Embed fixtures via `go:embed`* — rejected because embedding adds
  rebuild cost on fixture changes + makes it harder for contributors
  to inspect/diff fixture files in their editor. Read-from-disk on
  first dispatch is fast enough for tests and keeps fixtures as
  first-class files.

## R-003: Golden-diff message format

**Decision**: When a non-update golden comparison fails, the helper
emits a single `t.Errorf` containing:

1. The relative golden path.
2. The first divergent byte offset.
3. A 40-byte before-context + 40-byte after-context window of both
   `got` and `want`, with non-printable bytes rendered as `\xNN`.
4. The `len got=N want=N` summary (preserved from the existing
   per-test variants for grep-friendliness).
5. The hint `(run with -update to seed)` so first-runs are obvious.

**Rationale**: Solves the M7 self-review finding (SF #4 in the PR
#307 review: "SVG drift; len got=33451 want=33101" is actionably
empty). 40-byte window is the same size pattern test failure
reporters in popular Go libraries use (`stretchr/testify/assert`,
`google/go-cmp` truncation). Per-rune rendering avoids terminal
breakage when binary bytes leak into the SVG (rare but possible with
chromedp-rendered output).

**Alternatives considered**:
- *Full `cmp.Diff(got, want)` output* — rejected because Go's `cmp`
  library is designed for structured types, not raw byte slices.
  Stringifying 30+ KB SVGs through `cmp.Diff` produces unreadable
  multi-megabyte test logs.
- *Unix `diff -u` shell out* — rejected because it requires `diff` in
  PATH (not guaranteed on Windows CI runners + Docker minimal
  images).

## R-004: Lint additions for CI (FR-011 / US3)

**Decision**: Extend `.golangci.yml` with three additional linters in
the M9 surface:

- `staticcheck` QF rules (specifically QF1001 De Morgan + QF1003
  tagged switch + QF1004 strings.ReplaceAll) — staticcheck already
  enabled; we ensure the QF subset is on (it was off in the M7 PR
  #307 where a staticcheck QF1001 finding was filed manually).
- `gosec` — already enabled in `.golangci.yml`; ensure `G115`
  (integer overflow conversion) stays on after the M7 introduction
  of `byte(r)` paths.
- `gocritic.ifElseChain` — adds tagged-switch suggestions
  (complements the staticcheck QF1003 finding type).

**Rationale**: Each of these was a finding type that surfaced in
M6/M7 self-reviews. Centralizing them in `.golangci.yml` ensures CI
catches them before the next PR's self-review. The version pin
(v2.12.2 in `go-ci.yml`) supports all three.

**Alternatives considered**:
- *Add `errcheck` + `unparam`* — rejected: already in `.golangci.yml`
  default set. No-op for M9.
- *Add `gocyclo` / `funlen`* — rejected: would force a big-bang
  refactor of existing long functions outside M9 scope.

## R-005: CLI subprocess test unification (T-122)

**Decision**: `tests/integration/cli_test.go::TestCLI_OctocatSVG_Stdout`
(M6) already does the subprocess + INPUT_* injection. M9 keeps that
test file as the canonical e2e shape and migrates **only** its
`startGitHubMock` helper to consume `mocks.NewRESTMux` +
`mocks.NewGraphQLMux` underneath. No new `tests/integration/action_dryrun_test.go`
file lands.

**Rationale**: T-122 from `16-tasks-mvp.md` says "add
`tests/integration/action_test.go`" but the M6 PR landed equivalent
coverage as `cli_test.go::TestCLI_OctocatSVG_Stdout`. Creating a
parallel `action_dryrun_test.go` would duplicate the binary build +
httptest server setup. The M9 spec FR-010 explicitly chose to
consolidate rather than duplicate.

**Alternatives considered**:
- *Add a new `action_dryrun_test.go` that does literally what T-122
  says* — rejected: violates DRY and the M6 + M9 SC-001 LOC budget.
  Per `docs/design/16-tasks-mvp.md`, T-122 explicitly notes
  "`metrics-action --dryrun` を子プロセスで起動" which is what the
  existing M6 test already does.

## R-006: Migrated demonstration test — choice of victim

**Decision**: Migrate `internal/plugins/base/repository_test.go` as
the FR-012 demonstration. It is the largest currently-scattered mock
implementation (360 LOC including the local `restRouter` + 4 inlined
JSON fixture strings) so its LOC delta (target: <200 after migration
per SC-001) is the most observable validation of the M9 surface area.

**Rationale**: Three candidates qualified:

- `internal/plugins/base/repository_test.go` (M7) — 360 LOC,
  inline `restRouter` + inline fixtures.
- `internal/action/action_test.go` (M6) — 374 LOC, inline `fakeREST`
  + `fakeGraphQL`.
- `tests/integration/cli_test.go` (M6) — 246 LOC, inline
  `startGitHubMock`.

The first has the largest LOC gain potential + the most ad-hoc
fixture surface (the 4 inline `repositoryHelloWorldFixture` /
`repositoryOrgOwnerFixture` / `repositoryNotFoundFixture` /
parseLinkLastPage-table-fixture strings). Migrating it also seeds the
canonical `tests/fixtures/github/graphql/repository_*.json` files
that downstream tests can reuse.

**Alternatives considered**:
- *Migrate all three at once* — rejected: explodes M9 PR scope
  beyond the 6 documented tasks. The other two can migrate in
  follow-up PRs once the M9 surface lands and contributors validate
  the API.
- *Don't migrate any existing test (write only new ones)* —
  rejected: spec FR-012 mandates at least one migration as proof that
  the API surface is sufficient.

## R-007: Golden helper backward-compat shim

**Decision**: Keep the existing `updateGolden` flag declaration in
`tests/integration/output_json_test.go` as the source-of-truth flag
variable. The new `golden.Compare` family reads from the same
`flag.Lookup("update")` rather than declaring a parallel flag. Tests
that switch to the new helper drop their local update-flag handling
but the shared flag stays declared in the integration package.

**Rationale**: Avoids the "two `-update` flags" footgun where one
test honors one flag and another honors another. The existing flag
is already wired through `make test -update` in the lefthook config
and contributor muscle memory.

**Alternatives considered**:
- *Declare the flag in `internal/testutil/golden`* — rejected: would
  force every test that uses the helper to import golden _and_ the
  flag package, and would conflict with the existing flag
  declaration at first `make test` run.
- *Use an env var (`METRICS_GOLDEN_UPDATE=1`)* — rejected: breaks
  the existing `go test -update` muscle memory + lefthook wiring.

## R-008: Migration ordering + risk management

**Decision**: Phase the M9 commits to keep CI green at every step:

1. **Setup commit**: Add `internal/testutil/{mocks,golden}/` package
   skeleton + unit tests for the helpers themselves. No existing
   tests touched.
2. **Fixture seeding commit**: Move the inline JSON strings into
   `tests/fixtures/github/{rest,graphql}/`. Inline strings stay
   referenced from existing tests (no migration yet).
3. **Demonstration migration commit**: Migrate
   `internal/plugins/base/repository_test.go` to the new helpers.
   Validates the API + drops LOC.
4. **Lint config commit**: Extend `.golangci.yml` (FR-011). May
   surface staticcheck QF1001 findings in existing M6/M7 code; fix
   them inline.
5. **Polish commit**: Final cleanup + tasks.md completion marks.

**Rationale**: Each phase is independently revertable. Phase 1 is
purely additive (no risk to CI). Phase 2 duplicates content but
nothing breaks. Phase 3 is the only risky step (test logic moves);
it's small + bounded. Phase 4 may need 1-3 small fixes elsewhere in
the tree but each is mechanical.

**Alternatives considered**:
- *One big-bang commit* — rejected: makes review harder + a single
  CI failure forces full rollback. The 5-phase split lets reviewers
  evaluate each concern independently.
