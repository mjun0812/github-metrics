# 011 v2 plan — mjun0812 5-plugin focus (replaces v1 19-plugin sweep)

**Date**: 2026-05-19
**Status**: 🟡 In progress — planning recorded; implementation deferred to subsequent sessions
**Supersedes**: 011 v1 (US2 sweep attempt reverted at HEAD 26502f7)

## Why this rewrite

The 011 v1 plan attempted a "minimum viable parity" approach across all
19 adopted plugins (`tasks.md` T011-T028): add a `<h2 class="field">`
header to each partial, wrap any bare `<g>` in `<svg>`. v1 was
implemented and committed (41f4b34 sweep + 3295a36 010-unblock +
384626e CI), then visually validated against `mjun0812`'s actual
upstream SVGs at https://github.com/mjun0812/mjun0812.

**Result**: the output is **structurally a long way from upstream
parity**. Adding a header is cosmetic — the per-plugin partials still
don't:

1. Read upstream's sub-mode inputs (`plugin_languages_details`,
   `plugin_isocalendar_duration`, `plugin_topics_limit`,
   `plugin_topics_mode`, `plugin_calendar_limit`,
   `plugin_sponsors_sections`, `plugin_sponsors_past`, etc.)
2. Faithfully reproduce upstream EJS template's user-visible elements
   (per-language icons, indepth file/commit counts, section sub-headers,
   sponsor-tier groupings, calendar year grids, topic icon variants, etc.)
3. Match upstream's data-fetching logic (most of our plugin Go
   implementations are simplified ports of `org_repo/source/plugins/<slug>/index.mjs`)

v1 commits 41f4b34 + 3295a36 + 384626e have been **reset** (git reset
--hard to 26502f7); HEAD is back at the languages pilot. The
languages pilot itself stays (header + <svg>-wrap + indepth summary
+ a11y) — it remains the gold standard the other plugins should
match.

## New scope: mjun0812 actual workflow plugins

Per mjun0812's `.github/workflows/metrics.yml` (fetched 2026-05-19),
the production SVGs at github.com/mjun0812/mjun0812 use these 7
configurations:

| File | Plugin | Settings |
|---|---|---|
| metrics_base.svg | base only | `base: header, activity, community, repositories, metadata` |
| metrics_languages.svg | languages | `sections: most-used, recently-used`<br>`details: bytes-size, percentage, lines`<br>`recent_load: 500`<br>`recent_days: 30` |
| metrics_isocalendar.svg | isocalendar | `duration: full-year` |
| metrics_topics.svg | topics | `limit: 15` |
| metrics_topics_icons.svg | topics (icons mode) | `limit: 15, mode: icons` |
| metrics_calendar.svg | calendar | `limit: 3` |
| metrics_sponsors.svg | sponsors | `sections: goal, about, list, past: yes` |

All per-plugin SVGs use `base: ""` (NO avatar/header block — pure
plugin-only output).

**Target**: render the same 7 SVGs with our Go pipeline, achieving DOM
structural parity (per constitution principle II — DOM/JSON unit). The
upstream actual SVGs at github.com/mjun0812/mjun0812/raw/main/
metrics_*.svg are the reference; not byte-equivalent, but
structurally equivalent (same `<h2>`/`<h3>`/`<svg class="bar">`/
`<text class="...">` etc. hierarchy with same data).

## Per-plugin work breakdown

### 1. languages (PILOT — partially done at 26502f7)

**Current state**: Header + bar wrap + indepth summary + a11y added.
Per-language color-dot list rendered.

**Remaining for mjun0812 parity**:
- Wire `plugin_languages_details` input (currently ignored). When set
  to `bytes-size, percentage, lines`, the per-language entries must
  show all three: `<small>` with file size + percentage + line count.
  See upstream EJS lines 52-71.
- Wire `plugin_languages_recent_load` / `recent_days` inputs (currently
  ignored). The recent section needs to use these.
