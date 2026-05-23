# Data Model: Plugin rendering parity with upstream EJS templates

**Date**: 2026-05-19 | **Plan**: [./plan.md](./plan.md)

This feature's "data" is filesystem artefacts (Go source files, test files, screenshot PNGs, markdown checklists) rather than runtime data structures. Entities below describe what is produced / consumed / mutated by the per-plugin PR pipeline.

---

## E-001: Plugin partial source (existing entity, mutated)

**File location**: `internal/plugins/<slug>/partial.go` (one per adopted plugin, 19 files)

**Mutated fields** (per PR):

- Function body of `Partial(_ context.Context, pc *templates.PartialContext) (string, error)` — re-emits the upstream-equivalent markup
- Possibly: new helper functions (`writeHeader`, `writeIndepthSection` etc.) for clarity
- Possibly: new imports (`internal/render` for octicon helper, etc.)

**Invariants**:

- Function signature unchanged (call sites in `internal/templates/classic/*` unaffected)
- Return value is `(string, error)` — never panics
- Error returned only on truly exceptional conditions (template parse failure etc.); empty-data scenarios return an empty-state markup per R-004, not an error
- No new dependencies on packages outside `internal/`

**Validation rules**:

- New code passes `go vet`, `gofumpt`, `golangci-lint` (constitution PR gate §1)
- Existing table tests in `internal/plugins/<slug>/partial_test.go` extended with at least 1 new case per added feature (header text, empty-state markup, etc.)
- Byte-golden file at `tests/golden/classic/m4/<slug>.svg` re-baselined with `-update`

---

## E-002: Visual regression test file (new entity)

**File location**: `tests/visual/<slug>_test.go` (one per adopted plugin, 19 files)

**Fields**:

- `func Test<Slug>_Visual(t *testing.T)` — top-level test function
- One sub-test per assertion (3-5 sub-tests per plugin, per R-002)
- Each sub-test:
  - Renders the plugin's SVG via the shared `renderForVisualTest(t, plugin, inputs)` helper (`tests/visual/visual_test.go`)
  - Opens it in a chromedp tab against `about:blank` HTML wrapper
  - Runs a `chromedp.Evaluate` for one of the 5 assertion shapes (R-002 menu)
  - Asserts on the returned value with an actionable failure message

**Invariants**:

- Each test file is independent — failures in one plugin don't cascade
- Test files share a single `renderForVisualTest` helper + a single browser instance (recycled via M3 `*render.Browser`)
- All assertions use `t.Errorf` (not `t.Fatalf`) so a single plugin can report all its failing assertions at once

**Validation rules**:

- Uses the chromedp build tag pattern (`//go:build chromedp` if needed) so non-chromedp environments can skip; CI always runs with chromedp tag enabled
- 3-5 assertions per file — sub-tests named `Test<Slug>_Visual/<assertion_name>`
- Visual test for chromedp-dependent plugins (topics, starlists) uses mocked plugin data per R-003

---

## E-003: Per-plugin parity checklist (new entity)

**File location**: `specs/011-plugin-rendering-parity/plugins/<slug>.md` (one per adopted plugin, 19 files)

**Fields**:

- **Title**: "Plugin <slug>: visual parity checklist"
- **Upstream reference**: link to `org_repo/source/templates/classic/partials/<slug>.ejs` + the live upstream SVG for mjun0812 (if accessible)
- **Element inventory**: Markdown table listing each user-visible element in upstream's output:
  - Column 1: element label (e.g., "Languages count header `<h2>`")
  - Column 2: upstream status (✅ present / ❌ absent for the test user)
  - Column 3: ours-before status (✅ / ❌ at PR start)
  - Column 4: ours-after status (✅ / ❌ at PR end — populated by PR author)
  - Column 5: assertion ID (`A1`-`A5` from R-002 menu — linked to the corresponding visual test sub-test)
- **Empty-state pattern** (R-004): one of {A: "No X found" message, B: suppress section, C: always render placeholder}
- **Screenshot diff**: links to `screenshots/<slug>-before.png` + `screenshots/<slug>-after.png`
- **PR link**: populated when the PR opens

