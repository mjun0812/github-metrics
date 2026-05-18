# Contract: Release workflow

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-003

`.github/workflows/release.yml` extended from the M6 baseline to
support multi-arch GHCR push, SHA256 checksums, and cosign
keyless signing.

## 1. Triggers

| Trigger | Behavior |
|---------|----------|
| `push.tags: 'v*.*.*'` | Full release: build, sign, push, upload |
| `workflow_dispatch.inputs.dry_run='true'` (default) | Build all artifacts; skip push/upload/sign |
| `workflow_dispatch.inputs.dry_run='false'` | Same as tag push (for manual re-publish) |

## 2. Jobs

### 2.1 `release-docker`

| Step | Action |
|------|--------|
| Checkout | `actions/checkout@v4` with `fetch-depth: 0` |
| Extract tag | shell: parse `${GITHUB_REF#refs/tags/}` → `steps.tag.outputs.tag` and `outputs.short_sha` |
| Set up QEMU | `docker/setup-qemu-action@v3` |
| Set up Buildx | `docker/setup-buildx-action@v3` |
| Log in to GHCR | `docker/login-action@v3` (skipped under dry-run) |
| Build + push (multi-arch) | `docker/build-push-action@v5` with `platforms: linux/amd64,linux/arm64`, `tags: vX.Y.Z + latest + sha-<short>`, `push: ${{ inputs.dry_run != 'true' }}` |
| Size budget assertion | shell: `docker image inspect` → fail if > 600 MB per platform |
| Install cosign | `sigstore/cosign-installer@v3` |
| Sign manifest list | `cosign sign --yes ghcr.io/.../...:vX.Y.Z` (skipped under dry-run) |

### 2.2 `release-binary`

| Step | Action |
|------|--------|
| Checkout | `actions/checkout@v4` |
| Setup Go | `actions/setup-go@v5` with `go-version-file: go.mod` |
| Matrix build | shell: `go build -trimpath -ldflags="-s -w -X main.version=${TAG}" -o dist/metrics-action_${TAG}_${GOOS}_${GOARCH} ./cmd/metrics-action` for each matrix entry |
| Compute SHA256SUMS | shell: `cd dist && sha256sum metrics-action_* > SHA256SUMS` |
| Install cosign | `sigstore/cosign-installer@v3` |
| Sign each binary | shell: `cosign sign-blob --yes --output-signature ${f}.sig --output-certificate ${f}.cert --bundle ${f}.cosign.bundle ${f}` for each binary + SHA256SUMS (skipped under dry-run) |
| Upload to GitHub Release | shell: `gh release upload "${tag}" dist/* --clobber` (skipped under dry-run; under dry-run, `actions/upload-artifact@v4` instead) |

### 2.3 Matrix (binary cross-compile)

| `goos` | `goarch` | CGO | Notes |
|--------|----------|-----|-------|
| linux | amd64 | 0 | Standard |
| linux | arm64 | 0 | Standard |
| darwin | amd64 | 0 | Standard |
| darwin | arm64 | 0 | Standard |

Windows (`windows/amd64`) deliberately out of scope per spec
assumption (T-130 lists it; v1.0 scope ceiling is 4 variants).

## 3. Permissions

Workflow-level:

```yaml
permissions:
  contents: write     # GitHub Releases asset upload
  packages: write     # GHCR push
  id-token: write     # cosign keyless OIDC (NEW for M10)
```

`id-token: write` is the **only new permission** vs M6's release.yml.
It is required for cosign keyless signing and grants no privilege
beyond producing a short-lived OIDC certificate scoped to this
workflow run.

## 4. Cosign keyless OIDC

**Signing call shape**:

```bash
cosign sign --yes ghcr.io/mjun0812/github-metrics:vX.Y.Z
cosign sign-blob --yes \
  --output-signature metrics-action_vX.Y.Z_linux_amd64.sig \
  --output-certificate metrics-action_vX.Y.Z_linux_amd64.cert \
  --bundle metrics-action_vX.Y.Z_linux_amd64.cosign.bundle \
  metrics-action_vX.Y.Z_linux_amd64
```

**Identity claim**: GitHub Actions OIDC issuer ⇒ subject
`https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/vX.Y.Z`.

**Rekor transparency**: signatures are public; queryable via
`rekor-cli search --artifact <sha256>`.

**Consumer-side verification** (canonical): see
[research.md R-001](../research.md#r-001-cosign-keyless-oidc-signing--strategy--verification-command-shape)
for the exact commands.

## 5. Dry-run semantics

| Step | Real run | Dry run |
|------|---------|---------|
| Log in to GHCR | yes | skipped |
| Build image (buildx) | yes | yes (but `push: false`) |
| Size assertion | mandatory fail | advisory log line |
| Sign manifest | yes | skipped |
| Build binaries (matrix) | yes | yes |
| SHA256SUMS | yes | yes |
| Sign binaries | yes | skipped |
| GH Release upload | yes | replaced with `actions/upload-artifact@v4` (7-day retention) |

Dry-run mode is the recommended pre-tag verification path for
maintainers (per quickstart.md).

## 6. Failure handling

| Failure | Resolution |
|---------|-----------|
| `buildx` step fails | abort; no GHCR push; safe to retry |
| Size budget exceeded | abort; advisory in log; maintainer shrinks fonts or escalates |
| `cosign sign` fails (real run) | image is pushed but unsigned; maintainer re-runs cosign manually with `--yes` after addressing the OIDC token issue |
| Cross-compile step fails | abort the failing matrix entry; other variants continue |
| `gh release upload` rejects | typically tag-not-found; ensure tag exists first |

## 7. Idempotency

| Action | Idempotent? | Notes |
|--------|-------------|-------|
| GHCR image push | yes (tag-overwriting) | re-running same tag overwrites; pre-existing manifest list is replaced |
| Cosign Rekor entry | no | each run logs a new transparency entry; the original signatures remain auditable |
| GH Release asset upload | yes via `--clobber` | overwrites existing assets on the same tag |

A re-run against an existing tag is therefore safe to perform if
needed for forensic re-signing or fixing a botched upload.
