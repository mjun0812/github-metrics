# Feature Specification: Plugin rendering parity with upstream EJS templates

**Feature Branch**: `011-plugin-rendering-parity`

**Created**: 2026-05-19

**Status**: Draft

**Input**: User description: "M4 plugin partials produce visually broken / incomplete SVG/PNG when rendered via GitHub's `<img src=...svg>` path. Goal: bring all 19 adopted plugin partials to upstream lowlighter/metrics classic-template feature parity so the rendered output matches what users see when using upstream. Pilot with languages; sweep remaining 18; add chromedp-based visual regression tests so this class of bug never speced-in again. Blocker for resuming 010 docs-plugin-gallery."

## Overview

When the github-metrics action / CLI produces an SVG and a consumer embeds it on github.com via `<img src=".../metrics.svg">`, parts of the output are invisible or absent compared to the equivalent output from upstream `lowlighter/metrics`. The discrepancy spans two distinct defect classes:

1. **Structural invalidity** (5 plugins): the plugin emits bare `<g class="...">` or `<rect>` SVG-namespace elements directly inside an HTML `<foreignObject>`. Browsers parse the foreignObject contents as HTML; `<g>` is an unknown HTML element with no rendering, so the wrapped progress bars / icons / charts silently disappear. Affected: languages, habits, isocalendar, calendar, starlists.
2. **Content gap vs upstream** (~14 plugins): the Go partial emits a stripped-down subset of the upstream EJS template — missing section headers, per-entry octicons, summary statistics ("indepth" mode size estimates, "recent" mode commit counts), error visualisation, and empty-section suppression. The plugin "renders" but produces something a viewer would not recognise as the upstream feature.

Both classes shipped past M4 → v1.0.0 because the existing golden tests assert byte equality of the rendered SVG output, not visual rendering. The byte sequence is frozen as expected, but no test ever opens the SVG in a browser to confirm the user-facing result.

The 010 docs-plugin-gallery feature is blocked on this work — its sample SVGs would showcase the broken state. Constitution principle "drop-in replacement for upstream" is also at stake: a drop-in must match user-visible output, not just produce *some* SVG.

## Clarifications

### Session 2026-05-19

- Q: Should re-baselined golden files preserve the existing byte-format (e.g., still use `<g>` but with surrounding `<svg>`), or rewrite golden bytes wholesale to match upstream EJS output line-for-line? → A: Per-plugin, the new partial is the source of truth; goldens are re-baselined with `go test -update` after the partial change lands, and the maintainer eyeball-validates the diff before committing the new bytes. No requirement to byte-match upstream EJS — only visual / structural equivalence.
- Q: Does visual-regression test infrastructure live in `tests/visual/` (new top-level dir) or extend `internal/testutil/golden/` (alongside existing helpers)? → A: New top-level `tests/visual/` directory. Reason: existing `internal/testutil/golden/` is byte-comparison; visual is a fundamentally different mode (chromedp + DOM assertion), and keeping it separate prevents accidental coupling.
- Q: Each plugin lands as its own PR (19 PRs), one batched PR per priority tier (3 PRs: P1 MVP / P2 GraphQL / P3 chromedp), or one mega-PR? → A: 1 plugin = 1 PR. Reasons: per-plugin visual review burden is high; bisecting regressions later is easier; the 19-PR cadence matches the existing per-task issue pattern.
- Q: Should US1/US2 plugin rewrites preserve upstream's accessibility attributes (`<title>`, `aria-label`, `<desc>`, `role`)? → A: Preserve verbatim as part of parity. Each per-plugin rewrite mirrors upstream's a11y attributes. Visual test adds 1 assertion per plugin: "title or aria-label element exists for the SVG chart / progress bar". Reason: upstream is source of truth per FR-001; a11y attributes are user-visible to assistive-tech users; dropping them widens the parity gap and would need separate later remediation.
- Q: How should visual-test failures be observable in CI? → A: Upload the rendered SVG + a 720x800 PNG screenshot as CI artefacts ONLY when the visual test fails (`if: failure()` guard). Implementation: `actions/upload-artifact@v4` with `if: failure()` in the visual job. Reason: per-PR storage cost stays minimal (no upload on green runs); failed-test debug is self-contained from the CI tab without local re-render; matches the existing GHA artefact pattern used by M10 release-binary smoke tests.
- Q: What is the flakiness / re-try policy for chromedp-driven visual tests? → A: Retry each visual assertion up to 3 times inside Go test code (`for i := 0; i < 3; i++` around the chromedp.Evaluate call); only the third consecutive failure marks the assertion as failed. Implementation lives in the shared `tests/visual/visual_test.go` helpers (`assertElementExists`, `assertBoundingBoxNonZero`, `assertTextContent` each retry internally). Reason: typical chromedp flake (DOMContentLoaded race, font-loading race) clears on re-execution ≥95% of the time per empirical M3 chromedp test experience; in-test retry is frictionless for PR authors (no manual re-run), and 3 consecutive failures is a strong signal of genuine regression vs flake.
- Q: How far should each plugin's visual test cover sub-mode inputs (e.g., `plugin_languages_indepth=yes`, `plugin_achievements_threshold=B`, `plugin_calendar_limit=12`)? → A: Default config only — `plugin_<slug>=yes` enabled with no additional sub-mode inputs. Exception: the languages pilot's visual test covers `_indepth=yes` + `_sections=most-used,recently-used` because those modes produce the canonical user-visible feature set referenced in US1 acceptance scenarios (treated as "default-equivalent" for that plugin). All other plugins' visual tests use default config only. Reason: spec Edge Cases already declares "default-config rendering is the scope"; expanding sub-mode coverage per plugin would balloon per-PR scope and miss the 14-day completion target; sub-mode parity sweep is deferred to a potential follow-up feature.
- Q: How should the visual test suite behave when chromium is unavailable (broken / missing) in the runtime environment? → A: Behavior is conditional on the `CI` environment variable. In CI (GHA auto-sets `CI=true`) OR when `METRICS_VISUAL_STRICT=1` is explicitly set, chromium-unavailable MUST exit the suite non-zero (FR-005 PR gate enforced). In local dev (no `CI`, no `METRICS_VISUAL_STRICT`), chromium-unavailable skips the suite cleanly (`os.Exit(0)`) so `go test ./...` stays green on contributor machines lacking chromium. Implementation: `TestMain` branches on `os.Getenv("CI") == "true" || os.Getenv("METRICS_VISUAL_STRICT") != ""`. Reason: balances local-dev friction (M3 already requires chromium for `BenchmarkResize_FixedSVG`, but not all contributors run benchmarks) with CI-gate integrity (a chromium regression must not silent-pass the visual gate).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Languages plugin renders identically to upstream (Priority: P1) 🎯 MVP

