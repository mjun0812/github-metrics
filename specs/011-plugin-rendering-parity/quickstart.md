# Quickstart: implementing one plugin's parity PR

**Date**: 2026-05-19 | **Plan**: [./plan.md](./plan.md)

Maintainer flow for adding parity for one plugin (the per-PR loop
that gets executed 19 times across US1 + US2). This is what you
follow when picking up "plugin X PR" off the 011 issue board.

---

## 0. Prerequisites

```sh
# 1. classic PAT with read:user + repo scopes (for rendering against mjun0812)
export GITHUB_TOKEN=ghp_...

# 2. chromium / Google Chrome available (M3 dependency)
which chromium || ls "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
export METRICS_CHROME_PATH=/usr/bin/chromium  # or the Chrome.app path

# 3. local docker image (for sample generation + screenshot capture)
docker build -f deploy/Dockerfile -t github-metrics:local .

# 4. branch + spec dir
git checkout 011-plugin-rendering-parity
git pull --rebase

# Pick the plugin you're working on (e.g., languages, achievements, etc.):
export SLUG=languages  # change per PR
```

## 1. Create the parity checklist (5 min)

Copy the template at
`specs/011-plugin-rendering-parity/contracts/partial-parity-checklist.md`
to your plugin's path:

```sh
cp specs/011-plugin-rendering-parity/contracts/partial-parity-checklist.md \
   specs/011-plugin-rendering-parity/plugins/${SLUG}.md
```

Open `org_repo/source/templates/classic/partials/${SLUG}.ejs` side by
side with your new `plugins/${SLUG}.md`. Walk through the EJS top to
bottom, enumerating each visible element into the inventory table.
Mark "upstream" column ✅ for each present element.

## 2. Capture the "before" screenshot (3 min)

(US1 deliverable adds this script — for subsequent plugins it just runs.)

```sh
bash scripts/capture-plugin-screenshot.sh ${SLUG} before
# Writes specs/011-plugin-rendering-parity/plugins/screenshots/${SLUG}-before.png
```

## 3. Mark "ours-before" status in the checklist (5 min)

Open the before screenshot. For each row in the inventory table:

- If the element is visible → ✅ in "ours (before)" column
- If absent or invisible → ❌ in "ours (before)" column

Save the file. Commit so the parity checklist is in the PR history
even if subsequent commits change it.

```sh
git add specs/011-plugin-rendering-parity/plugins/${SLUG}.md \
        specs/011-plugin-rendering-parity/plugins/screenshots/${SLUG}-before.png
git commit -m "docs(011): ${SLUG} parity checklist + before screenshot"
```

## 4. Rewrite the partial (30-90 min, plugin-dependent)

Open `internal/plugins/${SLUG}/partial.go` and `org_repo/source/templates/classic/partials/${SLUG}.ejs`
side by side. For each missing element in the checklist:

- Find the corresponding EJS markup
- Translate to Go `fmt.Fprintf(&b, ...)` calls (preserve idiomatic
  upstream class names so CSS continues to apply)
- For bare-`<g>` cases: wrap in `<svg xmlns="http://www.w3.org/2000/svg" width="N" height="M">` so it renders inside foreignObject
- For empty-state: implement Pattern A / B / C per R-004

Run the table tests as you go:

```sh
go test ./internal/plugins/${SLUG}/...
# If the partial_test.go table cases fail because expected output
# changed, that's fine — update the expected strings as the partial
# changes shape. The table tests stay green by reflecting the new
# emit, not by being skipped.
```

## 5. Re-baseline the byte golden (1 min)

```sh
go test ./internal/plugins/${SLUG}/... -update
git diff tests/golden/classic/m4/${SLUG}.svg | head -50
# Eyeball the diff: structural additions (<h2>, <svg class="bar">) should be visible.
# If you see byte-level changes that don't correspond to your intentional rewrites,
# something else moved — investigate before committing.
```

## 6. Write the visual test (15-30 min)