- Verify byte-size formatting (upstream uses "%.2f kb of code" etc.;
  our `formatBytes` may produce different precision).
- Verify per-language icon — upstream emits a small color-dot SVG per
  entry. Currently we emit one in the `<div class="field center
  no-wrap language">` block. Visual diff against
  metrics_languages.svg should be close.

**Estimate**: 0.5-1 day to extend pilot to mjun0812 settings parity.

### 2. isocalendar (`duration: full-year`)

**Current state**: Bare-`<g class="calendar">` + bare-`<rect>` per
cell. Returns full year of contribution cells.

**Required**:
- Read `plugin_isocalendar_duration` input (currently ignored). For
  `full-year`, emit the full 52-week × 7-day grid.
- Wrap the grid in `<svg class="calendar" xmlns="..." viewBox=...>`
  with proper dimensions (~700×100px for full year).
- Emit `<rect class="calendar-day">` with proper x/y/width/height
  positioning + color (via `fill` based on count, upstream uses 5
  color levels).
- Emit `<text class="calendar-summary">` below the grid showing total
  contributions + streak max + current streak (upstream EJS line 56+).
- Read `internal/plugins/isocalendar/isocalendar.go` + compare with
  `org_repo/source/plugins/isocalendar/index.mjs` to align color
  thresholds + week-of-year logic.

**Estimate**: 1-1.5 days. Most complex of the 5 due to grid math.

### 3. topics (`limit: 15` + `mode: icons` variant)

**Current state**: Bare-`<g class="topic">` per topic, with
`<image class="topic-icon">` + `<text class="topic-name">`.

**Required**:
- Read `plugin_topics_limit` input (currently ignored — defaults to
  the full topic list; needs limit-to-15 truncation).
- Read `plugin_topics_mode` input (currently ignored). For default
  mode: emit text labels with icons. For `icons` mode: emit only
  the icons (no text), tiled in a grid.
- Wrap `<g class="topic">` in `<svg>` so the icons + text render
  correctly inside foreignObject.
- Compare layout with upstream `metrics_topics.svg` +
  `metrics_topics_icons.svg`.

**Estimate**: 0.5-1 day.

### 4. calendar (`limit: 3`)

**Current state**: Bare-`<g class="calendar-year">` with text content
like "2024 (123)".

**Required**:
- Read `plugin_calendar_limit` input (currently ignored). Limit the
  year list to the most-recent N years.
- Render per-year as a 53-week × 7-day heatmap (similar to
  isocalendar but multi-year stacked).
- Wrap in `<svg>` with appropriate dimensions.
- Emit per-year `<text>` label + per-cell `<rect>` with contribution
  color.
- Reference: `org_repo/source/plugins/calendar/index.mjs` + EJS.

**Estimate**: 1-1.5 days (heatmap rendering is the meaty part).

### 5. sponsors (`sections: goal, about, list, past: yes`)

**Current state**: Bare-`<g class="sponsor">` per sponsor with login +
tier text.

**Required**:
- Read `plugin_sponsors_sections` input (currently ignored). For
  `goal, about, list`, emit three sub-sections:
  - **Goal**: progress bar toward sponsorship goal (if set on profile)
  - **About**: short description text from profile
  - **List**: current sponsor list with avatars + tier badges
- Read `plugin_sponsors_past` input (currently ignored). When `yes`,
  also list past sponsors (separate section).
- Per-sponsor: avatar `<image>` + login `<text>` + tier
  `<text class="sponsor-tier">`.
- Replace bare-`<g>` with HTML `<div>` (since sponsor entries are
  text-only) or wrap in `<svg>` for avatar+text icon rendering.

**Estimate**: 1-1.5 days (3 sub-sections + past variant).

### Total: ~5-7 days of focused work across 5 plugins

## Methodology per plugin

For each plugin, follow this 6-step workflow (per session):

