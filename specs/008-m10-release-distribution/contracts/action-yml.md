# Contract: action.yml image reference

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-005

`action.yml` is **auto-generated** from
`internal/tools/gen-action-yml` (M6 T-128); M10 finalizes the
single `image:` line that points at the published GHCR tag.

## 1. The line being changed

The current M6 baseline (per `gen-action-yml`) emits one of two
shapes depending on the project's release status:

```yaml
runs:
  using: docker
  image: 'Dockerfile'                                       # pre-release: build from source
```

OR

```yaml
runs:
  using: docker
  image: 'docker://ghcr.io/mjun0812/github-metrics:v0.6.0'  # M6: pinned to an existing tag
```

M10 updates this to:

```yaml
runs:
  using: docker
  image: 'docker://ghcr.io/mjun0812/github-metrics:v1.0.0'
```

at v1.0.0 release time, and to `vX.Y.Z` on every subsequent
release.

## 2. Why pin by version, not `:latest`

- Consumers writing `uses: mjun0812/github-metrics@v1.0.0` expect
  reproducible behavior. Pinning the image by version means
  v1.0.0 ⇒ image `v1.0.0` permanently. The image hash is fixed.
- Pinning to `:latest` would silently bind v1.0.0 consumers to
  whatever the most recent push was — defeats the entire purpose
  of versioned Actions.
- The release workflow updates this `image:` line in a follow-up
  commit *after* the GHCR push succeeds. The order is:
  1. Push image to `ghcr.io/.../...:vX.Y.Z`
  2. Sign the manifest
  3. Update `action.yml` `image:` line
  4. Re-run `gen-action-yml` to confirm idempotency
  5. Commit + push the action.yml update on `main` (or include in
     the release commit)
  6. Tag the release commit `vX.Y.Z`

This ordering means a user pinning to `@v1.0.0` always resolves
to the exact same `action.yml` line + image bytes.

## 3. Drift detection

The existing lefthook `action-yml-drift` pre-commit hook re-runs
`gen-action-yml` and `git diff --quiet action.yml` to ensure the
checked-in `action.yml` matches what the generator would produce.
M10 keeps this guarantee intact — the M10 release script's
`action.yml` mutation must go through `gen-action-yml`, not via
sed/manual edit.

## 4. Why not separate `image:` input

We considered exposing an `INPUT_IMAGE` input that consumers
could override:

```yaml
- uses: mjun0812/github-metrics@v1.0.0
  with:
    image: ghcr.io/myfork/github-metrics:custom
```

**Rejected** because:

- Violates constitution principle I (input compatibility) — adds
  an input not present in upstream.
- A consumer who wants a custom image can simply check out the
  source and reference `uses: ./` against their own Dockerfile.
- Composite-action override semantics are confusing; pinning by
  `@<tag>` is the standard GitHub Actions discovery pattern and
  should not be circumvented.

## 5. action.yml elsewhere

The `inputs:` section (21 plugins × N keys per plugin, plus the
common `user`, `repo`, `token`, `template`, `output`, `filename`,
`dryrun`, `config_*`, `committer_*`, `committer_token`,
`committer_branch`, etc.) is **untouched** by M10. The auto-generator
owns it; constitution principle I locks the input shape; M9
compliance tests guard against drift.

## 6. Verification

Post-release, the `scripts/release-verify.sh` helper executes:

```bash
grep -E "^\s+image:\s+'docker://ghcr.io/mjun0812/github-metrics:${TAG}'" action.yml \
  || { echo "action.yml image: does not match tag ${TAG}"; exit 1; }
```

This is also called from the new docker-smoke test as a
post-release-only optional path.
