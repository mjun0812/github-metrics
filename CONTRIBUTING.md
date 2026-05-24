# Contributing to github-metrics

Thanks for your interest in contributing. This document covers the
development workflow, test categories, and asset-regeneration paths.
For the user-facing project overview, see [`README.md`](README.md);
for the migration story from upstream, see
[`docs/migration-to-go.md`](docs/migration-to-go.md).

## Scope discipline

This project is a **subset port** of [lowlighter/metrics](https://github.com/lowlighter/metrics).
The user-facing list of supported and unported features lives in
[`docs/migration-to-go.md`](docs/migration-to-go.md). CI gates in
[`tests/compliance/compliance_test.go`](tests/compliance/compliance_test.go)
enforce the adopted plugin set so that unported slugs cannot sneak into
production code.

Concretely, the following are **out of scope** and will be rejected:

- Web instance / OAuth / Insights HTML
- Social and external-API plugins (`anilist`, `leetcode`, `chess`,
  `wakatime`, etc. — 19 slugs)
- Community plugin / template extensions
- PDF / Markdown output formats

If you have a use case that needs one of these, please open a discussion
issue first.

## Development quickstart

```sh
git clone https://github.com/mjun0812/github-metrics.git
cd github-metrics

# Install developer tooling pinned to the same versions CI uses
# (gofumpt, golangci-lint, govulncheck, lefthook).
make tools

# Wire the lefthook git hooks so format + lint + `go mod tidy` run
# automatically on every `git commit`. The hooks run the same binaries
# CI runs, so a failing commit is the same failure CI would report.
make hooks-install

# Build, test, and lint.
make build
make test
make lint
```

To run the full pre-commit pipeline manually (e.g. before pushing a
branch with stacked commits):

```sh
make hooks-run
```

To bypass the hooks for a single commit (do not make a habit):

```sh
git commit --no-verify
```

## Toolchain

| Tool          | Pinned version | Where it lives                                          |
| ------------- | -------------- | ------------------------------------------------------- |
| Go            | 1.26.3         | `.github/workflows/go-ci.yml`, `go.mod` (minimum 1.26)  |
| golangci-lint | v2.12.2        | `Makefile`, `.github/workflows/go-ci.yml`, `lefthook.yml` |
| gofumpt       | latest         | `Makefile`, `lefthook.yml`                              |
| govulncheck   | latest         | `Makefile`, `.github/workflows/go-ci.yml`               |
| lefthook      | latest         | `Makefile`, `lefthook.yml` (the git-hook manager)       |

`lefthook` is a single Go binary — no Python, Node, or Ruby toolchain
required. Bump these together when upgrading; drift between local and
CI is the single biggest source of red builds.

## Test categories

Default `make test` deliberately keeps three categories optional so a
clean checkout passes on a stock Go toolchain. CI runs all of them in
parallel jobs.

### chromedp tests

The chromedp-backed render tests and the `topics` / `starlists` plugin
scrape tests are gated behind `//go:build chromedp` so contributors
without a chromium binary stay green. The plugin runtimes themselves
still compile and register on every build — they return `Skipped=true`
at runtime when `pc.Render` is not a real `*render.Browser`.

```sh
# macOS — point at the system Chrome (or `brew install chromium`).
METRICS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    make test-chromedp

# Linux — system chromium or chromedp/headless-shell container.
METRICS_CHROME_PATH=/usr/bin/chromium make test-chromedp
```

### heavy tests

The `languages` plugin's `recent` and `indepth` sub-modes pull in
go-enry and go-git; their fixture-heavy tests are gated behind
`//go:build heavy`.

```sh
make test-heavy
```

### docker-smoke tests

The production Dockerfile smoke test (build + `--help` invocation +
image-size assertion) is gated behind `//go:build docker_smoke` and
requires docker on PATH.

```sh
make docker-smoke
```

## Releasing

The maintainer release procedure is split into three steps — dry-run
gate, `action.yml` pinning, and tag push followed by post-release
verification. Run `make release-dry-run` before pushing a tag to catch
issues early; the [`scripts/release-verify.sh`](scripts/release-verify.sh)
helper covers the post-release manifest / signature / checksum checks.

```sh
make release-dry-run
```

## Asset regeneration

### Octicons

`assets/octicons/data.json` is generated from the npm-published
`@primer/octicons` build. To refresh it (when bumping the upstream
version):

```sh
npm install --no-save @primer/octicons
make gen-octicons
```

`make verify-octicons` re-runs the generator and diffs the result —
useful as a CI gate when the upstream version pin advances.

### `action.yml`

`action.yml` is auto-generated from `assets/plugins/<slug>/metadata.yml`
plus the core inputs by `internal/tools/gen-action-yml`. The lefthook
`action-yml-drift` hook re-runs the generator on every commit and
fails if the committed file would drift.

```sh
make gen-action-yml          # default (image: 'deploy/Dockerfile')
VERSION=v1.0.0 make gen-action-yml   # release pin (image: 'docker://...')
```

### Upstream fixtures (optional)

The compatibility test compares the engine's JSON output to a captured
upstream baseline at `tests/fixtures/upstream/octocat.json`. That
fixture is regenerated from a local `./org_repo` checkout via:

```sh
make sync-fixtures
```

`./org_repo` is intentionally `.gitignore`'d — the project must not
contain upstream history. Contributors who need to refresh the fixture
clone `lowlighter/metrics` to `./org_repo` first
(`cd org_repo && npm install`) and then run the target. Tests skip
gracefully when the fixture is absent, so a fresh checkout without
`./org_repo` still passes CI.

## Project layout

```
cmd/metrics-action/   GitHub Action / CLI entry point
cmd/metrics-cli/      Standalone CLI entry point
internal/             All non-public packages (logger, errors, ctxutil,
                      format, config, httpx, githubapi, plugins,
                      templates, plugins/core, ...)
internal/testutil/    Shared mocks + golden file helpers (test-only)
assets/               Embedded plugin / template metadata
                      (vendored from ./org_repo via scripts/sync-assets.sh)
deploy/               Production Dockerfile + deployment manifests
scripts/              release-verify.sh and other maintainer helpers
tests/                Fixtures, golden files, compliance + integration tests
docs/migration-to-go.md   User-facing migration guide (Japanese)
docs/design/          Design corpus (Japanese — internal reference)
```

`internal/` is the standard Go visibility boundary — nothing under it
is part of a public API. The project intentionally does **not** ship a
`pkg/` directory; the Action / CLI binaries are the only public
surfaces.

## Commit and PR conventions

- **Conventional Commits** — `feat:` / `fix:` / `chore:` / `docs:` /
  `test:` etc. The subject line is short and concrete; vague verbs
  like "update" or "fix" without context are rejected.
- **Semantic Versioning** — git tags use the `vX.Y.Z` form;
  pre-release suffixes follow SemVer 2.0.0 (e.g. `v1.0.0-rc1`).
- **PR description** — explain *why*, not *what*. Link to the issue or
  discussion that drove the change. Compatibility-relevant changes
  (input naming, output DOM structure, scope) must call out the impact
  explicitly so reviewers can gate on it.

## Reporting bugs / requesting features

- **Bug**: open an issue with a minimal repro (workflow YAML or CLI
  command + observed vs expected output).
- **Feature request**: please first check
  [`docs/migration-to-go.md`](docs/migration-to-go.md) §3 — many
  upstream features are intentionally unported. If your request is
  for an unported feature, the answer is likely to remain "out of
  scope" but open an issue and the maintainer will review.
- **Security issue**: please email the maintainer directly rather
  than opening a public issue.

## License

By contributing you agree that your contributions are licensed under
the project's [MIT License](LICENSE).
