# Research: M7 — repository template

**Date**: 2026-05-17 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

This document captures the technical decisions taken before Phase 1
design. All `NEEDS CLARIFICATION` markers from the spec's clarify
session (3 questions) are already resolved; this research expands the
chosen options into actionable design constraints.

## R-001: GraphQL query placement and codegen path

**Decision**: Add `internal/githubapi/queries/repository.graphql` and
run `genqlient` (already in `go.mod` + `genqlient.yaml`) to emit
`internal/githubapi/graphql_repository.go`. The new query is named
`Repository` (operation name) and parameterized as
`(login: String!, repo: String!)`. The response fragment matches the
upstream `template.mjs` payload shape: `repository(owner, name,
description, stargazerCount, forkCount, languages.edges.size,
licenseInfo.name, defaultBranchRef.name, primaryLanguage,
collaborators?.totalCount` (REST fallback for some fields).

**Rationale**: Mirrors the M1-M6 pattern (each existing query is a
`.graphql` sibling under `queries/`). genqlient codegen is already
wired through `make gen-graphql` + `lefthook.yml`'s drift hook, so the
new file goes through the same review path. Avoids a hand-written HTTP
client that would skip the centralized header / auth / retry layer.

**Alternatives considered**:
- *Hand-written `internal/githubapi/repo.go`* — rejected because it
  bypasses genqlient's type-safe response struct and creates a second
  authentication code path.
- *Reuse `UserRepositories` query and filter client-side* — rejected
  per spec clarify Q1 Option A: the single-repo response carries fields
  (community health, contributors count, license, default branch,
  sponsorshipsAsMaintainer) that aren't in `UserRepositories.nodes`.

## R-002: `data.Repo` struct placement and lifecycle

**Decision**: Add a new file `internal/plugins/repo.go` defining
`type Repo struct` with fields aligned to the GraphQL response from
R-001. Embed `*Repo` on `Data` as a pointer (`Data.Repo *Repo`), nil
when no `--repo` was provided (i.e., template != repository). Populate
during the `base.Compute` step — before any other plugin runs — so the
M2 partial nil-safety contract continues to hold (`pc.Data.Repo == nil`
yields empty partial output).

**Rationale**: Symmetric to existing `Data.User` / `Data.Organization`.
Centralizes the new type next to the existing entity definitions
(`internal/plugins/data.go` already declares `User`, `Organization`,
`AccountKind`). Pointer + nil-as-absent matches the existing partial
contract and avoids zero-value confusion.

**Alternatives considered**:
- *Inline `Repo` fields on `Data.User`* — rejected because it would
  conflate two different entities and break the M2 partial contract
  (partials currently key on `Data.User != nil`).
- *Store under `Data.Plugins["repo"]`* — rejected because `Plugins` is
  the per-plugin output map, not the base entity layer; mixing them
  would invite ordering bugs.

## R-003: Per-plugin "repo-mode" switching mechanism

**Decision**: Each of the 7 reused plugins (`languages`, `projects`,
`stargazers`, `people`, `activity`, `contributors`, `sponsors`) gets
an early-branch check at the top of its `Compute(...)`:

```go
if pc.Data.Repo != nil {
    return computeRepoMode(ctx, pc)
}
return computeUserMode(ctx, pc)
```

Both `computeRepoMode` and `computeUserMode` are package-private
helpers. The repo-mode helper reads from `pc.Data.Repo` (R-002); the
user-mode helper retains the M4 behavior. The plugin's exported
`Compute` signature does **not** change; the `Plugin` interface in
`internal/plugins/plugin.go` is untouched.

