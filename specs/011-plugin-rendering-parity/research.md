# Research: Plugin rendering parity with upstream EJS templates

**Date**: 2026-05-19 | **Plan**: [./plan.md](./plan.md)

5 Phase-0 research decisions resolving the technical unknowns in the spec. Each entry follows: **Decision** / **Rationale** / **Alternatives considered**.

---

## R-001: Defining "feature parity" per plugin (the rewrite spec)

**Decision**: For each plugin's PR, the spec input is **(a) the upstream EJS template at `org_repo/source/templates/classic/partials/<slug>.ejs` as the reference of user-visible elements**, plus **(b) a side-by-side rendered comparison** between upstream's actual SVG output (fetched from `lowlighter/metrics:latest` Docker image run against mjun0812) and ours. The PR author writes a per-plugin parity checklist (`specs/011-plugin-rendering-parity/plugins/<slug>.md`) listing each upstream element and its presence/absence in our output before starting the rewrite.

**Rationale**: The EJS template alone is not sufficient — it has conditional branches (`<% if ... %>`) gated on plugin output shape, and some upstream features (e.g., language icons) are emitted by helper functions outside the partial file. A rendered diff against actual upstream output for the *same user* captures the practical visible delta. The per-plugin checklist forces explicit enumeration so nothing gets missed.

**Alternatives considered**:

- **EJS template only**: rejected — misses helper-emitted content (icons, computed labels) and risks parity-by-template-fidelity without parity-by-visible-output
- **Pixel-diff against rendered upstream**: rejected — too sensitive to font/anti-aliasing variations across environments; fails reproducibly
- **Maintainer eyeball alone (no checklist)**: rejected — 19 plugins × 5-10 features each = ~150 distinct elements to track. Without a checklist, things slip

---

## R-002: Visual test assertion strategy

**Decision**: Each plugin's visual test (`tests/visual/<plugin>_test.go`) asserts on **3-5 specific DOM-level properties** via `chromedp.Evaluate` queries, chosen per-plugin from the following menu:

1. **Element existence**: `document.querySelectorAll('rect.language-bar').length >= 1`
2. **Rendered bounding box**: `document.querySelector('rect.language-bar').getBoundingClientRect().width > 0` (catches the bare-`<g>` bug — invisible elements have zero rendered width)
3. **Computed style sanity**: `getComputedStyle(document.querySelector('h2.field')).display !== 'none'`
4. **Text content presence**: `document.body.innerText.includes('Languages')`
5. **Hierarchical structure**: `document.querySelectorAll('section[data-section="languages"] > h2').length === 1`

Per-plugin assertions are chosen from this menu based on what's most representative of that plugin's user-visible features. Maintainer authoring a new plugin's visual test consults the per-plugin parity checklist (R-001) and translates each "must be present" item into one of the 5 assertion shapes above.

**Rationale**: Pixel diffs are too brittle; pure presence-of-element-in-XML is too weak (catches missing elements but not the bare-`<g>` invisible-render bug). Bounding-box `> 0` is the killer assertion for the bare-`<g>` class because invisible elements have zero rendered dimensions even when present in the DOM. Computed style + text content cover the rest of the parity surface. The 3-5 count per plugin is calibrated so failure messages stay actionable (single-cause per assertion).

**Alternatives considered**:

- **Pixel-diff vs upstream**: rejected — see R-001
- **`document.outerHTML` snapshot**: rejected — equivalent to byte-golden but viewed through chromedp lens; doesn't catch invisible-render bugs because the markup is "present" even when invisible
- **Lighthouse / Puppeteer screenshot comparison via 3rd-party services**: rejected — adds network dependency to CI, costs money, slow
- **Accessibility-tree (axe-core) assertion**: rejected as scope creep — useful but a separate concern (accessibility is its own feature spec, not parity)

---

## R-003: chromedp-gated plugins (topics, starlists) in visual tests

