# Research: Release tag automation

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

Three decisions feed Phase 1 design. The spec section referenced in
"Spec linkage" below frames each decision; the chosen approach becomes
the Phase 1 contract input.

---

## R-001: Release creation mechanism — `gh release create` vs `softprops/action-gh-release@v2`

**Decision**: Use `gh release create "${TAG}" <artifacts...> --generate-notes`
as a single command inside the existing `release-binary` job. If the
release already exists (re-run path), call `gh release upload "${TAG}"
<artifacts...> --clobber` instead. The two cases are detected by
`gh release view "${TAG}"` exit code.

**Rationale**:

- **Zero new external dependency** — `gh` is already on every
  `ubuntu-latest` runner with the auth context the workflow needs.
  `softprops/action-gh-release@v2` would add another third-party
  Action to pin (security review item from M10 already flagged
  third-party-action churn — see PR #352 Should Fix #14).
- **Auto-generated notes** — `--generate-notes` produces a
  PR-categorized changelog from labels since the previous release tag
  with no configuration. Equivalent functionality in
  `softprops/action-gh-release` requires the `generate_release_notes:
  true` input.
- **Single source of truth for permissions** — `gh` honors the
  workflow's `GITHUB_TOKEN` automatically; no separate secret to wire.
- **Idempotency on re-run** is straightforward: the existence-check
  `gh release view "${TAG}" >/dev/null 2>&1 && upload-only || create-with-upload`
  is a single shell line.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| `softprops/action-gh-release@v2` | Adds another third-party action to SHA-pin per the open M10 security review item; no functional capability we lack with `gh`. |
| Two-pass `gh release create` + `gh release upload` | Works but doubles the API round-trips and complicates the dry-run gate (two separate `if: dry_run != 'true'` conditions). Single-pass create-with-files is simpler. |
| `goreleaser` | Heavy dependency for what is fundamentally a 5-line shell change; the project does not need goreleaser's full feature set (custom changelog templating, snap/brew, etc.) for v1.x. |

**Spec linkage**: FR-001 (auto-create), FR-002 (notes), FR-006
(non-blocking on partial failures). Idempotency check addresses
SC-004.

**Plan-phase risk**: low. `gh release create … <file>...` is a 7-year-stable
CLI surface. The only subtle behavior is that `gh release view` returns
exit code 1 (not 0) when the release does not exist, which is the test
the conditional relies on — verified locally during v1.0.0 manual
recovery.

---

## R-002: Floating-tag advance mechanism — workflow step vs separate job

**Decision**: Add a new job `update-floating-tag` that runs after both
`release-docker` and `release-binary` succeed (`needs:
[release-docker, release-binary]`). The job:

1. Parses `${{ github.ref_name }}` (the just-pushed tag) into
   `<major>.<minor>.<patch>[-prerelease]`.
2. Skips silently if a prerelease suffix is present (FR-004).
3. Compares the new commit's semver against the current `vMAJOR` ref
   (if it exists). Skips if the new tag is older than the current
   `vMAJOR` target (FR-005, back-port case).
4. Force-updates `vMAJOR` via `git tag -f vMAJOR <sha> && git push
   --force origin refs/tags/vMAJOR`.

**Rationale**:

- **Separation of concerns** — release-docker / release-binary publish
  artifacts; update-floating-tag is a post-publish operation. Mixing
  the two muddles the job's responsibility and the `needs:` graph.
- **`needs:` gate** ensures the floating tag advances only if both
  publish paths succeeded. If release-binary fails mid-run (as v1.0.0
  did), the floating tag is not moved to a half-published release.
- **Workflow YAML readability** — the new job is ~40 lines and
  self-contained; readers can review the SemVer parsing + advance
  logic in one place.

**Why force-push is acceptable** — moving the `vX` ref is the entire
design intent of major floating tags. `--force-with-lease` would
mis-fire on legitimate advances (the previous `vX` SHA changes by
design). The CLAUDE.md / constitution gate on force-push applies to
named branches (e.g., `main`), not to floating tags whose advance is
the documented behavior.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Run as a step inside `release-binary` | Couples binary publish to tag advance; if release-binary partially succeeds (binaries uploaded, cosign step fails), should the floating tag advance? Cleaner as a separate job gated on full success. |
| Run as a `repository_dispatch` triggered by `release.published` event | Loses access to the GITHUB_REF tag context; would need to look up the tag from API. Extra plumbing for no benefit. |
| Pre-compute the major version in the existing extract-tag step + share via `outputs` | Adds an output to two jobs (release-docker + release-binary already share that logic); duplication M10 Should Fix #7 already flagged. A separate job re-computing from `github.ref_name` is simpler. |

**Spec linkage**: FR-003 (advance on stable), FR-004 (pre-release
skip), FR-005 (back-port no-regress), FR-006 (non-blocking — job-level
failure does not affect already-published artifacts).

**Plan-phase risk**: medium. The "is the new tag newer than current
vMAJOR target?" comparison must use SemVer-aware ordering, not
lexicographic. The bash impl uses `sort -V` (version sort, GNU
coreutils standard on `ubuntu-latest`) — verified to handle
`v1.0.10` > `v1.0.9` correctly. Pre-release suffixes are stripped
before the comparison so `v1.0.0-rc1` does not falsely "advance"
nothing.

---

## R-003: One-time `v1` backfill ordering — workflow extension vs manual

**Decision**: Backfill `v1` → `v1.0.0` commit as a one-time **manual
operation** documented in `quickstart.md` §2a:

```sh
git tag -f v1 v1.0.0
git push origin refs/tags/v1
```

The release-workflow change (R-002) creates the `vMAJOR` advance
machinery for future releases (`v1.0.1`, `v1.0.2`, …). It does NOT
attempt to backfill historical releases.

**Rationale**:

- **Surgical scope** — backfilling historical releases inside the
  release workflow requires conditional logic ("is this the first
  release of the major? has v1 ever been published?") that bloats
  the floating-tag advance step. A manual one-liner is clearer.
- **Trust path** — the v1.0.0 backfill is a maintainer-authorized
  action; documenting it in the maintainer-facing quickstart is the
  right level of friction.
- **Repeatability** — the same one-liner backfills any future
  major-version gap (e.g., if the workflow had been disabled when
  v3.0.0 shipped, `git tag -f v3 v3.0.0 && git push origin
  refs/tags/v3` recovers).

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Auto-backfill inside the release workflow on first run | Adds conditional logic for a one-time event; per FR-008 the backfill must happen but workflow code is the wrong place for it. |
| Separate `workflow_dispatch` workflow `backfill-floating-tag.yml` | Over-engineered for a one-line `git tag -f` + `git push`. |
| Skip backfill — start with v1.0.1 | Breaks SC-006 / user-story 2 acceptance scenario 1: consumers using `@v1` before v1.0.1 ships would get "unknown ref" errors. Backfill is necessary for the feature to feel complete on day one. |

**Spec linkage**: FR-008 (one-time v1 backfill). The backfill is a
maintainer step, not a workflow step.

**Plan-phase risk**: trivial. Two-line manual operation, no
automation.

---

## Summary

All 3 decisions resolved with informed choices. No `NEEDS
CLARIFICATION` carries into Phase 1.

- **R-001**: `gh release create … --generate-notes` for auto-create;
  re-run path uses `gh release upload --clobber`.
- **R-002**: New `update-floating-tag` job; `needs: [release-docker,
  release-binary]`; SemVer parsing via `sort -V`; force-push the
  `vMAJOR` ref.
- **R-003**: v1.0.0 backfill is documented in quickstart.md, not
  automated.

All chosen approaches preserve constitution principles (I, III, V
unaffected; II / IV strengthened by clearer post-release-state
guarantees).
