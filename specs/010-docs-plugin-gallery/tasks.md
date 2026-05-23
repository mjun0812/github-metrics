---

description: "Per-plugin docs with real example outputs — task list"
---

# Tasks: Per-plugin docs with real example outputs

**Input**: Design documents from `/specs/010-docs-plugin-gallery/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks include the new `gen-plugin-docs` tool's
table-driven unit tests (FR-007) and the new compliance test (FR-009).
The SVG samples themselves are not golden-tested — they are
maintainer-regenerated artifacts vendored under `docs/examples/`.

**Organization**: Tasks are grouped by user story so each story can
ship independently. US1 (hero on README) is the MVP slice; US2
(per-plugin docs + gallery) and US3 (regeneration flow) build on it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User-story label (US1 / US2 / US3) — only on
  user-story phase tasks
- **OPTIONAL**: marker noting tasks that may ship later per FR-010
  (loose TODO gating)
- Include exact file paths in descriptions

## Path Conventions

- `internal/tools/<tool-name>/` — Go generator tools (mirrors the
  existing `gen-action-yml` pattern)
- `scripts/` — shell helpers
- `docs/plugins/<slug>.md` — per-plugin doc pages
- `docs/examples/` — vendored SVG samples
- Plan path: `specs/010-docs-plugin-gallery/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Directory structure for the new artifacts.

- [ ] T001 Create empty directories `docs/plugins/` and `docs/examples/`. Add `.gitkeep` to both so the directories survive an empty initial commit. No content yet — content is created by the per-story tasks below.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: New tools and Makefile targets that every user story
phase depends on. Must complete before US1 / US2 / US3 work begins.

- [ ] T002 Create `internal/tools/normalize-svg-stream/main.go`: a thin Go tool that reads stdin, calls `internal/testutil/golden.NormalizeSVG`, writes stdout. ~30 LOC; per `contracts/sample-generation.md` §5. Includes `main_test.go` table-driven test that exercises a fixture SVG (with dynamic strings) and asserts the output masks them.
- [ ] T003 [P] Create `internal/tools/gen-plugin-docs/main.go`: the markdown generator described in `contracts/plugin-doc-template.md` and `contracts/readme-gallery.md`. Parses `assets/plugins/<slug>/metadata.yml` for the 19 adopted slugs (mirror of `tests/compliance/compliance_test.go::adoptedM4Plugins` minus base/core), emits docs/plugins/<slug>.md skeletons + refreshes README hero + plugins-gallery AUTOGEN blocks. Auto-skips the gallery block if any per-plugin SVG file is missing (allows US1 to ship hero alone). Hardcoded slug → tier mapping (P1 MVP / P2 GraphQL+REST / P3 chromedp) so each plugin doc shows its tier badge per Q1 clarification. ~250 LOC. Includes `main_test.go` with: (a) round-trip idempotency test (regenerating an already-generated file yields zero diff), (b) marker preservation test (human-authored zones survive verbatim), (c) metadata.yml input → config table row parity test (every input renders as a row).
- [ ] T004 [P] Create `scripts/gen-doc-samples.sh`: bash script implementing the invocation matrix from `contracts/sample-generation.md` §2-§3. Atomic-write per sample (`.tmp` → `mv`). Validates pre-conditions (GITHUB_TOKEN set, chromium available) and exits with actionable error if missing. Generates 21 per-plugin SVGs (19 single-panel + 2 sub-modes for languages) + 2 hero SVGs.
- [ ] T005 Add 4 Makefile targets per `quickstart.md` §1-§3:
  - `docs`: runs `go run ./internal/tools/gen-plugin-docs` (cheap, no token)
  - `docs-samples`: runs `bash scripts/gen-doc-samples.sh` (requires token + chromium)
  - `docs-examples`: umbrella running both in order (`docs-samples → docs`)
  - `docs-lint`: prints a TODO marker count per `docs/plugins/<slug>.md` file (FR-010 loose gating; non-blocking)

**Checkpoint**: Tools + script + Makefile targets in place. US1 / US2 / US3 implementation work can now begin in parallel.

---

## Phase 3: User Story 1 — README hero images (Priority: P1) 🎯 MVP

