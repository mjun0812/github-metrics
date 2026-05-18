# Contract: release.yml delta vs M10 baseline

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: M10 [`contracts/release-workflow.md`](../../008-m10-release-distribution/contracts/release-workflow.md) (baseline)

This contract documents the diff between the M10 release.yml and the
post-009 release.yml. Read alongside the M10 baseline contract for
the unchanged parts.

## 1. `release-binary` job — Upload step refactor

### Before (M10)

```yaml
- name: Upload to GitHub Release (production)
  if: ${{ github.event.inputs.dry_run != 'true' }}
  working-directory: dist
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    TAG: ${{ steps.tag.outputs.tag }}
  run: |
    set -euo pipefail
    gh release upload "${TAG}" * --clobber
```

**Failure mode**: `gh release upload "${TAG}" *` returns
`release not found` if the GitHub Release entry does not yet exist
for `${TAG}`. A fresh `git push origin vX.Y.Z` produces exactly that
state — the tag exists on the remote but no Release page does.

### After (009)

```yaml
- name: Upload to GitHub Release (production)
  if: ${{ github.event.inputs.dry_run != 'true' }}
  working-directory: dist
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    TAG: ${{ steps.tag.outputs.tag }}
  run: |
    set -euo pipefail
    if gh release view "${TAG}" >/dev/null 2>&1; then
      echo "Release ${TAG} already exists — uploading via clobber"
      gh release upload "${TAG}" * --clobber
    else
      echo "Release ${TAG} does not exist yet — creating with assets in one shot"
      gh release create "${TAG}" * --generate-notes --title "${TAG}"
    fi
```

**Behavior**:

- **First-run (typical)**: `gh release create` creates the Release
  page with auto-generated notes AND uploads all 20 assets in one
  command. No separate `gh release create` precondition needed.
- **Re-run (recovery)**: `gh release view` returns 0; the script
  falls through to `gh release upload --clobber` which overwrites
  previous asset uploads without erroring. Release notes are NOT
  regenerated (preserves any manual edits the maintainer made).
- **Dry-run**: the surrounding `if: dry_run != 'true'` gates the
  entire step; dry-run uses the existing `upload-artifact@v4` path.

**Spec linkage**: FR-001, FR-002, FR-006; SC-001, SC-004.

## 2. NEW `update-floating-tag` job

```yaml
update-floating-tag:
  name: Advance vMAJOR floating tag (stable releases only)
  needs: [release-docker, release-binary]
  runs-on: ubuntu-latest
  if: |
    github.event_name == 'push' &&
    startsWith(github.ref, 'refs/tags/v') &&
    github.event.inputs.dry_run != 'true'
  permissions:
    contents: write
  steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0

    - name: Compute major + advance decision
      id: decide
      run: |
        set -euo pipefail
        TAG="${GITHUB_REF#refs/tags/}"
        # Skip pre-releases (SemVer 2.0.0 form vX.Y.Z-...)
        if [[ "${TAG}" =~ -[0-9A-Za-z.-]+$ ]]; then
          echo "skip=prerelease" >> "$GITHUB_OUTPUT"
          echo "Pre-release ${TAG} — vMAJOR floating tag NOT updated"
          exit 0
        fi
        # Extract MAJOR
        if [[ ! "${TAG}" =~ ^v([0-9]+)\.[0-9]+\.[0-9]+$ ]]; then
          echo "Tag ${TAG} does not match v<MAJOR>.<MINOR>.<PATCH>; skipping"
          echo "skip=non-semver" >> "$GITHUB_OUTPUT"
          exit 0
        fi
        MAJOR="${BASH_REMATCH[1]}"
        FLOATING="v${MAJOR}"
        # If vMAJOR exists, compare semver
        if git ls-remote --exit-code origin "refs/tags/${FLOATING}" >/dev/null 2>&1; then
          CURRENT_SHA="$(git ls-remote origin "refs/tags/${FLOATING}" | awk '{print $1}')"
          # Find tag at CURRENT_SHA matching vMAJOR.*
          CURRENT_TAG="$(git tag --points-at "${CURRENT_SHA}" "v${MAJOR}.*.*" | head -1 || true)"
          if [[ -n "${CURRENT_TAG}" ]]; then
            # SemVer-aware comparison via sort -V
            NEWER="$(printf '%s\n%s\n' "${TAG}" "${CURRENT_TAG}" | sort -V | tail -1)"
            if [[ "${NEWER}" != "${TAG}" ]]; then
              echo "Back-port: ${TAG} is older than ${CURRENT_TAG} — vMAJOR NOT updated"
              echo "skip=backport" >> "$GITHUB_OUTPUT"
              exit 0
            fi
          fi
        fi
        echo "floating=${FLOATING}" >> "$GITHUB_OUTPUT"

    - name: Force-update floating tag
      if: steps.decide.outputs.floating != ''
      env:
        FLOATING: ${{ steps.decide.outputs.floating }}
        TARGET_SHA: ${{ github.sha }}
      run: |
        set -euo pipefail
        git tag -f "${FLOATING}" "${TARGET_SHA}"
        git push origin --force "refs/tags/${FLOATING}"
        echo "Advanced ${FLOATING} → ${TARGET_SHA} (= ${GITHUB_REF#refs/tags/})"
```

