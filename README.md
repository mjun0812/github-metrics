# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)

Go port of [lowlighter/metrics](https://github.com/lowlighter/metrics) for the
adopted feature subset documented in
[`docs/design/15-selection-answer.md`](docs/design/15-selection-answer.md).

**Status: M6 (Action / CLI) complete.** The `metrics-action` binary
is now usable as both a GitHub Action (Docker image at
`ghcr.io/mjun0812/github-metrics:<tag>`) and a standalone CLI on
macOS / Linux. Inputs flow through one unified pipeline that wraps
M1-M4: 21 plugins + classic template + M3 chromedp render. The
release pipeline (`.github/workflows/release.yml`) publishes a
multi-tag image (`vX.Y.Z` + `latest` + `sha-<short>`) and four
cross-compiled binaries (linux/darwin × amd64/arm64) on every semver
tag. See [`specs/005-m6-action-cli/`](specs/005-m6-action-cli/) for
the spec, plan, and tasks. M5 (Web instance) is intentionally
out-of-scope per
[`docs/design/15-selection-answer.md`](docs/design/15-selection-answer.md);
M7+ continues with snapshot / replay infrastructure.

## Usage as a GitHub Action

```yaml
# .github/workflows/metrics.yml
name: Metrics
on:
  schedule: [{cron: '0 0 * * *'}]
  workflow_dispatch:

jobs:
  github-metrics:
    runs-on: ubuntu-latest
    steps:
      - uses: mjun0812/github-metrics@v0.6.0
        with:
          user: octocat
          token: ${{ secrets.METRICS_TOKEN }}
          template: classic
          plugin_languages: 'yes'
          plugin_languages_limit: '5'
          committer_branch: main
          output_action: commit
          output_condition: data-changed
```

The action runs the `metrics-action` binary inside the
`ghcr.io/mjun0812/github-metrics` Docker image. Pin to a semver tag
(`@v0.6.0`) for reproducibility; the `@latest` tag is also published
for convenience.

## Usage as a CLI

```sh
# Install from a GitHub Release (linux/darwin × amd64/arm64).
curl -L -o metrics-action \
  https://github.com/mjun0812/github-metrics/releases/download/v0.6.0/metrics-action_v0.6.0_darwin_arm64
chmod +x metrics-action

# Or via go install (requires Go 1.26+).
go install github.com/mjun0812/github-metrics/cmd/metrics-action@v0.6.0

# Or via Docker (no install needed). The CLI's --filename is relative
# to the binary's working directory; pass an absolute path so the file
# lands inside the mounted volume, or override the working dir via -w.
docker run --rm \
  -v "$PWD/out:/renders" \
  -w /renders \
  -e GITHUB_TOKEN \
  ghcr.io/mjun0812/github-metrics:v0.6.0 \
  --user octocat --token-env GITHUB_TOKEN --template classic \
  --output svg --filename github-metrics.svg

# Pipe to xmllint for a quick sanity check. Requires a real GITHUB_TOKEN
# (set in your shell) because the base/core plugin fetches the user
# profile from api.github.com even when other plugin gates are off.
metrics-action --user octocat --token-env GITHUB_TOKEN \
  --output svg --dryrun --filename - | xmllint --format -
```

See [`specs/005-m6-action-cli/quickstart.md`](specs/005-m6-action-cli/quickstart.md)
for the full input matrix (`--config <path>.yaml`, `--preset`,
`--token-env`, all 21 plugin gates).

### Output paths at a glance

| Format | Wired in | Output | MIME |
| --- | --- | --- | --- |
| `json` | M2 | `engine.Marshal(data)` | `application/json` |
| `svg` | M2 + M3 deco/resize | classic template → octicon → optional css/xml → chromedp Resize | `image/svg+xml` |
| `png` / `jpeg` | M3 | same as `svg` + chromedp `page.CaptureScreenshot` | `image/png` / `image/jpeg` |

## Quickstart for contributors

```sh
git clone https://github.com/mjun0812/github-metrics.git
cd github-metrics

# Install developer tooling pinned to the same versions CI uses.
# This brings in gofumpt, golangci-lint, govulncheck, and lefthook.
make tools

# Wire the lefthook git hooks so format + lint + `go mod tidy` run
# automatically on every `git commit`. They run the same binaries CI
# runs, so a failing commit is the same failure CI would report.
make hooks-install

# Build, test, and lint.
make build
make test
make lint
```

### chromedp tests (M3+)

The default `make test` deliberately skips the chromedp-backed render
tests and the M4 chromedp plugins' browser-driven tests
(`*_chromedp_test.go` for topics / starlists) so contributors without a
chromium binary stay green. The plugin runtimes themselves still
compile and register on every build — they just return `Skipped=true`
at runtime when `pc.Render` is not a real `*render.Browser`. To
exercise the resize / PNG / JPEG path and the topics / starlists
scrape end-to-end on a machine with chromium:

```sh
# macOS — point at the system Chrome (or `brew install chromium`).
METRICS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    make test-chromedp

# Linux — system chromium or chromedp/headless-shell container.
METRICS_CHROME_PATH=/usr/bin/chromium make test-chromedp
```

### heavy tests (M4+)

M4 ships two `languages` sub-mode plugins (`languages.recent` /
`languages.indepth`) whose fixture-heavy tests pull in go-enry +
go-git. The runtimes ship without a build tag and register on every
build, but their fixture tests (`recent_heavy_test.go` /
`indepth_heavy_test.go` and `tests/integration/plugins_p3_heavy_test.go`)
are isolated behind `//go:build heavy` so `make test` stays fast. To
run them:

```sh
make test-heavy
```

CI runs all three test jobs in parallel (`test`, `test-chromedp` via
`chromedp/headless-shell:latest`, `test-heavy` on the standard runner),
so a fresh checkout that opts out of chromedp/heavy locally still
gates against regressions in CI.

### Octicon asset regeneration

`assets/octicons/data.json` is generated from the npm-published
`@primer/octicons` build. To refresh it (when bumping the upstream
version):

```sh
npm install --no-save @primer/octicons
make gen-octicons
```

`make verify-octicons` re-runs the generator and diffs the result —
useful as a CI gate when the upstream version pin advances. (The
embedded `_meta.generated_at` is non-deterministic; a Polish-phase
follow-up will flag this with a `--frozen-source` switch.)

To run the full pre-commit pipeline manually (e.g. before pushing a
branch that has stacked commits):

```sh
make hooks-run
```

To bypass the hooks for a single commit (do not make a habit):

```sh
git commit --no-verify
```

### Upstream fixtures (optional)

The SC-001 compatibility check compares the engine's JSON output to a
captured upstream baseline at `tests/fixtures/upstream/octocat.json`.
That fixture is regenerated from a local `./org_repo` checkout via:

```sh
make sync-fixtures
```

`./org_repo` is intentionally gitignored — the constitution forbids
mixing upstream history into this repository. Contributors who need to
refresh the fixture clone `lowlighter/metrics` to `./org_repo` first
(`cd org_repo && npm install`) and then run the target. Tests skip
gracefully when the fixture is absent, so a fresh checkout without
`./org_repo` still passes CI.

## Toolchain

| Tool | Pinned version | Where it lives |
| --- | --- | --- |
| Go | 1.26.3 | `.github/workflows/go-ci.yml`, `go.mod` (minimum 1.26) |
| golangci-lint | v2.12.2 | `Makefile`, `.github/workflows/go-ci.yml`, `lefthook.yml` |
| gofumpt | latest | `Makefile`, `lefthook.yml` |
| govulncheck | latest | `Makefile`, `.github/workflows/go-ci.yml` |
| lefthook | latest | `Makefile`, `lefthook.yml` (the git-hook manager) |

`lefthook` is a single Go binary — no Python, Node, or Ruby toolchain
required. Bump these together when upgrading; drift between local and
CI is the single biggest source of red builds.

## Project layout

See [`specs/001-project-foundation/plan.md`](specs/001-project-foundation/plan.md)
for the authoritative reference. The short version:

```
cmd/metrics-action/   GitHub Action / CLI entry point
cmd/metrics-cli/      Standalone CLI entry point
internal/             All non-public packages (logger, errors, ctxutil,
                      format, config, httpx, githubapi, plugins,
                      templates, plugins/core, ...)
assets/               Embedded plugin/template metadata (vendored from
                      ./org_repo via scripts/sync-assets.sh)
tests/                Fixtures, golden files, and integration tests
docs/design/          Design corpus (Japanese)
specs/                Spec Kit feature specifications and plans
.specify/             Spec Kit machinery (workflow templates, hooks)
```

## License

[MIT](LICENSE). Portions are derived from `lowlighter/metrics`
(MIT-licensed); the upstream attribution is preserved.