A user enables `plugin_languages: yes` in their `metrics.yml` and embeds the resulting `metrics.languages.svg` in their profile README via `<img src=".../metrics.languages.svg">`. When a viewer opens the README on github.com, they see: the languages-count header with a code icon ("27 Languages"); a colourful gradient progress bar showing language distribution; "Most used languages" and "Recently used languages" section headers (when `_sections` includes both); per-language entries each with the language's color dot and percentage; for `indepth` mode, the "estimation from N kb of code in M edited files across K commits" summary text.

**Why this priority**: Languages is the most-used plugin and the most visually complex partial — fixing it validates the approach (re-implement Go partial against upstream EJS) and surfaces the design pattern for the other 18. It is also the most-cited example in the 010 BLOCKED.md findings.

**Independent Test**: Build the Action / CLI, run `metrics-cli --user mjun0812 --plugin plugin_languages=yes --output svg --filename /tmp/out.svg --dryrun`, open `/tmp/out.svg` in Chrome via an `<img src="file:///tmp/out.svg">` HTML wrapper, and confirm the rendered output contains: visible colored progress bar, language count header text, both section headers when `_sections=most-used,recently-used`, and per-language percentage values. Run the same flow against the upstream `lowlighter/metrics` Docker image; visually compare. The two outputs need not be byte-equivalent but the structural elements above must all be present in ours.

**Acceptance Scenarios**:

1. **Given** a working `metrics-cli` binary and a valid `GITHUB_TOKEN`, **When** a maintainer runs `metrics-cli --user mjun0812 --plugin plugin_languages=yes --output svg --filename out.svg --dryrun`, **Then** `out.svg` opens in a browser via `<img>` and shows the language count header, the colored progress bar, the language list with values, and is visually recognisable as a "languages metric panel"
2. **Given** the same setup with `--plugin plugin_languages_indepth=yes`, **When** rendered, **Then** the output additionally shows the "estimation from N kb of code…" summary line and per-language size in bytes (e.g., "Rust 245.3 kB 36.2%")
3. **Given** the same setup with `--plugin plugin_languages_sections=most-used,recently-used`, **When** rendered, **Then** the output shows two distinct sections each with their own header, and the "Recently used languages" section is followed by the "estimation from N kb of code in M edited files across K commits over last D days" summary

