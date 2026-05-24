# github-metrics

[![CI](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml/badge.svg)](https://github.com/mjun0812/github-metrics/actions/workflows/go-ci.yml)
[![Release](https://img.shields.io/github/v/release/mjun0812/github-metrics?sort=semver)](https://github.com/mjun0812/github-metrics/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/mjun0812/github-metrics)](https://github.com/mjun0812/github-metrics/blob/main/go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Generate SVG / PNG / JPEG / JSON metrics for your GitHub profile or a single
repository — as a **GitHub Action**, a **standalone CLI**, or a **Docker
container**. Single Go binary, no Node runtime required.

This project is a Go reimplementation of a curated subset of
[lowlighter/metrics](https://github.com/lowlighter/metrics). Adopted inputs
are drop-in compatible: existing upstream workflows continue to run by
swapping one `uses:` line. See
[`docs/migration-to-go.md`](docs/migration-to-go.md) for the full migration
guide and unported-feature list.

---

## Highlights

- **Drop-in Action upgrade** — `uses: mjun0812/github-metrics@v1` reads the
  same `with:` inputs as `lowlighter/metrics@v3` for every adopted feature.
  Unported feature gates (`plugin_anilist: yes` etc.) are silently no-op'd
  so workflow files migrate without edits.
- **21 plugins** across two templates — see [Plugins](#plugins) below.
- **4 output formats** — SVG, PNG, JPEG, JSON (upstream byte-compatible).
- **Multi-arch container** — `linux/amd64` + `linux/arm64` on
  `ghcr.io/mjun0812/github-metrics`, signed with Sigstore cosign keyless
  OIDC (transparency log).
- **Cross-compiled binaries** — Linux / macOS × amd64 / arm64 attached to
  every release with `SHA256SUMS` and per-artifact cosign sign-blob
  bundles.
- **Hardened runtime** — non-root user (uid 10001), chromium + Noto fonts
  bundled, no Node toolchain.

## Quick start

### GitHub Action

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
      - uses: mjun0812/github-metrics@v1
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

`@v1` is the recommended pin — it resolves to the latest `v1.x.y`
release so consumers automatically receive bug-fix and feature
patches without edits. For immutable bytes pin to `@v1.0.0` (or any
exact `vX.Y.Z` form). `@latest` is also published for convenience but
is not recommended for production workflows.

### Repository template

`template: repository` re-centers the rendered SVG on a single repository
instead of a user profile:

```yaml
- uses: mjun0812/github-metrics@v1
  with:
    user: ${{ github.repository_owner }}
    repo: ${{ github.event.repository.name }}
    template: repository
    token: ${{ secrets.METRICS_TOKEN }}
```

The JSON output adds a `data.repo` field next to the existing `data.user`;
the SVG features the repo's avatar, description, community health and
recent activity.

### CLI

```sh
# Install from a GitHub Release (linux/darwin × amd64/arm64).
# Replace `linux_amd64` with your platform tag.
curl -L -o metrics-action \
  https://github.com/mjun0812/github-metrics/releases/download/v1.0.0/metrics-action_v1.0.0_linux_amd64
chmod +x metrics-action

# …or via go install (requires Go 1.26+):
go install github.com/mjun0812/github-metrics/cmd/metrics-action@v1.0.0

# Render an SVG to stdout. GITHUB_TOKEN must be set in your shell.
metrics-action --user octocat --token-env GITHUB_TOKEN \
  --output svg --dryrun --filename -
```

### Docker

```sh
docker run --rm \
  -v "$PWD/out:/renders" \
  -w /renders \
  -e GITHUB_TOKEN \
  ghcr.io/mjun0812/github-metrics:v1.0.0 \
  --user octocat --token-env GITHUB_TOKEN --template classic \
  --output svg --filename github-metrics.svg
```

The image is multi-arch — `docker pull` automatically resolves to your
host architecture.

## Authentication

Every invocation needs a GitHub token:

- **Action mode**: pass via `with.token` — typically
  `${{ secrets.METRICS_TOKEN }}` (a Personal Access Token) for full
  metrics, or `${{ github.token }}` for public-only data.
- **CLI / Docker mode**: set `GITHUB_TOKEN` in the environment and use
  `--token-env GITHUB_TOKEN`.

The token needs at minimum `public_repo` (classic PAT) or the read scopes
for each enabled plugin's data (issues, pulls, projects, etc.) for
fine-grained tokens. The `base` / `core` plugins always fetch the target
user's profile, so a missing token fails immediately.

## Plugins

<!-- AUTOGEN_START: plugins-gallery -->
| | | |
|:---:|:---:|:---:|
| [![achievements](docs/examples/plugin-achievements.svg)](docs/plugins/achievements.md) | [![activity](docs/examples/plugin-activity.svg)](docs/plugins/activity.md) | [![calendar](docs/examples/plugin-calendar.svg)](docs/plugins/calendar.md) |
| [`achievements`](docs/plugins/achievements.md) | [`activity`](docs/plugins/activity.md) | [`calendar`](docs/plugins/calendar.md) |
| [![contributors](docs/examples/plugin-contributors.svg)](docs/plugins/contributors.md) | [![habits](docs/examples/plugin-habits.svg)](docs/plugins/habits.md) | [![isocalendar](docs/examples/plugin-isocalendar.svg)](docs/plugins/isocalendar.md) |
| [`contributors`](docs/plugins/contributors.md) | [`habits`](docs/plugins/habits.md) | [`isocalendar`](docs/plugins/isocalendar.md) |
| [![languages](docs/examples/plugin-languages.svg)](docs/plugins/languages.md) | [![notable](docs/examples/plugin-notable.svg)](docs/plugins/notable.md) | [![people](docs/examples/plugin-people.svg)](docs/plugins/people.md) |
| [`languages`](docs/plugins/languages.md) | [`notable`](docs/plugins/notable.md) | [`people`](docs/plugins/people.md) |
| [![projects](docs/examples/plugin-projects.svg)](docs/plugins/projects.md) | [![reactions](docs/examples/plugin-reactions.svg)](docs/plugins/reactions.md) | [![repositories](docs/examples/plugin-repositories.svg)](docs/plugins/repositories.md) |
| [`projects`](docs/plugins/projects.md) | [`reactions`](docs/plugins/reactions.md) | [`repositories`](docs/plugins/repositories.md) |
| [![sponsors](docs/examples/plugin-sponsors.svg)](docs/plugins/sponsors.md) | [![sponsorships](docs/examples/plugin-sponsorships.svg)](docs/plugins/sponsorships.md) | [![stargazers](docs/examples/plugin-stargazers.svg)](docs/plugins/stargazers.md) |
| [`sponsors`](docs/plugins/sponsors.md) | [`sponsorships`](docs/plugins/sponsorships.md) | [`stargazers`](docs/plugins/stargazers.md) |
| [![starlists](docs/examples/plugin-starlists.svg)](docs/plugins/starlists.md) | [![stars](docs/examples/plugin-stars.svg)](docs/plugins/stars.md) | [![topics](docs/examples/plugin-topics.svg)](docs/plugins/topics.md) |
| [`starlists`](docs/plugins/starlists.md) | [`stars`](docs/plugins/stars.md) | [`topics`](docs/plugins/topics.md) |
| [![traffic](docs/examples/plugin-traffic.svg)](docs/plugins/traffic.md) | | |
| [`traffic`](docs/plugins/traffic.md) | | |
<!-- AUTOGEN_END: plugins-gallery -->

The 19 user-facing plugins above are always available; enable each via
`plugin_<slug>: yes` and click a tile for the full per-plugin doc
page. Two additional internal plugins (`base`, `core`) power the
metadata pipeline and run automatically.

The `languages` plugin ships `recent` and `indepth` sub-modes via
`plugin_languages_sections`. `topics` and `starlists` require the
Action / Docker runtime (Chromium is bundled there) — the standalone
Go binary skips them with a warning.

Every input is documented in [`action.yml`](action.yml) and is identical
to the corresponding upstream input. Inputs gating unported plugins
(e.g., `plugin_anilist`, `plugin_leetcode`) are accepted and silently
ignored — no migration of existing workflows is required.

## Output formats

| Format         | MIME              | Rendering pipeline                                  |
| -------------- | ----------------- | --------------------------------------------------- |
| `svg`          | `image/svg+xml`   | Go templates → chromedp `Resize`                    |
| `png` / `jpeg` | `image/png/jpeg`  | + chromedp `CaptureScreenshot`                      |
| `json`         | `application/json`| Upstream byte-compatible envelope                   |

JSON output is **byte-compatible** with upstream `lowlighter/metrics` for
the adopted plugins; downstream tools that consume the JSON envelope work
unchanged. SVG output is **DOM-structurally equivalent** — element /
attribute / class structure matches upstream, but dynamic strings
(version footer, generated timestamp) differ.

## Migrating from `lowlighter/metrics`

1. Swap the `uses:` line:

   ```diff
   - uses: lowlighter/metrics@v3.34
   + uses: mjun0812/github-metrics@v1
   ```

   `@v1` automatically resolves to the latest `v1.x.y` release; use
   `@v1.0.0` (or any exact `vX.Y.Z`) if you prefer to pin immutable
   bytes.

2. Re-run the workflow. Unported plugin gates remain in your `with:`
   block silently; remove them at your leisure.

3. Compare output: JSON should diff cleanly; SVG should match
   structurally (`xmllint --format` before diffing).

[`docs/migration-to-go.md`](docs/migration-to-go.md) walks through the
full matrix — adopted vs unported plugins, templates, output formats,
and the one-line rollback procedure.

## Release verification

Every release is signed with cosign keyless OIDC against the GitHub
Actions issuer. To verify the image manifest:

```sh
cosign verify ghcr.io/mjun0812/github-metrics:v1.0.0 \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Binaries on the GitHub Release ship with a `SHA256SUMS` file plus
per-artifact `.sig`, `.cert`, and `.cosign.bundle` files. Verify a
binary with `cosign verify-blob`:

```sh
cosign verify-blob metrics-action_v1.0.0_linux_amd64 \
  --bundle metrics-action_v1.0.0_linux_amd64.cosign.bundle \
  --certificate-identity-regexp \
    'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The helper [`scripts/release-verify.sh`](scripts/release-verify.sh) wraps
the manifest, checksum, signature, and `action.yml` reference checks
into a single command for maintainer post-release validation.

## Contributing

Bug reports and pull requests welcome. Before opening a PR, please read
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the development workflow
(toolchain, hooks, test categories) and the project's scope discipline.
The unported-feature list lives in
[`docs/migration-to-go.md`](docs/migration-to-go.md) §3 — proposals that
extend that scope are welcome via a discussion issue first.

## License

[MIT](LICENSE). This project is a Go reimplementation derived from
[lowlighter/metrics](https://github.com/lowlighter/metrics) (MIT-licensed);
upstream attribution is preserved per the MIT license terms.

GitHub Octicons assets (under `assets/octicons/`) are MIT-licensed by
GitHub, Inc. and embedded via `//go:embed`.
