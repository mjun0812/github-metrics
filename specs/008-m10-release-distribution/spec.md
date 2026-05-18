# Feature Specification: M10 — release / Docker distribution

**Feature Branch**: `008-m10-release-distribution`

**Created**: 2026-05-18

**Status**: Draft

**Input**: User description: "M10"

## Overview

M10 finalizes the release and distribution layer that M6 partially
stood up: the existing `deploy/Dockerfile` is explicitly labeled
`M3 placeholder; M10 T-126 owns the production-grade multi-arch
finalization`, and `.github/workflows/release.yml` currently builds
a single-arch Docker image plus 4 unsigned CLI binaries without a
checksum file. M10 ships:

- Production-grade Dockerfile (multi-arch linux/amd64 + linux/arm64,
  non-root runtime user, image-size budget verification)
- Release workflow extended with `docker buildx` multi-arch push +
  `SHA256SUMS` checksum file + cosign keyless OIDC signing for image
  and binaries
- `action.yml` end-to-end verified against the published GHCR image
- `docs/migration-to-go.md` — upstream → go-port diff (unsupported
  features, plugin gating, rollback procedure)

Source of truth:
[`docs/design/16-tasks-mvp.md` Phase M10 (T-126, T-128, T-130, T-131)](../../docs/design/16-tasks-mvp.md#phase-m10-リリース--docker-4-タスク),
[`docs/design/10-testing-deployment.md` §5-§6](../../docs/design/10-testing-deployment.md),
[`docs/design/11-go-migration.md` §4.4](../../docs/design/11-go-migration.md).

The constitution III invariant holds: no new plugin or template lands
in M10. The change is **purely delivery-pipeline finalization** —
hardening existing artifacts and adding the missing
release-engineering surfaces (signing, checksums, multi-arch,
migration guide) so the project can mint a publishable `v1.0.0`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cut a tagged release with multi-arch Docker image + signed binaries (Priority: P1)

A maintainer pushes a semver tag (e.g., `v1.0.0`) to `main`. The
release pipeline produces:

- a GHCR multi-arch Docker manifest list covering linux/amd64 and
  linux/arm64, tagged `vX.Y.Z` + `latest` + `sha-<short>`;
- 4 cross-compiled CLI binaries (linux × amd64/arm64, darwin ×
  amd64/arm64) attached to the GitHub Release;
- a `SHA256SUMS` file listing every binary's hash;
- cosign keyless OIDC signatures for both the Docker image and each
  binary, verifiable via `cosign verify ...`.

**Why this priority**: P1 because this is the single load-bearing
M10 outcome — without it the project cannot mint a reproducible
Action release. Every other story is downstream of this pipeline
working.

**Independent Test**: Push `v0.99.0-rc1` to a fork; release workflow
completes; `docker manifest inspect ghcr.io/<fork>/github-metrics:v0.99.0-rc1`
lists both platforms; GitHub Releases page lists 4 binaries +
`SHA256SUMS` + cosign signature bundles; `sha256sum -c SHA256SUMS`
returns success against the downloaded binaries; `cosign verify`
returns success against the image.

**Acceptance Scenarios**:

1. **Given** a clean working tree on `main`, **When** the maintainer
   pushes a `vX.Y.Z` tag, **Then** the `release.yml` workflow runs to
   completion and publishes all 3 artifact classes (image, binaries,
   checksums + signatures).
2. **Given** a published `vX.Y.Z` image on GHCR, **When** a consumer
   runs `docker pull ghcr.io/mjun0812/github-metrics:vX.Y.Z` on an
   ARM64 host, **Then** the pull resolves to the linux/arm64 entry
   in the manifest list without manual platform negotiation.
3. **Given** a maintainer wants to test the pipeline pre-tag, **When**
   they trigger `release.yml` via `workflow_dispatch` with
   `dry_run=true`, **Then** all artifact-build steps run but no GHCR
   push, no GitHub Release upload, and no cosign signing occur.

---

### User Story 2 - Action consumer pins to released tag and runs successfully (Priority: P2)

An end user adopts the Action by referencing
`uses: mjun0812/github-metrics@v1.0.0` in their workflow. The pinned
tag resolves to the published GHCR image; the action runs to
completion on both x86 and ARM GitHub-hosted runners; the generated
SVG / JSON output matches a locally-built (`uses: ./`) reference
on the same inputs.

**Why this priority**: P2 because it validates the entire packaging
chain end-to-end from the consumer perspective. Blocked by US1
(needs a release tag to pin to).

**Independent Test**: Add a smoke workflow that runs the action
twice — once via `uses: ./` (local source build) and once via
`uses: mjun0812/github-metrics@v1.0.0` — against the same `--user
octocat --dryrun --output svg` input, then byte-diffs the two
outputs (after the standard golden-normalization mask for the
dynamic footer). The two runs must produce identical normalized
output.

**Acceptance Scenarios**:

1. **Given** a published `v1.0.0` Action, **When** a consumer pins
   their workflow with `uses: mjun0812/github-metrics@v1.0.0`,
   **Then** the workflow step completes within 90 seconds on
   `ubuntu-latest` (x86_64).
2. **Given** the same workflow definition, **When** it runs on
   `ubuntu-24.04-arm` (or any arm64 runner), **Then** the step
   completes successfully with no `exec format error` and produces
   bytewise-identical output (post-normalization) to the x86 run.

---

### User Story 3 - Upstream user migrates with a documented guide (Priority: P3)

A developer currently running `lowlighter/metrics@v3.34` reads
`docs/migration-to-go.md` and switches to `mjun0812/github-metrics@v1.0.0`.
They learn:

- the 21 adopted plugin set (and which upstream plugins are not
  ported);
- the 2 adopted templates (`classic`, `repository`) and which
  upstream templates are not ported (terminal, markdown, community);
- the unsupported feature list (M5 web instance / OAuth / insights,
  M8 social plugins, PDF / Markdown output);
- input-name compatibility (the adopted subset is drop-in
  compatible; unported input names are silently ignored);
- the rollback procedure (revert the `uses:` line to upstream tag).

**Why this priority**: P3 because it improves adoption but does not
block release publication — the release can ship with documentation
landing in the same PR series.

**Independent Test**: Documentation review — a fresh reader who has
never seen this project can answer the following questions correctly
within 10 minutes of opening `docs/migration-to-go.md`:
(a) "which upstream plugins are not available in the go port?";
(b) "how do I roll back to lowlighter/metrics?"; (c) "are my
existing `with:` inputs still recognized?".

**Acceptance Scenarios**:

1. **Given** an upstream user's existing `metrics.yml` workflow
   using only adopted plugins, **When** they change only the `uses:`
   line from `lowlighter/metrics@v3.34` to
   `mjun0812/github-metrics@v1.0.0`, **Then** the workflow runs to
   completion and produces output for the same plugin set.
2. **Given** a workflow that references an unported plugin (e.g.,
   `plugin_anilist: yes`), **When** they migrate to v1.0.0,
   **Then** the unported input is silently ignored and the supported
   plugins still produce output (no hard failure).

---

### Edge Cases

- **Tag pushed without `v` prefix** (e.g., `1.0.0`) → the
  `release.yml` `v*.*.*` filter ignores it; no pipeline runs, no
  partial release is created.
- **arm64 buildx step on x86 runner** → buildx auto-uses QEMU
  emulation; build is slower but completes (verified in dry-run).
- **Image size exceeds 600 MB budget** → release workflow asserts the
  per-platform size after build and fails with an actionable error
  citing the actual size (so the maintainer can decide to bump the
  budget or shrink the image).
- **Cosign keyless OIDC unavailable** (e.g., non-`workflow_dispatch`
  manual invocation outside Actions) → signing step fails with a
  clear "OIDC token unavailable; run from Actions" error rather
  than producing an unsigned release.
- **Dry-run mode** (`workflow_dispatch` + `dry_run=true`) → all
  build steps run, all artifacts produced, but `docker push`, `gh
  release upload`, and `cosign sign` are skipped; artifacts are
  uploaded as workflow artifacts with 7-day retention.
- **Migration-guide rollback path** — if an upstream user finds the
  go-port produces incompatible output, they MUST be able to revert
  to upstream within one PR by reverting the `uses:` line; no
  config-file migration should be required for the adopted-feature
  subset.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The release pipeline MUST publish a multi-arch Docker
  manifest list to `ghcr.io/mjun0812/github-metrics` containing
  both linux/amd64 and linux/arm64, tagged `vX.Y.Z` + `latest` +
  `sha-<short>`.
- **FR-002**: The release pipeline MUST publish 4 cross-compiled
  CLI binaries (linux × amd64/arm64, darwin × amd64/arm64) as
  GitHub Releases assets, named
  `metrics-action_vX.Y.Z_<goos>_<goarch>`.
- **FR-003**: The release pipeline MUST publish a `SHA256SUMS` file
  listing every binary's hash, suitable for `sha256sum -c`
  verification.
- **FR-004**: The release pipeline MUST sign the Docker image
  manifest and each CLI binary via cosign keyless OIDC (sigstore)
  using the GitHub Actions OIDC issuer; signatures MUST be
  verifiable with `cosign verify ...` against the GitHub Actions
  workflow identity.
- **FR-005**: The runtime Dockerfile MUST run the `metrics-action`
  binary as a non-root user (uid >= 1000) with read-only access to
  the binary directory.
- **FR-006**: The runtime Dockerfile image size per platform MUST be
  ≤ 600 MB; the release pipeline MUST measure and assert this
  budget, failing the release on overrun.
- **FR-007**: `action.yml` MUST reference the published GHCR image
  via the `docker://ghcr.io/mjun0812/github-metrics:<version>`
  syntax so that consumers pinning to `@vX.Y.Z` automatically pull
  the matching image (no separate image-tag input).
- **FR-008**: `docs/migration-to-go.md` MUST document:
  (a) the 21 adopted plugins and the unported set;
  (b) the 2 adopted templates and the unported set;
  (c) the unsupported feature list (M5 web / OAuth / insights, M8
  social plugins, PDF / Markdown / community templates);
  (d) input-name compatibility behavior (unported inputs silently
  ignored);
  (e) the rollback procedure (one-line `uses:` revert).
- **FR-009**: The release workflow MUST support a manual
  `workflow_dispatch` dry-run mode that produces all artifacts but
  skips GHCR push, GitHub Release upload, and cosign signing;
  artifacts MUST be uploaded as workflow artifacts with ≥ 7 day
  retention for manual inspection.
- **FR-010**: The constitution III invariant — adopted plugin /
  template count — MUST remain unchanged after M10. The compliance
  test suite (`tests/compliance/`) MUST continue to pass against
  the M10 head commit.

### Key Entities

- **Release artifact set**: { multi-arch Docker manifest list, 4
  CLI binaries, `SHA256SUMS`, cosign signature bundles }. Produced
  per semver tag.
- **Migration guide document**: `docs/migration-to-go.md` — a
  human-readable diff vs upstream `lowlighter/metrics`, written in
  Japanese (per project doc convention), with a rollback section.
- **action.yml metadata**: existing auto-generated file (M6 T-128);
  M10 finalizes the `docker://...` image reference to the
  M10-published GHCR tag.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `docker manifest inspect ghcr.io/mjun0812/github-metrics:vX.Y.Z`
  reports both `linux/amd64` and `linux/arm64` platform entries.
- **SC-002**: Published image per-platform size ≤ 600 MB (verified
  in the release workflow before push).
- **SC-003**: `sha256sum -c SHA256SUMS` against the downloaded
  binaries returns success for all 4 entries.
- **SC-004**: `cosign verify ghcr.io/mjun0812/github-metrics:vX.Y.Z
  --certificate-identity-regexp 'https://github.com/mjun0812/.*'
  --certificate-oidc-issuer https://token.actions.githubusercontent.com`
  returns success.
- **SC-005**: A first-time reader of `docs/migration-to-go.md` can
  identify the unported plugin / template set and the rollback
  procedure within 10 minutes of opening the file.
- **SC-006**: A sample workflow consuming
  `uses: mjun0812/github-metrics@vX.Y.Z` against `--user octocat
  --dryrun --output svg` produces normalized SVG output bytewise
  identical to the same workflow consuming `uses: ./` (local build).
- **SC-007**: The release pipeline's end-to-end runtime from tag
  push to all artifacts published ≤ 25 minutes (per
  10-testing-deployment.md §6 budget).
- **SC-008**: All existing M1-M9 success criteria continue to pass
  on the M10 head commit (CI gate: `make test`, `make lint`,
  `tests/compliance/...`).

## Assumptions

- **Windows binary** cross-compile remains out of scope for v1.0.
  T-130 lists `windows/amd64` as desirable but the release.yml's
  current 4-variant matrix (linux/darwin × amd64/arm64) is treated
  as the v1.0 ceiling. Windows can land as a follow-up patch
  release without breaking the M10 contract.
- **Docker Hub** is not used; GHCR is the canonical registry per
  M6 SC clarifications (`specs/005-m6-action-cli/spec.md` Q1).
- **Cosign keyless OIDC** signing uses the GitHub Actions OIDC
  issuer; no separate cosign key management is required, and
  signatures are publicly verifiable via sigstore Rekor.
- **Image size budget of 600 MB** is taken from
  `docs/design/16-tasks-mvp.md` T-126 acceptance criteria. If the
  chromium + Noto fonts combination cannot fit within 600 MB at
  build time, the plan phase will surface this as an explicit
  trade-off (either prune fonts or raise the budget); the spec
  does not pre-decide.
- **action.yml** has already been auto-generated in M6 (T-128); M10
  only updates the `docker://...` image reference line, not the
  input matrix.
- **Migration guide** ships in Japanese (per project doc convention
  — see `docs/design/15-selection-answer.md`); an English
  translation can ship as a follow-up if upstream community
  requests it.
- **Tag-driven release** is the only release trigger;
  branch-driven nightly publishes are out of scope.
- **M1-M9 baselines** are present on `main` (latest commit
  `e24c849` per `git log`); M10 does not depend on M5 (web
  instance) or M8 (social plugins) which remain intentionally
  unimplemented per `docs/design/15-selection-answer.md`.