**Goal**: First-time visitors opening README.md see two rendered hero images (classic + repository templates) within 30 seconds.

**Independent Test**: Open README.md on github.com (after merge). Within 30 seconds, the viewer sees two rendered SVGs showing the project's actual output. Per spec SC-001.

### Implementation for User Story 1

- [ ] T006 [US1] Run `make docs-samples` with `METRICS_DOC_USER=mjun0812` and a configured `GITHUB_TOKEN` + `METRICS_CHROME_PATH`. Confirm `docs/examples/hero-classic.svg` and `docs/examples/hero-repository.svg` are produced (the script always emits the 2 heroes plus the per-plugin set; US1 commits only the 2 heroes here, and US2's T008 confirms the per-plugin set). Commit the 2 hero SVGs to `docs/examples/`.
- [ ] T007 [US1] Run `make docs`. The generator (a) inserts the hero AUTOGEN markers into README.md at the canonical anchor (immediately after the project description, before the Highlights section), (b) populates the hero block referencing both hero SVGs, (c) does NOT emit the plugins-gallery block (no per-plugin samples committed yet — auto-skip per T003 design). Commit the README diff.

**Checkpoint**: US1 alone is mergeable. A reader opening README sees two hero images. No per-plugin docs or gallery yet — that comes in US2.

---

## Phase 4: User Story 2 — Per-plugin docs + plugins gallery (Priority: P2)

**Goal**: Each adopted plugin has a dedicated doc page with sample + configuration spec; README plugins gallery replaces the old tier table per Q1 clarification.

**Independent Test**: Open `docs/plugins/languages.md` on github.com. Within 5 minutes, the reader can identify (a) the rendered languages output (image visible), (b) the full input list (table), (c) how to enable + configure the plugin (usage snippet). Per spec SC-002, SC-003, SC-005.

### Implementation for User Story 2

- [ ] T008 [P] [US2] (If T006's `make docs-samples` run did not already produce them) generate the 21 per-plugin sample SVGs (19 single-panel slugs + `plugin-languages-recent.svg` + `plugin-languages-indepth.svg`) by running the per-plugin section of `scripts/gen-doc-samples.sh`. Commit all 21 SVGs to `docs/examples/`.
- [ ] T009 [US2] Run `make docs` (second invocation). Generator now (a) refreshes the hero block (idempotent re-run), (b) emits the plugins-gallery AUTOGEN block at the existing `## Plugins` section position, **replacing** the current tier-based markdown table (per Q1 clarification — gallery becomes the sole plugin index; tier info moves into per-plugin docs), (c) generates 19 `docs/plugins/<slug>.md` skeletons (each containing the title + description + sample image embed + tier badge + config table + usage snippet sections per `contracts/plugin-doc-template.md`). Commit all 19 .md files + the README diff.
- [ ] T010 [P] [US2] OPTIONAL per FR-010 (loose initial gating). Fill in the human-authored zones (`<!-- TODO: ... -->` blocks for "このプラグインを使うべきケース" and "既知の制約 / 注意点") in the **5 MVP-tier plugin docs**: `docs/plugins/achievements.md`, `docs/plugins/activity.md`, `docs/plugins/languages.md`, `docs/plugins/repositories.md`, `docs/plugins/isocalendar.md`. Each plugin needs 1-2 paragraphs of context per zone. If skipped, the docs ship with TODO placeholders that `make docs-lint` surfaces as a warning.
- [ ] T011 [P] [US2] OPTIONAL per FR-010. Fill human-authored zones for the **12 GraphQL+REST-tier plugin docs**: `docs/plugins/{calendar,habits,stars,people,notable,contributors,reactions,projects,sponsors,sponsorships,stargazers,traffic}.md`.
- [ ] T012 [P] [US2] OPTIONAL per FR-010. Fill human-authored zones for the **2 chromedp-tier plugin docs**: `docs/plugins/topics.md`, `docs/plugins/starlists.md`. Note: these docs should call out the chromium runtime requirement explicitly in the "既知の制約" zone.

**Checkpoint**: US1 + US2 complete. README has hero + gallery, all 19 plugin docs exist with sample images, and the gallery thumbnails resolve correctly on github.com. T010-T012 may ship as TODOs and be filled in over time.

---

## Phase 5: User Story 3 — Maintainer regeneration flow (Priority: P3)

**Goal**: Documented maintainer flow for regenerating the example set after a rendering or metadata change. End-to-end runnable from a clean checkout.

**Independent Test**: A fresh maintainer clones the repo, follows the steps in `CONTRIBUTING.md` (chromium install, token setup), runs `make docs-examples`, and produces a byte-stable diff (zero diff if no semantic change). Per spec SC-004, SC-006.

### Implementation for User Story 3

- [ ] T013 [US3] Add a new section to `CONTRIBUTING.md` titled "Regenerating plugin docs + example SVGs": documents the prerequisites (chromium, GITHUB_TOKEN, recommended scopes per plugin), the 3 make targets (`docs` / `docs-samples` / `docs-examples`) with their cost / token requirements, the `docs-lint` warning target, and the v1.x roadmap to strict-mode TODO gating per Q2 clarification. Cross-link to `specs/010-docs-plugin-gallery/quickstart.md` for the long-form maintainer flow.
- [ ] T014 [US3] From a clean checkout (or `git clean -fdx` reset), with `GITHUB_TOKEN` + `METRICS_CHROME_PATH` set, run `make docs-examples`. Confirm: (a) all 21 SVGs are regenerated, (b) all 19 .md files are refreshed, (c) README.md AUTOGEN blocks are refreshed, (d) total runtime ≤ 5 min per SC-004, (e) `git diff` against committed state is empty when no upstream-data or code change happened (idempotency).

**Checkpoint**: US3 complete. The regeneration flow is documented and exercised end-to-end.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Compliance test, lint warning, full regression, post-merge visual review.

- [ ] T015 Add `TestCompliance_DocsPluginPagesMatchAdoptedSet` to `tests/compliance/compliance_test.go` per FR-009: enumerate `docs/plugins/*.md`, strip the `.md` suffix, compare to the canonical 19-plugin set (`adoptedM4Plugins` minus base/core). Failure messages: "docs/plugins/X.md exists for unadopted plugin X" (extra) and "adopted plugin X has no docs/plugins/X.md page" (missing). The test runs in the existing `compliance` CI job.
- [ ] T016 [P] Implement `make docs-lint` (T005 wired the target name) as a bash one-liner that runs `grep -lr '<!-- TODO' docs/plugins/` and prints a summary count. Exit code 0 regardless (loose gating per FR-010). The maintainer reviews the warning summary as a quality signal but the lint does not block PRs.
- [ ] T017 Run full local regression: `make test` + `make lint` + `make hooks-run`. Confirm: (a) M1-M10 baseline tests stay green, (b) `TestCompliance_DocsPluginPagesMatchAdoptedSet` passes, (c) `gen-plugin-docs/main_test.go` + `normalize-svg-stream/main_test.go` pass, (d) lefthook hooks all skip cleanly (no Go source changes outside `internal/tools/`).
- [ ] T018 After PR push + merge, visually verify on github.com: (a) README opens with both hero images rendered above the fold, (b) clicking 1-2 gallery thumbnails resolves to the matching `docs/plugins/<slug>.md` page, (c) each visited plugin doc shows the sample image rendered correctly, (d) the gallery has exactly 19 thumbnails matching the adopted set (no broken icons unless T010-T012 partial), (e) the config table on at least 2 plugin docs lists all inputs declared in `metadata.yml`. Record observations in the PR description.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)** has no dependencies — can start immediately.
- **Phase 2 (Foundational)** depends on Phase 1 completion. Tasks T002 / T003 / T004 / T005 are file-disjoint and partially parallelizable (T002 ∥ T003 ∥ T004; T005 sequential after T002-T004 since the Makefile references all three).
- **Phase 3 (US1)** depends on Phase 2 (uses scripts/gen-doc-samples.sh + gen-plugin-docs).
- **Phase 4 (US2)** depends on Phase 2. US2 can start in parallel with US1 if staffed (file-disjoint within `docs/`).
- **Phase 5 (US3)** depends on Phase 2 + a working US1 or US2 to exercise the maintainer flow.
- **Phase 6 (Polish)** depends on US1 + US2 implementations being in place (the compliance test gates the actual `docs/plugins/*.md` set).

### User Story Dependencies

- **US1 (P1)** — Independent. Ships hero alone if needed.
- **US2 (P2)** — Independent of US1 implementation but functionally complementary (README has both blocks).
- **US3 (P3)** — Documents the flow US1/US2 already produce; can be drafted in parallel with them.

### Within Each User Story

- **US1**: T006 (samples) → T007 (run generator). Sequential.
- **US2**: T008 (samples — may overlap with T006 if both run via `make docs-samples`) → T009 (generator). T010-T012 are P-tagged and OPTIONAL per FR-010; can run after T009 or be deferred entirely.
- **US3**: T013 (write docs) → T014 (e2e verify). Sequential.

### Parallel Opportunities

- T002 ∥ T003 ∥ T004 (Foundational tool authoring, disjoint files)
- T008 ∥ T010 ∥ T011 ∥ T012 (sample generation, then doc-page authoring — disjoint files)
- US1 ∥ US2 (different parts of README + different file sets in docs/)
- T015 ∥ T016 (Polish — different files)

---

## Parallel Example: Foundational phase

```bash
# Phase 2: 3 disjoint files can be authored in parallel.
Task: "Create internal/tools/normalize-svg-stream/main.go + test"  # T002
Task: "Create internal/tools/gen-plugin-docs/main.go + tests"      # T003
Task: "Create scripts/gen-doc-samples.sh"                          # T004

# After T002-T004, wire Makefile targets:
Task: "Add docs / docs-samples / docs-examples / docs-lint targets to Makefile"  # T005
```

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1 (T001) + Phase 2 (T002-T005).
2. Complete Phase 3 (T006-T007): hero SVGs + README hero block.
3. **STOP and VALIDATE**: open README on the PR branch on github.com — both hero images render above the fold.
4. Ship US1 alone — value: first-time visitors finally see what the
   project produces. US2 + US3 land in a follow-up.

### Recommended Incremental Delivery

1. Phase 1 + 2 — tooling foundation (~250 LOC + scripts).
2. Phase 3 (US1) — hero on README. Ship as MVP.
3. Phase 4 (US2) T008 + T009 — per-plugin docs skeletons +
   gallery. Ship the structure even with TODO placeholders.
4. T010-T012 — fill in human-authored zones over multiple PRs
   (per FR-010 loose gating).
5. Phase 5 (US3) — maintenance docs + e2e verify.
6. Phase 6 (Polish) — compliance test + lint + regression +
   visual review.

This pattern matches the M9 (consolidation) and M10 (release)
approach: ship core machinery first, then iterate.

### Parallel Team Strategy

- Developer A: US1 (hero + README hero block).
- Developer B: US2 T008-T009 (samples + gallery + skeleton docs).
- Developer C: T013-T014 (US3 docs + e2e verify) + Polish T015-T016.

T010-T012 (human-authored prose) are best authored by the project
maintainer (mjun0812) since plugin-specific value framing benefits
from project expertise.

---

## Notes

- **No production Go code changes**: this feature adds 2 new tools
  under `internal/tools/`, a shell script, Makefile targets, doc
  pages, and SVG samples. Existing `cmd/metrics-cli` is invoked
  as-is.
- **No new permission scopes** at the CI level (the new compliance
  test runs in the existing `compliance` job).
- **TODO gating policy**: loose for v1 of this feature per Q2
  clarification (FR-010). Strict mode planned for v1.x once
  human-authored zones are fully populated.
- **Per Q1 clarification**: the existing README tier table is
  **replaced** by the gallery; tier info migrates into each
  per-plugin doc's title badge.
- **Constitution III invariant**: `TestCompliance_M10_PluginTemplateInvariant`
  + the new `TestCompliance_DocsPluginPagesMatchAdoptedSet` form a
  belt-and-suspenders gate against plugin / doc-page set drift.
- **OPTIONAL tasks T010-T012**: their non-completion does NOT block
  PR merge or the compliance test. They are tracked as the v1.x
  roadmap to strict-mode TODO gating.