**Gating rules** (per FR-003 / FR-004 / FR-005):

| Condition | `vMAJOR` advanced? | Why |
|-----------|--------------------|-----|
| Stable tag `vX.Y.Z`, newer than current `vX` target | yes | FR-003 |
| Stable tag `vX.Y.Z`, no current `vX` ref | yes (creates) | FR-003 |
| Pre-release tag `vX.Y.Z-...` | no | FR-004 |
| Stable tag older than current `vX` target (back-port) | no | FR-005 |
| Tag does not match SemVer 2.0.0 shape | no (skipped silently) | safety |
| `release-docker` or `release-binary` failed | job does not run | `needs:` gate |
| Workflow_dispatch with `dry_run=true` | job does not run | `if:` gate |

**Spec linkage**: FR-003, FR-004, FR-005, FR-006; SC-002, SC-003,
SC-006, SC-007.

## 3. Permissions changes

| Permission | M10 | 009 | Notes |
|------------|-----|-----|-------|
| `contents: write` | yes | yes | Required by both Release upload and floating-tag force-push. No expansion. |
| `packages: write` | yes | yes | GHCR push unchanged. |
| `id-token: write` | yes (workflow) | yes (workflow) | Cosign keyless OIDC unchanged. (M10 Should Fix #1 — separately tracked — proposes per-job scoping; out of scope for 009.) |

**No new permission scope is required.** The `update-floating-tag`
job inherits the workflow-level `contents: write` already in place.

## 4. Failure semantics

Add to the M10 failure table:

| Failure | Resolution |
|---------|-----------|
| `gh release create` fails (network / quota / permission) | Job exits non-zero; cosign artifacts on the runner are uploaded as workflow artifacts via the existing dry-run path. Maintainer re-runs the failed job. |
| `gh release view` returns non-zero AND `gh release create` also fails | Same as above — surface the create error. The `view` check is purely for branching, not for triggering different failure modes. |
| Floating-tag advance step fails (rare: API quota, network) | Job-level failure is logged but does NOT roll back the published release. FR-006 requires non-blocking; maintainer can re-trigger via manual `git tag -f vMAJOR <sha> && git push origin refs/tags/vMAJOR`. |
| Tag is not SemVer-shaped (e.g. `release-1`) | `update-floating-tag` job exits 0 with "tag does not match v<MAJOR>.<MINOR>.<PATCH>" log line. No-op. |

## 5. Idempotency matrix (extending M10 §7)

| Action | Idempotent? | Notes |
|--------|-------------|-------|
| `gh release create` re-run against existing release | no (errors) — but we detect first via `gh release view` and use upload-clobber instead | Detection branch handles the re-run path. |
| `gh release upload --clobber` | yes | Overwrites existing assets cleanly. |
| `git push --force refs/tags/vMAJOR` | yes (intentionally) | Floating-tag advance is the documented behavior; force-push is not a destructive operation in this context. |
| Cosign Rekor entries | no (new entry per signing) | Same as M10 — historical entries remain auditable. |
