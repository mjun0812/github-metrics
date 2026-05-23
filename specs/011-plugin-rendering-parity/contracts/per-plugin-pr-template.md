# Contract: Per-plugin PR description template

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [./partial-parity-checklist.md](./partial-parity-checklist.md)

Template for the PR description used when landing each plugin's
parity rewrite. The 19 per-plugin PRs of US2 each follow this exact
shape so reviewers can validate quickly without re-running rendering.

---

## File template (PR description body)

```markdown
## What

Bring `internal/plugins/<slug>/partial.go` to upstream classic-template
visual parity per [011 spec §US2](../../specs/011-plugin-rendering-parity/spec.md#user-story-2---all-19-adopted-plugins-render-at-upstream-parity).

Closes #<issue-number> (one of the 19 011 issues — to be linked).

## Why

The v1.0.0 partial emitted [list specific missing elements: e.g.,
"no `<h2>` header, no progress bar wrapped in `<svg>`, no per-entry
icons"]. When the rendered SVG is embedded via `<img src=...svg>` on
github.com, the missing elements cause [observed user impact: e.g.,
"the language bars are invisible because bare `<g>` elements drop
silently in foreignObject HTML context"].

## What changed

- Rewrote `internal/plugins/<slug>/partial.go` to match upstream
  `org_repo/source/templates/classic/partials/<slug>.ejs` per the
  per-plugin parity checklist
  ([specs/011-plugin-rendering-parity/plugins/<slug>.md](../../specs/011-plugin-rendering-parity/plugins/<slug>.md))
- Re-baselined `tests/golden/classic/m4/<slug>.svg` via `go test -update`
- Added `tests/visual/<slug>_test.go` with N visual-regression assertions
- Added before/after screenshots: [before](../../specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-before.png) | [after](../../specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-after.png)
- Added per-plugin parity note: [plugins/<slug>.md](../../specs/011-plugin-rendering-parity/plugins/<slug>.md)

## Parity checklist diff

| Element | Before | After | Visual test |
|---------|:------:|:-----:|:------------|
| Header (`<h2>`) | ❌ | ✅ | A1 |
| Progress bar (`<svg>`-wrapped) | ❌ | ✅ | A2 (bbox non-zero) |
| Section headers (`<h3>`) | ❌ | ✅ | A4 (text content) |
| Per-entry icons | ❌ | ✅ | A1 |
| Indepth/recent stats | ❌ | ✅ | A4 |

(See full checklist at `plugins/<slug>.md` for context.)

## Visual proof

<img src="https://github.com/mjun0812/github-metrics/raw/<sha>/specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-before.png" width="360" /> <img src="https://github.com/mjun0812/github-metrics/raw/<sha>/specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-after.png" width="360" />

## Test plan

- [ ] `go test ./internal/plugins/<slug>/...` green (table tests for
      new helpers + re-baselined byte golden)
- [ ] `go test ./tests/visual/<slug>_test.go -v` green (N assertions
      pass against `mjun0812` data)
- [ ] `go test ./tests/compliance/...` green (FR-009 static-check
      still passes — no new bare-`<g>` introduced)
- [ ] Manual: build local docker image, render the plugin, open in
      browser, eyeball-confirm match with after-screenshot

## Re-baselined golden file delta

`tests/golden/classic/m4/<slug>.svg`: <X> bytes → <Y> bytes (<+/-Z%>)

Largest structural changes:
- (list the 2-3 most impactful changes, e.g., "added <h2> header
  with octicon path", "wrapped progress bar in <svg class="bar">")

## Constitution gates

- I. Input compatibility: ✅ no `action.yml` / `metadata.yml` changes
- II. Output contract (DOM): ✅ closes a pre-existing violation —
  rendered output now matches upstream user-visible elements; JSON
  shape unchanged
- III. Scope discipline: ✅ confined to the adopted plugin set
- IV. Table tests + golden file: ✅ re-baselined goldens; new visual
  test layer adds DOM-level assertion coverage
- V. Go conventions: ✅ no new external dependencies; English code
  comments preserved

## Notable deviations from upstream

(None — full parity. OR: list any intentional differences.)
```

---

## When to use this template

- All 19 US2 per-plugin PRs use this exact shape
- The US1 pilot PR (languages) can use a slightly extended version
  that also documents the helper-script + visual-test-harness setup
  (`scripts/capture-plugin-screenshot.sh`, `tests/visual/visual_test.go`)
- US3 (visual regression CI integration + compliance test) uses a
  different shape — that's a single PR covering infrastructure, not
  parity work

## What the reviewer focuses on

1. **Open the before/after screenshots** — visual delta is the
   primary acceptance signal
2. **Scan the parity-checklist diff table** — every ❌→✅ should
   correspond to a code change
3. **Confirm CI green** — visual test for the plugin passes
4. **Spot-check the re-baselined golden diff** — major structural
   adds should be visible in the byte diff (don't read every byte)

## What the reviewer does NOT do

- Re-run rendering locally (the committed screenshots are sufficient)
- Verify byte-perfect match with upstream (we don't promise that —
  only structural / visual equivalence)
- Approve a re-baselined golden without screenshots (per SC-007)

## Anti-patterns to flag

- ❌ Re-baselined golden without before/after screenshots in PR
- ❌ Visual test has <3 assertions (too weak; menu R-002 is the
  guideline)
- ❌ Empty-state pattern (R-004) unspecified in the parity checklist
- ❌ Per-plugin parity note (`plugins/<slug>.md`) not created or not
  populated
- ❌ PR touches multiple plugins (violates the 1-plugin-per-PR rule
  from spec Clarification §3)
