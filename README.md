# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)

Go port of [lowlighter/metrics](https://github.com/lowlighter/metrics) for the
adopted feature subset documented in
[`docs/design/15-selection-answer.md`](docs/design/15-selection-answer.md).
**Status: M1 in progress** — see
[`specs/001-project-foundation/tasks.md`](specs/001-project-foundation/tasks.md)
for the current task list and progress.

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
