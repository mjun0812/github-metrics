---

description: "Plugin rendering parity with upstream EJS templates — task list"
---

# Tasks: Plugin rendering parity with upstream EJS templates

**Input**: Design documents from `/specs/011-plugin-rendering-parity/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: This feature mandates a new visual-regression test layer (FR-004, FR-009). Per-plugin table tests + byte goldens (existing M9 infrastructure) are also extended in each per-plugin task.

**Organization**: Tasks are grouped by user story so each story can ship independently. US1 (languages pilot) validates the approach + builds the shared infrastructure; US2 sweeps the remaining 18 plugins (1 per task = 1 PR); US3 wires CI + adds the regression-prevention compliance gate.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User-story label (US1 / US2 / US3) — only on user-story phase tasks
- Include exact file paths in descriptions

## Path Conventions

- `internal/plugins/<slug>/partial.go` — per-plugin source (mutated)
- `tests/visual/<slug>_test.go` — per-plugin visual test (new)
- `tests/visual/visual_test.go` — shared chromedp harness (new)
- `tests/golden/classic/m4/<slug>.svg` — re-baselined byte goldens
- `specs/011-plugin-rendering-parity/plugins/<slug>.md` — per-plugin parity checklist
- `specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-{before,after}.png` — visual proof
- `scripts/capture-plugin-screenshot.sh` — screenshot helper (new)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Directories and the screenshot helper that every per-plugin PR depends on.

- [ ] T001 Verify `specs/011-plugin-rendering-parity/plugins/screenshots/` exists (created in plan phase). Confirm `.gitkeep` if directory is otherwise empty; no further action — directory is populated by per-plugin tasks below.
- [ ] T002 Create `scripts/capture-plugin-screenshot.sh`: bash helper that takes `<slug>` + `<before|after>` arguments, renders the plugin's SVG via the local docker image (`github-metrics:local`), opens it via `file://` HTML wrapper in headless Chromium, captures a 720x800 PNG, writes to `specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-{before,after}.png`. Validates `GITHUB_TOKEN` + docker image presence with actionable errors. ~60 LOC.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The shared visual-test harness all per-plugin visual tests depend on. Must complete before US1 begins.

