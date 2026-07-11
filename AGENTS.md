# github-metrics

A GitHub profile card generator that ports a **subset** of upstream `lowlighter/metrics` (Node.js/EJS) to Go. It runs in two forms: GitHub Action and CLI (the Web instance is not adopted). Since v4.0.0 (2026-07-09), rendering is fully browser-free.

## Adopted MVP scope (DO NOT DEVIATE)

Before starting on any new feature, always check the following source of truth for what is in scope.

- **Adopted feature definitions / MVP scope**: [docs/scope.md](docs/scope.md)

### Skipped (implementation forbidden)

- **M5**: Web instance (HTTP-facing features such as the chi server / OAuth / insights) — decided as "high operational cost, not needed"
- **M8**: Social / external API plugins (anilist / leetcode / chess / steam / music / pagespeed / tweets / stackoverflow / wakatime, etc.) — none adopted

### Enforcement (compliance tests)

`tests/compliance/compliance_test.go` gates the following. Fix immediately if any of them fail:

- `TestCompliance_M4_AdoptedPlugins`: exact match of the 19 adopted plugin dirs
- `TestNoUnadoptedPluginReference`: detects non-adopted plugin slugs leaking into production code
- `TestNoRemovedSentinelComments`: forbids `// removed:`-style deletion-history comments — **code comments must describe only the current behavior, never "what was removed"**

## Rendering architecture (since #409)

v4.0.0 removed chromedp/Chromium and fully migrated to a native SVG + resvg pipeline. Whenever you touch anything visual, work from these premises:

- **Partials emit native SVG and self-report their consumed height**: the signature is `(markup string, height int, err error)`. Templates stack them vertically with `chrome.StackSections`, and the root `<svg height>` is finalized on the Go side (there is no `height="99999"` placeholder or measurement pass)
- **SVG primitives live in `internal/templates/chrome/svg.go`** (`WrapSection` / `SVGText` / `SVGField` / `SVGColumn` / `SVGAvatarGrid`, etc.). New partial implementations should reuse them
- **Text width is measured with `internal/render/fontmetrics`** (embedded Liberation Sans). Fallback fonts in viewers' browsers can be up to ~13.5% wider, so **give fixed-width elements ~14% headroom** (`chipLabelSafety` / `achValueSafety` are precedents) and center overflow-prone ones with `text-anchor="middle"`
- **PNG/JPEG are rendered by `internal/render/resvg.go`** via the resvg binary (subprocess, `METRICS_RESVG_PATH`). resvg does not interpret `var()` / `:root` / `:not`, etc., so **emit colors as literal fill/stroke attributes** (`@keyframes` are ignored and drawn with the inline final values, so animation CSS for gauges and the like may stay)
- **Nested `<svg>` viewports are clipped to the card width (480px) and the reported height**: unlike HTML, overflow never becomes visible. Don't forget to clamp / account for height so elements don't cross the boundary (PR #722 is a precedent)
- The "chrome" in `chrome_*` inputs and `internal/templates/chrome` is **a UI term (the card's frame) and has nothing to do with the browser**

See [docs/rendering.md](docs/rendering.md) for details.

## Development workflow

- **Tests**: `make test` (unit). To update goldens: `go test ./tests/integration/... -update` + `-update` for the affected packages + `UPDATE_GOLDEN=1 go test ./internal/render -run TestHash_GoldenOctocat`. **Revert goldens whose only change is the footer timestamp — do not commit them**
- **resvg-dependent tests**: `make test-resvg` (requires the resvg binary + `METRICS_RESVG_PATH`). Auto-skipped when unset
- **lint**: CI's golangci-lint has prealloc / unparam / revive / gosec enabled, which `make test` cannot catch. **Always run `golangci-lint run ./internal/... ./tests/...` locally to 0 issues before pushing**
- **Input validation**: there is no layer that applies metadata.yml min/max at runtime. **Always clamp new integer inputs at the read site** (fall back to the default for non-positive values; cap GraphQL connection `first` at 100. `habits.go` / `reactions.go` #472 are precedents)
- **Verifying visual changes**: rasterize golden SVGs to PNG with resvg and inspect them visually. resvg skips all `<text>` when it cannot resolve font-family, so pass a generic mapping such as `--sans-serif-family "Liberation Sans"`
- **Doc samples**: `docs/examples/` is updated via draft PRs by the regen-doc-samples workflow (`gh workflow run regen-doc-samples.yml -f branch=main`). Do not edit locally
- **Releases**: pushing a semver tag (`vX.Y.Z`) is all it takes — release.yml does everything (multi-arch images + binaries + cosign + vMAJOR floating tag)

## Historical context

- M1–M10 (porting MVP): released as v1.0.0 on 2026-05-18
- #409 (chromedp removal, native SVG + resvg): released as v4.0.0 on 2026-07-09. The background and decision log are recorded in issue #409 and sub-issues #682–#695
