# Feature Specification: Per-plugin docs with real example outputs

**Feature Branch**: `010-docs-plugin-gallery`

**Created**: 2026-05-19

**Status**: Draft

**Input**: User description: "このリポジトリで生成されるものがどんなものかを実際の画像やsvgを使って、README.mdに添付しないとと何もわからないと思います。ローカルで実行して、README.mdとdocs以下にpluginごとのmarkdown fileを作成して、そこに貼るようにしてください。pluginごとのdocには設定の仕様なども書きましょう。userはmjun0812(私)を使っていいです"

## Overview

The current README.md describes what `github-metrics` produces in prose
("Generate SVG / PNG / JPEG / JSON metrics …") but a first-time
visitor has no way to *see* an actual output until they install +
configure + run the tool. For an OSS project where the dominant
adoption question is "what does the output look like?", this is a
hard barrier.

This feature closes the gap by:

- **Adding hero example images** at the top of README.md showing the
  two adopted templates (`classic` + `repository`) in action.
- **Creating per-plugin doc pages** under `docs/plugins/<slug>.md`,
  each containing a rendered sample image plus the full
  configuration spec extracted from `assets/plugins/<slug>/metadata.yml`.
- **Wiring a maintainer-facing make target** (`make docs-examples`)
  that regenerates the complete example set from a clean checkout
  using the GitHub user **`mjun0812`** (the project maintainer's
  account) as the data source.

The committed example assets stay in sync with the rendering code via
explicit regeneration; CI does not auto-render.

## Clarifications

### Session 2026-05-19

- Q: README の既存「Plugins」セクション (tier 別 markdown table) を新 gallery でどう扱うか? → A: Option A — 置換 (gallery が唯一の plugin インデックスになる; tier 情報は各 per-plugin doc 内で記述)
- Q: Plugin doc の人間執筆ゾーン (TODO 残置) を CI で gate するか? → A: Option C — Hybrid: 初回 merge は loose (doc 存在のみ gate)、後続で strict 化するロードマップを CONTRIBUTING.md に追記

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-time visitor sees concrete examples on README (Priority: P1)

A developer browsing the repo on github.com opens README.md. Within
30 seconds, they have seen at least one rendered example showing
**what kind of image the project produces** — the classic user
profile metrics SVG. They also see (via a small gallery) that 19
plugins each contribute panels to that output and that a second
template (`repository`) re-centers the rendering on a single repo.

**Why this priority**: P1 because this is the #1 adoption barrier
for an output-generation tool. Without visible examples, README.md
is abstract prose; with examples the value proposition is
self-evident.

