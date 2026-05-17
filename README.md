# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)

Go port of [lowlighter/metrics](https://github.com/lowlighter/metrics) for the
adopted feature subset documented in
[`docs/design/15-selection-answer.md`](docs/design/15-selection-answer.md).

**Status: M4 (GitHub plugins) complete.** All 21 採用 plugins are now
live: P1 MVP 5 (languages / activity / achievements / repositories /
isocalendar), P2 GraphQL+REST 12 (calendar / habits / stars / people /
notable / contributors / reactions / projects / sponsors / sponsorships
/ stargazers / traffic), and P3 chromedp + heavy 4 (topics + starlists
behind `chromedp` build tag, languages.recent + languages.indepth
behind `heavy` build tag). `engine.Compute` drives the full plugin
pipeline, the classic template renders per-plugin DOM, and the M3
chromedp render path (octicon → optional CSS purge → optional XML
format → chromedp Resize → PNG / JPEG) wraps the result. See
[`specs/004-m4-github-plugins/tasks.md`](specs/004-m4-github-plugins/tasks.md)
for the per-task breakdown. M5 (deferred-async + caching) is next.

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
tests + the M4 chromedp plugins (topics / starlists) so contributors
without a chromium binary stay green. To exercise the resize / PNG /
JPEG path + chromedp plugins on a machine with chromium:

```sh
# macOS — point at the system Chrome (or `brew install chromium`).
METRICS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    make test-chromedp

# Linux — system chromium or chromedp/headless-shell container.
METRICS_CHROME_PATH=/usr/bin/chromium make test-chromedp
```

### heavy tests (M4+)

M4 ships two `heavy`-tagged plugins (languages.recent / languages.indepth)
that pull in go-enry + go-git fixtures. Their tests are isolated behind
`//go:build heavy` so `make test` stays fast. To run them:

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
