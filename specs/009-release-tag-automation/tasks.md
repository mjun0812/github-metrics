---

description: "Release tag automation — task list"
---

# Tasks: Release tag automation

**Input**: Design documents from `/specs/009-release-tag-automation/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: This feature is workflow-only (release.yml + docs + a one-time
maintainer backfill). No Go source code changes; no new unit tests
required. The release pipeline itself is validated via the existing
`dry_run=true` workflow_dispatch path (T007) and the eventual real
release (T008, deferred to post-merge).

**Organization**: Tasks are grouped by user story so each story can
ship independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `.github/workflows/`, `scripts/`, `README.md`,
  `docs/`, `specs/` at repository root (per plan.md "Project Structure").

---

## Phase 1: Setup (Shared Infrastructure)

*Not needed for this feature.* The release-tag-automation work is a
delta on the existing M10 release pipeline; no new dependencies,
scripts, or directories are introduced before the per-story work begins.
Skip to Phase 3.

---

## Phase 2: Foundational (Blocking Prerequisites)

*Not needed for this feature.* US1 and US2 are file-disjoint changes
to `release.yml` and can be implemented in either order. There is no
shared infrastructure they both depend on.

---

## Phase 3: User Story 1 — Maintainer cuts a release in one push (Priority: P1) 🎯 MVP

**Goal**: A `git push origin vX.Y.Z` triggers the release pipeline and
publishes the GitHub Release (with auto-generated notes + all 20
assets attached) in one shot — no manual `gh release create` step.

**Independent Test**: From a fork or branch, push `v0.99.0-rc1`. The
release workflow run completes green and a public Release page exists
at `https://github.com/<fork>/github-metrics/releases/tag/v0.99.0-rc1`
with all 20 assets attached, no operator intervention needed. Per
spec SC-001 / SC-004.

### Implementation for User Story 1

- [ ] T001 [US1] Refactor `.github/workflows/release.yml` `release-binary` job — "Upload to GitHub Release (production)" step. Replace the unconditional `gh release upload "${TAG}" *` with a `gh release view "${TAG}" >/dev/null 2>&1`-gated conditional: existing release → `gh release upload "${TAG}" * --clobber`; missing release → `gh release create "${TAG}" * --generate-notes --title "${TAG}"`. The single-command create-with-files variant uploads all 20 assets in one shot. Keep the surrounding `if: ${{ github.event.inputs.dry_run != 'true' }}` gate untouched. Reference `contracts/release-workflow-delta.md` §1.

**Checkpoint**: US1 is fully implemented and independently testable
via a real tag push (dry-run does NOT exercise this step — it is
gated behind the `dry_run != 'true'` condition).

---

## Phase 4: User Story 2 — Consumer pins to a major floating tag (Priority: P2)

**Goal**: After publishing a stable `vX.Y.Z`, the `vX` floating tag is
auto-advanced to the same commit. Pre-releases and back-ports are
skipped per FR-004 / FR-005. Consumers can `uses: <repo>@v1` and
receive patches automatically.

**Independent Test**: After a stable `v1.0.1` release runs through
the workflow, `git ls-remote origin refs/tags/v1` resolves to the
same SHA as `refs/tags/v1.0.1`. Push `v1.1.0-rc1` next; verify `v1`
stays on the `v1.0.1` SHA. Per spec SC-002 / SC-003 / SC-006.

### Implementation for User Story 2

- [ ] T002 [US2] Add new job `update-floating-tag` to `.github/workflows/release.yml` after the existing `release-docker` and `release-binary` jobs (`needs: [release-docker, release-binary]`). The job MUST: (a) gate on `github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v') && github.event.inputs.dry_run != 'true'`; (b) parse the tag into MAJOR/MINOR/PATCH + optional pre-release suffix via bash regex; (c) skip silently when a pre-release suffix is present (FR-004); (d) when `vMAJOR` ref already exists on `origin`, compare the new tag against the current target using `sort -V` and skip silently if older (FR-005 back-port); (e) on advance, run `git tag -f vMAJOR ${{ github.sha }} && git push origin --force refs/tags/vMAJOR`. Reference `contracts/release-workflow-delta.md` §2 and `contracts/floating-tag-policy.md` §2-§3 for exact decision-tree semantics.

**Checkpoint**: US2 is implemented. Validation requires a real stable
release tag push; dry-run does NOT exercise this job (the same
`dry_run != 'true'` gate applies).

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates (FR-007), one-time `v1` backfill
recipe (FR-008), regression validation, and dry-run smoke.

- [ ] T003 [P] Update `README.md` Quick start example: change the
  `uses: mjun0812/github-metrics@v1.0.0` line to
  `uses: mjun0812/github-metrics@v1` and add a one-line note that
  `@v1` resolves to the latest v1.x.y release (forward-compatible
  patches without consumer-side edits).
- [ ] T004 [P] Update `docs/migration-to-go.md` §5 "移行手順" Step 1:
  change the recommended `uses:` line from `@v1.0.0` to `@v1`;
  document `@v1.0.0` as the exact-pin alternative for consumers who
  want immutable references.
