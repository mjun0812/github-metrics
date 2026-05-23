# Implementation Plan: Per-plugin docs with real example outputs

**Branch**: `010-docs-plugin-gallery` | **Date**: 2026-05-19 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/010-docs-plugin-gallery/spec.md`

## Summary

The current README.md describes what `github-metrics` produces in
prose. A first-time visitor cannot see actual examples until they
install + configure + run the tool — a hard adoption barrier. This
feature ships:

- Hero example images (classic + repository templates) embedded
  near the top of README.md.
- Per-plugin doc pages at `docs/plugins/<slug>.md` for the 19
  user-facing plugins, each containing a rendered sample image +
  the full configuration spec extracted from
  `assets/plugins/<slug>/metadata.yml`.
- A new internal tool `internal/tools/gen-plugin-docs` that
  generates the per-plugin .md files (config tables) from
  `metadata.yml` (cheap, no token needed).
- A new make target `make docs-samples` that renders the SVG
  samples by invoking `metrics-cli --user mjun0812 …` per plugin
  (requires GITHUB_TOKEN + chromium).
- An umbrella `make docs-examples` running both.
- A new compliance test (`TestCompliance_DocsPluginPagesMatchAdoptedSet`)
  asserting that the set of `docs/plugins/<slug>.md` files matches
  the canonical 19-plugin adopted set, per FR-009 and constitution III.

Source data: GitHub user `mjun0812` (the project maintainer).

## Technical Context

**Language/Version**: Go 1.26 (for the new `gen-plugin-docs` tool;
mirrors the existing `gen-action-yml` pattern). Make / shell for the
recipes. Markdown for the doc pages. No production-code Go changes.

**Primary Dependencies**: existing only —
`gopkg.in/yaml.v3` (already used by `internal/tools/gen-action-yml`)
for metadata.yml parsing; the `cmd/metrics-cli` binary
(M6 baseline) for SVG generation; `chromium` (M3 baseline) for the
two chromedp-gated plugins (`topics`, `starlists`).

**Storage**: vendored under `docs/examples/` and `docs/plugins/`.
~25-50 KB per SVG × ~22 files ≈ 0.5-1 MB committed.

**Testing**:

- New `internal/tools/gen-plugin-docs/main_test.go` — table-driven
  tests for: metadata.yml parsing, input-table rendering, all-19-
  plugins-covered guard.
- New `tests/compliance/compliance_test.go::TestCompliance_DocsPluginPagesMatchAdoptedSet`
  per FR-009: enumerates `docs/plugins/*.md` and asserts strict
  equality with the M4 adopted set (less `base` / `core`).
- Manual: render samples via `make docs-samples` and visually
  verify on github.com after merge.

**Target Platform**: maintainer's local host
(macOS / Linux) for sample regeneration; github.com for end-user
rendering of the committed markdown + SVG.

**Project Type**: Single-binary Go monorepo (unchanged).

**Performance Goals**: SC-004 — `make docs-examples` from a clean
checkout completes within 5 minutes. The dominant cost is the
chromedp-driven render of 21+ SVG outputs (≈ 5-10 s each = 2-4 min)
+ the chromium cold start (~5 s) + the GitHub API fetches (~5 s per
plugin).

**Constraints**:

- No production-code changes. The new tool lives under
  `internal/tools/`. The existing `cmd/metrics-cli` is invoked
  as-is for SVG generation.
- Constitution III invariant: doc-page set MUST equal the adopted
  plugin set; new compliance test gates.
- Reproducibility: dynamic strings (version footer, "Last updated"
  timestamp) in the SVG output cause spurious diffs when the
  maintainer regenerates. The M9 `NormalizeSVG` mask
  (`internal/testutil/golden/svg_normalize.go`) is reused at write
  time so committed bytes only change on semantic diffs.
- `mjun0812` is the data source per the spec's locked assumption.

**Scale/Scope**:

- 19 plugin doc pages (one per user-facing slug).
- 2 hero images (classic + repository).
- 2 sub-mode samples for `languages` (recent / indepth).
- ≈ 22 SVG files committed under `docs/examples/`.
- 1 new Go tool (~250 LOC + tests).
- 1 new compliance test (~30 LOC).
- ~50-LOC additions to Makefile (3 new targets) and README.md
  (plugin gallery section + hero block, between explicit
  HTML-comment markers so the tool can regenerate that block
  idempotently).
- ~100 LOC additions to CONTRIBUTING.md (chromium / token setup
  for the maintenance script).

Total LOC delta: **~600 LOC added** + **22 vendored SVG files**.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. 入力互換性 (NON-NEGOTIABLE) | ✓ N/A | No `action.yml` input changes. The new tool emits `docs/` markdown only; it does not touch the action.yml schema. |
| II. 出力契約 (DOM/JSON 単位) | ✓ N/A | No rendered SVG / JSON output changes. The samples are produced by the existing `cmd/metrics-cli` with no rendering-code modification. |
| III. スコープ規律 (採用機能のみ実装) | ✓ PASS (strengthened) | Pure documentation. The new `TestCompliance_DocsPluginPagesMatchAdoptedSet` test (FR-009) is a third invariant layer atop the existing `TestCompliance_M4_AdoptedPlugins` (directory set) and `TestCompliance_M7_TemplateInvariant` (template set), specifically gating doc-page-set drift. |
| IV. テーブルテスト + Golden File | ✓ PASS | The new `gen-plugin-docs` tool has table-driven tests for the metadata.yml → markdown table transform. The SVG samples themselves are NOT golden-tested (they vendor at maintainer regeneration time, not CI-gated). |
| V. Go 規約と言語ポリシー | ✓ PASS | New Go tool follows the existing `internal/tools/gen-action-yml` pattern (single `main.go` package, gopkg.in/yaml.v3 dep, table-driven tests). gofumpt / golangci-lint baselines unchanged. New markdown docs in `docs/plugins/` are Japanese per project doc convention. |

**Result: ALL GATES PASS** — no Complexity Tracking entry needed.

## Project Structure

### Documentation (this feature)

```text
specs/010-docs-plugin-gallery/
├── plan.md              # This file
├── research.md          # Phase 0 — R-001..R-004 (sample isolation,
│                        # reproducibility, doc-generation approach,
│                        # README gallery layout)
├── data-model.md        # Phase 1 — E-001..E-005 (plugin doc page,
│                        # hero image, config-table fragment,
│                        # plugin-gallery README section, generation
│                        # invocation matrix)
├── quickstart.md        # Phase 1 — maintainer flow: install deps,
│                        # run `make docs-examples`, verify diff, commit
├── contracts/
│   ├── plugin-doc-template.md      # required sections + autogen markers
│   ├── sample-generation.md        # make recipes + metrics-cli invocations
│   └── readme-gallery.md           # README hero + plugins-gallery markers
├── tasks.md             # Phase 2 (NOT created by /speckit-plan)
└── checklists/
    └── requirements.md  # 16/16 PASS (created in /speckit-specify)
```

### Source Code (repository root)

```text
docs/
├── plugins/                       # NEW: 19 per-plugin doc pages (FR-003)
│   ├── achievements.md
│   ├── activity.md
│   ├── calendar.md
│   ├── contributors.md
│   ├── habits.md
│   ├── isocalendar.md
│   ├── languages.md               # (covers recent / indepth sub-modes)
│   ├── notable.md
│   ├── people.md
│   ├── projects.md
│   ├── reactions.md
│   ├── repositories.md
│   ├── sponsors.md
│   ├── sponsorships.md
│   ├── stargazers.md
│   ├── starlists.md
│   ├── stars.md
│   ├── topics.md
│   └── traffic.md
└── examples/                      # NEW: vendored SVG samples (FR-005)
    ├── hero-classic.svg           # FR-001 hero (classic template)
    ├── hero-repository.svg        # FR-001 hero (repository template)
    ├── plugin-achievements.svg
    ├── plugin-activity.svg
    ├── plugin-calendar.svg
    ├── plugin-contributors.svg
    ├── plugin-habits.svg
    ├── plugin-isocalendar.svg
    ├── plugin-languages.svg                # default mode
    ├── plugin-languages-recent.svg         # sub-mode
    ├── plugin-languages-indepth.svg        # sub-mode
    ├── plugin-notable.svg
    ├── plugin-people.svg
    ├── plugin-projects.svg
    ├── plugin-reactions.svg
    ├── plugin-repositories.svg
    ├── plugin-sponsors.svg
    ├── plugin-sponsorships.svg
    ├── plugin-stargazers.svg
    ├── plugin-starlists.svg
    ├── plugin-stars.svg
    ├── plugin-topics.svg
    └── plugin-traffic.svg

internal/tools/gen-plugin-docs/
├── main.go                        # NEW generator: emits docs/plugins/<slug>.md
│                                  # by composing:
│                                  #   1) a human-authored prose preamble
│                                  #      (from a small template in tool code,
│                                  #       OR pulled from assets/plugins/<slug>/doc.md
│                                  #       if such a file exists)
│                                  #   2) auto-generated config-table fragment
│                                  #      from assets/plugins/<slug>/metadata.yml
│                                  #   3) auto-generated usage snippet
│                                  #   4) sample-image reference (path is fixed)
│                                  # Uses HTML-comment markers (
│                                  #   <!-- AUTOGEN_START: config-table -->
│                                  #   <!-- AUTOGEN_END: config-table -->
│                                  # ) so re-runs only update the auto sections.
└── main_test.go                   # table-driven tests for the
                                   # metadata.yml → markdown table transform

scripts/
└── gen-doc-samples.sh             # NEW (optional): wraps the
                                   # per-plugin metrics-cli invocations
                                   # so the Makefile target stays
                                   # readable. The script enables exactly
                                   # one plugin at a time and emits the
                                   # SVG to docs/examples/plugin-<slug>.svg.

Makefile                           # MODIFIED:
                                   # - docs:           run gen-plugin-docs to
                                   #                   refresh docs/plugins/*.md
                                   #                   (cheap, no token needed)
                                   # - docs-samples:   run scripts/gen-doc-samples.sh
                                   #                   (renders SVGs; requires
                                   #                   GITHUB_TOKEN + chromium)
                                   # - docs-examples:  umbrella (docs + docs-samples)

README.md                          # MODIFIED:
                                   # - Hero images (FR-001) embedded near top
                                   # - "Plugins" section becomes a gallery
                                   #   between HTML-comment AUTOGEN markers
                                   #   (gen-plugin-docs writes this block too)

CONTRIBUTING.md                    # MODIFIED:
                                   # - New section: regenerating plugin docs
                                   #   + samples (token scopes, chromium
                                   #   precondition, expected wall-clock)

tests/compliance/
└── compliance_test.go             # EXTENDED: new
                                   # TestCompliance_DocsPluginPagesMatchAdoptedSet
                                   # per FR-009 — enumerates docs/plugins/*.md
                                   # and asserts strict equality with the
                                   # 19-plugin adoptedM4Plugins set
                                   # (less base / core).

assets/plugins/<slug>/
└── doc.md                         # NEW (optional, one per plugin):
                                   # human-authored prose for the plugin's
                                   # doc preamble. If absent, the generator
                                   # falls back to metadata.yml's
                                   # description field. This file holds
                                   # the "why use this plugin?" /
                                   # "common pitfalls" prose that
                                   # metadata.yml's terse description
                                   # doesn't cover.
```

**Structure Decision**: Single-binary Go monorepo (unchanged). One
new internal tool (`gen-plugin-docs`) follows the existing
`gen-action-yml` pattern. One new shell helper script
(`gen-doc-samples.sh`) wraps `metrics-cli` invocations for sample
generation; this keeps the Makefile readable. All committed
artifacts live under `docs/` (markdown) and `docs/examples/`
(SVG). No production-code packages are touched.

## Complexity Tracking

*No violations — all 5 constitution gates pass. This section is
omitted per template guidance.*
