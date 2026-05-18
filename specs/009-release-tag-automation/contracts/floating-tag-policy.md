# Contract: Floating-tag policy

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002

Codifies the rules the `update-floating-tag` job enforces. This is
the single source of truth for what triggers a `vMAJOR` advance.

## 1. Tag shape recognition

A tag is **SemVer 2.0.0 conformant** in this project if and only if
it matches `^v([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`.

| Tag                | Recognized | Pre-release? | Notes |
|--------------------|------------|--------------|-------|
| `v1.0.0`           | yes        | no           | Stable |
| `v1.0.0-rc1`       | yes        | yes          | Pre-release |
| `v1.0.0-rc.1`      | yes        | yes          | Pre-release |
| `v1.0.0+build.1`   | yes        | no           | Build metadata is informational; SemVer treats `vX.Y.Z+...` as stable |
| `v1.0.0-rc1+build` | yes        | yes          | Pre-release + build metadata |
| `release-1`        | no         | n/a          | Not recognized; floating-tag job no-ops |
| `latest`           | no         | n/a          | Not a SemVer tag — handled separately by docker tags only |

## 2. Floating-tag advance decision (per push)

```text
                    ┌──────────────────────────────────────┐
                    │ Tag pushed: vX.Y.Z[-prerelease]      │
                    └──────────────────────────────────────┘
                                       │
                ┌──────────────────────┴────────────────────┐
                │                                            │
        not SemVer shape                            SemVer shape
                │                                            │
                ▼                                            ▼
    ┌─────────────────────┐                  ┌────────────────────────┐
    │  no-op; skip log    │                  │ pre-release suffix?    │
    └─────────────────────┘                  └────────────────────────┘
                                                    │           │
                                                    yes          no
                                                    │           │
                                                    ▼           ▼
                                          ┌──────────┐ ┌──────────────────────────┐
                                          │  skip;   │ │ vMAJOR ref already exist?│
                                          │  no-op   │ └──────────────────────────┘
                                          └──────────┘    │              │
                                                          no             yes
                                                          │              │
                                                          ▼              ▼
                                            ┌──────────────────┐ ┌──────────────────────────────────┐
                                            │  create vMAJOR   │ │ vX.Y.Z newer than current vMAJOR?│
                                            │  pointing at SHA │ └──────────────────────────────────┘
                                            └──────────────────┘    │              │
                                                                  yes            no
                                                                    │              │
                                                                    ▼              ▼
                                                          ┌──────────────────┐ ┌───────────────┐
                                                          │ force-update     │ │ back-port —   │
                                                          │ vMAJOR → SHA     │ │ skip; no-op   │
                                                          └──────────────────┘ └───────────────┘
```

## 3. Comparison ordering — "newer than current vMAJOR"

The "is the new tag newer than the current `vMAJOR` target?" decision
uses `sort -V` (GNU coreutils version sort, present on every
`ubuntu-latest` runner since 2019):

```bash
NEWER="$(printf '%s\n%s\n' "${NEW_TAG}" "${CURRENT_TAG}" | sort -V | tail -1)"
if [[ "${NEWER}" == "${NEW_TAG}" ]]; then
  # NEW_TAG is strictly newer; advance.
fi
```

### Edge-case verifications (executed in research.md)

| Inputs | `sort -V | tail -1` result | Decision |
|--------|----------------------------|----------|
| `v1.0.9`, `v1.0.10` | `v1.0.10` | `v1.0.10` advances `v1` past `v1.0.9` (correct numeric ordering) |
| `v1.0.0`, `v1.0.0-rc1` | `v1.0.0` | A pre-release input would not reach this comparison (filtered earlier), but if it did, the stable `v1.0.0` wins per SemVer §11 |
| `v1.0.5`, `v1.0.10` then push `v1.0.6` (back-port) | comparison: `v1.0.6` vs `v1.0.10` → `v1.0.10` | Back-port detected; `v1` stays at `v1.0.10` |
| `v1.10.0`, `v1.9.0` | `v1.10.0` | Minor-version numeric ordering correct |

## 4. Major-only by design (FR-005 spec assumption)

The floating-tag set is **only** `v1`, `v2`, ..., `vN`. No
`v1.0`, `v1.1`, `v1.0.x` floating tags.

### Why not minor

- Minor floating tags imply a release cadence where consumers want
  patch-only updates within a minor. The v1.x cadence is expected to
  be infrequent enough that exact-pin (`v1.0.5`) is sufficient for
  consumers who need that granularity.
- Adding minor floating tags doubles the advance logic (now must
  also track `vMAJOR.MINOR`); not worth the complexity for v1.x.
- Easy to revisit in a future minor release if cadence pressure
  demands it; the major-only design is forward-compatible.

### Why not patch

- Patch floating tags would mean `v1.0` always points at `v1.0.<latest>`
  — but this contradicts SemVer (consumers expecting bug-fix-only
  patches under `v1.0` would be surprised by a `v1.0.5 → v1.0.6` move
  that introduces unforeseen behavior).
- No upstream Actions project (`actions/checkout`, `docker/build-push-action`,
  `golangci/golangci-lint-action`) ships patch-floating tags. Sticking
  with the community norm.

## 5. v1 backfill (one-time maintainer task — FR-008)

For the already-published v1.0.0 release (commit `b3bd975`), the
floating tag must be created manually:

```sh
git tag -f v1 v1.0.0
git push origin refs/tags/v1
```

This is **outside the workflow** because:

- The workflow change (R-002) is fired by *future* tag pushes; it
  cannot retroactively create a `v1` for v1.0.0 without explicit
  invocation.
- Adding conditional "is this the first release of the major?" logic
  to the workflow bloats it for a one-time event.
- Manual one-liner is idempotent (`-f` on `git tag` is safe even if
  the local `v1` already exists; the remote force-push is similarly
  idempotent).

After backfill, `git ls-remote origin refs/tags/v1` returns the same
SHA as `git ls-remote origin refs/tags/v1.0.0` (commit `b3bd975`).
