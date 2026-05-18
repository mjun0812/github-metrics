# Feature Specification: Release tag automation (auto-create Release + major floating tags)

**Feature Branch**: `009-release-tag-automation`

**Created**: 2026-05-18

**Status**: Draft

**Input**: User description: "セマンティックバージョニングのtagを打ったら自動でreleasesが作成されるようにしてください。また、GHAのusesで使えるようにv1, v2などのタグが、最新のセマンティックバージョンになるようにもしてください。"

## Overview

The v1.0.0 release uncovered two gaps in the M10 release pipeline:

1. **Manual `gh release create` step required.** The `release-binary` job
   runs `gh release upload "${TAG}" * --clobber`, which assumes the
   GitHub Release already exists. Pushing a fresh `vX.Y.Z` tag triggers
   the workflow but no GitHub Release entry is created automatically,
   so the upload step fails with `release not found`. Recovery requires
   the maintainer to manually run `gh release create v1.0.0 --generate-notes`
   and re-run the failed job.

2. **No major-version floating tag.** Consumers writing
   `uses: mjun0812/github-metrics@v1` (the GitHub-Actions community
   convention for pinning to "latest 1.x.y") get an "unknown ref" error
   because only the immutable `v1.0.0` tag exists. Comparable Actions
   (`actions/checkout`, `actions/setup-go`, `docker/build-push-action`)
   all maintain floating major tags that move with each compatible
   release.

This feature closes both gaps so the release pipeline runs end-to-end
on a single `git push origin vX.Y.Z` with zero manual intervention, and
consumers can pin to a stable major-version ref.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer cuts a release in one push (Priority: P1)

A maintainer pushes a semver tag (e.g., `v1.0.1`). The release pipeline
runs to completion: the multi-arch image lands on GHCR, the 4 cross-
compiled binaries + `SHA256SUMS` + cosign signature bundles are
uploaded to a newly created GitHub Release, and release notes are
auto-generated from the merged PRs since the previous tag. **No manual
`gh release create` step is needed.**

**Why this priority**: P1 because the absence of this automation broke
the v1.0.0 release (a workflow re-run + manual release creation was
needed). Every future release will hit the same failure mode without
this fix; the fix is small and high-leverage.

**Independent Test**: From a fork or branch, push a test tag
`v0.99.0-rc1`. Without touching the GitHub UI or running `gh release
create` locally, observe that the release workflow run completes green
and a public Release page exists at
`https://github.com/<fork>/github-metrics/releases/tag/v0.99.0-rc1`
with all 20 assets (4 binaries × {raw, .sig, .cert, .cosign.bundle} +
SHA256SUMS × {raw, .sig, .cert, .cosign.bundle}) attached.

**Acceptance Scenarios**:

1. **Given** the `vX.Y.Z` tag has never been pushed, **When** the
   maintainer pushes the tag, **Then** the release workflow creates
   the GitHub Release entry (with auto-generated notes) and uploads
   all 20 release-asset files without manual intervention.
2. **Given** the maintainer manually re-runs the release workflow
   against an existing tag (e.g. after fixing a CI flake), **When**
   the workflow runs, **Then** the existing Release is detected and
   asset uploads use `--clobber` to overwrite previous attempts
   without erroring.
3. **Given** a release is published successfully, **When** the
   maintainer or consumer visits the Releases page, **Then** the
   release notes list the merged PRs since the previous release tag
   in a readable format.

---

### User Story 2 - Consumer pins to a major floating tag (Priority: P2)

An end user writes their workflow with `uses: mjun0812/github-metrics@v1`
(GitHub Actions community pinning convention). On every workflow run
they receive the latest stable v1.x.y release without editing the
`uses:` line each time a patch ships. When v2.0.0 ships, they
explicitly bump to `@v2` to opt into the new major.

**Why this priority**: P2 because consumers can pin to the exact
`@v1.0.0` today and still receive releases; the floating major tag is
an ergonomic improvement that lets consumers get patch / minor updates
automatically. It is the standard pattern in OSS GitHub Actions.

