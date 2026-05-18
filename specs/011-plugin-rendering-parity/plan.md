# Implementation Plan: Plugin rendering parity with upstream EJS templates

**Branch**: `011-plugin-rendering-parity` | **Date**: 2026-05-19 | **Spec**: [./spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-plugin-rendering-parity/spec.md`

## Summary

The 19 adopted M4 plugin partials (`internal/plugins/<slug>/partial.go`) emit a stripped-down subset of upstream `lowlighter/metrics` classic-template output, and a 5-plugin subset additionally emits structurally invalid markup (bare `<g>` SVG-namespace elements inside HTML `<foreignObject>`) that silently disappears in `<img src=...svg>` rendering contexts (the dominant GitHub README embed path). v1.0.0 shipped with these defects unnoticed because the only render-output test layer is byte-equality goldens. This feature: (a) rewrites each plugin partial to match upstream's user-visible feature set; (b) re-baselines the byte goldens behind that rewrite; (c) adds a new chromedp-driven visual-regression layer at `tests/visual/<plugin>_test.go` that asserts on rendered DOM properties so this class of bug can never be byte-frozen-in again. Strategy: 1 plugin per PR over ~2 weeks, languages as the pilot, then a 18-plugin sweep. Unblocks 010 docs-plugin-gallery on completion.

## Technical Context

**Language/Version**: Go 1.26 (per `go.mod`; constitution principle V — latest stable)

**Primary Dependencies**: `github.com/chromedp/chromedp` (reused from M3 — visual test browser launch); `gopkg.in/yaml.v3` (existing); `internal/testutil/golden` (existing byte-compare); `internal/render/Browser` (existing M3 chromedp wrapper, reused as the visual test driver). No new external dependencies.

**Storage**: N/A (file-based artefacts only — partial.go source files, golden SVG/JSON files, screenshot PNGs under `specs/011-*/plugins/screenshots/`)

**Testing**: `go test` (table tests + new visual layer); chromedp tag-gated tests for the visual layer (mirrors M3 / M4 P3 plugin pattern). Each plugin's visual test asserts on ≥3 DOM-level structural properties via `chromedp.Evaluate` calls against the rendered SVG in `about:blank`.

**Target Platform**: Linux + macOS dev machines; GitHub-hosted `ubuntu-latest` runners for CI (constitution Technical Constraints — multi-arch Docker is M10's concern, unchanged here).

**Project Type**: CLI / GitHub Action (no new project type — extends existing `cmd/metrics-action` + `internal/plugins/*` package layout).

**Performance Goals**: Per-plugin partial run-time unchanged (template emission is pure string building, sub-millisecond). Visual regression suite: full 19-plugin run ≤ 10 min wall-clock on `ubuntu-latest` (SC-003). Per-plugin visual test: ≤ 30 sec including chromedp tab spin-up (matches M3 `BenchmarkResize_FixedSVG` budget of 2.5 sec per render × ~10 assertions amortized).

**Constraints**: No semver-breaking changes to `action.yml` inputs or `metrics_*.json` output (constitution principles I + II.JSON); only the rendered SVG/DOM structure changes — and only in the direction of upstream parity. Visual test failures MUST block PR merge (FR-005). Re-baselined goldens MUST be accompanied by before/after screenshots in the PR description (SC-007).

**Scale/Scope**: 19 plugin partials rewritten (per `tests/compliance/compliance_test.go::adoptedM4Plugins` minus base/core); 19 visual test files added; 19 PRs over ~14 calendar days. Estimated LOC delta: +1500-3000 production (Go partials), +800-1200 test, +19 per-plugin parity notes under `specs/011-*/plugins/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| **I. 入力互換性** | ✅ PASS | No `action.yml` / `metadata.yml` / `settings.json` schema changes. Plugin INPUT keys unchanged; only rendered SVG output changes. |
| **II. 出力契約 (DOM/JSON)** | ✅ PASS (restores compliance) | This feature exists *because* §II was being violated: SVG DOM was not at parity with upstream. JSON output is untouched. Goldens are re-baselined per plugin; the new bytes become the source of truth, and the new `tests/visual/` layer (FR-004) detects future DOM regressions structurally. |
| **III. スコープ規律** | ✅ PASS | Affects the 19 adopted plugins only (`adoptedM4Plugins` minus base/core). No new plugins added, no unadopted-plugin code touched. Compliance test `TestCompliance_M4_AdoptedPlugins` continues to gate. |
| **IV. テーブルテスト + Golden File** | ✅ PASS (strengthens) | Existing table tests + byte goldens kept; `tests/visual/` adds a structural layer the byte goldens cannot provide. `-update` flag workflow preserved for byte goldens. |
| **V. Go 規約と言語ポリシー** | ✅ PASS | Pure Go (latest stable); changes confined to `internal/plugins/*` and `tests/visual/` (new). No new `pkg/`. Source comments + identifiers in English (existing pattern). `log/slog` already wired. No new external dependencies. |

**Verdict**: All 5 gates PASS. This feature is required to bring the project back into Principle II compliance (the v1.0.0 shipped output is not at DOM parity with upstream).

## Project Structure

### Documentation (this feature)

```text
specs/011-plugin-rendering-parity/
├── plan.md                       # This file
├── spec.md                       # User-visible spec (already exists)
├── research.md                   # Phase 0 — R-001..R-005 decisions
├── data-model.md                 # Phase 1 — E-001..E-006 entities
├── contracts/                    # Phase 1 contracts
│   ├── partial-parity-checklist.md  # Per-plugin gap audit template
│   ├── visual-test-shape.md         # tests/visual/ file contract
│   └── per-plugin-pr-template.md    # PR-description template
├── quickstart.md                 # Phase 1 maintainer flow
├── plugins/                      # Per-plugin parity notes (one .md per plugin, populated as each PR lands)
│   ├── languages.md              # Pilot — populated by US1 PR
│   ├── achievements.md           # Populated by US2 PR
│   └── ...                       # 19 .md files total once US2 sweeps
└── checklists/
    └── requirements.md           # Already exists (passed validation)
```

### Source Code (repository root)

```text
# Files modified by this feature (19 plugin partials):
internal/plugins/
├── achievements/partial.go       # Rewritten — add header / icons / per-entry octicons (US2)
├── activity/partial.go           # Rewritten — add upstream event icons / dates (US2)
├── calendar/partial.go           # Rewritten — wrap <g> in <svg>, add heatmap legend (US2)
├── contributors/partial.go       # Rewritten — add avatar grid, contribution counts (US2)
├── habits/partial.go             # Rewritten — wrap <g>/<rect>, add habit category labels (US2)
├── isocalendar/partial.go        # Rewritten — wrap <g>/<rect>, add year heatmap chrome (US2)
├── languages/partial.go          # 🎯 Pilot (US1) — full upstream parity (header / bar / sections / indepth)
├── notable/partial.go            # Rewritten — add author info / star counts (US2)
├── people/partial.go             # Rewritten — add per-person avatar / role tag (US2)
├── projects/partial.go           # Rewritten — add status pill / progress (US2)
├── reactions/partial.go          # Rewritten — add emoji column / counts (US2)
├── repositories/partial.go       # Rewritten — add per-repo stargazer / fork chips (US2)
├── sponsors/partial.go           # Rewritten — add tier badges / past-section toggle (US2)
├── sponsorships/partial.go       # Rewritten — add tier badges (US2)
├── stargazers/partial.go         # Rewritten — add chart / time-series (US2)
├── starlists/partial.go          # Rewritten — wrap <g>, add list metadata (US2)
├── stars/partial.go              # Rewritten — add per-star repo chips (US2)
├── topics/partial.go             # Rewritten — add topic chips (US2)
└── traffic/partial.go            # Rewritten — add views / clones charts (US2)

# Files added by this feature:
tests/visual/                     # NEW top-level dir (per spec Clarification §2)
├── visual_test.go                # Shared test harness — chromedp browser bootstrap, common DOM assertion helpers
├── languages_test.go             # US1 (pilot)
├── achievements_test.go          # US2
├── activity_test.go              # US2
├── calendar_test.go              # US2
├── contributors_test.go          # US2
├── habits_test.go                # US2
├── isocalendar_test.go           # US2
├── notable_test.go               # US2
├── people_test.go                # US2
├── projects_test.go              # US2
├── reactions_test.go             # US2
├── repositories_test.go          # US2
├── sponsors_test.go              # US2
├── sponsorships_test.go          # US2
├── stargazers_test.go            # US2
├── starlists_test.go             # US2
├── stars_test.go                 # US2
├── topics_test.go                # US2
└── traffic_test.go               # US2

# Files modified for regression-prevention compliance gate:
tests/compliance/
└── compliance_test.go            # +TestCompliance_PluginPartialNoBareSVGChildren (FR-009) — US3 final

# Files re-baselined (byte goldens — bytes change per plugin PR):
tests/golden/classic/m4/
├── achievements.svg              # Re-baselined in achievements PR
├── activity.svg                  # Re-baselined in activity PR
├── ...                           # All 19 SVG goldens re-baselined as each PR lands
├── achievements.json             # Unchanged (JSON contract from Principle II)
├── activity.json                 # Unchanged
└── ...                           # All 19 JSON goldens unchanged

# Files touched by US3 polish (CI workflow):
.github/workflows/
└── ci.yml                        # +`visual` job that runs `go test ./tests/visual/...` (US3)
```

**Structure Decision**: Single Go project (per constitution principle V — no `pkg/`, all under `internal/` + `tests/`). New top-level `tests/visual/` dir per spec Clarification §2 — keeps the structural-rendering test mode separate from `internal/testutil/golden/` byte-compare to avoid accidental coupling. Per-plugin parity notes live under the feature spec dir (`specs/011-*/plugins/<slug>.md`) so the audit + before/after screenshots stay co-located with the spec.

## Complexity Tracking

No constitution violations to track — all 5 gates PASS as documented above. The feature exists to *restore* Principle II compliance.