**Independent Test**: A fresh viewer opens README.md on
[github.com/mjun0812/github-metrics](https://github.com/mjun0812/github-metrics)
in a browser and within 30 seconds can answer:
(a) "What does the project produce?" (visually — an SVG with
profile metrics panels)
(b) "What is the difference between classic and repository
templates?" (two side-by-side hero images)
(c) "Which plugins contribute panels to the output?" (gallery
listing all 19).

**Acceptance Scenarios**:

1. **Given** a fresh github.com viewer with no prior context,
   **When** they open README.md, **Then** the first visible image
   below the project description renders a real metrics SVG within
   the page's initial viewport (not below a scroll fold).
2. **Given** the same viewer, **When** they scroll past the Quick
   Start section, **Then** a "Plugins" gallery section lists each
   of the 19 adopted plugins with a thumbnail and a link to the
   per-plugin doc.

---

### User Story 2 - User explores a specific plugin's doc (Priority: P2)

A user evaluating whether plugin X solves their need clicks through
from README's plugin gallery to `docs/plugins/X.md`. On that page they
see:

- A rendered sample showing what panel plugin X produces
- A 1-paragraph description of what the plugin computes
- A configuration table listing every input the plugin accepts
  (`plugin_X`, `plugin_X_limit`, `plugin_X_*`, …) with description /
  default / required columns
- A copy-paste Action-mode usage snippet
- A pointer back to the canonical input matrix at `action.yml`

**Why this priority**: P2 because discovery and configuration are
entangled — users need to *see* output AND know *how* to configure
it. The current single `action.yml` is a flat list of 200+ inputs
that is hard to navigate for someone interested in one plugin.

**Independent Test**: A user lands on
`docs/plugins/languages.md` (linked from README) and can answer
within 5 minutes:
(a) "What does the languages panel look like?" (image visible)
(b) "How do I enable it?" (usage snippet)
(c) "How do I filter out certain languages?" (input table row
`plugin_languages_ignored`)
(d) "Are there sub-modes?" (the doc covers recent + indepth + default).

**Acceptance Scenarios**:

1. **Given** the maintainer has run `make docs-examples` and
   committed the result, **When** a user opens
   `docs/plugins/languages.md` on github.com, **Then** the sample
   SVG image renders inline (no broken-link icon).
2. **Given** the same doc, **When** the user inspects the
   "Configuration" table, **Then** every input declared in
   `assets/plugins/languages/metadata.yml` (currently 13+ inputs:
   `plugin_languages`, `plugin_languages_ignored`,
   `plugin_languages_skipped`, `plugin_languages_limit`,
   `plugin_languages_threshold`, …) is present with a description.

---

### User Story 3 - Maintainer regenerates the example set after a render change (Priority: P3)

After a Dockerfile bump (e.g., chromium upgrade), template change
(e.g., adding a new partial), or plugin metadata edit (e.g., a new
input), the maintainer runs **one make target** to regenerate the
full example set against their own GitHub data. The committed
examples stay in sync with the current rendering code.

**Why this priority**: P3 because without this, examples drift from
actual output over time (the same problem upstream `lowlighter/metrics`
partially faces). It's a maintainability concern, not a first-time
adopter concern.

**Independent Test**: From a clean checkout, with `GITHUB_TOKEN` set
and chromium available on PATH:

```sh
make docs-examples
```

Should regenerate all sample images under `docs/examples/` (or the
equivalent location) within 5 minutes (per SC-004) and produce a
diff that consists only of byte changes in the rendered files — no
new doc pages auto-created (the per-plugin .md skeletons are
human-authored).

**Acceptance Scenarios**:

1. **Given** a fresh checkout with chromium + GITHUB_TOKEN
   configured, **When** the maintainer runs `make docs-examples`,
   **Then** the full example set regenerates within 5 minutes and
   `git status` shows only changes to the rendered image files +
   any auto-extracted input table fragments (no source-code
   changes).
2. **Given** the maintainer adds a new input to a plugin's
   `metadata.yml`, **When** they re-run `make docs-examples`,
   **Then** the affected plugin doc's configuration table includes
   the new input row.

---

### Edge Cases

- **Plugin requires auth scope not in maintainer's PAT** (e.g.,
  `sponsors` data needs `read:user`, `traffic` needs `repo:admin`)
  → the make target documents the required token scopes; if a scope
  is missing the affected plugin's sample renders with an empty /
  placeholder panel. The plugin's doc page explicitly notes the
  scope requirement.
- **Plugin has no data for `mjun0812`** (e.g., no GitHub Projects
  yet, no sponsors) → the empty-state sample is committed as-is,
  with a note in the plugin doc that the panel can render richer
  content for users with applicable data.
- **chromedp-gated plugins** (`topics`, `starlists`) → require
  chromium on the maintainer's host. The make target depends on
  `METRICS_CHROME_PATH` per existing M3 conventions. CONTRIBUTING.md
  documents this precondition.
- **Long-tail panel sizes** — some plugins render small (5 stars),
  some large (full activity timeline). README's plugin gallery uses
  uniform-sized thumbnails (e.g., a fixed-height crop or a small
  preview) so the README layout stays readable regardless of which
  plugins are enabled.
- **Output reproducibility** — dynamic strings in the SVG output
  (version footer, "Last updated" timestamp) differ per build. The
  M9 `NormalizeSVG` mask is reused so committed SVGs only diff when
  semantic content changes; cosmetic noise (timestamps) is
  suppressed at write time. Alternatively, the committed SVGs may
  keep dynamic strings and accept the small diff noise — the plan
  phase decides.