- [ ] T005 [P] Update `specs/008-m10-release-distribution/quickstart.md`
  §2 "Tag the release": remove the manual `gh release create`
  instruction (the workflow now handles it via T001). Add a NEW §2a
  "Back-fill the v1 floating tag" — one-time recipe for the existing
  v1.0.0 release: `git tag -f v1 v1.0.0 && git push origin refs/tags/v1`.
  Reference `contracts/floating-tag-policy.md` §5.
- [ ] T006 Run the full regression locally — `make test` + `make lint`
  + `make hooks-run`. M9 / M10 baselines stay green; the M10
  compliance invariant test continues to pass (constitution III).
- [ ] T007 Push the `009-release-tag-automation` branch and trigger the
  release pipeline in dry-run mode via `gh workflow run release.yml
  --ref 009-release-tag-automation -f dry_run=true`. The
  `release-binary` upload step skips entirely (gated by
  `dry_run != 'true'`), but YAML parse + step graph validate. The
  `update-floating-tag` job also skips by the same gate. Confirm the
  run completes green within the 25-min budget.
- [ ] T008 (DEFERRED to post-merge) After the PR merges to `main`,
  perform the one-time `v1` backfill per the new §2a recipe:
  `git tag -f v1 v1.0.0 && git push origin refs/tags/v1`. Then cut
  the next release (`v1.0.1` or similar) to validate end-to-end:
  the auto-create path lands the Release; the `update-floating-tag`
  job advances `v1` to the new commit. Both can be verified via
  `scripts/release-verify.sh v1.0.1` + `git ls-remote origin
  refs/tags/v1`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 3 (US1) and Phase 4 (US2)** are file-disjoint within
  `release.yml`: US1 modifies the `release-binary` job's Upload
  step; US2 adds a new top-level job. Both can be implemented in
  parallel by different developers if staffed; in practice one
  developer takes both in the same edit pass.
- **Phase 5 (Polish)** depends on US1 + US2 being decided (the
  documentation references the new behavior). T003-T005 are
  file-disjoint among themselves and can run in parallel. T006-T007
  are sequential (regression then dry-run). T008 is post-merge.

### User Story Dependencies

- **US1 (P1)**: Independent. Ships the auto-create fix that v1.0.0
  hit; can be released as v1.0.1 alone if US2 is deferred.
- **US2 (P2)**: Independent of US1 in terms of code, but the
  recommended pinning convention (`@v1`) advertised in the docs
  assumes both stories ship together. Order does not matter.

### Parallel Opportunities

- T003 ∥ T004 ∥ T005 (three different doc files)
- US1 (T001) ∥ US2 (T002) — file-disjoint within `release.yml` but
  practically edited in the same pass

### Within Each User Story

- **US1**: Single task (T001). No internal ordering.
- **US2**: Single task (T002). No internal ordering.

---

## Parallel Example: Polish phase docs

```bash
# T003-T005 are file-disjoint, can run together:
Task: "Update README.md @v1.0.0 → @v1 in Quick start"          # T003
Task: "Update docs/migration-to-go.md §5 step 1 to @v1"        # T004
Task: "Update M10 quickstart §2 + add §2a v1 backfill recipe"  # T005
```

---

## Implementation Strategy

### MVP First (US1 alone)

1. Complete Phase 3: T001 (release.yml refactor).
2. Skip Phase 4 / 5 / 6 for the moment.
3. Cut v1.0.1 — observe that release auto-creates without manual
   `gh release create`.
4. This MVP alone resolves the v1.0.0 release pain point. US2 +
   docs are quality-of-life improvements that can ship in v1.0.2.

### Incremental Delivery

1. US1 (T001) — auto-create path.
2. US2 (T002) — floating-tag advance.
3. Polish (T003-T007) — docs + dry-run validation.
4. Post-merge (T008) — backfill + first real verification release.

This is the recommended ordering for the 009 PR — single PR, all
three phases bundled, since each is a small change touching a
distinct concern.

---

## Notes

- **No Go code changes**. Only YAML (`release.yml`), Markdown
  (README, docs, M10 quickstart), and an optional `scripts/update-
  floating-tag.sh` if T002's inline bash grows beyond 30 lines (per
  plan.md "Project Structure"). Initial implementation uses inline
  bash; promote to a script only on growth pressure.
- **No new permissions** needed. `contents: write` (M10 baseline)
  covers both Release management and floating-tag force-push.
- **Constitution III invariant**: `TestCompliance_M10_PluginTemplateInvariant`
  continues to pass — no plugin / template / output-format scope
  change.
- **Verification limits in dry-run**: the existing `dry_run != 'true'`
  gate on the upload step means the T001 refactor is exercised
  end-to-end only on real tag pushes. T007's dry-run only validates
  YAML parse + step graph; full validation requires T008.
- **T008 is post-merge** and intentionally deferred — it requires the
  PR to be on `main` so `git tag -f v1 v1.0.0` operates against the
  authoritative ref state.