**Invariants**:

- File exists for all 19 plugins after US2 completes (one per PR)
- Pilot file (languages) is the template — authored during US1
- ✅ count in "ours-after" column equals or exceeds "upstream" count (the PR brings ours up to parity, but ours may have features upstream lacks — those are preserved)

---

## E-004: Visual test browser bootstrap (new entity, shared across all 19 visual tests)

**File location**: `tests/visual/visual_test.go`

**Fields**:

- `TestMain(m *testing.M)` — sets up + tears down the shared chromedp browser instance once per `go test ./tests/visual/...` invocation
- `renderForVisualTest(t *testing.T, plugin string, inputs map[string]string) string` — renders the plugin's SVG via the existing `internal/action` pipeline with `dryrun=yes`, returns the raw SVG string
- `evalInBrowser(t *testing.T, svg string, expr string) any` — opens the SVG in `about:blank` HTML wrapper, runs `chromedp.Evaluate(expr, &result)`, returns result
- `assertBoundingBoxNonZero(t *testing.T, svg string, selector string)` — convenience for the most-used assertion shape (R-002 A2)
- `assertElementExists(t *testing.T, svg string, selector string, minCount int)` — convenience (R-002 A1)
- `assertTextContent(t *testing.T, svg string, expectedSubstring string)` — convenience (R-002 A4)

**Invariants**:

- Single browser instance recycled across all 19 plugin tests in one `go test` run (matches M3 R-002 recycling pattern)
- Browser is torn down via `defer` in `TestMain` even on test failure
- The HTML wrapper served to `about:blank` uses `<img src="data:image/svg+xml;base64,...">` so the SVG is rendered in `<img>` context (matches GitHub's render path)

**Validation rules**:

- TestMain detects missing chromedp / chromium and skips the entire suite with a clear message (don't fail; some envs lack chromium)
- Helper functions are stable API across the 19 visual tests — changing them requires updating all 19 tests

---

## E-005: Re-baselined byte golden (existing entity, mutated)

**File location**: `tests/golden/classic/m4/<slug>.svg` (one per adopted plugin, 19 files)

**Mutated fields** (per PR):

- The full file contents — re-emitted by the partial after rewrite
- The expected byte sequence becomes whatever the new partial produces

**Invariants**:

- Re-baseline only happens via `go test -update` flag (existing M9 contract, unchanged)
- Re-baselined file must be accompanied by a before/after screenshot (E-006) + parity checklist update (E-003) in the same PR (per spec SC-007)
- JSON shape golden (`tests/golden/json/m4/<slug>.json`) is NOT re-baselined unless the JSON contract changes — and per Principle II.JSON it should not change

**Validation rules**:

- New file passes the existing M9 `TestM9_SVGGoldensCompareAfterUpdate` golden-vs-fixture round-trip
- File size delta logged in PR description (large deltas — say >50% — get extra reviewer attention)

---

## E-006: Per-plugin screenshot pair (new entity)

**File location**: `specs/011-plugin-rendering-parity/plugins/screenshots/<slug>-{before,after}.png` (two per adopted plugin, 38 files total after US2)

**Fields**:

- `<slug>-before.png`: the plugin's rendered output BEFORE the PR's partial rewrite (i.e., the v1.0.0 state)
- `<slug>-after.png`: the plugin's rendered output AFTER the PR's partial rewrite

**Generation**: via `bash scripts/capture-plugin-screenshot.sh <slug>` (new helper script — added in US1)

**Invariants**:

- Both PNGs use the same viewport (720x800), same user (mjun0812), same input set (`plugin_<slug>=yes` default config)
- "Before" is captured against the parent commit (or fetched from a tagged baseline branch)
- "After" is captured against the PR's head commit
- Both PNGs are committed alongside the partial.go change in the same PR

**Validation rules**:

- File names match the slug exactly (compliance test could enforce, but lightweight: PR review catches typos)
- Files are PNG, not SVG/PDF (deterministic raster for reviewer comparison)
- File size <500 KB each (720x800 PNG of a metrics panel typically ~100-300 KB)