- **README image rendering on github.com** — github.com renders
  embedded SVGs inline. PNG fallback is available if SVG rendering
  has known issues; the plan phase decides whether to commit only
  SVG (smaller, scalable) or also PNG (most-compatible).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: README.md MUST display at least 2 hero example images
  near the top (before the Quick Start section): one for the
  `classic` template and one for the `repository` template. Both
  rendered against user `mjun0812`'s GitHub data.
- **FR-002**: README.md MUST include a **Plugins** gallery section
  that lists every adopted plugin with a small representation
  (thumbnail OR text-with-link) and a link to the per-plugin doc
  page. The new gallery **replaces** the existing tier-based
  `## Plugins` table (the table is removed); tier information
  (P1 MVP / GraphQL+REST / chromedp) moves into each per-plugin doc
  page (see FR-004).
- **FR-003**: Each of the 19 user-facing plugins MUST have a doc
  page at `docs/plugins/<slug>.md`. Plugin slug list:
  `achievements`, `activity`, `calendar`, `contributors`, `habits`,
  `isocalendar`, `languages`, `notable`, `people`, `projects`,
  `reactions`, `repositories`, `sponsors`, `sponsorships`, `stargazers`,
  `starlists`, `stars`, `topics`, `traffic`.
- **FR-004**: Each plugin doc page MUST include:
  - **Title** + 1-paragraph description (sourced from
    `assets/plugins/<slug>/metadata.yml` `description` field).
  - **Tier badge / line** identifying the plugin's adoption tier
    (P1 MVP / P2 GraphQL+REST / P3 chromedp) — this moves into the
    per-plugin doc because the README tier table is removed in
    favor of the gallery (FR-002).
  - **Sample output** image (SVG embedded via Markdown image
    syntax). For plugins with functionally distinct sub-modes
    (e.g., `languages.recent` vs `languages.indepth`, `topics` icon
    vs spdx layout), include one sample per sub-mode.
  - **Configuration** table covering every input the plugin
    accepts, with columns: name, description, default, required,
    type. Source: `assets/plugins/<slug>/metadata.yml` `inputs` map.
  - One **Action mode usage** YAML snippet showing the plugin
    enabled with representative `plugin_<slug>_*` settings.
  - Cross-link to the master `action.yml` for the canonical
    schema.
- **FR-005**: Sample images MUST be vendored under
  `docs/examples/` (flat, file-per-plugin) or
  `docs/plugins/<slug>/` (co-located with the doc) — the plan phase
  picks the layout but MUST commit the assets (no live-rendering
  fetches from a CDN).
- **FR-006**: A make target `make docs-examples` MUST regenerate
  the full example set in a deterministic order. The target uses
  GitHub user `mjun0812` and a maintainer-supplied `GITHUB_TOKEN`
  passed via env. The target's pre-conditions (chromium installed,
  token scopes) MUST be documented in CONTRIBUTING.md.
- **FR-007**: The per-plugin configuration tables (FR-004 row 3)
  MUST be regenerable from `assets/plugins/<slug>/metadata.yml` —
  either by a code generator (similar to
  `internal/tools/gen-action-yml`) that emits a markdown table
  fragment, OR by structuring the doc so the table is human-edited
  but a CI check verifies row-count parity against metadata.yml.
  Plan phase chooses; either approach satisfies FR-007.
- **FR-008**: Per-plugin docs MUST be discoverable from README via
  the plugin gallery (FR-002). Per-plugin docs MAY also link to
  each other (e.g., `languages` → `repositories` for related
  plugins) — optional cross-linking.
- **FR-009**: Constitution III invariant — the set of doc pages
  (one per adopted plugin) MUST match the
  `tests/compliance/compliance_test.go::adoptedM4Plugins` set
  exactly. A new compliance test asserts: every adopted plugin has
  a doc page; no doc page exists for a non-adopted slug.
- **FR-010**: Human-authored zones inside doc pages MAY contain
  `<!-- TODO: ... -->` placeholders on first merge (loose gating).
  The compliance test in FR-009 does NOT block on TODO presence;
  it only verifies the doc-page set. A separate `make docs-lint`
  target (non-blocking) emits a warning summary of remaining TODO
  markers so the maintainer can track progress. `CONTRIBUTING.md`
  documents a planned tightening (target: v1.x — strict TODO
  gating after all 19 human-authored zones have been filled).