1. **Read upstream EJS template** at
   `org_repo/source/templates/classic/partials/<slug>.ejs` — list
   every visible element + every conditional branch.
2. **Read upstream JS plugin** at
   `org_repo/source/plugins/<slug>/index.mjs` — list every data
   field consumed by the EJS.
3. **Read our current Go plugin** at
   `internal/plugins/<slug>/<slug>.go` + `partial.go` — list every
   data field our `Result` struct provides + every emission in our
   partial.
4. **Identify the gap**: which data fields are missing from our
   `Result`, which sub-mode inputs are unwired in our plugin's
   `Run`, which EJS conditionals are unhandled in our partial.
5. **Port faithfully**: extend `Result` struct (without breaking
   JSON contract per Principle II — additive only), wire the
   missing inputs in `Run`, re-implement the partial to mirror the
   EJS hierarchy.
6. **Visual verify**: render against mjun0812's actual data, compare
   side-by-side with the upstream SVG fetched from
   `github.com/mjun0812/mjun0812/raw/main/metrics_<slug>.svg`.

## What stays from v1

- **Spec-kit baseline** (commit 42badca): spec / plan / research /
  data-model / contracts / quickstart / tasks. Tasks list and
  per-plugin parity checklist template still valid — just narrow
  the scope from 19 plugins to 5.
- **Languages pilot** (commit 26502f7): the v1 implementation is a
  good starting baseline. Will be extended (not redone) for the new
  details-mode + recent-load wiring.
- **Visual test infrastructure** (committed at 26502f7):
  `tests/visual/visual_test.go` harness + `scripts/capture-plugin-screenshot.sh`
  helper. Reusable as-is.
- **010 docs-plugin-gallery foundation** (commit 997d166): unchanged.
  010 remains BLOCKED on 011 — but now 011 means "5-plugin focus" not
  "19-plugin sweep". BLOCKED.md updated accordingly.

## What was rolled back from v1

- 41f4b34 (14-plugin sweep): reverted via reset --hard
- 3295a36 (010 unblock): reverted — 010 is still blocked
- 384626e (CI workflow visual job): reverted — to be re-added when 011
  v2 sweep completes and the visual suite has assertions for all 5
  target plugins
- Per-plugin screenshots for 18 non-languages plugins: discarded
- Re-baselined goldens for 13 non-languages plugins: discarded (back
  to v1.0.0 byte-equivalent)
- `internal/templates/classic/partials/plugin_header.go` helper:
  discarded (will be re-added if 011 v2 needs it; per-plugin headers
  are emitted inline in upstream EJS, not via a shared helper)

## Constitution compliance

All 5 gates remain PASS:

- I. 入力互換性: action.yml + metadata.yml unchanged
- II. 出力契約: this work *moves toward* compliance (DOM parity with
  upstream); JSON shapes will be extended additively (no breaking
  changes)
- III. スコープ規律: confined to 5 plugins from the 21 adopted set;
  no unadopted plugin touched
- IV. テーブルテスト + Golden: existing test infra reused; per-plugin
  goldens re-baselined as each plugin's Go partial is updated
- V. Go 規約: pure Go, no new external deps

## Resume plan

When ready to continue:

1. Pick one of the 5 plugins (recommend: **isocalendar** as next, since
   it's high-visual-impact + structurally similar to the languages
   pilot's bar-wrap pattern).
2. Follow the 6-step methodology above.
3. Capture before/after screenshots via `scripts/capture-plugin-screenshot.sh`.
4. Compare to mjun0812's upstream SVG (fetch with
   `curl -O https://github.com/mjun0812/mjun0812/raw/main/metrics_<slug>.svg`).
5. Commit per-plugin once parity is achieved.
6. Repeat for next plugin.

Target completion: 1 plugin per workday at ~80% parity. Full 5-plugin
sweep complete in ~1-2 calendar weeks of dedicated work.
