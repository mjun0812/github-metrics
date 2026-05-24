# 010 docs-plugin-gallery — UNBLOCKED

**Status**: unblocked on 2026-05-23 (was blocked since 2026-05-19)
**Resolved blocker**: feature 011 (plugin-rendering-parity) merged via PR #383 on 2026-05-23, bringing the 19 adopted plugins to upstream EJS-template visual parity. Re-running `bash scripts/gen-doc-samples.sh` now produces visually-correct SVG/PNG samples.
**Next step**: When/if 010 is picked back up, start from issue #365 (010-T001 — directory setup) and walk through the tasks.md sequence under `specs/010-docs-plugin-gallery/tasks.md`. The 18 GitHub issues (#365-#382) remain open as the per-task checklist.

Note: 010 is **not** in the adoption scope of `docs/design/15-selection-answer.md §4.2` (the 21-plugin core). It is an independent documentation-automation feature that ships docs/plugins/* and docs/examples/* — useful for repo polish but not required for the migration goal. Resume decision is at maintainer discretion.

## What's committed (010 foundation, ships now)

- Full spec-kit artefacts under `specs/010-docs-plugin-gallery/`
- `internal/tools/normalize-svg-stream/` — regex masker for dynamic SVG strings
- `scripts/gen-doc-samples.sh` — docker-backed orchestration for 23 renders
- `docs/{plugins,examples}/.gitkeep` — directory placeholders

## What's deferred

The actual SVG/PNG samples under `docs/examples/` and the per-plugin doc
pages under `docs/plugins/`. The script + tool are ready to run once
rendering parity (011) lands; rerun `bash scripts/gen-doc-samples.sh`
to regenerate the full sample set.

## Why blocked — concrete evidence

A trial generation with mjun0812's data on 2026-05-19 produced 23
SVG/PNG samples that are visibly broken on github.com:

| symptom | example |
|---|---|
| Language progress bar invisible | `plugin-languages.svg` — language bars in `<g class="languages-progress">` are dropped because bare `<g>` is invalid HTML inside `<foreignObject>` |
| Section headers missing | upstream renders `<h2 class="field"><svg/>27 Languages</h2>` + `<h3>Most used languages</h3>` — we render neither |
| Per-language icons missing | upstream renders one octicon per language entry — we render none |
| Indepth file/commit stats missing | upstream shows "estimation from 196kb of code in 66 edited files…" summary — we render nothing |
| Empty section placeholders consume vertical space | `<section data-section="activity-community">` etc. produce ~400 px of empty whitespace when their data is absent |

Side-by-side screenshot capture saved during the investigation
(`compare.png`, regenerated locally — not committed) showed our SVG
versus the upstream reference at https://github.com/mjun0812/mjun0812
`metrics_languages.svg` had drastically different visual output.

## Why this wasn't caught earlier

- **Golden tests are byte-comparison only** — they freeze the broken
  output as the expected value instead of verifying it renders.
- M4 spec line 27 explicitly calls for "`<g class="languages-progress">`"
  output, so the byte format was speced-in. The render-time invalidity
  was never validated.
- v1.0.0 shipped without any browser-based visual verification.

## What 011 will do (sketch)

1. For each of the 19 adopted plugins, bring `internal/plugins/<slug>/partial.go`
   up to feature parity with `org_repo/source/templates/classic/partials/<slug>.ejs`:
   - Wrap SVG primitives (`<g>`, `<rect>`) in proper `<svg>` containers.
   - Add the section headers (`<h2>`, `<h3>`) that upstream emits.
   - Add the per-entry octicon icons.
   - Add the indepth / recent / variant-specific summary lines.
   - Suppress empty section placeholders when data is absent.
2. Re-baseline `tests/golden/classic/m4/<plugin>.svg` with `-update` once
   parity is verified by visual inspection.
3. Add a new browser-based visual regression test
   (`tests/visual/<plugin>_test.go`) that renders each plugin's SVG
   via a real Chromium tab and asserts on observed DOM structure (e.g.,
   "the `<rect class="language-bar">` elements have non-zero rendered
   width on viewport") — closes the gap that allowed this to ship.

## How to resume 010 after 011 lands

```sh
# Once 011 PRs are merged and the 19 plugins render correctly:
git checkout 010-docs-plugin-gallery
git rebase main
docker build -f deploy/Dockerfile -t github-metrics:local .
export GITHUB_TOKEN=<PAT with read:user + repo>
bash scripts/gen-doc-samples.sh   # regenerates 23 × {svg,png}
git add docs/examples/
# Continue with T003 (gen-plugin-docs) + T005 (Makefile) + T007/T009.
```