### Key Entities

- **Hero example image**: 1 large SVG per template (classic +
  repository) rendered against user `mjun0812`. Lives near the top
  of README.md.
- **Per-plugin sample image**: 1 SVG per adopted plugin (+ extra
  for plugins with sub-modes) showing the plugin's output panel.
  Either embedded in the full template render or rendered as a
  cropped panel — plan phase decides per consistency vs context
  trade-off.
- **Plugin doc page**: `docs/plugins/<slug>.md` per FR-004
  structure. Markdown only, no executable content.
- **Configuration table fragment**: either a generated markdown
  table block or a human-authored block validated against
  `metadata.yml`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A first-time README visitor identifies what the
  project produces (visually, what an SVG output looks like) within
  30 seconds of opening README.md on github.com.
- **SC-002**: A user clicking from README's plugin gallery to any
  `docs/plugins/<slug>.md` lands on a complete doc (sample +
  config + usage) within 1 click.
- **SC-003**: All 19 `docs/plugins/<slug>.md` pages exist; each
  sample image renders correctly on github.com (no broken-link
  icon, no oversized layout breakage).
- **SC-004**: `make docs-examples` from a clean checkout
  regenerates the full example set within 5 minutes (using
  chromium for chromedp-gated plugins).
- **SC-005**: Every input declared in
  `assets/plugins/<slug>/metadata.yml` is documented in the
  corresponding plugin doc's configuration table (validated by a
  new test or by an audit during plan phase).
- **SC-006**: After a render-affecting change (template edit,
  Dockerfile bump, plugin code change), the maintainer can
  regenerate examples + commit the diff in under 10 minutes total
  wall-clock effort.
- **SC-007**: The new constitution III compliance test
  (`TestCompliance_DocsPluginPagesMatchAdoptedSet`) passes on the
  M10 baseline + the post-010 head — no doc page drift.

## Assumptions

- **User identity for sample generation**: GitHub user `mjun0812`
  (per the user's explicit instruction). A future change could
  parametrize the user via env var, but `mjun0812` is the v1
  default.
- **Output format**: SVG is the primary format for samples (smaller
  footprint than PNG, scalable, native browser rendering on
  github.com). One PNG hero may be added if SVG rendering on
  github.com has known issues (plan phase decides).
- **Sub-mode coverage**: `languages.recent` and `languages.indepth`
  get separate sample images since they're functionally distinct.
  Other plugins are covered with a single representative sample.
- **chromedp-gated plugins** (`topics`, `starlists`) require
  chromium for sample generation. The maintainer's host must have
  `METRICS_CHROME_PATH` set or chromium on PATH.
- **Token scope coverage**: the maintainer's `GITHUB_TOKEN` for
  example generation has scopes for all 19 plugins (public_repo at
  minimum; sponsors / traffic etc. may need additional scopes — the
  affected plugin docs note the scope requirement explicitly).
- **Reproducibility scope**: dynamic strings in SVG output (version
  footer, `Last updated` timestamp) are masked at write time using
  the M9 `NormalizeSVG` machinery so committed bytes only change
  on semantic diffs. Alternative: accept timestamp noise in
  committed bytes. Plan phase picks; SC-006's "regenerate in <10min"
  target works either way.
- **Generation cadence**: manual via `make docs-examples`, not CI-
  automated. CI generation would require chromium + GITHUB_TOKEN +
  chrome cold-start cost, which is heavy and brittle. Maintainers
  regenerate when rendering changes meaningfully.
- **Plugin set freeze**: this feature MUST NOT add or remove
  adopted plugins. The constitution III invariant continues to gate
  the plugin / template set via `tests/compliance/...` — this
  feature only adds *documentation* for the existing set.
- **No spec-kit phase ordering impact**: this is a documentation /
  ergonomics polish — it does not block any future phase. M10
  (v1.0.x) is shipped and stable; 010 is post-release polish.