### User Story 2 - All 19 adopted plugins render at upstream parity (Priority: P2)

After US1 lands and validates the approach, the same rewrite + re-baseline pattern is applied to the remaining 18 adopted plugins. A consumer of the action / CLI can enable any subset of `plugin_<slug>: yes` inputs and the embedded SVG renders all enabled plugins faithfully — section headers, icons, summary text, error visualisation, and empty-state suppression all match upstream's user-facing output.

**Why this priority**: Once US1 proves the approach, parity for the remaining 18 is mechanical-but-deliberate work. Each plugin is independently shippable (1 PR per plugin per the clarification) so consumers benefit incrementally. The constitution principle of "drop-in replacement for upstream" is restored once all 19 land.

**Independent Test**: For each plugin in `{achievements, activity, calendar, contributors, habits, isocalendar, notable, people, projects, reactions, repositories, sponsors, sponsorships, stargazers, starlists, stars, topics, traffic}`: run `metrics-cli --user mjun0812 --plugin plugin_<slug>=yes --output svg --filename out.svg --dryrun`, open in `<img>` context, confirm the rendered output matches the visual elements present in upstream's equivalent output for the same user. Each plugin's PR is independently mergeable and ships value on its own.

**Acceptance Scenarios**:

1. **Given** a viewer opens an `<img src=".../metrics.svg">` embed with `plugin_achievements: yes`, **When** the SVG loads, **Then** they see the achievements grid with badge icons, names, percentage progress indicators, and the achievements header
2. **Given** `plugin_isocalendar: yes` enabled, **When** the SVG loads, **Then** the year-view calendar heatmap renders as a visible grid with color-coded contribution density (not as missing / blank squares)
3. **Given** any plugin with no data for the rendered user (e.g., `plugin_sponsors: yes` for a non-sponsored account), **When** the SVG loads, **Then** the plugin's section either shows an explicit empty-state message ("No sponsors found") or is suppressed entirely — it does NOT consume vertical space with a blank rectangle

### User Story 3 - Visual regression infrastructure prevents future regressions (Priority: P3)

A new test layer at `tests/visual/` exercises each adopted plugin's rendered SVG through a real Chromium tab (chromedp) and asserts on DOM-level structural properties — e.g., for languages: the `<rect class="language-bar">` elements collectively have non-zero rendered width; the `<h2 class="field">` containing the language count is present and contains text. The test runs in CI on every PR. A maintainer adding a future plugin (or editing an existing partial) cannot land a change that re-introduces invisible-render bugs without the visual test failing first.

**Why this priority**: Without this layer, US1/US2 fixes can silently regress on a future maintainer's edit, and the byte-only golden tests will still pass (they merely freeze whatever bytes the new partial emits). The visual layer closes the gap and prevents this exact class of bug from being speced-in again.

**Independent Test**: Run `go test ./tests/visual/...` against the current main branch *before* US1 lands → some / all visual tests fail (existing rendering bugs). After US1 + US2 land, the visual suite goes green. Force a regression (e.g., locally edit `internal/plugins/languages/partial.go` to drop the `<svg class="bar">` wrapper) and re-run the visual suite → it fails with an actionable message naming the missing DOM element.

**Acceptance Scenarios**:

1. **Given** the v0.x rendering bugs are intentionally present (e.g., languages partial reverted), **When** `go test ./tests/visual/languages_test.go` runs, **Then** the test fails with a message like "expected language-bar rect element count >= 1 with non-zero width; got 0 rendered"
2. **Given** all plugins are at upstream parity (US1 + US2 complete), **When** the full visual suite runs in CI on a PR, **Then** all 19 plugins' visual tests pass, and the suite completes within 10 minutes of CI wall-clock
3. **Given** a new contributor adds an unadopted plugin's partial (or edits an existing partial), **When** they push the change, **Then** if their change introduces a render regression for any of the 19 adopted plugins, CI blocks the PR with a clear test-failure pointing at the regressed plugin

### Edge Cases

