# Contract: Per-plugin parity checklist

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-003

Template for the per-plugin parity-checklist Markdown files at
`specs/011-plugin-rendering-parity/plugins/<slug>.md`. One per
adopted plugin. The PR author copies this template at the start of
each plugin's PR, populates it, and updates it as the rewrite
progresses.

---

## File template

```markdown
# Plugin <slug>: visual parity checklist

**Plugin**: <slug>
**Upstream EJS reference**: [org_repo/source/templates/classic/partials/<slug>.ejs](../../../org_repo/source/templates/classic/partials/<slug>.ejs)
**Live upstream sample**: <link to lowlighter/metrics-generated SVG for mjun0812 if available, else "N/A">
**PR**: #<number> (filled in once opened)
**Status**: 🟡 In progress / 🟢 Merged

## Empty-state pattern

(Per R-004 — choose one)

- [ ] Pattern A: emit "No <X> found" message when data is empty
- [ ] Pattern B: suppress the section entirely when data is empty
- [ ] Pattern C: always render with placeholder / scaffold

## Element inventory

| # | Element | Upstream | Ours (before) | Ours (after) | Visual test assertion |
|---|---------|:--------:|:-------------:|:------------:|:---------------------:|
| 1 | `<h2 class="field">` with plugin icon + title | ✅ | ❌ | ✅ | A1 (element exists) + A2 (non-zero bbox) |
| 2 | `<section data-section="...">` per logical sub-section | ✅ | ❌ | ✅ | A5 (hierarchical structure) |
| 3 | Progress bar / chart wrapped in `<svg class="bar">` | ✅ | ❌ (bare `<g>`) | ✅ | A2 (non-zero bbox) |
| 4 | Per-entry icon (octicon or language color dot) | ✅ | ❌ | ✅ | A1 (element exists) |
| 5 | Summary text / statistics (indepth, recent counts) | ✅ | ❌ | ✅ | A4 (text content) |
| 6 | Empty-state markup (Pattern A/B/C per above) | ✅ | ❌ | ✅ | A5 (presence or absence) |

## Visible features SUMMARY

**Before (v1.0.0)**: <2-3 sentence prose summary of what a viewer saw before the rewrite. Cite specific missing elements from the inventory above.>

**After (this PR)**: <2-3 sentence prose summary of what a viewer sees after the rewrite. Should be drop-in equivalent to the upstream output for the same user.>

## Screenshots

- Before: [screenshots/<slug>-before.png](./screenshots/<slug>-before.png) (captured at SHA: <parent-commit-sha>)
- After: [screenshots/<slug>-after.png](./screenshots/<slug>-after.png) (captured at SHA: <PR-head-sha>)

## Visual test assertions

The PR's `tests/visual/<slug>_test.go` exercises:

- `Test<Slug>_Visual/header_exists` — A1: assert `<h2 class="field">` element count >= 1
- `Test<Slug>_Visual/bar_renders` — A2: assert `<rect class="X-bar">` getBoundingClientRect().width > 0
- `Test<Slug>_Visual/has_section_data_section_X` — A5: assert section element exists
- (optional) `Test<Slug>_Visual/has_count_text` — A4: assert innerText includes "27 Languages" (or equivalent)

## Notable deviations from upstream

(List any places where ours intentionally differs from upstream and why.
Empty list = full parity.)

- (none / e.g., "Use `<small>` tag instead of upstream's inline span for the indepth summary — equivalent semantics, less HTML")

## Re-baselined goldens

- `tests/golden/classic/m4/<slug>.svg`: re-baselined (was N KB, now M KB, +X%)
- `tests/golden/json/m4/<slug>.json`: unchanged (JSON contract preserved per Principle II)

## Reviewer guide

1. Open `screenshots/<slug>-before.png` and `screenshots/<slug>-after.png` side by side — does the "after" look like upstream's equivalent output?
2. Confirm each row in the inventory has ✅ in the "ours (after)" column
3. Spot-check the re-baselined byte golden — major structural elements (section headers, progress bars) should be visible in the diff
4. CI: `go test ./tests/visual/<slug>_test.go` green; full `go test ./...` green
```

---

## Why this template

- **Element inventory table** forces the author to enumerate parity items rather than eyeball-comparing — catches gaps that are easy to miss
- **Empty-state pattern** is called out explicitly per R-004 because it's a frequent source of regression (empty boxes consuming space)
- **Notable deviations section** allows ours-vs-upstream divergence to be documented inline (not hidden in code comments)
- **Reviewer guide** at the bottom = the reviewer doesn't need to read this entire spec; they have a 4-step procedure

## When does the template get authored?

- **Languages (US1 pilot)**: drafted as part of the US1 PR. Per-plugin checklist for languages becomes the de-facto template the other 18 follow
- **Remaining 18 (US2)**: each PR author copies this template + populates per-plugin

## How does the template get validated?

- The compliance test FR-009 (`TestCompliance_PluginPartialNoBareSVGChildren`) statically asserts no bare-`<g>` remains; doesn't validate the checklist Markdown structure
- Checklist completeness is a reviewer responsibility (per the "Reviewer guide" at the bottom of each instance)
