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
make tools

# Wire the local pre-commit hook so format + lint + go mod tidy run
# automatically on every `git commit`. This catches the same issues CI
# does, before the PR opens.
pip install pre-commit          # or `pipx install pre-commit` / `brew install pre-commit`
make pre-commit-install

# Build, test, and lint.
make build
make test
make lint
```

To run the full pre-commit pipeline manually:

```sh
make pre-commit-run
```

## Toolchain

| Tool | Pinned version | Where it lives |
| --- | --- | --- |
| Go | 1.26.3 | `.github/workflows/go-ci.yml`, `go.mod` (minimum 1.26) |
| golangci-lint | v2.12.2 | `Makefile`, `.github/workflows/go-ci.yml`, `.pre-commit-config.yaml` |
| gofumpt | latest | `Makefile`, `.pre-commit-config.yaml` |
| govulncheck | latest | `Makefile`, `.github/workflows/go-ci.yml` |
| pre-commit | any recent | developer-installed; hooks declared in `.pre-commit-config.yaml` |

Bump these together when upgrading — drift between local and CI is the
single biggest source of red builds.

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