**Rationale**: Per spec clarify Q2 Option A — switching at the plugin
level (not template level) ensures the partial dispatch order from
`assets/templates/repository/partials/_.json` produces semantically
correct output. Keeping the switch internal to each plugin avoids
adding a new flag to the `Plugin` interface (which would propagate
through every adopted plugin including the ones the repository
template doesn't use, e.g., `achievements` / `calendar` / `habits`).
The 14 non-repository plugins are unchanged.

**Alternatives considered**:
- *New `RepoModePlugin` interface* — rejected because it would require
  callers to type-assert and would duplicate the existing `Plugin`
  registration path. Internal branching is the simpler symmetric.
- *Template-level pre-filter that injects `data.Repo` as `data.User`*
  — rejected because it would silently break user-mode-only invariants
  (e.g., `data.User.Repositories.TotalCount` semantics differ from a
  single-repo count) and require careful unwinding in classic-template
  partials.

## R-004: `repo` input — top-level vs plugin-namespaced

**Decision**: Top-level input `repo` (env `INPUT_REPO`, CLI `--repo
<name>`, YAML `repo:` key in `--config`). Surfaced as a first-class
field on `action.CLIFlags` (`Repo string`) and on the merged
`Invocation.Inputs` map (key `"repo"`). The action.yml gains one new
input entry: `repo: { description: "Repository name when template is
'repository'", required: false, default: "" }`.

**Rationale**: Per spec clarify Q3 Option B. Upstream `q.repo` is a
top-level URL/query parameter, not under `plugins.`. Placing it at the
top level keeps the M6 `plugin_*` namespace pure (plugin-specific
inputs) and matches upstream input semantics. The M6 `--plugin
key=value` CLI flag stays unchanged.

**Alternatives considered**:
- *`plugin_repo` under the M6 plugin namespace* — rejected: would
  conflate a template-shaping input with plugin-specific inputs (the
  M6 contract). Also non-upstream-compatible.
- *`template_repo`* — rejected: more verbose and non-upstream.

## R-005: Template registration and partial ordering

**Decision**: `internal/templates/repository/repository.go` implements
the `templates.Template` interface (`Name() / Check() / Run() /
Metadata() / SupportedFormats()`). `Name()` returns `"repository"`.
`Check()` asserts:

1. `req.Account == AccountRepository`, otherwise return 406-equivalent
   error (in this project: `xerrors.NewInputError("template", ...)`
   for fail-fast).
2. `req.Inputs["repo"] != ""`, otherwise the same fail-fast error
   (FR-002).

`Run()` reads partial ordering from
`assets/templates/repository/partials/_.json` (already present in the
repo, untouched). It calls `engine.MergePartials(userOrder,
templateOrder)` (existing M2 helper) to honor `config_order`. The
intersection with the adopted 21 plugins yields the rendered set —
non-adopted partials (`pagespeed`, `posts`, `rss`, `screenshot`,
`stock`, `crypto`, `licenses`, `followup`) are silently skipped via
the M2 partial registry (registry lookup miss → "").

**Rationale**: Reuses the M2 template scaffolding (`classic.go` is
the reference implementation). Fail-fast in `Check()` rather than
later in `Run()` keeps the M6 `output_action` fail-fast contract.
Partial registry miss = "" is the M2 nil-safe contract — it's
already the way M4 plugins handle missing data.

**Alternatives considered**:
- *Inline the partial list in `repository.go`* — rejected: duplicates
  state already in `assets/templates/repository/partials/_.json`
  (single source of truth principle from M2).
- *Strict mode (error on unknown partial)* — rejected: would force
  M7 to commit to either adopting the missing plugins (Constitution
  III violation: M8 skipped) or modifying the upstream `_.json` (loss
  of source-of-truth parity).

## R-006: Validation timing for missing `repo` input

**Decision**: Validation lives in `internal/action/action.go::Run` /
`runCLIWith` — specifically, after input parsing (step 2) but
**before** building engine deps (step 5). When `inv.Template ==
"repository"` and `inv.Inputs["repo"] == ""`, return a typed
`*ConfigError` ("repository template requires --repo input") that
the M6 output_action wrapper recognizes as non-retryable, exit 1
without contacting the GitHub API.

**Rationale**: Per spec SC-003 (5-second exit limit + no API call).
M6's `runWith` already has this pattern for `output_action=gist`
validation (`TestRun_OutputAction_UnsupportedFailFast`). Reusing the
same scaffolding keeps the error type and exit semantics consistent.

**Alternatives considered**:
- *Validate inside `templates.Get("repository").Check(req)`* — rejected
  because Check runs *after* engine deps are built and the token
  validator has already issued `/rate_limit`. SC-003 caps total time
  at 5 seconds with zero API contact — Check-level validation would
  miss the budget.
- *Validate at `engine.Compute` entry* — rejected for the same reason
  + would require leaking the `repo`-input requirement into the engine
  layer (which today is template-agnostic).

## R-007: Test fixtures and golden file strategy

**Decision**: Two new golden files per format:

- `tests/golden/repository/octocat_hello-world.svg` — normalized via
  the existing `tests/integration/svg_normalize.NormalizeSVG` helper
  (attribute sort + whitespace collapse + dynamic footer mask), so
  the comparison tolerates Go-version / timestamp drift.
- `tests/golden/repository/octocat_hello-world.json` — direct byte
  compare via the M2 `output_json_test.go` pattern.

Mocked GraphQL fixture: `tests/fixtures/graphql/repository_hello_world.json`
returns canned responses for both the existing `User` query and the
new `Repository` query (mirrors the M6 integration test pattern that
selects fixtures by `operationName`).

**Rationale**: Reuses the existing M2 + M6 golden/fixture
infrastructure (`tests/golden/` directory layout, NormalizeSVG helper,
`-update` flag pattern). Keeps the M7 test surface familiar to
contributors.

**Alternatives considered**:
- *Skip golden file + only assert presence of repo name + community
  panel substrings* — rejected: spec SC-002 requires format-valid
  output across all 4 formats. Substring matching wouldn't catch DOM
  ordering regressions.
- *Reuse `tests/golden/classic/` files with template-name suffix* —
  rejected: separate sub-directory is the existing convention (cf.
  `tests/golden/classic/m4/<plugin>.svg`).

## R-008: Compliance test extension

**Decision**: Add a new test `TestCompliance_M7_TemplateInvariant`
to `tests/compliance/compliance_test.go`. It enumerates
`internal/templates/` and asserts the set of subdirectories equals
exactly `{classic, repository}` (excluding `partials/` which is a
sibling under each template). Any drift in either direction (missing
adopted template OR unadopted template landed) fails the test, paired
with the existing `TestCompliance_M4_AdoptedPlugins` invariant.

**Rationale**: Same defense-in-depth pattern as `TestCompliance_M4_AdoptedPlugins`
(M4 added it for plugins; M7 adds the template analogue). Without it,
a future PR could silently land an M8-scoped template (markdown /
terminal) and only be caught by manual review.

**Alternatives considered**:
- *Extend `TestCompliance_M6_NoNewPlugins` to also check
  `internal/templates/`* — rejected: that test's purpose is to assert
  zero plugins under M6 surfaces; conflating it with template gating
  would muddy the failure messages.
