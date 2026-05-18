# Quickstart: cutting a release post-009

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md)

This is the maintainer flow for publishing `vX.Y.Z` after 009 lands.
The procedure is **simpler than M10's quickstart**: there is no
longer a manual `gh release create` step, and the new floating-tag
job handles `vMAJOR` advancement automatically.

## 0. Prerequisites

(unchanged from M10 — clean main, `make test`, `make lint`,
`tests/compliance/` all green; optionally `make test-chromedp`,
`make test-heavy`, `make docker-smoke` for local pre-flight)

## 1. Pre-tag dry-run (recommended)

```sh
make release-dry-run
```

Expected outcome:

- All jobs (docker-smoke + release-docker + release-binary +
  update-floating-tag) finish green within ~25 minutes.
- `update-floating-tag` runs but exits as a no-op for dry-run
  invocations (the job's `if:` gate skips it on `dry_run=true`).
- Workflow artifacts panel lists the 4 binary bundles +
  `SHA256SUMS` + cosign bundle files. No GHCR push, no GH Release
  entry, no Rekor entry.

## 2. Pin action.yml to the release tag, commit, and tag

(unchanged from M10 — `VERSION=v1.0.1 make gen-action-yml`, commit
`--no-verify` to bypass the drift hook, annotated `git tag`, push)

```sh
git checkout main
git pull

# Pin action.yml to the release tag.
VERSION=v1.0.1 make gen-action-yml
git add action.yml
git commit --no-verify -m "chore(release): pin action.yml to v1.0.1"

git tag -a v1.0.1 -m "Release v1.0.1"
git push origin main v1.0.1
```

The release workflow auto-triggers on the tag push:

1. `docker-smoke` gates `release-docker` + `release-binary`.
2. `release-docker` builds + pushes the multi-arch GHCR image +
   cosign signs.
3. `release-binary` cross-compiles 4 binaries, generates SHA256SUMS,
   cosign sign-blob's each, AND **creates the GitHub Release with
   auto-generated notes in one shot** (no manual `gh release create`
   needed — this is the 009 improvement over M10).
4. `update-floating-tag` (NEW in 009) parses the just-pushed tag and
   advances `v1` to the v1.0.1 commit if and only if v1.0.1 is a
   stable release and not a back-port (per `contracts/floating-tag-policy.md` §2).

## 2a. Back-fill v1 floating tag (one-time, for the existing v1.0.0)

The v1.0.0 release predates 009, so the `v1` floating tag must be
created manually as a one-time maintainer action:

```sh
git tag -f v1 v1.0.0
git push origin refs/tags/v1
```

Verify:

```sh
git ls-remote origin refs/tags/v1 refs/tags/v1.0.0
# Both should return the same SHA (b3bd975).
```

After this, `uses: mjun0812/github-metrics@v1` in consumer workflows
resolves to v1.0.0. The next release (`v1.0.1`) will auto-advance `v1`
via the new workflow job.

## 3. Post-release verification

(unchanged from M10) Run the helper:

```sh
scripts/release-verify.sh v1.0.1
```

Sections 1-4 still apply. Additionally, manually verify the floating
tag advance:

```sh
git ls-remote origin refs/tags/v1 refs/tags/v1.0.1
# Both should return the same SHA.
```

For a pre-release publish (e.g. `v1.1.0-rc1`), verify that `v1`
remains pointing at the previous stable (e.g. v1.0.1):

```sh
git ls-remote origin refs/tags/v1
# Should still be at the v1.0.1 SHA, not the v1.1.0-rc1 SHA.
```

## 4. Smoke-test the published Action

(unchanged from M10) Trigger
`.github/workflows/sample-action-smoke.yml` and assert both x86 + arm64
jobs complete within 90s.

If you want to verify the **floating-tag pin** works end-to-end,
trigger a smoke workflow with `uses: mjun0812/github-metrics@v1`
(instead of `@v1.0.1`) and confirm the action resolves to the same
docker image.

## 5. Update README + announce

(unchanged from M10) — bump README's `@v1.0.0` → `@v1.0.1` for exact
pins, OR change to `@v1` to advertise floating-tag consumers. Per
009, README's Quick start example is already `@v1` (floating).

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Release workflow fails at "Upload to GitHub Release (production)" with `release not found` | This is the M10 bug fixed by 009 — should no longer occur. If it does, the `gh release view` branch detection is broken; manually create the release: `gh release create v1.0.1 --generate-notes` then re-run. |
| `v1` floating tag did NOT advance after a stable release | Either (a) the new tag was a pre-release suffix (`vX.Y.Z-...`) — expected behavior per FR-004; or (b) the workflow's `update-floating-tag` job failed. Check the run; for a quick fix, manually run `git tag -f v1 v1.0.1 && git push origin refs/tags/v1`. |
| `v1` floating tag regressed (now points at older `v1.0.5` instead of `v1.0.6`) | Should never happen per FR-005 (the back-port skip). If observed, the SemVer comparison logic in `update-floating-tag` is broken — file a bug, then manually `git tag -f v1 v1.0.6 && git push origin refs/tags/v1`. |
