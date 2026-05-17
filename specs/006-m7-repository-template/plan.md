# Implementation Plan: M7 — repository template

**Branch**: `006-repository-template` | **Date**: 2026-05-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-m7-repository-template/spec.md`

## Summary

M7 ships the **second** adopted template (`repository`) alongside the
existing `classic` template (M2). The template re-centers rendered SVG
on a single GitHub repository (`<owner>/<repo>`) instead of a user
profile. It introduces (a) a new top-level `repo` input (env
`INPUT_REPO`, CLI `--repo <name>`), (b) a new `base.repository(login,
repo)` GraphQL query, (c) a new `data.Repo` field on the plugin Data
envelope, and (d) per-plugin "repo-mode" switching for the 7 M4 plugins
that the upstream `repository` template lists. The M3 chromedp render
pipeline and the M6 committer remain template-agnostic; no new plugin
slug enters `internal/plugins/`.

## Technical Context

**Language/Version**: Go 1.26 (per `go.mod`; no toolchain bump)

**Primary Dependencies**: existing only — `github.com/Khan/genqlient`
(GraphQL codegen, used to add the new `base.repository` query),
`gopkg.in/yaml.v3` (metadata + partial _.json parsing — M2 baseline),
`github.com/chromedp/chromedp` (M3 render; unchanged), no new modules

**Storage**: N/A (the action is a stateless per-run computation; the
M6 committer writes back to the user's repo via the GitHub REST API,
which is unchanged)

**Testing**: existing harness — Go `testing` + table-driven tests,
golden files under `tests/golden/<template>/<case>.{svg,json}`,
httptest-backed integration suites in `tests/integration/`, compliance
guard in `tests/compliance/`. M7 adds golden files for the new
template (e.g. `tests/golden/repository/octocat_hello-world.svg`,
`.json`) and a new compliance test ensuring `internal/templates/` only
contains `classic` + `repository` (no surprise template slugs)

**Target Platform**: same as M6 — linux/amd64 (Docker image), and
linux/darwin × amd64/arm64 (release binaries)

**Project Type**: Go single-binary monorepo (`cmd/metrics-action/` is
the only shipped binary after M6; `cmd/metrics-cli/` is the legacy M1
stub)

**Performance Goals**: per spec SC-001 / SC-002 — repository template
SVG / PNG / JPEG / JSON each under **30 seconds** wall-clock against
mocked deps. The new `base.repository` GraphQL query adds **one**
network round-trip per run (issued once, before plugin dispatch). No
new chromedp render — PNG / JPEG reuse the existing M3 pipeline

**Constraints**: must continue to pass M1-M6 success criteria
(classic golden, banner snapshot, output_action variants, compliance
21-plugin invariant). The M3 chromedp render pipeline is treated as
template-agnostic and MUST NOT receive M7-specific branches

**Scale/Scope**: 1 new template + 1 new GraphQL query + 1 new input
schema entry + 1 new `data.Repo` struct + 7 plugins gain a per-plugin
`repo-mode` branch. Estimated LOC: ~1500-2000 (template + partials
~600, query + genqlient regen ~200, plugin repo-mode branches ~400,
tests ~600)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. 入力互換性 (NON-NEGOTIABLE) | ✓ PASS | New `repo` input mirrors upstream `q.repo` exactly (top-level, not under `plugins.`). action.yml extension is additive — no existing key renamed or removed. metadata.yml-derived `presets` precedence unchanged |
| II. 出力契約 (DOM/JSON 単位) | ✓ PASS | repository template emits the same partial ordering as upstream `assets/templates/repository/partials/_.json` (intersected with adopted 21). JSON adds `data.repo` field upstream-compatible (matches `data.repo` shape from `template.mjs`). SVG/DOM match via partial-by-partial parity |
| III. スコープ規律 (採用機能のみ実装) | ✓ PASS | T-089 is explicitly in `docs/design/16-tasks-mvp.md` adopted phase M7. The 7 reused plugins are all already in M4 adopted 21. Zero new plugin slug under `internal/plugins/`. No template touched beyond `repository` (M5/M8 still skipped) |
| IV. テーブルテスト + Golden File | ✓ PASS | Plan adds golden files (`tests/golden/repository/<case>.{svg,json}`) + table tests for the new partials. Each repo-mode plugin gains a table test pair (user-mode + repo-mode). Follows the `-update` flag pattern from M2 |
| V. Go 規約と言語ポリシー | ✓ PASS | gofumpt / golangci-lint / go-vet / govulncheck unchanged. Per CLAUDE.md コミットメッセージは Conventional Commits, 日本語 docstring 維持 |

**Result: ALL GATES PASS** — no Complexity Tracking entry needed.

## Project Structure

### Documentation (this feature)

```text
specs/006-m7-repository-template/
├── plan.md              # This file
├── research.md          # Phase 0 — R-001..R-005 (GraphQL query design, repo-mode mechanism, etc.)
├── data-model.md        # Phase 1 — E-001..E-005 (Template registry entry, engine.Request.Repo, data.Repo, repo-mode flag, output paths)
├── quickstart.md        # Phase 1 — local + Action examples + 4-format dryrun
├── contracts/
│   ├── repository-template.md  # SVG partial ordering + JSON shape
│   ├── base-repository-query.md # GraphQL query schema + fixture format
│   ├── repo-input.md           # action.yml + CLI input schema
│   └── repo-mode-plugin.md     # per-plugin repo-mode switching contract
├── tasks.md             # Phase 2 (NOT created by /speckit-plan)
└── checklists/
    └── requirements.md   # 16/16 PASS (created in /speckit-specify)