```sh
cp tests/visual/languages_test.go tests/visual/${SLUG}_test.go
# Edit the new file to match the plugin's slug:
# - rename TestLanguages_Visual → Test<Slug>_Visual
# - update renderForVisualTest() inputs to the plugin's input set
# - replace assertions with the 3-5 most representative for this plugin
#   (per R-002 menu + the per-plugin assertion count budget in
#   contracts/visual-test-shape.md §3)
```

Run the visual test:

```sh
go test ./tests/visual/ -run Test${SLUG^}_Visual -v
```

Iterate until green. If the test fails with `"X.Y-bar": rendered
width 0`, your partial still has the bare-`<g>` bug somewhere.

## 7. Capture the "after" screenshot (3 min)

```sh
bash scripts/capture-plugin-screenshot.sh ${SLUG} after
```

Open before / after screenshots side by side. Confirm the rewrite
matches the upstream-equivalent output.

## 8. Update the parity checklist with "ours-after" status (5 min)

Walk through the inventory table; mark "ours (after)" column ✅ for
each now-present element. Save.

## 9. Commit + open PR (5 min)

```sh
git add internal/plugins/${SLUG}/partial.go \
        internal/plugins/${SLUG}/partial_test.go \
        tests/golden/classic/m4/${SLUG}.svg \
        tests/visual/${SLUG}_test.go \
        specs/011-plugin-rendering-parity/plugins/${SLUG}.md \
        specs/011-plugin-rendering-parity/plugins/screenshots/${SLUG}-after.png

git commit -m "fix(011): ${SLUG} partial visual parity with upstream classic template

<2-3 sentence description of what was missing + what was added>

Closes #<011-issue-number>"

git push -u origin 011-plugin-rendering-parity
gh pr create --title "fix(011): ${SLUG} partial visual parity" \
             --body-file <path-to-PR-body-following-contracts/per-plugin-pr-template.md>
```

## 10. Verify CI + request review

CI runs:

- `go test ./...` (full suite)
- `go test ./tests/visual/... -timeout 15m` (the new visual suite —
  runs all 19, but only this plugin's should differ; the other 18
  are at whatever baseline they're at right now)
- `go test ./tests/compliance/...` (FR-009 gate after US3)

Once green, request review. Reviewer follows the "Reviewer guide" in
the per-plugin checklist.

---

## Total time per plugin

- Pilot (languages, US1): 4-6 hours (includes writing visual_test.go
  harness + screenshot script + parity-checklist template instance)
- Sweep (each of 18, US2): 1.5-2 hours per plugin (most of the
  scaffolding is reused)

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Visual test passes but eyeball says it looks wrong | Assertion menu R-002 picked the wrong 3-5 items for this plugin | Add another assertion that captures the visible issue |
| Re-baselined golden diff is massive (>50% of bytes) | Partial rewrite changed structure radically | Verify your output by comparing to upstream rendering — if structure matches upstream, the diff size is fine |
| `bash scripts/capture-plugin-screenshot.sh` returns blank PNG | docker image not built / GITHUB_TOKEN missing | Re-check Prerequisites step 1 + 3 |
| Visual test for chromedp-gated plugin (topics, starlists) fails on "chromedp scrape returned no data" | Test isn't mocking plugin data per R-003 | Set up mocked plugin data via the existing `internal/testutil/mocks/` pattern |
| Multiple plugins changed in one PR | Violates 1-plugin-per-PR (spec Clarification §3) | Split into separate PRs; each its own parity checklist |

## Resuming 010 after 011 completes

Once all 19 plugin parity PRs are merged + `tests/visual/` is green in CI:

```sh
git checkout 010-docs-plugin-gallery
git rebase main  # pulls in 011
docker build -f deploy/Dockerfile -t github-metrics:local .
bash scripts/gen-doc-samples.sh   # regenerates 23 × {svg,png} — now correct
# Remove specs/010-docs-plugin-gallery/BLOCKED.md
# Continue with the deferred 010 issues (#367, #369, #371, #373)
```