- A plugin whose data is empty for the test user (e.g., `sponsors` for a non-sponsored account) — the partial should either render an explicit empty-state message or suppress its section entirely (not emit a blank rectangle)
- A plugin whose chromedp dependency is unavailable (topics, starlists) — the plugin already declares `Skipped=true` upstream; the visual test should detect skipped output and treat it as PASS (not assert on absent content)
- An upstream EJS template that includes a feature gated on an input we have not yet wired (e.g., `plugin_languages_colors=github` for custom color schemes) — the spec scope is parity for default-config rendering; non-default inputs are out of scope for v1 of this feature and tracked separately. Note: this includes already-wired sub-mode inputs (e.g., `plugin_achievements_threshold=B`, `plugin_calendar_limit=12`) — per the Q4 clarification, visual tests cover only `plugin_<slug>=yes` default config, with the languages pilot's `_indepth` + `_sections` coverage as a documented exception
- A re-baselined golden file whose new bytes are accidentally still invalid (e.g., the maintainer fixed one bare `<g>` but introduced another) — the visual test catches this because it asserts on rendered DOM, not bytes

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST emit, for each of the 19 adopted plugin partials, SVG markup that renders the upstream lowlighter/metrics classic-template feature set when embedded via `<img src=...svg>`. Specifically: section headers (where upstream has them), per-entry icons (where upstream has them), summary text / statistics (where upstream has them), progress bars / heatmaps / charts wrapped in proper `<svg>` containers so they render inside `<foreignObject>` instead of being silent-dropped as invalid HTML, AND upstream's accessibility attributes (`<title>`, `aria-label`, `<desc>`, `role`) preserved verbatim on the corresponding rendered elements.
- **FR-002**: System MUST NOT emit bare `<g>` or `<rect>` SVG-namespace elements as direct HTML children of `<foreignObject>`. Such elements MUST be wrapped in an explicit `<svg width=... height=... xmlns="http://www.w3.org/2000/svg">` container so browsers parse the contents as SVG.
- **FR-003**: System MUST suppress empty-section placeholder boxes when a plugin has no data to display for the rendered user — either by emitting an explicit empty-state message ("No data found") or by skipping the section entirely. An empty `<section>` element that consumes vertical space is NOT acceptable.
- **FR-004**: System MUST provide a new test layer at `tests/visual/<plugin>_test.go` that, for each of the 19 adopted plugins, renders the plugin's SVG via a real Chromium tab (chromedp) and asserts on at least 3 DOM-level structural properties of the rendered output (e.g., element count, computed style, rendered bounding-box). The test fails with an actionable message naming the missing element when a regression is introduced. Each assertion MUST retry internally up to 3 times to absorb chromedp flake (DOMContentLoaded race, font-load race); only the 3rd consecutive failure marks the assertion as failed.
- **FR-005**: System MUST run the visual regression suite in CI on every PR touching `internal/plugins/`, `internal/templates/`, `internal/render/`, or `assets/`. A failing visual test MUST block the PR until resolved. On failure, the CI job MUST upload the rendered SVG plus a 720x800 PNG screenshot of the failing plugin as CI artefacts (`actions/upload-artifact@v4` with `if: failure()`), so a reviewer can diagnose the regression from the CI tab without re-rendering locally. The suite MUST distinguish CI (strict mode: `CI=true` or `METRICS_VISUAL_STRICT=1` set — chromium-unavailable fails non-zero) from local dev (default: chromium-unavailable skips cleanly with `os.Exit(0)`) so the CI gate cannot silent-pass on a chromium regression.
- **FR-006**: System MUST re-baseline the byte-level golden files at `tests/golden/classic/m4/<plugin>.svg` after each plugin's partial change lands. The new bytes become the source of truth; no requirement to byte-match upstream EJS output (only structural / visual equivalence).
- **FR-007**: System MUST ship the 19 plugin parity fixes as 19 separate PRs (1 per plugin), starting with languages as the pilot. Each PR MUST land independently and ship visible user value (the affected plugin's rendering is fixed) before the next PR opens.
- **FR-008**: System MUST add a per-plugin VISUAL_PARITY.md (or equivalent) note under `specs/011-plugin-rendering-parity/plugins/` documenting the specific gap closed in that plugin's PR, with a before/after screenshot pair so reviewers can validate the fix without re-running the rendering pipeline.
- **FR-009**: System MUST add a regression-prevention compliance test (`tests/compliance/compliance_test.go::TestCompliance_PluginPartialNoBareSVGChildren`) that statically inspects each plugin's `partial.go` source for the bare-`<g>` / bare-`<rect>` pattern and fails the build if any plugin reintroduces it.
- **FR-010**: System MUST treat 010 docs-plugin-gallery as unblocked once US1 + US2 + US3 complete. The 010 BLOCKED.md note is removed and the deferred 010 tasks (#370, #372, #367, #369, #371, #373 etc.) resume.

### Key Entities

- **Plugin partial**: a Go function in `internal/plugins/<slug>/partial.go` that emits a fragment of HTML+SVG markup for one plugin's section in the classic template. After 011 each partial is faithful to the upstream `org_repo/source/templates/classic/partials/<slug>.ejs` template's user-visible output.
- **Upstream EJS template**: the upstream lowlighter/metrics template at `org_repo/source/templates/classic/partials/<slug>.ejs`, mirrored locally in the org_repo/ subdirectory. Treated as the source of truth for visual parity; ours must produce structurally equivalent rendered output (not byte-equivalent source).
- **Visual regression test**: a new test under `tests/visual/<plugin>_test.go` that opens the plugin's rendered SVG in a real Chromium tab and asserts on rendered DOM properties (element existence, count, bounding-box, computed style). One test file per plugin; 19 total.
- **Per-plugin parity note**: a per-plugin Markdown document under `specs/011-plugin-rendering-parity/plugins/<slug>.md` summarising the gap fixed in that plugin's PR, including a before/after screenshot pair (PNG files under `specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-before.png` and `<slug>-after.png`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 19 adopted plugin partials, rendered via `<img src=...svg>` for `mjun0812`, show the same set of user-visible elements as upstream `lowlighter/metrics`'s output for the same user — verified by visual inspection in a maintainer review of before/after screenshots in each PR
- **SC-002**: Zero plugin emits a bare `<g>` or `<rect>` directly under `<foreignObject>` — enforced by FR-009 compliance test
- **SC-003**: The visual regression suite passes for all 19 adopted plugins in CI; the suite completes within 10 minutes of CI wall-clock for the full sweep
- **SC-004**: A maintainer adding a regression (e.g., dropping a section header from `internal/plugins/languages/partial.go`) is blocked from landing the PR by the failing visual test, with an error message that names the missing element
- **SC-005**: The 010 docs-plugin-gallery feature unblocks: `bash scripts/gen-doc-samples.sh` produces 46 visually correct SVG/PNG samples (was 23 broken samples), and the gallery + hero blocks in README render the actual product faithfully
- **SC-006**: Per-plugin PR cadence holds: from 011 kick-off to all-19-merged is ≤14 calendar days of active development, averaging ~1 plugin per workday
- **SC-007**: Re-baselined golden files are accepted in review only when accompanied by either (a) a before/after screenshot pair in the PR description, or (b) a visual-test-pass screenshot — never a blind `-update` re-baseline without visual validation

## Assumptions

- Upstream lowlighter/metrics classic-template output is the source of truth for visual parity. The upstream EJS templates mirrored under `org_repo/source/templates/classic/partials/` are treated as authoritative; non-classic templates (terminal, repository, etc.) are out of scope for 011 v1.
- chromedp + a chromium binary (already required by the M3 rendering pipeline) is available in the CI environment and in maintainer local environments. The visual-regression test layer reuses the same browser-launch infrastructure; no new dependencies introduced.
- The 19 adopted plugin set per constitution principle III is stable for the duration of 011 — no new plugins added or removed mid-flight.
- Per-plugin visual parity is achievable without modifying the M3 rendering pipeline (chromedp resize, octicon replacement, etc.) — fixes are confined to `internal/plugins/<slug>/partial.go` and possibly the shared `internal/templates/classic/` glue. If a specific plugin requires pipeline changes (e.g., a new chromedp post-process step), that is escalated as a separate task within 011 but not assumed up-front.
- The maintainer (mjun0812) has time and a real user profile to validate each PR's before/after visually. Automated visual diff is out of scope for v1 (the regression suite asserts on structural DOM properties, not pixel diffs).
- v1.0.x consumers using `mjun0812/github-metrics@v1` will see rendering changes as each plugin's PR ships. The 011 feature collectively rolls into a future v1.1.0 release once all 19 plugins land; no semver-breaking changes expected (the action interface is unchanged — only rendered output improves).
- The 010 docs-plugin-gallery feature is paused. Its `scripts/gen-doc-samples.sh` will be re-run after 011 completes; no changes to the 010 spec are required.

## Dependencies

- Upstream EJS templates under `org_repo/source/templates/classic/partials/` must remain in sync with the upstream `lowlighter/metrics` master branch reference for parity validation. If the org_repo/ mirror is stale relative to current upstream, the maintainer refreshes it before starting each plugin's PR.
- chromedp browser-launch infrastructure from M3 (`internal/render/browser.go`, `*render.Browser`). Reused by the visual-regression test layer.
- Existing golden test infrastructure (`internal/testutil/golden/`) for re-baselining bytes after each partial change.
- M9 `tests/compliance/` job for the FR-009 static-check addition.
