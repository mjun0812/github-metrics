# Feature Specification: M9 — test infrastructure consolidation

**Feature Branch**: `007-m9-test-infrastructure`

**Created**: 2026-05-18

**Status**: Draft

**Input**: User description: "次のM9をやりましょう"

## Overview

M9 consolidates the test-helper scaffolding that landed ad-hoc across
M1-M7 into a single shared `internal/testutil/` package. M1-M7 each
grew their own mock GraphQL / REST RoundTrippers, their own golden
file helpers, their own httptest fixtures — so the same patterns now
live in 8+ test files (`internal/action/action_test.go`,
`internal/plugins/base/testhelper_test.go`,
`internal/plugins/base/repository_test.go`,
`tests/integration/cli_test.go`,
`tests/integration/repository_test.go`,
`tests/integration/foundation_test.go`, plus chromedp-gated variants).
The duplication invites drift (e.g., the `parseLinkLastPage` bug
discovered in M7 was masked because each plugin's test mocked the
Link header slightly differently).

Per [`docs/design/16-tasks-mvp.md` Phase M9 (T-118..T-125)](../../docs/design/16-tasks-mvp.md#phase-m9-テスト基盤-6-タスク)
+ [`docs/design/10-testing-deployment.md` §1-§2](../../docs/design/10-testing-deployment.md),
M9 ships:

- `internal/testutil/mocks/rest.go` — shared REST `MockTransport` with
  fixture-file dispatch (T-118)
- `internal/testutil/mocks/graphql.go` — shared genqlient `Doer` mock
  dispatching on `operationName` (T-119)
- `internal/testutil/golden/` — golden-file framework with SVG
  normalization + `-update` flag handling (T-120)
- `tests/integration/compute_test.go` — classic SVG end-to-end
  round-trip using the new mocks + golden helpers (T-121)
- `tests/integration/action_dryrun_test.go` — `metrics-action --dryrun`
  subprocess e2e via `os/exec` with INPUT_* injection (T-122 — partial
  already exists in `cli_test.go`, M9 consolidates)
- `.github/workflows/go-ci.yml` extended with linter additions
  per spec §2 (T-125 — most of this is already wired; M9 fills the
  gap)

The constitution III invariant holds: no new plugin / template lands.
The change is **purely test-infrastructure**, refactoring the existing
~600 LOC of duplicated test scaffolding into a shared package and
backfilling the missing fixture-file dispatch path that
`docs/design/10-testing-deployment.md §2` specifies but was never
implemented.

## Clarifications

### Session 2026-05-18

No clarifications required — the source-of-truth docs
[16-tasks-mvp.md M9](../../docs/design/16-tasks-mvp.md#phase-m9-テスト基盤-6-タスク)
and [10-testing-deployment.md §2](../../docs/design/10-testing-deployment.md#2-mocks-の設計)
specify the design surface unambiguously.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - New plugin author wires mocked tests in <30 lines (Priority: P1)

A contributor adding a new plugin (e.g., porting a previously-skipped
upstream plugin) needs to write its unit tests. Without M9 they must
copy 60+ lines of mock-roundtripper scaffolding from one of the
existing plugin tests + maintain a parallel fixture loader.

With M9 they `import "github.com/.../internal/testutil/mocks"` and
write:

```go
gql := mocks.NewGraphQLMux(t)
gql.OnFile("User", "github/graphql/user_octocat.json")
gql.OnFile("UserRepositories", "github/graphql/user_repositories_250.json")

rest := mocks.NewRESTMux(t)
rest.OnFile("/repos/octocat/hello-world/contributors",
    "github/rest/contributors_hello_world.json")

pc := mocks.NewPluginContext(t, mocks.WithGraphQL(gql), mocks.WithREST(rest))
got, err := myplugin.Plugin.Run(ctx, pc)
```

**Why this priority**: P1 because this is the load-bearing reason for
M9 — the existing scattered scaffolding has already caused observable
drift (the M7 self-review found a Link-header parser bug masked by
each plugin's slightly different mock setup). Every M9 task other
than T-125 (CI gates) directly supports this story.

**Independent Test**: Refactor one existing plugin's tests
(`internal/plugins/base/repository_test.go` — 360 LOC of locally-
defined `restRouter` + duplicated fixture strings) to use the new
`mocks` package and verify the test still passes + LOC count drops
below 200.

**Acceptance Scenarios**:

1. **Given** the `internal/testutil/mocks` package exists, **When** a
   plugin test imports it and calls `mocks.NewGraphQLMux(t).OnFile(...)`
   with a fixture path, **Then** the GraphQL client routed through the
   mock returns the canned response without per-test JSON literal
   strings.
2. **Given** the same mock is constructed twice in two parallel
   subtests (`t.Run` + `t.Parallel()`), **When** both subtests run,
   **Then** they do not race or share fixture state.
3. **Given** a fixture file referenced by the mock does not exist,
   **When** the test runs, **Then** the failure is reported with
   `t.Fatalf("fixture not found: <path>")` — never as a silent empty
   response or a confusing JSON-decode error downstream.

---

### User Story 2 - Golden-file workflow stays consistent across the test suite (Priority: P2)

A maintainer regenerating goldens after a deliberate output change
runs `go test -update ./...` and expects every golden test in the
project to use the same `-update` flag semantics + normalization rules
(SVG attribute-sort + whitespace collapse + dynamic-footer mask + the
classic `-update` flag declared in `output_json_test.go`).

Currently four packages define their own variant of "compare +
update" (`tests/integration/output_json_test.go`, `output_svg_test.go`,
`banner_test.go`, the new M7 `repository_test.go`). M9 extracts the
shared shape into `internal/testutil/golden` so all golden tests pick
up the same `-update` flag, the same normalization helper, and the
same diff-on-failure error message.

**Why this priority**: P2 because the existing per-test variants
already work — this is consolidation that reduces drift risk + makes
golden-failure debugging consistent. The M7 self-review flagged
"SVG drift; len got=N want=N" as actionably-empty; M9 fixes that for
every golden test at once.

**Independent Test**: Trigger a deliberate golden drift in one
template (e.g., add a stray space) and run any golden test. Assert
the failure message contains a first-divergent-offset + 40-byte
window around it (vs the current length-only output).

**Acceptance Scenarios**:

1. **Given** `internal/testutil/golden.Compare(t, got, goldenPath)`
   is the single entry point, **When** a test invokes it with
   `-update`, **Then** the golden file is rewritten and the test
   passes.
2. **Given** the same call without `-update` and a drifted got,
   **When** the test runs, **Then** the failure prints the first
   divergent byte offset + a 40-byte before/after window.
3. **Given** the SVG golden helper, **When** the input has the
   dynamic footer (timestamp / version) replaced by sentinels, **Then**
   the masked SVG compares equal regardless of Go runtime version.

---

### User Story 3 - CI catches additional classes of bugs the current lint set misses (Priority: P3)

A reviewer pushing a PR with subtle code smells (unused imports
disguised by build tags, untyped string constants where a typed alias
would be safer, error-class mismatches) wants CI to flag them before
self-review. The current `go-ci.yml` runs golangci-lint + govulncheck
+ vet but the M7 self-review surfaced gaps (e.g., untyped Mode
constants in `repo_mode.go`).

**Why this priority**: P3 because golangci-lint already covers most
classes; the additions are incremental rather than load-bearing.

**Independent Test**: Push a branch that intentionally violates one
of the new lints (e.g., an untyped constant where a typed alias is
recommended); CI fails with the specific lint name + actionable
message.

**Acceptance Scenarios**:

1. **Given** `.github/workflows/go-ci.yml` carries the M9-extended
   lint set, **When** a PR triggers CI, **Then** the lint job runs
   `golangci-lint run --timeout=10m` with the documented lint
   subset enabled (staticcheck QF rules, gosec, gocritic
   ifElseChain, gofumpt) explicitly declared in `.golangci.yml`.
2. **Given** a PR introduces a typed-alias-recommended pattern,
   **When** CI runs, **Then** the lint job reports the issue with a
   precise file:line + fix suggestion.

---

### Edge Cases

- **Fixture file missing**: mock dispatch MUST `t.Fatalf` with a
  clear path; no silent empty response.
- **`operationName` collision**: two test setups in the same package
  both register a handler for the same operation — last-write-wins is
  acceptable BUT MUST be detected by a `t.Helper` warning so debugging
  is fast.
- **Golden file missing on first run**: the framework's `-update`
  path MUST create the file under `tests/golden/<case>/...` with
  `os.MkdirAll(dir, 0o750)`; subsequent runs without `-update` use
  the seeded file.
- **Race on shared global `engine.Version`**: tests that call
  `engine.SetVersionForTest(t, ...)` already exist; M9 must not
  regress this — the new mocks package MUST NOT mutate engine version
  or any other process-wide state.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001** (T-118): System MUST provide
  `internal/testutil/mocks.NewRESTMux(t *testing.T)` returning a
  `*RESTMux` value that implements `http.RoundTripper` and supports
  `OnFile(path, fixtureRelativePath)` + `OnBody(path, status, body)`
  + `OnHeader(path, status, body, http.Header)` constructors.
  Unknown paths return 404. Fixture files MUST be loaded from
  `tests/fixtures/github/rest/<relative>.json` relative to the repo
  root.
- **FR-002** (T-119): System MUST provide
  `internal/testutil/mocks.NewGraphQLMux(t *testing.T)` returning a
  `*GraphQLMux` value that implements the genqlient `graphql.Doer`
  interface and dispatches on the request's `operationName`. It MUST
  support `OnFile(opName, fixtureRelativePath)` + `OnBody(opName,
  status, body)` + `OnFunc(opName, func(vars) (status, body))`
  constructors. Fixture files MUST be loaded from
  `tests/fixtures/github/graphql/<relative>.json` relative to the
  repo root. Unknown `operationName` MUST `t.Fatalf` so missing
  handlers surface immediately.
- **FR-003** (T-118 / T-119): Both mocks MUST be **goroutine-safe**
  (RWMutex around handler maps) so concurrent `t.Run` + `t.Parallel`
  subtests sharing one mux do not race.
- **FR-004** (T-118 / T-119): Both mocks MUST expose
  `Calls(path|opName) int` helpers so tests can assert call counts
  without manual counter plumbing.
- **FR-005** (T-118 / T-119): Both mocks MUST automatically register
  via `t.Cleanup` so callers do not need to remember to invalidate
  per-test handler state.
- **FR-006** (T-120): System MUST provide
  `internal/testutil/golden.Compare(t *testing.T, got []byte, goldenRelativePath string)`
  that, on the `-update` flag (already declared in
  `tests/integration/output_json_test.go`), rewrites the golden file
  under `tests/golden/<relative>` (creating the parent dir with mode
  0o750). On non-update runs, MUST diff against the seeded golden
  and emit a failure message containing **the first divergent byte
  offset + a 40-byte before/after window** when bytes differ.
- **FR-007** (T-120): System MUST provide
  `internal/testutil/golden.CompareSVG(t, got []byte, goldenRelativePath)`
  that applies the existing M2 NormalizeSVG normalization
  (attribute-sort + whitespace collapse + dynamic-footer mask) before
  diffing. The bare `Compare` does byte-equal; `CompareSVG` does
  semantic-equal.
- **FR-008** (T-120): System MUST provide
  `internal/testutil/golden.CompareJSON(t, got []byte, goldenRelativePath)`
  that re-marshals both sides via `json.MarshalIndent` so per-key
  whitespace drift between Go versions does not break the comparison.
- **FR-009** (T-121): The classic SVG end-to-end round-trip
  (`tests/integration/output_svg_test.go::TestComputeSVG_ClassicOctocatGolden`)
  MUST be migrated to use `mocks.NewGraphQLMux` + `golden.CompareSVG`,
  serving as the canonical example of the new infrastructure. No
  golden file content changes.
- **FR-010** (T-122): The CLI subprocess test
  (`tests/integration/cli_test.go::TestCLI_OctocatSVG_Stdout`) already
  exercises the dryrun + INPUT injection path. M9 MUST refactor it to
  share the new mocks package via the integration server's httptest
  endpoints (no behavior change; same fixture surface).
- **FR-011** (T-125): The CI workflow `.github/workflows/go-ci.yml`
  MUST run `golangci-lint run --timeout=10m` with the additional
  linter set (staticcheck QF rules + gosec + gocritic ifElseChain)
  explicitly enabled in `.golangci.yml`. The existing
  `govulncheck ./...` step MUST continue to run on every push.
- **FR-012** (T-118 / T-119): At least one existing scattered mock
  (e.g., `internal/plugins/base/repository_test.go::restRouter`) MUST
  be migrated to the new shared package as a demonstration that the
  surface area is sufficient. Drift-prone fixtures inlined as Go
  strings (the canned `repositoryHelloWorldFixture`) MUST move to
  `tests/fixtures/github/graphql/repository_hello_world.json`.
- **FR-013**: System MUST NOT change any production code path
  (`internal/action/`, `internal/engine/`, `internal/plugins/`,
  `internal/templates/`, `cmd/`) beyond what is necessary to expose
  type-only helpers consumed by the new mocks package. All currently
  passing tests + golangci-lint + race tests + chromedp + heavy
  builds MUST continue to pass after the refactor.

### Key Entities *(include if feature involves data)*

- **`mocks.RESTMux`** (new): `http.RoundTripper` implementation with
  a per-path handler map (file-backed / inline-body / inline-header).
  Calls tracked per path.
- **`mocks.GraphQLMux`** (new): `graphql.Doer` implementation with a
  per-operationName handler map (file-backed / inline-body /
  variables-aware function). Calls tracked per operationName.
- **`mocks.PluginContext`** (new helper, optional): builder that
  bundles a `*GraphQLMux` + `*RESTMux` + `plugins.NewData()` into a
  `*plugins.PluginContext` ready for `Plugin.Run`.
- **`golden.Compare*`** (new family): three comparators —
  bytes-exact, SVG-normalized, JSON-normalized — sharing one
  `-update` flag.
- **`tests/fixtures/github/{rest,graphql}/<name>.json`** (new
  directory tree): canonical canned responses keyed by short
  filename. M9 seeds at least one fixture per existing inline JSON
  literal that gets migrated.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001** (US1): A new plugin test can be wired in **under 30 LOC**
  using only `internal/testutil/mocks` imports (no per-test
  RoundTripper struct, no per-test JSON literal strings). Verified by
  migrating `internal/plugins/base/repository_test.go` (currently 360
  LOC of scaffolding + tests) and showing it shrinks below **200
  LOC** without losing any test coverage.
- **SC-002** (US2): A deliberate byte change in any golden file
  produces a failure message that pinpoints the first divergent byte
  offset within a 40-byte window — **all** golden tests in the
  project emit this format, not just the M7 new ones.
- **SC-003** (US3): CI `golangci-lint run` MUST detect a single-PR
  introduction of an untyped string constant where the package
  already has a typed alias precedent (e.g., add a stub `const Foo =
  "foo"` next to `AccountKind` and observe the lint failure).
- **SC-004** (regression): All M1-M7 success criteria continue to
  pass on the same checkout. `make test` / `make test-chromedp` /
  `make test-heavy` / `make lint` all green. No golden file content
  changes from M9 alone.
- **SC-005** (compliance): The constitution III invariant
  (`TestCompliance_M4_AdoptedPlugins` +
  `TestCompliance_M6_NoNewPlugins` +
  `TestCompliance_M7_TemplateInvariant`) continues to pass.
  `internal/plugins/` + `internal/templates/` directory sets do NOT
  change.
- **SC-006** (test scaffolding parity): At least **3** existing
  per-package mock implementations (e.g., `restRouter` in
  base/repository_test, `restRouter` precedent in action_test,
  `graphQLMux` in base/testhelper_test) MUST be removed entirely or
  thinned to a single-line re-export. Verified by `git diff --stat`
  showing a net negative LOC count for `*_test.go` files outside the
  new testutil package.

## Assumptions

- The existing M2-M7 golden files stay binary-stable across the M9
  refactor. The new `golden.CompareSVG` applies the same M2
  `NormalizeSVG` helper, which is the only normalization the existing
  goldens have been seeded against.
- The existing M6 `cli_test.go::startGitHubMock` httptest pattern
  (binary subprocess + httptest server for the API URLs) is the
  canonical e2e shape; M9's `tests/integration/action_dryrun_test.go`
  refactor inherits it rather than introducing a new subprocess shape.
- The `tests/fixtures/` directory already exists for plugin fixtures
  (`plugins/`, `render/`, `settings/`, `upstream/`); adding
  `github/{rest,graphql}/` subdirs is additive.
- `golangci-lint` v2.12.2 (pinned in `go-ci.yml`) supports the
  staticcheck QF rules + gosec + gocritic plugins; no version bump
  required.
- Constitution III's "no unadopted plugin" guard does not apply to
  test scaffolding; the `internal/testutil/` package is permitted by
  the project layout (per `docs/design/10-testing-deployment.md §2.1`
  which explicitly names it).