**Independent Test**: After publishing `v1.0.1`, run
`git ls-remote origin refs/tags/v1` and assert the output references
the same commit SHA as `git ls-remote origin refs/tags/v1.0.1`. A
sample workflow using `uses: <repo>@v1` resolves to that commit at
runtime.

**Acceptance Scenarios**:

1. **Given** `v1.0.0` has just been published, **When** the release
   workflow finishes, **Then** `git ls-remote origin refs/tags/v1`
   resolves to the same commit as `refs/tags/v1.0.0`.
2. **Given** `v1.0.0` exists and `v1` points at it, **When** the
   maintainer publishes `v1.0.1`, **Then** the `v1` tag is moved
   (force-updated) to the `v1.0.1` commit so consumers using `@v1`
   automatically pick it up.
3. **Given** the maintainer publishes a pre-release like
   `v1.0.0-rc1`, **When** the release workflow finishes, **Then**
   the `v1` tag is **not** updated or created — pre-release versions
   must not advance the floating tag.
4. **Given** the maintainer eventually publishes `v2.0.0`, **When**
   the release workflow finishes, **Then** the `v2` tag is created
   pointing at the v2.0.0 commit. The `v1` tag is NOT touched (it
   continues to point at the last v1.x.y release for consumers who
   have not yet migrated to v2).

---

### Edge Cases

- **Tag pushed twice / workflow re-run** → existing Release is
  detected; uploads use `--clobber`; cosign re-signs (Rekor records a
  new entry, prior entries remain auditable); the `v<major>` floating
  tag is force-updated if the re-pushed tag is the newest in its
  major.
- **Pre-release / build metadata in tag** (e.g. `v1.0.0-rc1`,
  `v1.0.0+sha.abc123`) → release publishes normally; `v1` floating
  tag NOT updated (per acceptance scenario US2-3).
- **`v1` tag already exists pointing elsewhere** (e.g. from a prior
  manual experiment) → workflow force-updates to the new commit.
  No prompt or error.
- **Network failure during release create** → the workflow step
  fails loudly with the GitHub API error; cosign artifacts on the
  runner are preserved as workflow artifacts (existing dry-run
  upload path) so the maintainer can re-run.
- **Pushing an older version after a newer one already exists** (e.g.
  publishing `v1.0.5` after `v1.0.6`) → the workflow creates the
  Release for v1.0.5 but **must not** move the `v1` floating tag
  backward; if v1.0.6 already exists, `v1` continues to point at
  v1.0.6.
- **Workflow lacks `contents: write` permission to force-push tags** →
  fail loudly with a clear error message (and document the required
  permission in the release-workflow contract).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The release workflow MUST create the GitHub Release
  entry if it does not already exist before attempting to upload
  release assets. Re-runs against an existing release MUST be
  idempotent (no error on the create step).
- **FR-002**: When the GitHub Release is created, the workflow MUST
  populate the release notes via GitHub's auto-generated changelog
  (PRs merged since the previous release tag). Maintainers MAY
  override the notes via a workflow input or release-time edit;
  defaulting to auto-generation MUST work without configuration.
- **FR-003**: After publishing a stable `vX.Y.Z` tag (no pre-release
  suffix), the workflow MUST update the `vX` floating tag to point
  at the same commit. If the `vX` tag does not exist, the workflow
  MUST create it.
- **FR-004**: For pre-release tags (`vX.Y.Z-...`), the workflow MUST
  NOT create or move any `vX` floating tag. Pre-release publishes
  finish normally for everything else.
- **FR-005**: If the new stable `vX.Y.Z` is **older** than the
  current `vX` target (i.e., a back-port release of `v1.0.5` after
  `v1.0.6` already shipped), the workflow MUST NOT regress the
  `vX` tag. The publishing of the older Release MUST still succeed;
  only the floating-tag advance is skipped.
- **FR-006**: Failure of the floating-tag update MUST be reported as
  a non-blocking warning (the release publish does not roll back).
  The workflow exits non-zero only if a load-bearing step (release
  create, asset upload, cosign sign, GHCR push, docker-smoke) fails.