- [ ] T003 Create `tests/visual/visual_test.go` per [contracts/visual-test-shape.md §1](./contracts/visual-test-shape.md#1-testsvisualvisual_testgo-shared-harness-us1-deliverable): `TestMain` bootstrap that skips the whole suite cleanly if chromium is unavailable, `renderForVisualTest(t, plugin, inputs) string` helper that invokes `internal/action.Run` in dryrun mode, `evalInBrowser(t, svg, jsExpr) any` that wraps the SVG in `<img src="data:image/svg+xml;base64,...">` HTML and runs `chromedp.Evaluate`, plus 3 convenience helpers (`assertElementExists`, `assertBoundingBoxNonZero`, `assertTextContent`). Recycles a single `*render.Browser` instance across all tests per M3 R-002 pattern. ~180 LOC including helpers.

**Checkpoint**: T002 + T003 in place. US1 (and subsequently US2) can now begin.

---

## Phase 3: User Story 1 — Languages plugin pilot (Priority: P1) 🎯 MVP

**Goal**: Languages plugin renders at upstream classic-template parity (header / `<svg>`-wrapped bar / Most-used + Recently-used sections / indepth file-size estimation summary / per-language icon).

**Independent Test**: Build the CLI, run `metrics-cli --user mjun0812 --plugin plugin_languages=yes --plugin plugin_languages_indepth=yes --plugin plugin_languages_sections=most-used,recently-used --output svg --filename /tmp/out.svg --dryrun`, open via `<img src="file:///tmp/out.svg">` HTML wrapper in Chrome, eyeball-confirm: visible colored progress bar, "27 Languages" count header, both section headers, per-language values + indepth estimation line. Per spec US1 acceptance scenarios.

### Implementation for User Story 1

- [ ] T004 [US1] Create `specs/011-plugin-rendering-parity/plugins/languages.md` by copying `contracts/partial-parity-checklist.md`'s File template (§1) and populating the element inventory table from `org_repo/source/templates/classic/partials/languages.ejs`. Mark upstream column ✅ for: `<h2 class="field">` count header with code octicon, `<small>` indepth/recent estimation summary, `<h3 class="field">` section headers, `<svg class="bar">`-wrapped progress bar with `<rect mask="url(#languages-bar)">` per-language rects, per-language `<svg>` icons (16x16), `<span class="language-name">` + `<span class="language-value">` per entry. Define empty-state pattern (R-004 — likely Pattern B for recent section when no recent activity).
- [ ] T005 [US1] Run `bash scripts/capture-plugin-screenshot.sh languages before` (T002 dependency). Confirm `plugins/screenshots/languages-before.png` captures the v1.0.0 broken rendering (no bar visible, no headers, just text list). Mark "ours (before)" column in T004's checklist with ❌ for each missing element.
- [ ] T006 [US1] Rewrite `internal/plugins/languages/partial.go` per the parity checklist from T004. Specifically: (a) replace each bare `<g class="languages-progress">` / `<g class="languages-recent">` / `<g class="languages-indepth">` with `<svg class="bar" xmlns="http://www.w3.org/2000/svg" width="460" height="8">` wrapping (FR-002), (b) emit the `<h2 class="field">` count header with embedded code octicon (use existing `internal/render/octicon` package if available, else inline the SVG path data), (c) emit `<h3 class="field">` per-section header with the "Most used languages" / "Recently used languages" text, (d) emit the `<small>` indepth/recent estimation summary when the respective mode is enabled, (e) emit per-language `<svg viewBox="0 0 16 16">` icons (use the colorset color as fallback when octicon name unavailable). Add table-test cases in `internal/plugins/languages/partial_test.go` for each new emission path. ~300 LOC.
- [ ] T007 [US1] Re-baseline `tests/golden/classic/m4/languages.svg` via `go test ./internal/plugins/languages/... -update`. Inspect the diff to confirm structural additions (`<h2>`, `<svg class="bar">`, `<h3>`, `<small>` estimation, per-language icons) are visible. Confirm `tests/golden/json/m4/languages.json` UNCHANGED (JSON contract preservation per Principle II).
- [ ] T008 [US1] Create `tests/visual/languages_test.go` per [contracts/visual-test-shape.md §2](./contracts/visual-test-shape.md#2-testsvisualslug_testgo-per-plugin-us1--languages-pilot-us2--18-others). 5 sub-tests: `header_exists` (A1: `h2.field` count >= 1), `bar_renders` (A2: `rect.language-bar` bbox.width > 0 — catches bare-`<g>` regression), `has_most_used_section` (A4: text "Most used languages"), `has_recently_used_section` (A4: text "Recently used languages"), `has_indepth_estimation` (A4: text "estimation from"). Run `go test ./tests/visual/ -run TestLanguages_Visual -v`; iterate T006 until all 5 sub-tests green.
- [ ] T009 [US1] Run `bash scripts/capture-plugin-screenshot.sh languages after`. Confirm `plugins/screenshots/languages-after.png` shows the fully-rendered output matching upstream. Mark "ours (after)" column in T004's checklist with ✅ for each element now present.
- [ ] T010 [US1] Open PR per [contracts/per-plugin-pr-template.md](./contracts/per-plugin-pr-template.md): title `fix(011): languages partial visual parity with upstream classic template`, body includes the before/after screenshot pair, parity-checklist diff table, re-baselined golden delta summary, constitution gates check (all 5 PASS), and the CI test plan checklist. Request maintainer review per the contract's "Reviewer guide" — reviewer validates by comparing screenshots + confirming checklist ✅ deltas.

**Checkpoint**: US1 alone is mergeable. languages plugin renders identically to upstream + visual test prevents future regression. Maintainer + reviewer have validated the entire 7-step workflow that US2 will repeat 18 times.

---

## Phase 4: User Story 2 — 18-plugin parity sweep (Priority: P2)

**Goal**: All 18 remaining adopted plugins render at upstream classic-template parity, each shipped as its own PR per spec Clarification §3.

**Independent Test**: After each T0XX merges, re-render the affected plugin via `metrics-cli`, open via `<img>` wrapper, eyeball-confirm the upstream-parity output. Per spec US2 acceptance scenarios.

### Implementation for User Story 2

Each of the 18 tasks below follows the **same 7-step workflow** US1 validated (T004-T010), shrunk into a single task because the scaffolding (T002 screenshot script, T003 visual harness, T004 checklist template) is reused. The PR author per task:

1. Creates `plugins/<slug>.md` from the checklist template
2. Runs `capture-plugin-screenshot.sh <slug> before`, marks before-state
3. Rewrites `internal/plugins/<slug>/partial.go` per the parity checklist
4. Re-baselines `tests/golden/classic/m4/<slug>.svg` via `-update`
5. Adds `tests/visual/<slug>_test.go` with 3-5 assertions (R-002 menu + [contracts/visual-test-shape.md §3](./contracts/visual-test-shape.md#3-per-plugin-assertion-count-budget))
6. Runs `capture-plugin-screenshot.sh <slug> after`, marks after-state ✅
7. Opens PR per [contracts/per-plugin-pr-template.md](./contracts/per-plugin-pr-template.md)

The 18 tasks below are all [P] relative to each other (different files in different plugin dirs); they share dependencies only on T002 + T003 from Phase 2.

#### P1-tier plugins (4 — were in M4 P1 MVP; high visibility)

- [ ] T011 [P] [US2] Plugin parity for `achievements` per the 7-step workflow above. Files: `internal/plugins/achievements/partial.go`, `tests/visual/achievements_test.go`, `tests/golden/classic/m4/achievements.svg`, `specs/011-plugin-rendering-parity/plugins/achievements.md` + screenshots. Upstream EJS: `org_repo/source/templates/classic/partials/achievements.ejs` (57 LOC). Assertion budget: 5 (badge grid + per-badge icon + progress indicators).
- [ ] T012 [P] [US2] Plugin parity for `activity`. Files: `internal/plugins/activity/partial.go`, `tests/visual/activity_test.go`, `tests/golden/classic/m4/activity.svg`, `specs/011-plugin-rendering-parity/plugins/activity.md` + screenshots. Upstream EJS (182 LOC). Assertion budget: 4 (event list + per-event icon + dates).
- [ ] T013 [P] [US2] Plugin parity for `repositories`. Files: `internal/plugins/repositories/partial.go`, `tests/visual/repositories_test.go`, `tests/golden/classic/m4/repositories.svg`, `specs/011-plugin-rendering-parity/plugins/repositories.md` + screenshots. Upstream EJS (77 LOC). Assertion budget: 3 (per-repo chip list).
- [ ] T014 [P] [US2] Plugin parity for `isocalendar` (has bare-`<g>` + `<rect>` bug). Files: `internal/plugins/isocalendar/partial.go`, `tests/visual/isocalendar_test.go`, `tests/golden/classic/m4/isocalendar.svg`, `specs/011-plugin-rendering-parity/plugins/isocalendar.md` + screenshots. Upstream EJS (50 LOC). Assertion budget: 4 (year heatmap grid + bbox > 0 to catch bare-`<g>` regression + summary stats).

#### P2-tier plugins (12 — GraphQL+REST single-source)

- [ ] T015 [P] [US2] Plugin parity for `calendar` (has bare-`<g>` bug). Files: `internal/plugins/calendar/partial.go`, `tests/visual/calendar_test.go`, `tests/golden/classic/m4/calendar.svg`, `specs/011-plugin-rendering-parity/plugins/calendar.md` + screenshots. Upstream EJS (33 LOC). Assertion budget: 4 (calendar heatmap + bbox > 0 catches regression).
- [ ] T016 [P] [US2] Plugin parity for `habits` (has bare-`<g>` + `<rect>` bug). Files: `internal/plugins/habits/partial.go`, `tests/visual/habits_test.go`, `tests/golden/classic/m4/habits.svg`, `specs/011-plugin-rendering-parity/plugins/habits.md` + screenshots. Upstream EJS (114 LOC). Assertion budget: 4 (habits chart + category labels + bbox > 0).
- [ ] T017 [P] [US2] Plugin parity for `stars`. Files: `internal/plugins/stars/partial.go`, `tests/visual/stars_test.go`, `tests/golden/classic/m4/stars.svg`, `specs/011-plugin-rendering-parity/plugins/stars.md` + screenshots. Upstream EJS (68 LOC). Assertion budget: 3 (per-star repo chips).
- [ ] T018 [P] [US2] Plugin parity for `people`. Files: `internal/plugins/people/partial.go`, `tests/visual/people_test.go`, `tests/golden/classic/m4/people.svg`, `specs/011-plugin-rendering-parity/plugins/people.md` + screenshots. Upstream EJS (42 LOC). Assertion budget: 3 (per-person card grid + role tag).
- [ ] T019 [P] [US2] Plugin parity for `notable`. Files: `internal/plugins/notable/partial.go`, `tests/visual/notable_test.go`, `tests/golden/classic/m4/notable.svg`, `specs/011-plugin-rendering-parity/plugins/notable.md` + screenshots. Upstream EJS (69 LOC). Assertion budget: 3 (per-entry author info + star counts).
- [ ] T020 [P] [US2] Plugin parity for `contributors`. Files: `internal/plugins/contributors/partial.go`, `tests/visual/contributors_test.go`, `tests/golden/classic/m4/contributors.svg`, `specs/011-plugin-rendering-parity/plugins/contributors.md` + screenshots. Upstream EJS (N/A — see plan.md for note that classic doesn't include this file; use repository partial as reference if classic missing, or fall back to compute equivalent structure). Assertion budget: 3 (per-contributor avatar grid).
- [ ] T021 [P] [US2] Plugin parity for `reactions`. Files: `internal/plugins/reactions/partial.go`, `tests/visual/reactions_test.go`, `tests/golden/classic/m4/reactions.svg`, `specs/011-plugin-rendering-parity/plugins/reactions.md` + screenshots. Upstream EJS (61 LOC). Assertion budget: 3 (emoji column + counts).
- [ ] T022 [P] [US2] Plugin parity for `projects`. Files: `internal/plugins/projects/partial.go`, `tests/visual/projects_test.go`, `tests/golden/classic/m4/projects.svg`, `specs/011-plugin-rendering-parity/plugins/projects.md` + screenshots. Upstream EJS (85 LOC). Assertion budget: 3 (project status pill + progress).
- [ ] T023 [P] [US2] Plugin parity for `sponsors`. Files: `internal/plugins/sponsors/partial.go`, `tests/visual/sponsors_test.go`, `tests/golden/classic/m4/sponsors.svg`, `specs/011-plugin-rendering-parity/plugins/sponsors.md` + screenshots. Upstream EJS (76 LOC). Assertion budget: 3 (per-tier section + past-section toggle).
- [ ] T024 [P] [US2] Plugin parity for `sponsorships`. Files: `internal/plugins/sponsorships/partial.go`, `tests/visual/sponsorships_test.go`, `tests/golden/classic/m4/sponsorships.svg`, `specs/011-plugin-rendering-parity/plugins/sponsorships.md` + screenshots. Upstream EJS (40 LOC). Assertion budget: 3 (per-tier section).
- [ ] T025 [P] [US2] Plugin parity for `stargazers`. Files: `internal/plugins/stargazers/partial.go`, `tests/visual/stargazers_test.go`, `tests/golden/classic/m4/stargazers.svg`, `specs/011-plugin-rendering-parity/plugins/stargazers.md` + screenshots. Upstream EJS (62 LOC). Assertion budget: 4 (chart + time-series + bbox > 0).
- [ ] T026 [P] [US2] Plugin parity for `traffic`. Files: `internal/plugins/traffic/partial.go`, `tests/visual/traffic_test.go`, `tests/golden/classic/m4/traffic.svg`, `specs/011-plugin-rendering-parity/plugins/traffic.md` + screenshots. Upstream EJS (0 LOC — no classic partial; use repository/sponsors-like reference, document the synthesized parity target in the per-plugin checklist). Assertion budget: 3 (views + clones chart).

#### P3-tier plugins (2 — chromedp-gated, use mocked plugin data per R-003)

- [ ] T027 [P] [US2] Plugin parity for `topics`. Files: `internal/plugins/topics/partial.go`, `tests/visual/topics_test.go`, `tests/golden/classic/m4/topics.svg`, `specs/011-plugin-rendering-parity/plugins/topics.md` + screenshots. Upstream EJS (33 LOC). Assertion budget: 3 (topic chip list). Per R-003: visual test uses mocked topic data (no real chromedp scrape).
- [ ] T028 [P] [US2] Plugin parity for `starlists` (has bare-`<g>` bug). Files: `internal/plugins/starlists/partial.go`, `tests/visual/starlists_test.go`, `tests/golden/classic/m4/starlists.svg`, `specs/011-plugin-rendering-parity/plugins/starlists.md` + screenshots. Upstream EJS (99 LOC). Assertion budget: 4 (list grid + per-list metadata + bbox > 0 catches bare-`<g>` regression). Per R-003: visual test uses mocked starlist data.

**Checkpoint**: All 19 plugins (US1 + US2) at upstream parity. Each shipped as its own PR. Maintainer has merged 19 PRs over ~14 calendar days.

---

## Phase 5: User Story 3 — Visual regression infrastructure CI gate (Priority: P3)

**Goal**: CI runs the full 19-plugin visual suite on every PR; a regression-prevention compliance test statically gates against bare-`<g>` reintroduction.

**Independent Test**: Force a regression (locally edit a plugin partial to drop a section header or reintroduce bare `<g>`), push to a PR branch, observe CI blocks the PR with an actionable failure naming the missing element. Per spec US3 acceptance scenarios.

### Implementation for User Story 3

- [ ] T029 [US3] Update `.github/workflows/ci.yml` to add a `visual` job: runs `go test ./tests/visual/... -timeout 15m` on every push to PR / main. Job needs `METRICS_CHROME_PATH=/usr/bin/chromium` env (chromium already installed on `ubuntu-latest` for M3 / M4 tests). PR blocked on failure (FR-005).
- [ ] T030 [US3] Add `TestCompliance_PluginPartialNoBareSVGChildren` to `tests/compliance/compliance_test.go` per FR-009: enumerates each `internal/plugins/<slug>/partial.go` file, runs a regex matching `"<g\\s` or `` `<g\s `` patterns NOT immediately wrapped by a preceding `<svg ...>` opening tag, fails with the offending file + line if any plugin reintroduces bare `<g>`. Failure message: `"internal/plugins/X/partial.go:NN: bare <g> element emission detected (must be wrapped in <svg width=... height=...>)"`. Test runs in the existing `compliance` CI job.
- [ ] T031 [US3] End-to-end regression rehearsal: locally edit `internal/plugins/languages/partial.go` to drop the `<svg class="bar">` wrapper (reintroducing the bare-`<g>` bug); run `go test ./tests/visual/ -run TestLanguages_Visual -v`; confirm the `bar_renders` sub-test fails with actionable message; run `go test ./tests/compliance/...`; confirm `TestCompliance_PluginPartialNoBareSVGChildren` also fails. Restore the file. Document the rehearsal outcome in the US3 PR description.

**Checkpoint**: US3 complete. The byte-golden + visual-regression + static-compliance triad collectively prevents this class of bug from being re-speced-in.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Unblock 010, close out 011 with documentation + final visual review.

- [ ] T032 Remove `specs/010-docs-plugin-gallery/BLOCKED.md` and update `CLAUDE.md` to restore 010 as the active plan. Re-run `bash scripts/gen-doc-samples.sh` to regenerate the 23 SVG + 23 PNG samples — they will now match upstream rendering (per SC-005). Commit the regenerated samples to the 010 branch (or open a follow-up PR on 010 to unblock its remaining tasks #367 / #369 / #371 / #373).
- [ ] T033 Update `specs/011-plugin-rendering-parity/plan.md` Constitution Check post-completion: confirm all 5 gates remain PASS after all 19 PRs landed. Update the per-plugin checklist files to ✅ Status (merged). Tag a v1.1.0 release candidate per the spec Assumptions section.
- [ ] T034 Run full local regression: `make test` + `make lint` + `make hooks-run` against `main` post-merge. Confirm: (a) M1-M10 baseline tests stay green, (b) all 19 visual tests pass, (c) `TestCompliance_PluginPartialNoBareSVGChildren` passes, (d) no new lints introduced by the 19 plugin rewrites. Record the run results in a polish-PR description.
- [ ] T035 Visual regression sweep on github.com: for each of the 19 plugins, open the re-generated `docs/examples/plugin-<slug>.svg` directly via raw.githubusercontent.com `<img>` URL, eyeball-confirm rendered output. For 3 sampled plugins (languages + isocalendar + sponsors), compare against the upstream `lowlighter/metrics` Docker image's output for mjun0812. Record observations in this task's tracking issue.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)** has no dependencies — start immediately.
- **Phase 2 (Foundational)** depends on Phase 1 (`capture-plugin-screenshot.sh` exists). T003 is one task; not parallelizable with itself, but T002 ∥ T003 in time (T003 doesn't depend on T002).
- **Phase 3 (US1 pilot)** depends on Phase 2 completion. T004-T010 are STRICTLY SEQUENTIAL within the pilot — each step depends on the prior.
- **Phase 4 (US2 sweep)** depends on Phase 2 + US1 pilot merge (so the workflow + harness + checklist template are battle-tested). T011-T028 are 18 INDEPENDENT tasks (1 plugin = 1 PR each), parallelizable in time across multiple PRs but each PR's internal 7-step workflow is sequential.
- **Phase 5 (US3 CI / compliance)** depends on US2 substantially complete (at least 17 of 19 plugins merged) so the visual suite exercises real assertions, not stubs.
- **Phase 6 (Polish)** depends on US3 complete + 010 branch availability for the unblock.

### User Story Dependencies

- **US1 (P1, pilot)** — independent. Ships languages alone; validates 7-step workflow.
- **US2 (P2, sweep)** — depends on US1 merged. Each of the 18 sub-tasks is independent of the others.
- **US3 (P3, CI gate)** — depends on US2 substantially complete.

### Within Each User Story

- **US1 (pilot)**: T004 (checklist) → T005 (before screenshot) → T006 (rewrite partial) → T007 (re-baseline golden) → T008 (visual test) → T009 (after screenshot) → T010 (PR). Strict sequence.
- **US2 (sweep)**: Each of T011-T028 internally follows the same 7-step sequence as US1. The 18 are parallelizable across multiple PRs.
- **US3 (CI)**: T029 → T030 → T031. Strict sequence (T031 rehearses against both T029 + T030).

### Parallel Opportunities

- T002 ∥ T003 (Phase 2 — different files: shell script vs Go test harness)
- T011 through T028 are all [P] relative to each other (Phase 4 — 18 different `internal/plugins/<slug>/` dirs + 18 different `tests/visual/<slug>_test.go` files + 18 different `plugins/<slug>.md` checklists)
- T029 ∥ T030 (Phase 5 — different files: `.github/workflows/ci.yml` vs `tests/compliance/compliance_test.go`)
- T032 ∥ T033 (Phase 6 — different files: 010 unblock vs 011 closure)

---

## Parallel Example: US2 sweep

```bash
# Phase 4: 18 PRs can be in flight simultaneously.
Task T011: "languages parity (already pilot in US1, skip if pilot covers)"
Task T012: "achievements parity PR"
Task T013: "activity parity PR"
...
Task T028: "starlists parity PR"

# All 18 PRs are file-disjoint at the plugin level.
# Maintainer review is the bottleneck — pace at ~1-2 PRs per workday.
```

---

## Implementation Strategy

### MVP First (US1 only — languages pilot)

1. Complete Phase 1 (T001-T002) + Phase 2 (T003).
2. Complete Phase 3 (T004-T010) for languages alone.
3. **STOP and VALIDATE**: open the languages PR's after-screenshot side-by-side with the upstream lowlighter/metrics output for mjun0812. Confirm visual parity. Merge the PR.
4. Languages alone is mergeable + ships value: that one plugin renders correctly + the visual test infrastructure exists for future plugins. US2 + US3 land in follow-up PRs.

### Recommended Incremental Delivery

1. Phase 1 + 2 — infrastructure (T001-T003).
2. Phase 3 — US1 pilot (T004-T010) → ship as MVP.
3. Phase 4 — US2 sweep: open ~3-4 PRs per workday, merge as reviewed. ~14 calendar days to complete all 18.
4. Phase 5 — US3 CI gate (T029-T031) ships immediately after US2 substantially complete.
5. Phase 6 — Polish (T032-T035) unblocks 010 + final regression sweep.

This matches the M4 / M10 / 009 incremental-PR pattern: ship core machinery first, then iterate.

### Parallel Team Strategy (if multi-contributor)

- Developer A: Pilot (US1, T004-T010) + US3 (T029-T031) + Polish (T032-T035) — owns the infrastructure end-to-end.
- Developers B, C, D, …: Each picks 4-5 plugin PRs from T011-T028. Each PR is self-contained; minimal cross-coordination needed.

---

## Notes

- **Scope discipline (constitution III)**: confined to the 19 adopted plugins per `tests/compliance/compliance_test.go::adoptedM4Plugins`. The base + core plugins are out of scope (they don't have `partial.go`).
- **JSON contract preservation (constitution II)**: every per-plugin task explicitly confirms `tests/golden/json/m4/<slug>.json` is UNCHANGED. SVG re-baseline is expected; JSON re-baseline would be a Principle II violation needing explicit constitution exception.
- **Golden re-baseline policy (SC-007)**: a re-baselined SVG golden MUST be accompanied by before/after screenshots in the PR description. A reviewer rejecting a blind `-update` PR is encouraged.
- **chromedp dependency**: shared with M3 + M4 P3. No new external deps introduced by 011.
- **`tests/visual/` is a NEW top-level dir** (per spec Clarification §2). Keeping separate from `internal/testutil/golden/` prevents accidental coupling between byte-compare and visual-render concerns.
- **010 BLOCKED resume**: T032 explicitly unblocks 010 by removing its BLOCKED.md note + restoring CLAUDE.md active plan. The 010 deferred tasks (#367, #369, #371, #373) resume on their own track after T032.