**Decision**: Treat topics + starlists visual tests as **always-running but always-permissive**: the test sets `INPUT_PLUGIN_TOPICS=yes` etc., invokes the renderer in `--dryrun` mode with mocked plugin data (no real chromedp scraping — the upstream chromedp scrape feeds the partial's data, but the partial renders pure SVG/HTML from that data). The visual test asserts on the **rendered partial structure** independent of whether the chromedp scrape produced any data. If the plugin's partial outputs `Skipped=true` style fallback markup, the visual test asserts on the fallback structure; if it outputs full markup with seeded data, it asserts on the full structure.

**Rationale**: The visual test's concern is "does the plugin's render output look right", not "does the upstream chromedp scraper work". Mocking the plugin data (e.g., 5 fake topics with names + star counts) decouples the visual test from chromedp scrape availability + network conditions, while still exercising the full template emission path. This mirrors the existing M4 P3 plugin test pattern where chromedp scraping is mocked behind `internal/testutil/mocks/`.

**Alternatives considered**:

- **Build-tag isolation (`//go:build chromedp_visual`)**: rejected — fragments the CI surface; the visual test for topics doesn't actually need chromedp scrape (only chromedp browser for the *visual rendering*, which we need anyway)
- **Skip topics/starlists from visual suite entirely**: rejected — bypasses the highest-value plugins from the spec's perspective (chromedp-gated plugins were the most likely to have subtle render bugs)
- **Run real upstream scrape against fixture URL**: rejected — flaky, slow, requires network access in CI

---

## R-004: Empty-section suppression strategy (FR-003)

**Decision**: For each plugin partial, the rewrite implements **upstream's empty-state behavior verbatim** — which generally means one of three patterns per upstream EJS template:

- **Pattern A** ("No X found" message): emit a small `<div class="field empty">` with text. Applies to: `sponsors` (no sponsors), `notable` (no notable contributions), `starlists` (no lists).
- **Pattern B** (suppress section entirely): omit the `<section>` element when the data slice is empty. Applies to: `recent` languages section when no recent commits, `contributors` when no other-than-self contributors found.
- **Pattern C** (always render with placeholder): show a "Loading..." or minimal scaffold. Applies to: `calendar`, `isocalendar` (always render the grid even if all cells are zero — empty grid is meaningful).

The per-plugin parity checklist (R-001) calls out which pattern that plugin uses; the partial implements it; the visual test asserts on the corresponding behavior (e.g., for Pattern B: `document.querySelectorAll('section[data-section="recent"]').length === 0` when data is empty).

**Rationale**: Upstream has already made these decisions per-plugin and they are part of the user-visible parity goal. Inventing our own empty-state strategy would re-introduce drift from upstream. The 3-pattern menu covers all 19 plugins per spot-check of upstream EJS templates.

**Alternatives considered**:

- **Uniform "suppress always when empty"**: rejected — breaks Pattern C (calendar should always render the grid)
- **Uniform "show placeholder always"**: rejected — produces "Loading..." artifacts in the final output, doesn't match upstream
- **Defer empty-state to a follow-up feature**: rejected — empty-section vertical-space-consuming boxes are one of the visible defects in the 010 trial; can't ship 011 calling it "parity" if this is unaddressed

---

## R-005: PR review approach — validating before/after without re-running

**Decision**: Each per-plugin PR description MUST include:

1. A **before/after screenshot pair** (`specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-before.png` + `<slug>-after.png`) — committed to the repo alongside the partial.go changes
2. A **parity checklist diff** linking to the per-plugin `plugins/<slug>.md` showing which checklist items moved from "missing" to "present"
3. **CI status**: green visual test for that plugin (and unchanged for the other 18, when they run)

The reviewer validates by looking at the screenshot pair (does it look like upstream?) and the checklist diff (are the right items now ✅?). No need to re-run the rendering pipeline locally unless the reviewer wants to spot-check.

Screenshots are generated by the PR author via a small helper script: `bash scripts/capture-plugin-screenshot.sh <slug>` which (a) renders the plugin's SVG via the local docker image, (b) opens it in headless Chromium via `<img>` wrapper HTML, (c) takes a screenshot at fixed 720x800 viewport.

**Rationale**: Re-running rendering for each PR review is high-friction (requires GITHUB_TOKEN + docker + chromium setup) and would gate review on environment readiness. Committed screenshots + per-plugin parity checklist diff = reviewer has all the context inline. The capture script is small enough to add as part of US1 (pilot PR).

**Alternatives considered**:

- **Re-rendering at review time**: rejected — too high friction
- **Screenshots as PR attachments only (not committed)**: rejected — PR attachments expire / get re-uploaded on edit; committed screenshots survive in history for diagnosing future regressions
- **Auto-generate screenshots in CI**: rejected for v1 — CI doesn't have a valid GITHUB_TOKEN for mjun0812 data; might add later via a fixture-data fallback. v1 captures screenshots locally
- **Skip screenshots entirely**: rejected — visual parity is the spec's whole point; the artefact of "what changed visually" must be inspectable

---

## Plan-phase risks

The following risks are surfaced now so the per-plugin PR cadence can pace around them, not discovered mid-flight:

| Risk | Likelihood | Mitigation |
|---|---|---|
| Upstream EJS template uses a feature we have not wired (e.g., custom CSS class for color theming) | Medium — 3-4 plugins likely | Per-plugin PR scope is "parity for default-config rendering"; non-default theming is explicitly out of scope per spec Assumptions. PR notes when this happens and tracks separately. |
| Re-baselined golden bytes diff is enormous (e.g., 80% of bytes change) — hard to eyeball-review | High — every plugin | Per spec SC-007, golden diff is reviewed alongside before/after screenshots. Reviewer trusts the screenshot, not the golden diff. |
| chromedp tab spin-up cost makes the 19-plugin visual suite exceed 10 min wall-clock | Low | Per R-002, each plugin's test makes 3-5 evaluations against a single tab. With `*render.Browser` recycling (existing M3 R-002), per-plugin cost is dominated by browser launch (~5 sec) + render (~2 sec) + assertions (~1 sec) = 8 sec. 19 × 8 = ~2.5 min. Comfortably within budget. |
| A plugin's upstream EJS uses a feature that requires M3 pipeline changes (e.g., a new chromedp post-process) | Medium — 1-2 plugins | Per plan Assumptions, plugin-only fixes are the default scope; pipeline changes escalate to a separate task within 011. Plan budget includes 1-2 day buffer for this. |
| Maintainer (mjun0812) finds a plugin's "parity" output doesn't match their own preference / visual taste | Low | Spec is explicit: parity with upstream, not a redesign. If maintainer wants visual changes beyond parity, that is a separate v1.x feature, not 011 |
