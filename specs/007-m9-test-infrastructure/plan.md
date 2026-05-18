# Implementation Plan: M9 — test infrastructure consolidation

**Branch**: `007-test-infrastructure` | **Date**: 2026-05-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/007-m9-test-infrastructure/spec.md`

## Summary

M9 consolidates the test-helper scaffolding that landed ad-hoc across
M1-M7 into a single shared `internal/testutil/` package, builds out
the missing fixture-file dispatch path documented in
[`docs/design/10-testing-deployment.md §2`](../../docs/design/10-testing-deployment.md#2-mocks-の設計),
unifies the golden-file workflow under one `Compare` family, and
extends the CI lint set. Purely test-infrastructure — no production
code path changes.

## Technical Context

**Language/Version**: Go 1.26 (unchanged; per `go.mod`)

**Primary Dependencies**: existing only — `github.com/Khan/genqlient`
v0.8.2-pre (for the `graphql.Doer` interface the GraphQL mock
implements), stdlib `testing` / `net/http` / `net/http/httptest`. No
new go.mod entries.

**Storage**: N/A — test infrastructure is process-local.

**Testing**: This IS the testing infrastructure feature. The new
helpers themselves get unit-tested in `internal/testutil/*/*_test.go`
(table-driven cases for the mocks + a self-check for the golden
framework). FR-009 / FR-012 migrate one existing scattered mock as a
demonstration that the surface area is sufficient; FR-013 mandates no
regression in any existing test.

**Target Platform**: same as M1-M7 — Go test runner on
linux/darwin × amd64/arm64. The chromedp-gated tests
(`*_chromedp_test.go`) continue to opt into the chromedp build tag.

**Project Type**: Go single-binary monorepo (no change from M6/M7).

**Performance Goals**: per-test setup MUST stay sub-millisecond. The
new mocks construct in-memory handler maps + read fixture files lazily
on first dispatch; per-test overhead is negligible.

**Constraints**: must keep M1-M7 success criteria green; no
production code in `internal/action/` / `internal/engine/` /
`internal/plugins/` / `internal/templates/` / `cmd/` may change
beyond exposing type-only helpers consumed by the new mocks package
(FR-013). Existing golden files MUST stay byte-stable across the
refactor (FR-009 + spec assumption).

**Scale/Scope**: 1 new internal package (`internal/testutil/` with
2 sub-packages: `mocks/`, `golden/`), 5-10 new fixture files under
`tests/fixtures/github/{rest,graphql}/`, 1 migrated demonstration
test (`internal/plugins/base/repository_test.go`), 1 demonstration
golden test (`tests/integration/output_svg_test.go`), 1 CI workflow
update. Estimated LOC delta: **+800 testutil package + new tests**,
**-400 from migrated scattered scaffolding** = net +400 LOC, of which
~80% is shared infrastructure that future tests reuse.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. 入力互換性 (NON-NEGOTIABLE) | ✓ N/A | M9 touches no production input parsing — action.yml + INPUT_* env handling unchanged |
| II. 出力契約 (DOM/JSON 単位) | ✓ N/A | No rendered output changes; goldens stay byte-stable. The golden-compare helper is more rigorous than the existing per-test comparators (first-divergent-offset + window) but the comparison rules + masked-footer normalization are unchanged |
| III. スコープ規律 (採用機能のみ実装) | ✓ PASS | Pure test infrastructure. The `internal/testutil/` package is explicitly named in `docs/design/10-testing-deployment.md §2.1` as adopted M9 scope. `internal/plugins/` directory set unchanged (`TestCompliance_M4_AdoptedPlugins` invariant holds); `internal/templates/` unchanged (`TestCompliance_M7_TemplateInvariant` invariant holds); no M5/M8-scoped feature lands |
| IV. テーブルテスト + Golden File | ✓ PASS (strengthens) | M9 unifies golden-file usage under a single helper family. Existing per-test variants get migrated to the new shared `Compare` / `CompareSVG` / `CompareJSON` API. The shared helper's first-divergent-offset diff message strengthens the principle's debuggability |
| V. Go 規約と言語ポリシー | ✓ PASS | gofumpt / golangci-lint baseline unchanged. M9 extends the lint set (FR-011) via `.golangci.yml` — strengthens rather than relaxes the principle. Conventional Commits + 日本語 docstring 維持 |

**Result: ALL GATES PASS** — no Complexity Tracking entry needed.

## Project Structure

### Documentation (this feature)

```text
specs/007-m9-test-infrastructure/
├── plan.md              # This file
├── research.md          # Phase 0 — R-001..R-005 (mock interface choice, fixture loading strategy, golden diff format, lint additions, CLI subprocess unification)
├── data-model.md        # Phase 1 — E-001..E-005 (RESTMux, GraphQLMux, fixture file index, golden Comparator family, PluginContext builder)
├── quickstart.md        # Phase 1 — examples: plugin test using the mocks, golden compare for SVG/JSON, fixture seeding workflow
├── contracts/
│   ├── rest-mock.md           # RESTMux handler API + fixture path conventions + dispatch rules
│   ├── graphql-mock.md        # GraphQLMux operationName dispatch + variables handling + fixture path conventions
│   ├── golden-compare.md      # Compare / CompareSVG / CompareJSON contract + -update flag handling + diff message format
│   └── lint-config.md         # .golangci.yml linter additions (M9 surface)
├── tasks.md             # Phase 2 (NOT created by /speckit-plan)
└── checklists/
    └── requirements.md   # 16/16 PASS (created in /speckit-specify)
```

### Source Code (repository root)

```text
internal/
├── testutil/                # NEW: M9 root package
│   ├── doc.go               # Package docs + adopted-scope marker
│   ├── mocks/
│   │   ├── rest.go          # RESTMux (http.RoundTripper) + OnFile/OnBody/OnHeader
│   │   ├── rest_test.go     # table tests for handler resolution + 404 fallback + Calls counter
│   │   ├── graphql.go       # GraphQLMux (genqlient graphql.Doer) + OnFile/OnBody/OnFunc
│   │   ├── graphql_test.go  # table tests for operationName dispatch + unknown-op t.Fatalf path
│   │   ├── plugin_context.go # PluginContext builder + WithGraphQL/WithREST options
│   │   └── plugin_context_test.go
│   └── golden/
│       ├── golden.go        # Compare + CompareSVG + CompareJSON + the shared -update flag
│       ├── golden_test.go   # self-check (seed-then-compare, normalization round-trip)
│       └── svg_normalize.go # M2 NormalizeSVG helper moved here from tests/integration (re-export shim left behind)

tests/
├── fixtures/
│   └── github/              # NEW: M9 canonical fixture tree
│       ├── rest/
│       │   ├── rate_limit.json
│       │   ├── contributors_hello_world.json
│       │   └── ... (5-10 seed files driven by FR-012 migration)
│       └── graphql/
│           ├── user_octocat.json
│           ├── user_repositories_250.json
│           ├── repository_hello_world.json
│           └── repository_organization.json
└── integration/
    ├── output_svg_test.go    # MIGRATED: use mocks.NewGraphQLMux + golden.CompareSVG (FR-009)
    └── cli_test.go           # MIGRATED: startGitHubMock helper consumes the new mocks package (FR-010)

internal/plugins/base/
└── repository_test.go        # MIGRATED: 360 LOC → <200 LOC (FR-012 demonstration)

.github/workflows/
└── go-ci.yml                 # EXTENDED: explicitly declare the M9 lint set per FR-011

.golangci.yml                 # EXTENDED: M9 linter additions (staticcheck QF rules + gosec + gocritic ifElseChain)
```

**Structure Decision**: Single-binary Go monorepo (unchanged). M9
adds one new internal package tree (`internal/testutil/`) plus a new
fixture sub-tree (`tests/fixtures/github/`) and migrates a small
number of existing test files to use the new helpers. No production
code packages change.

## Complexity Tracking

*No violations — all 5 constitution gates pass. This section is omitted
per template guidance.*
