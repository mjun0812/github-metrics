# Quickstart: M10 — cutting a release

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md)

This is the maintainer-facing flow for publishing `vX.Y.Z` once
M10 lands. Read this before tagging a release.

## 0. Prerequisites

- Working tree on `main` is clean.
- All M1-M9 tests pass (`make test`).
- `make lint` returns 0 issues.
- The constitution III compliance test passes
  (`go test ./tests/compliance/...`).
- (Optional, recommended) chromedp / heavy test jobs locally green
  (`make test-chromedp`, `make test-heavy`) — CI runs them anyway,
  but local pre-flight saves a round trip.

## 1. Pre-tag dry-run (recommended)

Run the release workflow in `dry_run` mode from `main` to verify
all artifact-build steps succeed without publishing anything:

```bash
gh workflow run release.yml -f dry_run=true
gh run watch  # follow the run to completion
```

Expected outcome:

- All jobs finish green within ~25 minutes (per SC-007).
- Workflow artifacts panel lists:
  - `metrics-action-linux-amd64/` … `…darwin-arm64/` (4 binary
    bundles, each with the binary + `.sig` + `.cert` +
    `.cosign.bundle` files)
  - `SHA256SUMS` (1 file)
  - Optional: image-build log mentioning the per-platform size.
- No GHCR push, no GitHub Release entry, no Rekor transparency
  entry.

If the dry-run surfaces a size-budget overrun → see R-003 for the
escalation (drop CJK fonts or escalate the budget); do not tag
the release.

## 2. Pin action.yml to the release tag, commit, and tag

`action.yml` on `main` points at `deploy/Dockerfile` so contributors
working off a local checkout (`uses: ./`) build from source. The
release commit pins it to the published GHCR tag so consumers using
`uses: mjun0812/github-metrics@vX.Y.Z` resolve to the exact image
built for that tag.

```bash
git checkout main
git pull

# Pin action.yml to the release tag.
VERSION=v1.0.0 make gen-action-yml
git add action.yml
git commit -m "chore(release): pin action.yml to v1.0.0"

git tag -a v1.0.0 -m "Release v1.0.0 — M10 publish"
git push origin main v1.0.0
```

The release workflow auto-triggers on the tag push. The
`docker-smoke` gate runs first; release-docker and release-binary
fan out once smoke is green.

> **Note**: the lefthook `action-yml-drift` hook runs `make
> gen-action-yml` *without* `VERSION` set, so the release commit
> above will fail the hook (the pre-commit hook sees a diff between
> what the generator emits and what is staged). Use
> `git commit --no-verify` for this single release commit, or
> temporarily disable the drift hook. After the tag is pushed,
> regenerate without `VERSION` (`make gen-action-yml`) and commit
> the revert as `chore(release): unpin action.yml after v1.0.0`
> so `main` returns to the local-dev path.

## 3. Monitor the release run

```bash
gh run watch  # the most recent run, which will be the tag-triggered release
```

Expected outcome:

- `release-docker` job pushes `ghcr.io/mjun0812/github-metrics:v1.0.0`
  + `:latest` + `:sha-<short>` (multi-arch manifest list).
- `release-binary` job uploads 4 binaries + `SHA256SUMS` +
  cosign bundle files as a GitHub Release.
- Size-budget assertion passes (≤ 900 MB per platform — per FR-006 escalation, see `contracts/dockerfile.md` §1 Note).
- Cosign signs the image manifest + each binary; signatures land
  in Rekor.

## 4. Post-release verification

Run the maintainer verification script:

```bash
scripts/release-verify.sh v1.0.0
```

This script asserts:

- `docker manifest inspect ghcr.io/mjun0812/github-metrics:v1.0.0`
  contains both `linux/amd64` and `linux/arm64` entries (SC-001).
- Each binary downloaded from the release page satisfies
  `sha256sum -c SHA256SUMS` (SC-003).
- `cosign verify ghcr.io/.../...:v1.0.0 --certificate-identity-regexp
  'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*'
  --certificate-oidc-issuer https://token.actions.githubusercontent.com`
  returns success (SC-004).
- `action.yml` `image:` line references `:v1.0.0` (FR-007).

All assertions must pass before announcing the release.

## 5. Smoke-test the published Action

Create or trigger a sample workflow that consumes the published
Action:

```yaml
# .github/workflows/sample-action-smoke.yml
name: Sample (post-release smoke)
on: workflow_dispatch

jobs:
  smoke-x86:
    runs-on: ubuntu-latest
    steps:
      - uses: mjun0812/github-metrics@v1.0.0
        with:
          user: octocat
          token: ${{ secrets.METRICS_TOKEN }}
          template: classic
          output: svg
          dryrun: 'yes'

  smoke-arm:
    runs-on: ubuntu-24.04-arm
    steps:
      - uses: mjun0812/github-metrics@v1.0.0
        with:
          user: octocat
          token: ${{ secrets.METRICS_TOKEN }}
          template: classic
          output: svg
          dryrun: 'yes'
```

Both jobs must complete green within 90s per SC-006.

## 6. Update README + announce

- README.md: bump the `@v0.6.0` reference to `@v1.0.0`; add a
  one-line link to `docs/migration-to-go.md` for upstream users.
- (Optional) write a release-notes Markdown block summarizing the
  M10 surface (multi-arch image, signing, migration guide) for
  the GitHub Release page body.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `release-docker` fails at "Size budget exceeded" | apt/chromium upstream package bloated | drop CJK fonts (R-003) or escalate budget |
| Cosign sign step says "OIDC token unavailable" | Workflow lacks `permissions: id-token: write` | add the permission to `release.yml` (already in scope for M10) |
| Sample-smoke arm64 job fails with `exec format error` | image was not multi-arch | re-check the buildx `platforms:` argument; rerun the release |
| `sha256sum -c SHA256SUMS` fails on a downloaded binary | GitHub Release upload corruption (rare) | re-run the release workflow against the same tag with `gh workflow run release.yml -f dry_run=false` |
| Migration-guide reader confused by §3 (unported plugins) | guide is missing a row | add the missing plugin/template to the table; re-run the SC-005 reader-test |

## Local dev shortcuts

- `make docker-smoke` — runs `tests/integration/dockerfile_test.go`
  under `//go:build docker_smoke`; builds the image + asserts the
  size budget + `--help` smoke. Use before pushing a Dockerfile
  change.
- `make release-dry-run` — wraps `gh workflow run release.yml -f
  dry_run=true` for ergonomics.
