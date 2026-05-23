# Research: Per-plugin docs with real example outputs

**Date**: 2026-05-19 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

Four decisions feed Phase 1 design. The spec section referenced in
"Spec linkage" below frames each decision; the chosen approach
becomes the Phase 1 contract input.

---

## R-001: Sample-image isolation — single-panel vs full-template render?

**Decision**: Per-plugin samples use a **single-panel render**
produced by running `metrics-cli` with **only the target plugin
enabled** (plus the always-on `base` + `core`). README hero images
use the **full-template render** with **all 19 plugins enabled** to
showcase the integrated output.

**Rationale**:

- **Per-plugin docs need focus.** A full-template SVG embedded in
  every per-plugin doc would duplicate ~200-500 KB across 19 files
  (~5 MB total noise) and force the reader to scroll the full
  output to find the panel they care about. A single-panel render
  of just the languages panel weighs ~10-30 KB and is immediately
  legible.
- **README needs context.** A first-time visitor seeing only an
  isolated `languages` panel does not get the "what does this
  tool produce as a whole?" answer that the hero is supposed to
  deliver. The integrated render shows the relationships between
  panels (e.g., the activity timeline above the languages chart),
  which is the project's actual value proposition.
- **Generation cost is the same.** `metrics-cli` accepts
  `plugin_<slug>=yes` for each enable; the single-panel mode just
  toggles fewer flags. There is no rendering-code change required —
  the existing pipeline handles the "only base + one plugin"
  scenario gracefully (M3 / M4 baseline behavior).

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Full-template SVG in every per-plugin doc | Bloats the repo with duplicated bytes (~5 MB), buries the per-plugin panel under unrelated content, slows github.com page load. |
| Crop the full-template render post-process to extract per-plugin panels | Requires adding a SVG-cropping helper (XML manipulation); more code than just disabling other plugins; output position depends on template-partial ordering which is brittle. |
| Embed PNG thumbnails of the SVG in the doc, full SVG linked | PNG conversion needs chromium screenshot; doubles the sample-generation chain. SVG-only is simpler. |

**Spec linkage**: FR-004 (per-plugin doc image), FR-001 (hero
image). SC-003 (samples render on github.com).

**Plan-phase risk**: low. `metrics-cli --user mjun0812
--plugin_languages yes --dryrun --output svg --filename -`
already works for any single plugin; verified during M4.

---

## R-002: Reproducibility — version footer + timestamp normalization?

**Decision**: The maintainer-side sample-generation script masks
the version footer + `Last updated` timestamp at write time using
the M9 `internal/testutil/golden/svg_normalize.go::NormalizeSVG`
function (re-purposed). The committed SVG bytes therefore only diff
when semantic content changes; a `make docs-samples` re-run on
the same upstream-data state produces a zero-byte diff.

**Rationale**:

- **Spurious noise is the enemy of regeneration discipline.** If
  every `make docs-samples` run produces a multi-file diff just
  because the timestamp moved, the maintainer will stop running
  it (because PR diffs become noisy and reviewable). Masking the
  dynamic strings makes the regeneration cost truly proportional
  to actual content change.
- **The mask already exists.** M9's `NormalizeSVG` replaces:
  - `Last updated YYYY-MM-DDTHH:MM:SSZ` → `Last updated __MASKED__`
  - `github-metrics@vX.Y.Z` → `github-metrics@__MASKED__`
  This is exactly the noise we want to suppress in committed
  sample bytes. Re-using it (vs. writing a parallel masking pass)
  keeps the contract surface small.
- **Semantic diffs are preserved.** Plugin-output bytes (chart
  values, language percentages, activity counts) are NOT masked —
  if mjun0812 stars a new repo, the next regenerated sample
  reflects it. That's the desired behavior.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Commit raw SVG with timestamp; tolerate noisy diffs | Maintainers will avoid regeneration to avoid PR noise; samples will silently drift. |
| Add a `--frozen-version` flag to `metrics-cli` that pins the version string | More invasive (touches production code); doesn't address the timestamp; mask works for both. |
| Render via `engine.SetVersionForTest` from the new tool | Same as previous; production binary needs the testonly hook exposed; awkward. |

**Spec linkage**: SC-006 (regenerate in <10 min including review).
Reproducibility note in Assumptions resolved by this decision.

**Plan-phase risk**: low. `NormalizeSVG` is well-tested under M9.
The script invokes it via a tiny Go wrapper that reads stdin and
writes stdout:

```sh
metrics-cli --user mjun0812 ... --filename - \
  | go run ./internal/tools/normalize-svg \
  > docs/examples/plugin-<slug>.svg
```

Or `metrics-cli` writes a temp file, the tool reads-normalizes-
writes the final destination. Plan-phase picks the exact shape.

---

## R-003: Plugin doc structure — auto-generate vs hand-write vs hybrid?

**Decision**: **Hybrid.** Each `docs/plugins/<slug>.md` page has
two zones:

1. **Auto-generated zone** — bounded by HTML-comment markers:

   ```html
   <!-- AUTOGEN_START: <section-name> -->
   ... regenerated by gen-plugin-docs ...
   <!-- AUTOGEN_END: <section-name> -->
   ```

   Used for the configuration table (driven by metadata.yml) and
   the title + 1-paragraph description.