- **FR-007**: The workflow MUST update existing M10 documentation
  (`specs/008-m10-release-distribution/quickstart.md` §2, `README.md`
  Quick start, `docs/migration-to-go.md` examples) so the documented
  procedure no longer instructs maintainers to run `gh release create`
  manually before the tag push.
- **FR-008**: The `v1.0.0` release already published (commit
  `b3bd975`) MUST receive a one-time backfill: a `v1` floating tag is
  created pointing at `v1.0.0`'s commit so existing consumers writing
  `uses: mjun0812/github-metrics@v1` can adopt the convention
  immediately.

### Key Entities

- **Release artifact set** (unchanged from M10): the multi-arch
  manifest, 4 binaries, `SHA256SUMS`, plus cosign sign-blob bundles
  per artifact.
- **GitHub Release page** (now auto-created): the publish target on
  `https://github.com/<owner>/<repo>/releases/tag/vX.Y.Z` with
  auto-generated changelog and 20 attached assets.
- **Major floating tag**: a mutable ref of the form `vX` (e.g. `v1`,
  `v2`) that resolves to the most recent stable `vX.Y.Z` commit and
  is updated automatically on each new stable release in the same
  major-version line.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A single `git push origin vX.Y.Z` triggers a release
  pipeline that ends with the Release page populated and all 20 assets
  attached, with zero manual operator intervention (no `gh release
  create`, no asset upload, no UI edit).
- **SC-002**: Within the same release workflow run (no separate human
  trigger), `git ls-remote origin refs/tags/v<major>` resolves to the
  same SHA as `refs/tags/vX.Y.Z` for stable releases.
- **SC-003**: For a pre-release tag (`vX.Y.Z-rc1` etc.), no `v<major>`
  ref is created or modified — confirmed by `git ls-remote origin
  refs/tags/v<major>` returning the previous SHA (or "ref not found"
  if no stable release in the major exists yet).
- **SC-004**: Re-running the release workflow against an existing tag
  via `gh workflow run release.yml --ref vX.Y.Z` completes green; the
  Release entry remains and assets are re-uploaded with `--clobber`.
- **SC-005**: The released action.yml shows the same `image:` line as
  M10 (`docker://ghcr.io/.../...:vX.Y.Z`); FR-007 documentation
  updates do not change the action.yml emission semantics.
- **SC-006**: An end user updating their workflow from `uses:
  mjun0812/github-metrics@v1.0.0` to `uses: mjun0812/github-metrics@v1`
  produces the same workflow behavior on the first run after the
  change; subsequent runs automatically resolve to whatever `v1`
  currently points at (so a `v1.0.1` publish reaches them without a
  consumer-side edit).
- **SC-007**: The release pipeline runtime budget (≤ 25 min per
  M10 SC-007) is preserved; the new release-create + floating-tag
  steps together add ≤ 30 seconds to the wall-clock.

## Assumptions

- Floating-tag scheme uses **major version only** (`v1`, `v2`). Minor
  floating tags (`v1.0`, `v1.1`) are out of scope for v1.x — patches
  are infrequent enough that consumers wanting tighter pinning can
  use the exact `v1.0.5` form. Revisit if the cadence changes.
- Pre-release detection follows SemVer 2.0.0: any `vX.Y.Z-...` is a
  pre-release; `vX.Y.Z` and `vX.Y.Z+build` are stable.
- Release notes auto-generation uses GitHub's `--generate-notes`
  flag (which categorizes PRs by label) — maintainers may also pass
  custom notes via workflow input, but the default zero-config path
  produces a usable changelog without configuration.
- The release workflow inherits the existing `contents: write`
  permission scope already in place for asset upload; force-updating
  tags falls under the same scope, so no additional permission grant
  is needed.
- FR-008's backfill for `v1.0.0` runs as a one-time maintainer
  action (separate workflow invocation or local `git push`); it does
  not require the release pipeline to be modified to re-publish v1.0.0.
- Constitution III invariant (21 adopted plugins + 2 templates) is
  unaffected — this is a release-pipeline polish change with no
  plugin / template / output-format scope impact.
