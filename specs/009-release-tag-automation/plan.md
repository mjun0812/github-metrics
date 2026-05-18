# Implementation Plan: Release tag automation

**Branch**: `009-release-tag-automation` | **Date**: 2026-05-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-release-tag-automation/spec.md`

## Summary

The v1.0.0 release uncovered two operational gaps left by M10:

1. `release-binary` job's `gh release upload "${TAG}" *` assumes the
   GitHub Release entry exists; a fresh tag push fails with
   `release not found` until the maintainer runs `gh release create`
   manually + re-runs the failed job.
2. No `vX` floating tag exists, so consumers cannot pin to the
   GitHub-Actions community-convention `uses: <repo>@v1` shape and
   automatically receive patch / minor updates.

This feature closes both gaps by:

- Replacing the workflow's "upload to existing release" step with a
  single `gh release create … --generate-notes` invocation that
  creates the Release inline and attaches all assets in one shot
  (idempotent against re-runs via `gh release upload --clobber`).
- Adding a post-publish `update-floating-tag` job that force-updates
  the `vX` ref for stable releases only (pre-releases / older
  back-ports skip the advance per FR-004 / FR-005).
- Backfilling the `v1` ref for the already-published v1.0.0 as a
  one-time maintainer task documented in the quickstart.

Constitution III invariant unaffected: pure release-pipeline polish,
no plugin / template / output-format scope change.

## Technical Context

**Language/Version**: GitHub Actions YAML + bash for the release
workflow extension. No Go source code changes.

**Primary Dependencies**: existing only — `gh` CLI (already installed
on `ubuntu-latest`), `git` (already on runner). No new third-party
GitHub Actions are introduced; the `softprops/action-gh-release@v2`
alternative was considered and rejected (see R-001).

**Storage**: N/A. Pipeline-only feature.

**Testing**:

- Existing `make test` / `make lint` continue to pass on the M10
  baseline — no Go code touched.
- New integration test path: a dry-run `workflow_dispatch` from a
  branch confirms the new release-create + floating-tag steps run
  to completion without erroring (the workflow already supports
  `dry_run=true`; release-create is gated on the same flag).
- An end-to-end manual verification step lands in `quickstart.md`:
  push `v0.99.0-rc1` to a fork, observe Release auto-created +
  `v0` floating tag NOT updated (pre-release).

**Target Platform**: GitHub Actions `ubuntu-latest` runner (unchanged).

**Project Type**: Single-binary Go monorepo (unchanged).

**Performance Goals**: SC-007 — added steps must keep the total
release pipeline runtime ≤ 25 minutes (M10 SC-007 ceiling). Expected
overhead: `gh release create` ~3-5s; floating-tag force-push ~1-2s.
Net new wall-clock: ≤ 30 seconds.

**Constraints**:

- No change to action.yml emission (M10 contract preserved).
- Floating-tag scheme is **major-only** (`v1`, `v2`). Minor floating
  tags (`v1.0`, `v1.1`) deliberately out of scope per spec
  assumption.
- `contents: write` permission already in place; no permission scope
  changes required.
- Release-pipeline correctness verified via the existing dry-run
  workflow path; no new unit-test suite.

**Scale/Scope**: 1 file modified (`.github/workflows/release.yml` —
extend release-binary job + add a new post-release-tag-update job),
plus 3 documentation files updated per FR-007 (`README.md`,
`docs/migration-to-go.md`, `specs/008-m10-release-distribution/quickstart.md`).
Optional addition: 1 small helper file
(`scripts/update-floating-tag.sh`) if the bash step grows beyond
~30 lines. Total LOC delta: **~100 lines added / ~20 lines
modified**, no Go changes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. 入力互換性 (NON-NEGOTIABLE) | ✓ N/A | `action.yml` input matrix unchanged. No effect on `with:` semantics. |
| II. 出力契約 (DOM/JSON 単位) | ✓ N/A | No rendered output changes. SVG / JSON output identical for the same input. |
| III. スコープ規律 (採用機能のみ実装) | ✓ PASS | Pure release-pipeline polish. No plugin / template / output-format additions. The existing M10 `TestCompliance_M10_PluginTemplateInvariant` continues to gate the 21-plugin + 2-template set. |
| IV. テーブルテスト + Golden File | ✓ PASS (extended) | No rendering logic added. The release pipeline itself gets exercised via the existing `dry_run=true` workflow_dispatch path; the new release-create + floating-tag steps run inside that path so a single dry-run validates the change. |
| V. Go 規約と言語ポリシー | ✓ PASS | No Go source code added. Workflow YAML + bash + Japanese-language doc updates only. Existing gofumpt / golangci-lint baselines remain. |

**Result: ALL GATES PASS** — no Complexity Tracking entry needed.

## Project Structure

### Documentation (this feature)

```text
specs/009-release-tag-automation/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 — R-001..R-003 (release-create approach, floating-tag scheme, backfill ordering)
├── data-model.md        # Phase 1 — E-001..E-003 (Release page entity, major floating tag, backfill task)
├── quickstart.md        # Phase 1 — maintainer cut-a-release flow updated for the new auto-create path + v1 backfill recipe
├── contracts/
│   ├── release-workflow-delta.md   # Diff against M10 contracts/release-workflow.md (release-create + floating-tag steps + their dry-run gates)
│   └── floating-tag-policy.md      # SemVer → vX advance rules (FR-003 / FR-004 / FR-005 codified)
├── tasks.md             # Phase 2 (NOT created by /speckit-plan)
└── checklists/
    └── requirements.md  # 16/16 PASS (created in /speckit-specify)