2. **Human-authored zone** — everything outside the markers,
   including "When to use this plugin", "Common pitfalls", and
   the sample image embed (path is fixed but the surrounding
   prose is per-plugin).

The `gen-plugin-docs` tool **only rewrites between the markers**
on re-runs; hand-edits outside the markers survive.

**Rationale**:

- **Config tables are mechanical.** Every input declared in
  `metadata.yml` MUST appear in the table; missing rows are bugs.
  Auto-generation guarantees this (and the related FR-007 / SC-005
  guarantees are testable as a side effect).
- **Description prose needs human nuance.** `metadata.yml`'s
  one-liner description is too terse for a doc page. Hand-writing
  the "When to use this" section adds value the tool can't.
- **HTML-comment markers are the standard pattern.** github.com
  renders the markers as invisible, and the convention is widely
  used (e.g., `actions/setup-go`'s README). No new tooling needed
  to parse them; a regex pass is sufficient.
- **Idempotency is preserved.** The tool reads the existing file
  (if present), extracts the human-authored zones, regenerates
  the autogen zones, and writes the merged result. Pure
  refactoring is safe.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Pure auto-generation | Eliminates the "When to use this plugin" prose, which is the main reason users land on the doc; the tool would have to inline that prose somewhere (metadata.yml extension?) — more complexity than markers. |
| Pure hand-authoring + a parity test | Achievable but maintenance-heavy. Every new input in metadata.yml must be manually mirrored into the doc; the parity test would just nag the maintainer to do the work the tool could do. |
| Frontmatter (YAML) for the autogen data, body for human prose | Similar to markers but adds frontmatter parsing complexity; markers are simpler and github.com-render-safe. |

**Spec linkage**: FR-004 (doc structure), FR-007 (regenerable
config table), SC-005 (every input documented).

**Plan-phase risk**: low. The HTML-comment marker pattern is
well-established; the regex (`<!-- AUTOGEN_START: <name> -->.*?<!-- AUTOGEN_END: <name> -->`)
is unambiguous because the markers don't appear in valid markdown
content (the `: <name>` discriminator prevents accidental matches).

---

## R-004: README plugin-gallery layout — table vs list vs grid?

**Decision**: A **fixed-width 3-column markdown table** with each
cell showing a small thumbnail (linked to the plugin's doc page)
plus the plugin slug. The gallery section sits between HTML-comment
markers so the same `gen-plugin-docs` tool can regenerate it
idempotently.

```markdown
<!-- AUTOGEN_START: plugins-gallery -->
| [![languages](docs/examples/plugin-languages.svg)](docs/plugins/languages.md) | [![activity](docs/examples/plugin-activity.svg)](docs/plugins/activity.md) | [![achievements](docs/examples/plugin-achievements.svg)](docs/plugins/achievements.md) |
|:---:|:---:|:---:|
| [`languages`](docs/plugins/languages.md) | [`activity`](docs/plugins/activity.md) | [`achievements`](docs/plugins/achievements.md) |
| ...19 rows total in 7 rows × 3 columns... |
<!-- AUTOGEN_END: plugins-gallery -->
```

**Rationale**:

- **3 columns × 7 rows fits 19 plugins** with one cell empty.
  3 columns gives roughly square thumbnails (each ~33% width)
  that render legibly at github.com's content width.
- **The same `gen-plugin-docs` tool** can emit this block, sharing
  the metadata.yml read.
- **Thumbnails are the same SVG** as on the per-plugin doc (no
  separate thumbnail generation). github.com scales SVG to the
  cell width automatically.
- **Slug + link in the second row** disambiguates plugins whose
  SVG looks similar at thumbnail size.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Flat bullet list | Loses the visual scan affordance; 19 items as a list is a wall of text. |
| Compact 5-column grid | Thumbnails too small to be useful; some plugins have very different aspect ratios (e.g., calendar is wide, achievements is square). |
| Custom HTML `<details>` summary per plugin | Doesn't work well in github.com's rendering; clutters the README. |
| External assets gallery on github.io | Adds a second hosting surface to maintain. The README is the discovery point; keep it self-contained. |

**Spec linkage**: FR-002 (gallery section), US1 acceptance scenario
2 (discoverable within page).

**Plan-phase risk**: low. github.com's markdown renderer handles
this layout cleanly. SVG thumbnails may render at varying heights;
visual review during the first regeneration will confirm
acceptable layout.

---

## Summary

All 4 decisions resolved with informed choices. No `NEEDS
CLARIFICATION` carries through to Phase 1.

- **R-001**: Per-plugin docs show single-panel renders; README hero
  shows full-template renders.
- **R-002**: Sample-generation script normalizes via M9's
  `NormalizeSVG` so committed bytes diff only on semantic change.
- **R-003**: Doc pages are hybrid — auto-generated zones between
  HTML-comment markers; human prose outside.
- **R-004**: README gallery is a 3-column markdown table with
  thumbnail + slug-link cells, also between AUTOGEN markers.

All approaches preserve constitution III (no plugin/template
additions) and are testable via the new compliance gate (FR-009).
