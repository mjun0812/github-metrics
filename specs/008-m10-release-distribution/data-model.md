# Data Model: M10 — release / Docker distribution

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

M10 has no runtime data model — the change surface is build pipeline
artifacts and configuration files. This document enumerates those as
"entities" (each backed by a contract under `contracts/`) so the
plan-phase shape is captured before tasks generation.

## E-001: Release artifact set (per semver tag)

- **Trigger**: a push of a tag matching `v*.*.*` to `main`
  (or a manual `workflow_dispatch` invocation with
  `dry_run=false`).
- **Outputs**: bundle of 9 artifacts plus signatures.

| Artifact | Location | Per-platform | Signature |
|----------|----------|-------------|-----------|
| Docker manifest list | `ghcr.io/mjun0812/github-metrics:vX.Y.Z` + `:latest` + `:sha-<short>` | Yes (amd64, arm64) | cosign keyless (image-level) |
| `metrics-action_vX.Y.Z_linux_amd64` | GitHub Releases asset | linux/amd64 | cosign sign-blob + bundle file |
| `metrics-action_vX.Y.Z_linux_arm64` | GitHub Releases asset | linux/arm64 | cosign sign-blob + bundle file |
| `metrics-action_vX.Y.Z_darwin_amd64` | GitHub Releases asset | darwin/amd64 | cosign sign-blob + bundle file |
| `metrics-action_vX.Y.Z_darwin_arm64` | GitHub Releases asset | darwin/arm64 | cosign sign-blob + bundle file |
| `SHA256SUMS` | GitHub Releases asset | N/A | cosign sign-blob + bundle file |

- **Provenance pin**: All signatures are verifiable against
  `--certificate-identity-regexp 'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*'`
  + `--certificate-oidc-issuer https://token.actions.githubusercontent.com`.
- **Idempotency**: Re-running the workflow against the same tag is
  refused by `gh release upload --clobber` semantics (clobber on
  the same tag overwrites, but cosign Rekor entries are immutable
  so the audit trail of the original run remains).
- **Dry-run mode**: artifact-build steps still run; all publish
  steps (`docker push`, `gh release upload`, `cosign sign`) are
  skipped. Built artifacts are uploaded as workflow artifacts with
  7-day retention for manual inspection.

**Spec linkage**: FR-001..FR-004, FR-009; SC-001..SC-004, SC-007.

## E-002: Production Dockerfile

- **Source file**: `deploy/Dockerfile`
- **Build stages**: 2 — `build` (`golang:1.26-bookworm`) + `runtime`
  (`debian:bookworm-slim`).
- **Build args**: `VERSION` (string, no default — release.yml passes
  the tag).
- **Build-stage outputs**: 2 statically-linked Go binaries
  (`metrics-action`, `metrics-cli`) at `/out/`, built with
  `-trimpath -ldflags="-s -w -X main.version=${VERSION}"` per
  constitution Technical Constraints.
- **Runtime environment**:
  - apt packages: `ca-certificates chromium fonts-noto-color-emoji
    fonts-noto-cjk fonts-liberation`
  - User: `metrics` (uid 10001, gid 10001, no home, `/sbin/nologin`
    shell) — see R-004.
  - ENV: `METRICS_CHROME_PATH=/usr/bin/chromium`.
  - Binary location: `/usr/local/bin/metrics-action`,
    `/usr/local/bin/metrics-cli` (mode 0755, owned by root).
  - ENTRYPOINT: `["/usr/local/bin/metrics-action"]`.
  - HEALTHCHECK: NONE (Action container is short-lived per
    invocation; healthchecks have no consumer).
- **Size budget**: ≤ 600 MB per platform after build (asserted by
  release.yml — see R-003).
- **Multi-arch posture**: Builds cleanly under `docker buildx
  build --platform linux/amd64,linux/arm64`. No platform-specific
  RUN steps; Go binary is built on the host arch via multi-stage
  (buildx auto-applies the `--platform` flag to the runtime stage,
  pulling the matching `debian:bookworm-slim` arch).

**Spec linkage**: FR-005, FR-006; SC-002, SC-003.

## E-003: Release workflow (`.github/workflows/release.yml`)

