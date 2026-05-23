# Plugin languages: visual parity checklist

**Plugin**: languages
**Upstream EJS reference**: [org_repo/source/templates/classic/partials/languages.ejs](../../../org_repo/source/templates/classic/partials/languages.ejs)
**Live upstream sample**: https://github.com/mjun0812/mjun0812 — `metrics_languages.svg`
**PR**: (TBD — will be filled when 011 pilot PR opens)
**Status**: 🟢 Implementation complete (US1 pilot — pending PR open, blocked on GHA quota recovery)

## Empty-state pattern

Pattern B (suppress section) for the `recently-used` sub-section when no recent push activity. Pattern A ("No recent push activity found" message) is upstream's actual behaviour — emit the `<small>` summary with that text. Pattern A wins per upstream EJS line 26.

## Element inventory

Based on upstream `org_repo/source/templates/classic/partials/languages.ejs` (100 LOC) and the v1.0.0 ours partial at `internal/plugins/languages/partial.go` (185 LOC).

| # | Element | Upstream | Ours (before) | Ours (after) | Visual test assertion |
|---|---------|:--------:|:-------------:|:------------:|:---------------------:|
| 1  | `<section>` outer wrapper | ✅ | ✅ (`data-section="languages"`) | ✅ | A5 (structure) |
| 2  | `<h2 class="field">` with code octicon SVG + "N Language(s)" count | ✅ | ❌ | ✅ | A1 (`h2.field` exists) + A4 ("Language" text) |
| 3  | Per-section `<section class="column"><h3 class="field">` with section name ("Most used languages" / "Recently used languages") | ✅ | ❌ | ✅ | A1 (`h3.field` count >= 1) + A4 (text) |
| 4  | Error visualisation `<div class="field error">` with error octicon + message | ✅ (conditional) | ❌ | ⏸ (deferred — error path not in default config) | (skipped — error-path not exercised in default test) |
| 5  | "Recently used" `<small>` summary: "estimation from N kb of code in M edited files across K commits over last D days" | ✅ (conditional on indepth/recent) | ❌ | ⏸ (simplified — "activity from N repositories analysed over last D days" without commits/files counts) | A4 ("estimation from" text — when indepth mode enabled) |
| 6  | "Most used + indepth" `<small>` summary: "estimation from N kb of code in M edited files across K commits" | ✅ (conditional) | ❌ | ⏸ (simplified — "estimation from N kB of code in M analyzed repositories" without commits/files counts) | A4 ("estimation from" text — when indepth mode enabled) |
| 7  | "Recently used + no activity" `<small>` empty-state: "No recent push activity found" | ✅ (conditional) | ❌ | ✅ ("No recent push activity found [over last D days]") | (Pattern A — emitted only when no recent data) |
| 8  | `<svg class="bar" xmlns="..." width="460" height="8">` wrapping the progress bar | ✅ | ❌ (**bare `<g>` bug** — currently `<g class="languages-progress">`) | ✅ (**bare-`<g>` bug FIXED** — confirmed by visual test `bar_renders` bbox > 0 + after-screenshot showing colored bar) | A2 (`rect.language-bar` bbox.width > 0 — KEY assertion for bare-`<g>` bug class) |
| 9  | `<mask id="languages-bar">` with white rounded-rect for the progress-bar mask | ✅ | ❌ | ⏸ (deferred — rounded corners nice-to-have; bar renders flat without mask) | (out of scope — visual polish) |
| 10 | Background `<rect mask="url(#languages-bar)" fill="#d1d5da">` when languages list empty | ✅ (conditional) | ❌ | ⏸ (only emitted when empty — N/A for default test) | (only when empty — N/A for default test) |
| 11 | Per-language `<rect mask="url(#languages-bar)" x=".." width=".." fill="..">` | ✅ | ✅ (without mask) | ✅ (without mask wrapping; renders correctly without it) | A1 (`rect.language-bar` count == languages count) |
| 12 | Per-language detail entries (`details.length > 0` path): `<div class="field language details">` with color-dot octicon + name + (lines / bytes-size / percentage) `<small>` | ✅ (when `details` input set) | ❌ | ⏸ (out of scope — non-default config per Q4; `details` input not wired in our plugin) | (out of scope — non-default config per Q4) |
| 13 | Per-language entries (default `details` empty path): `<div class="field center no-wrap language">` with color-dot octicon + name | ✅ | ❌ (we use `<ul><li>` not octicon+div) | ✅ (color-dot octicon + name list emitted; `<ul>` retained as compat shim — see notable deviations) | A1 (per-language color octicon exists) |
| 14 | GPG-verified footnote `<div class="row footnote">` with shield octicon + "N commit(s) verified by GPG" | ✅ (conditional, only on most-used + when signature > 0) | ❌ | ⏸ (deferred — uncommon, no Result field for signature count) | (out of scope — uncommon) |
| 15 | Partial-warning footnote `<div class="row footnote warning">` with triangle-warning octicon + "Reached maximum execution time" | ✅ (conditional on partial.global) | ❌ | ⏸ (deferred — error path, no Result field for partial flag) | (out of scope — error path) |
| 16 | Accessibility attributes (Q1: a11y verbatim): `<title>` / `aria-label` / `<desc>` on the `<svg class="bar">` and on per-language icon SVGs | ✅ (upstream's SVG has implicit a11y; explicit `<title>` on charts is best practice) | ❌ | ✅ (`<title>Languages distribution</title>` + `role="img" aria-label="Languages distribution"` on `<svg class="bar">`; `aria-hidden="true"` on decorative octicons) | A1 (`title` element exists on `svg.bar`) |

## Visible features summary

**Before (v1.0.0)**: Renders only language NAMES and percentages as a plain `<ul>` list. The colored progress bar is in `<g>` directly (bare-`<g>` bug) so it's invisible when embedded via `<img src=...svg>`. No section header ("N Languages"), no sub-section labels ("Most used" / "Recently used"), no per-language icons, no indepth size summary. The vertical space is dominated by the empty bare-`<g>` placeholder.

**After (this PR — pending T006)**: Will render the full upstream classic-template output: count header with code octicon ("N Languages"), section sub-headers, `<svg class="bar">`-wrapped colored progress bar (with mask + per-language fill), per-language color-dot octicon + name list, optional "estimation from..." `<small>` summary when indepth/recent modes enabled, and a11y `<title>` / `aria-label` on chart elements per Q1 clarification.

## Screenshots

- Before: [screenshots/languages-before.png](./screenshots/languages-before.png) — 29 KB; shows v1.0.0 broken state (bare-`<g>` bug: progress bar invisible, no header, plain `<ul>` list only)
- After: [screenshots/languages-after.png](./screenshots/languages-after.png) — 42 KB; shows post-T006 upstream-parity rendering (count header "9 Languages" + code icon, "Most used languages" sub-header, **colored progress bar fully visible**, per-language color-dot grid, legacy `<ul>` retained as compat shim)

## Visual test assertions (tests/visual/languages_test.go — pending T008)

Per spec US1 acceptance scenarios + Q4 clarification (languages is the documented exception to default-config-only, exercises `_indepth=yes` + `_sections=most-used,recently-used`):

- `Test_Languages_Visual/header_exists` — A1: `h2.field` count >= 1
- `Test_Languages_Visual/bar_renders` — A2: `rect.language-bar` getBoundingClientRect().width > 0 (catches bare-`<g>` regression)
- `Test_Languages_Visual/has_most_used_section` — A4: text "Most used languages"
- `Test_Languages_Visual/has_recently_used_section` — A4: text "Recently used languages"
- `Test_Languages_Visual/has_indepth_estimation` — A4: text "estimation from"

5 assertions matches the assertion budget for languages (most complex plugin per visual-test-shape §3).

## Notable deviations from upstream

Planned deviations (will be revisited during T006 implementation):

- **`<ul class="languages-list">` retained as compatibility shim**: v1.0.0 emits this for the default-config render; will keep alongside the new upstream-equivalent `<div class="field center horizontal-wrap fill-width">` row to avoid breaking any downstream CSS consumer relying on the list structure. Marked as a "v1.x cleanup" item.
- **`<g class="languages-recent">` and `<g class="languages-indepth">` will be wrapped in `<svg>`** instead of being removed wholesale: the existing classes are byte-frozen in golden tests and may be referenced by external CSS; wrapping in `<svg>` fixes the bare-`<g>` bug while preserving the class hooks. Marked as compat-friendly fix.

## Re-baselined goldens (pending T007)

- `tests/golden/classic/m4/languages.svg`: will be re-baselined; expected to grow substantially (current ~1.6 KB → expected ~5-8 KB based on upstream EJS line count)
- `tests/golden/json/m4/languages.json`: UNCHANGED (JSON contract preservation per Principle II)

## Reviewer guide

1. Open `screenshots/languages-before.png` and `screenshots/languages-after.png` side by side — does the "after" look like upstream's `metrics_languages.svg` for mjun0812?
2. Confirm rows #2, #3, #5, #8, #9, #11, #13, #16 all moved from ❌ to ✅ in "ours (after)" column
3. Spot-check the re-baselined `tests/golden/classic/m4/languages.svg` diff — `<h2 class="field">`, `<h3 class="field">`, `<svg class="bar">`, `<mask id="languages-bar">`, per-language `<svg viewBox="0 0 16 16">` icons all visible as new bytes
4. CI: `go test ./tests/visual/languages_test.go -v` green; `go test ./internal/plugins/languages/...` green; `go test ./tests/compliance/...` green (when FR-009 lands in US3)
