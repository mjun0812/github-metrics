# Quickstart: regenerating per-plugin docs + samples

**Date**: 2026-05-19 | **Plan**: [plan.md](./plan.md)

This is the maintainer flow for regenerating the per-plugin doc
pages + their sample SVG images. Run it after:

- Any plugin's `metadata.yml` changes (new input, description
  edit) — re-emit the affected doc's config table.
- The rendering code changes meaningfully (chromedp upgrade,
  template edit, new partial) — re-render samples.
- Routine refresh (e.g., monthly) to keep mjun0812's sample data
  reflective of current activity.

## 0. Prerequisites

```sh
# 1. Token in env. The PAT must have at minimum public_repo;
#    sponsors / traffic etc. need additional scopes (see the
#    individual plugin docs).
export GITHUB_TOKEN=<your-PAT>

# 2. chromium available — required for chromedp-gated plugins
#    (topics, starlists) and the SVG resize pipeline.
#    macOS:  brew install chromium  (or Google Chrome.app path)
#    Linux:  apt install chromium   (or use chromedp/headless-shell)
export METRICS_CHROME_PATH=/path/to/chromium

# 3. Build the project (produces bin/metrics-cli used by the
#    sample-generation script).
make build
```

## 1. Regenerate plugin docs (cheap, no token needed)

```sh
make docs
```

This invokes `internal/tools/gen-plugin-docs` and:

- Refreshes the auto-generated zones inside every
  `docs/plugins/<slug>.md` (title / description / config table /
  usage snippet).
- Refreshes the README.md hero block + plugins-gallery section
  between AUTOGEN markers.
- Human-authored zones (the "このプラグインを使うべきケース"
  prose, "既知の制約 / 注意点" prose) are preserved byte-for-byte.

Expected runtime: <5 seconds. No GITHUB_TOKEN or chromium needed.

## 2. Regenerate sample SVGs (requires token + chromium)

```sh
make docs-samples
```

This invokes `scripts/gen-doc-samples.sh` which:

- Iterates the 19 adopted plugins.
- For each plugin, runs `metrics-cli --user mjun0812 --plugin_<slug>
  yes ...` to produce a single-panel SVG.
- Pipes through `internal/tools/normalize-svg-stream` to mask
  dynamic strings (version footer, "Last updated" timestamp).
- Writes `docs/examples/plugin-<slug>.svg` atomically (.tmp then
  mv).
- Also generates `docs/examples/hero-classic.svg` and
  `docs/examples/hero-repository.svg` with the full plugin set.
- Plus the 2 sub-mode variants for languages
  (`plugin-languages-recent.svg`, `plugin-languages-indepth.svg`).

Total: ~24 SVG files; ~2-4 min wall-clock.

## 3. Umbrella target — regenerate both

```sh
make docs-examples
```

Equivalent to `make docs && make docs-samples`. This is the
maintainer's canonical command after any rendering / metadata
change.

## 4. Verify diff

After regeneration:

```sh
git status
# Expect: modified docs/examples/*.svg + maybe docs/plugins/*.md
#         + maybe README.md (if AUTOGEN content moved).
git diff --stat
```

Spot-check 1-2 of the regenerated SVGs by opening them locally
to confirm they render correctly. The committed sample's git
diff shows only semantic changes — dynamic noise is masked by
NormalizeSVG.

## 5. Commit

```sh
git add docs/examples/ docs/plugins/ README.md
git commit -m "docs(samples): refresh plugin examples (mjun0812 data, $(date +%Y-%m-%d))"
```

Conventional Commits per project convention. If the regeneration
was triggered by a code change, bundle the commit with that
change instead.

## 6. Fill in human-authored zones (first-time only)

For each newly-created `docs/plugins/<slug>.md` page, the tool
emits TODO markers in the human-authored zones:

```markdown
## このプラグインを使うべきケース

<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー /
リポジトリで価値を持つか、どんな入力データに依存するか、
を書いてください。 -->
```

Fill in those `TODO` blocks before merging. Subsequent
`make docs` re-runs preserve your edits.

## 7. Adding the compliance test on the first PR

The new
`tests/compliance/compliance_test.go::TestCompliance_DocsPluginPagesMatchAdoptedSet`
test (FR-009) gates the doc-page set against the canonical
19-plugin adopted set. After the first PR lands, any new plugin
addition triggers this test as a reminder to also add the doc
page.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `make docs-samples` fails on a specific plugin | Token lacks required scope (e.g., sponsors needs `read:user`) | Add the scope to your PAT; re-run only the failed plugin |
| chromium not found | `METRICS_CHROME_PATH` unset / wrong path | Set env to the real chromium binary path |
| Diff shows huge SVG churn after re-run | NormalizeSVG mask not applied (script error) or upstream-data changed substantially | Inspect a sample SVG — if Last-updated / version strings have leaked, the normalize step failed |
| `make docs` shows zero diff after `metadata.yml` change | The tool's marker regex didn't match the file | Confirm the file has the AUTOGEN markers; on first-ever generation, the tool inserts them |
| README plugin gallery has broken-image icons | `docs/examples/plugin-<slug>.svg` files don't exist | Run `make docs-samples` first |