- **Triggers**: `push.tags: 'v*.*.*'` (semver tags only) +
  `workflow_dispatch.inputs.dry_run` (boolean string, default
  `'true'`).
- **Jobs** (renamed and expanded from the M6 baseline):
  - `release-docker`: buildx multi-arch image → GHCR push → cosign
    sign manifest list. Adds the new size-budget assertion step
    (per R-003).
  - `release-binary`: cross-compile matrix (4 variants) → SHA256SUMS
    generation → cosign sign-blob each artifact → GitHub Release
    upload. Adds the new checksum + sign-blob steps.
- **Permissions** required (workflow-level):
  - `contents: write` — GitHub Release asset upload.
  - `packages: write` — GHCR push.
  - `id-token: write` — cosign keyless OIDC (NEW for M10).
- **Failure semantics**:
  - Size budget exceeded → release-docker job fails before push
    (so no oversized image lands in GHCR).
  - Cosign signing failure → corresponding job fails after artifact
    upload but before release tag finalization; partial release is
    visible but flagged.
  - Dry-run mode → no failure asserts (size-budget check is still
    advisory and printed in the log, but does not fail the
    workflow).
- **End-to-end runtime budget**: ≤ 25 minutes per SC-007
  (representative breakdown: buildx setup 30s + amd64 build 3min
  + arm64 emulated build 6-8min + cosign 30s × 2 + cross-compile
  4 binaries 2min + SHA256SUMS + cosign blob × 5 + GH Release
  upload 1min ≈ 18-22min total).

**Spec linkage**: FR-001..FR-004, FR-009; SC-007.

## E-004: Migration guide (`docs/migration-to-go.md`)

- **Language**: Japanese (constitution V).
- **Length budget**: ≤ 250 lines (10-minute-reader target per SC-005).
- **Required sections** (6, per R-005):
  1. **概要 (Overview)** — purpose, audience, scope.
  2. **採用機能一覧 (Adopted feature matrix)** — table of 21
     plugins + 2 templates + adopted output formats, with links
     back to `docs/design/15-selection-answer.md`.
  3. **未対応機能一覧 (Unported feature matrix)** — mirror table
     of unported plugins (M8 social), unported templates
     (terminal/markdown/community), unported output formats
     (PDF, Markdown), unported runtime (M5 web instance).
  4. **入力互換性 (Input compatibility)** — semantics of `with:`
     input handling (drop-in compat for adopted, silent no-op for
     unported); worked example.
  5. **移行手順 (Migration steps)** — 4 steps from pinning the
     new `uses:` to validating DOM-equivalent output.
  6. **ロールバック (Rollback)** — one-line revert procedure +
     why no config migration is needed.
- **Cross-references**:
  - Source of truth for adopted set: `docs/design/15-selection-answer.md`
  - Source of truth for output contract: constitution principle II
  - Source of truth for input compat: constitution principle I
- **Tone**: technical, neutral. No promotional language. Assumes a
  reader who is already running upstream `lowlighter/metrics` in
  production.

**Spec linkage**: FR-008; SC-005.

## E-005: action.yml `runs.image:` reference (minor edit)

- **Source file**: `action.yml` (auto-generated; see
  `internal/tools/gen-action-yml`).
- **Change surface**: 1 line — `runs.image:` (or
  `runs.steps[].image:` depending on the current composite shape).
- **From**: `./Dockerfile` (or the existing `docker://` reference
  pinned at the M6 image tag).
- **To**: `docker://ghcr.io/mjun0812/github-metrics:vX.Y.Z` —
  pinned to the M10-published tag at release time. The release
  workflow updates this line in a follow-up commit *after* the
  GHCR push succeeds and before tagging the release.
- **Idempotency**: the gen-action-yml generator template hosts the
  `docker://` line as a parameterized string; the M10 release
  script substitutes the current tag in. lefthook's
  `action-yml-drift` hook ensures the substitution doesn't drift.
- **Why not always `:latest`** — pinning by version means
  consumers who do `uses: mjun0812/github-metrics@v1.0.0` get the
  exact same image bytes the v1.0.0 tag was built against.
  `:latest` would defeat that guarantee.

**Spec linkage**: FR-007; SC-006.