```

### Source Code (repository root)

```text
cmd/metrics-action/
├── main.go              # No change — RunCLI / Run already template-agnostic
└── plugins.go           # Add side-effect import: _ "internal/templates/repository"

internal/
├── action/
│   ├── action.go        # Add validation: template==repository → require repo input
│   ├── cli.go           # Add --repo flag + CLIFlags.Repo field + ToInvocation wire
│   ├── inputs.go        # Recognize top-level `repo` key alongside `user` / `template`
│   └── *_test.go        # New tests for --repo flag + repo-template validation
│
├── githubapi/
│   ├── queries/
│   │   └── repository.graphql       # NEW: base.repository(login, repo)
│   └── graphql_repository.go        # NEW: genqlient-generated client (auto, drift-guarded)
│
├── plugins/
│   ├── data.go          # Add `Repo *Repo` field; document repo-mode contract
│   ├── repo.go          # NEW: `type Repo struct` (owner, name, description, stargazers, forks, contributors, activity, license, defaultBranch, sponsorshipsAsMaintainer)
│   ├── base/
│   │   ├── base.go      # Branch: when Account==Repository → call new query, populate data.Repo
│   │   └── repository.go # NEW: extracted single-repo fetch helper
│   ├── activity/        # Add IfRepoMode branch — read data.Repo.Activity instead of data.User.Activity
│   ├── contributors/    # Add IfRepoMode branch — already repo-centric; minor refactor
│   ├── languages/       # Add IfRepoMode branch — analyze data.Repo only
│   ├── people/          # Add IfRepoMode branch
│   ├── projects/        # Add IfRepoMode branch
│   ├── sponsors/        # Add IfRepoMode branch
│   └── stargazers/      # Add IfRepoMode branch
│
└── templates/
    ├── template.go      # Already exposes Register/Get; no change
    ├── classic/         # No change
    └── repository/      # NEW
        ├── repository.go    # templates.Template impl (Check rejects account!=repository)
        ├── repository_test.go
        └── partials/
            ├── partials.go      # Per-partial render funcs (base.repository, base.community, base.activity, introduction)
            ├── partials_test.go
            └── plugins.go        # Re-export hooks to plugin-partial registry (mirrors classic/partials/plugins.go)

internal/tools/gen-action-yml/
└── main.go              # Add `repo` to the synthesized top-level inputs alongside `user`

assets/templates/repository/  # Existing — unchanged (drives metadata + partial ordering)

tests/
├── golden/
│   └── repository/      # NEW: octocat_hello-world.svg / .json snapshots
├── integration/
│   ├── output_test.go   # Extend with repository-template cases
│   └── cli_test.go      # Add --repo flag pair to equivalence test
└── compliance/
    └── compliance_test.go # Add TestCompliance_M7_TemplateInvariant: only {classic, repository} subdirs in internal/templates/
```

**Structure Decision**: Single-binary Go monorepo (no change from M6).
The plan reuses every cross-cutting M6 surface (Action / CLI dispatch,
committer, retry policy, output_action) and adds only **template-shaped**
code paths: one new template package, one new GraphQL query, one new
top-level input. The 7 repo-mode plugin changes are deliberately
*internal* (a `repo-mode` flag inside each plugin's Compute path) so
no new plugin slug lands and the constitution III invariant holds.

## Complexity Tracking

*No violations — all 5 constitution gates pass. This section is omitted
per template guidance.*