```

### Source Code (repository root)

```text
.github/workflows/
└── release.yml                      # EXTENDED:
                                     #   - release-binary "Upload to GitHub Release" step
                                     #     replaced with `gh release create … --generate-notes
                                     #     <artifacts>` (creates the Release + uploads in one
                                     #     shot, idempotent via `gh release upload --clobber`
                                     #     on re-run)
                                     #   - NEW `update-floating-tag` job after release-docker
                                     #     + release-binary succeed; runs only on tag pushes
                                     #     and skips pre-release tags

scripts/
└── update-floating-tag.sh           # NEW (optional, only if bash grows >30 LOC):
                                     # one-shot helper invoked from the workflow with a
                                     # single positional argument (the just-pushed tag).
                                     # Parses semver, decides whether to advance `vMAJOR`,
                                     # and force-pushes via `git push --force` on the ref.

README.md                            # MODIFIED:
                                     # - Quick start example: bump `@v1.0.0` to `@v1` so
                                     #   consumers see the floating-tag pin
                                     # - Add a one-line note that `@v1` resolves to the
                                     #   latest v1.x.y release

docs/migration-to-go.md              # MODIFIED:
                                     # - Step 1 of "移行手順" uses `@v1` as the recommended
                                     #   pinning, mentions `@v1.0.0` as the exact-pin alternative

specs/008-m10-release-distribution/quickstart.md
                                     # MODIFIED:
                                     # - §2 "Tag the release" removes the manual
                                     #   `gh release create` instruction (the workflow
                                     #   handles it). The `--no-verify` action.yml-drift
                                     #   workaround remains.
                                     # - NEW §2a: "Back-fill the v1 floating tag" — one-time
                                     #   action for the existing v1.0.0 release.
```

**Structure Decision**: Single-binary Go monorepo (unchanged). This is
a workflow-only polish phase. The optional `scripts/update-floating-tag.sh`
helper materializes only if the inline bash exceeds 30 lines or if the
team wants to unit-test the SemVer parsing logic separately. Initial
implementation will use an inline `run:` block; the helper file is a
clean refactor path if growth occurs.

## Complexity Tracking

*No violations — all 5 constitution gates pass. This section is omitted
per template guidance.*
