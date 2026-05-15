# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)

Go port of [lowlighter/metrics](https://github.com/lowlighter/metrics) for the
adopted feature subset documented in
[`docs/design/15-selection-answer.md`](docs/design/15-selection-answer.md).

**Status: M2 (classic template + JSON output) complete.** Building on the
M1 foundation, `engine.Compute` now emits an upstream-compatible JSON
envelope (`account / user / config / computed / plugins / errors`) and
renders the classic SVG template with four MVP partials. The format
dispatcher routes `json` / `svg` / `png` / `jpeg` according to
[`specs/002-output-classic-json/contracts/result-dispatch.md`](specs/002-output-classic-json/contracts/result-dispatch.md);
`png` and `jpeg` stage SVG bytes with a warn log until the M3 chromedp
rendering pipeline lands. See
[`specs/002-output-classic-json/tasks.md`](specs/002-output-classic-json/tasks.md)
for the per-task breakdown. M3 (rendering pipeline) and M4 (plugins) are
next.

### Output paths at a glance

| Format | Wired in | Output | MIME |
| --- | --- | --- | --- |
| `json` | M2 | `engine.Marshal(data)` | `application/json` |
| `svg` | M2 | `templates.classic.Run` | `image/svg+xml` |
| `png` / `jpeg` | M2 interim | SVG bytes + warn log | `image/png` / `image/jpeg` |

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
