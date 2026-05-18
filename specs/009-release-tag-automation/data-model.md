# Data Model: Release tag automation

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

This feature has no runtime data model — the change surface is the
release pipeline and post-publish ref state on the origin remote.
Two "entities" are captured below to ground the contract docs.

## E-001: GitHub Release page (now auto-created)

- **Trigger**: a push of a `vX.Y.Z` tag to `origin`, OR a
  `workflow_dispatch` invocation with `dry_run=false`.
- **Created by**: `release.yml` `release-binary` job, via
  `gh release create "${TAG}" <artifacts...> --generate-notes`.
- **Existence check (idempotency)**: re-runs against an existing
  release route through `gh release view "${TAG}" >/dev/null 2>&1`
  and fall through to `gh release upload "${TAG}" <artifacts...>
  --clobber` instead of `create`.
- **Contents** (per release):
  - **Auto-generated release notes** — categorized list of merged
    PRs since the previous release tag (uses repo's PR labels per
    `--generate-notes` semantics).
  - **20 attached assets** — 4 binaries (`linux/darwin × amd64/arm64`)
    + `SHA256SUMS`, each plus `.sig` / `.cert` / `.cosign.bundle`
    cosign sign-blob artifacts (= 4 × 4 + 4 = 20).
- **Visibility**: public if the repository is public (GitHub default).
- **Idempotency**: re-running against an existing release does not
  duplicate; `--clobber` overwrites each asset. The release notes are
  NOT regenerated on re-run (preserves any manual edits the
  maintainer made post-publish).
- **Dry-run mode**: skipped entirely; existing `release.yml` dry-run
  path uploads `dist/` to workflow artifacts with 7-day retention
  instead.

**Spec linkage**: FR-001, FR-002, FR-006; SC-001, SC-004.

## E-002: Major floating tag

- **Form**: `vMAJOR` (e.g. `v1`, `v2`). No `vMAJOR.MINOR` or
  `vMAJOR.MINOR.PATCH-major-only` variants — major-only by design
  per spec assumption.
- **Created / advanced by**: a new `update-floating-tag` job in
  `release.yml`, gated on `needs: [release-docker, release-binary]`
  success and on the just-pushed tag being a **stable** `vX.Y.Z` (no
  prerelease suffix per SemVer 2.0.0).
- **Skip conditions** (FR-004 / FR-005):
  - Pre-release tag (`vX.Y.Z-...`) → skip silently.
  - The just-pushed `vX.Y.Z` is **older** than the current `vMAJOR`
    target (back-port scenario) → skip silently.
  - The `update-floating-tag` job's `needs:` dependency failed → job
    does not run.
- **Update mechanism**: `git tag -f vMAJOR <commit-sha> && git push
  origin --force refs/tags/vMAJOR`. The force-push is essential —
  it is the documented behavior of moving a major-version pointer.
  Constitution / CLAUDE.md force-push guidance applies to named
  branches (e.g. `main`), not floating tags whose advance is the
  contract.
- **Permissions**: requires `contents: write` (already granted to
  the release workflow per M10). No additional permission scope
  changes.

**Spec linkage**: FR-003, FR-004, FR-005, FR-006, FR-008; SC-002,
SC-003, SC-006.

## E-003: One-time v1 backfill (maintainer task)

- **Not part of the workflow** — a manual operation performed once
  by the maintainer to align the already-published v1.0.0 with the
  new floating-tag convention.
- **Commands** (documented in `specs/008-m10-release-distribution/
  quickstart.md` §2a):

  ```sh
  git tag -f v1 v1.0.0
  git push origin refs/tags/v1
  ```

- **Idempotency**: re-running against an existing v1 ref is harmless
  — it force-updates to v1.0.0's commit, which is also v1.0.0's
  commit (no-op effectively).
- **Verification**: `git ls-remote origin refs/tags/v1` and
  `git ls-remote origin refs/tags/v1.0.0` resolve to the same SHA.

**Spec linkage**: FR-008.
