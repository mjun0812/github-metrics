# Research: M10 — release / Docker distribution

**Date**: 2026-05-18 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

This document resolves the 5 build-pipeline decisions M10 has to
make before Phase 1 design. The spec sections referenced in
"Spec linkage" below frame each decision; the chosen approach
becomes the Phase 1 contract input.

---

## R-001: Cosign keyless OIDC signing — strategy + verification command shape

**Decision**: Use cosign keyless OIDC signing via the GitHub Actions
OIDC issuer (`https://token.actions.githubusercontent.com`) — no
maintainer-managed signing keys. Signatures are written to the
public sigstore Rekor transparency log; verification proves the
artifact was built by *this specific workflow on this specific
repository*. Both the Docker image manifest and each CLI binary
are signed.

The canonical consumer-side verification command is:

```bash
cosign verify ghcr.io/mjun0812/github-metrics:v1.0.0 \
  --certificate-identity-regexp 'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

For binaries (cosign sign-blob path):

```bash
cosign verify-blob metrics-action_v1.0.0_linux_amd64 \
  --bundle metrics-action_v1.0.0_linux_amd64.cosign.bundle \
  --certificate-identity-regexp 'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

**Rationale**:

- **No key management** — cosign keyless uses ephemeral certificates
  scoped to the workflow run, so there is no long-lived secret to
  rotate, leak, or escrow.
- **Public provenance** — the certificate-identity-regexp pin proves
  the artifact was produced by `release.yml` on a `v*` tag of this
  repository, not by an attacker who compromised the GHCR push token.
- **Transparency log** — Rekor entries are queryable
  (`rekor-cli search --artifact <hash>`) so downstream consumers can
  audit the entire signing history.
- **Industry standard for OSS** — kubernetes, distroless, github-cli
  itself all use this exact pattern; consumers' tooling already knows it.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| GPG signing | Requires maintainer key generation + management + escrow plan; verification UX is poor (recipient needs the pubkey out-of-band); no transparency log. |
| Cosign key-pair (non-keyless) | Same key-management burden as GPG with no real upside. Pre-1.0 sigstore default; obsolete pattern. |
| No signing, checksum only | SHA256SUMS proves byte-integrity but not provenance — an attacker who controls the release bucket could swap both the binary and the checksum. Signing closes that gap. |
| Notary / Docker Content Trust | Notary v1 is end-of-life; notary v2 is not yet broadly tooled for OSS. |

**Spec linkage**: FR-004, SC-004. Edge case: cosign OIDC token
unavailable → signing step fails fast with actionable error.

**Plan-phase risk**: cosign-installer + the OIDC token grant
(`id-token: write` permission on the release job) are the only new
moving pieces. Both are well-documented at sigstore.dev. Expected
release-pipeline overhead: ≤ 30s per artifact.

---

## R-002: Docker buildx multi-arch — QEMU emulation vs native runners

**Decision**: Use `docker/setup-qemu-action@v3` + `docker/setup-buildx-action@v3`
on the standard `ubuntu-latest` runner with `platforms:
linux/amd64,linux/arm64`. linux/arm64 is built under QEMU emulation
on x86_64; that doubles the build time vs native but stays well
within the 25-minute pipeline budget (per SC-007) for the current
image content (single Go binary compiled at host arch + apt install
chromium + fonts).

**Rationale**:

- **Single runner, single job** — keeps `release.yml` simple. A
  matrix of native runners (ubuntu-latest + ubuntu-24.04-arm) would
  also work but doubles the job count and requires an additional
  `docker manifest create` step to glue them into one manifest list.
  buildx + QEMU produces the manifest list in one shot.
- **Build is already I/O-bound** — most of the runtime is apt-get
  installing chromium + Noto fonts, not Go compilation. QEMU
  emulation overhead is modest on this workload (measured ~3-4x
  slowdown on arm64 emulated builds, but the Go binary is built
  **outside** the emulated layer via the multi-stage Dockerfile's
  builder stage running natively).
- **Future-proofing** — if QEMU build time later balloons (e.g., if
  we ever shrink chromium and the Go compile becomes dominant), the
  fallback is a native arm64 matrix job. The Dockerfile multi-arch
  semantics don't change.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Native arm64 runner (`ubuntu-24.04-arm`) | Currently in public preview for GitHub-hosted runners but billing semantics are still evolving; reserve as v1.x optimization once GA. |
| Self-hosted arm64 runner | Maintenance burden (one more piece of infra to keep secure + updated) outweighs the build-time savings for a project this size. |
| Build amd64 + arm64 in separate jobs then `docker manifest create` | Doubles job count, introduces an extra synchronization step, and complicates dry-run mode. |

**Spec linkage**: FR-001, SC-001, SC-007. Edge case: buildx
auto-uses QEMU on x86 runners — explicitly documented in the
release.yml comments.

**Plan-phase risk**: QEMU step adds ~30-60s setup time per workflow
run. Total arm64 emulated build estimated 6-8 min; well within the
25-min ceiling.

---

## R-003: Image-size budget — staying ≤ 600 MB per platform

**Decision**: Keep the `debian:bookworm-slim` runtime base and the
existing `chromium + fonts-noto-color-emoji + fonts-noto-cjk +
fonts-liberation` apt set. Add `apt-get clean` + `rm -rf
/var/lib/apt/lists/*` + remove `/var/cache/apt/archives/*.deb` at
the end of the runtime stage (the existing Dockerfile already does
the first two; the third is the new shrink). Drop `fonts-noto-cjk`
**only if** the post-shrink image exceeds 600 MB per platform —
CJK fonts add ~80 MB but matter for users rendering on accounts
with Japanese/Chinese/Korean characters in repo names.

The release workflow asserts the size after build:

```bash
size_bytes=$(docker image inspect ghcr.io/.../...:vX.Y.Z --format '{{.Size}}')
size_mb=$((size_bytes / 1024 / 1024))
if [ "$size_mb" -gt 600 ]; then
  echo "::error::Image size $size_mb MB exceeds 600 MB budget"
  exit 1
fi
```

**Rationale**:

- **Chromium dominates** — `apt-get install chromium` pulls ~200 MB
  on bookworm-slim. There is no smaller realistic option for the
  chromedp resize/PNG/JPEG path; switching to puppeteer-style
  prebuilt chromium binaries would re-introduce the Node toolchain
  the Go port removed.
- **Fonts are the only real lever** — Noto CJK is 80 MB; emoji 30 MB;
  Liberation 5 MB. The base-+-chromium-+-Liberation+emoji floor is
  ~450 MB; CJK pushes us to ~530 MB; the existing Go binaries add
  ~25 MB; layer overhead ~20 MB; total ~575 MB amd64 / ~600 MB arm64
  (chromium arm64 build is slightly larger).
- **Budget headroom is thin** — the spec's 600 MB target is roughly
  what upstream lowlighter/metrics's Node-based image runs at, so the
  Go port should not regress.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Distroless base | No `apt` means we cannot install chromium + fonts at build time; would have to multi-stage-copy them from a debian stage anyway, with no real size win. |
| Alpine base + chromium-via-musl | Alpine's chromium package is broken intermittently and CJK font availability is poor; community reports of chromedp instability on Alpine. |
| `headless-shell` instead of full chromium | Saves ~60 MB but the upstream chromedp/headless-shell image is itself ~250 MB; we already use this image *only* for the chromedp CI test job, not for the action runtime. Mixing two chromium variants would complicate behavior. |
| Drop CJK fonts unconditionally | Users with CJK characters in repo names or display names get glyph fallback boxes in the rendered SVG. Acceptable for an MVP but worth keeping by default for upstream compatibility. |

**Spec linkage**: FR-006, SC-003. Edge case: image size overrun →
release workflow asserts and fails before push.

**Plan-phase risk**: chromium upstream package size can drift
release-to-release. The CI size assertion catches drift early; if
the budget becomes infeasible the plan-phase escalation is to drop
CJK fonts (saving ~80 MB) before relaxing the budget.

---

## R-004: Non-root user in Dockerfile — uid choice + fs ownership

**Decision**: Add a dedicated `metrics` user (uid 10001, gid 10001)
in the runtime stage. `metrics-action` runs as that user. Binary
location (`/usr/local/bin/metrics-action`) is owned by root and
mode 0755 so the non-root user can execute but not modify. No
writable filesystem locations are required by the binary itself —
the Action invocation typically writes via GitHub API or to
`$GITHUB_WORKSPACE` which the runtime can read/write as the
runner-injected user.

```dockerfile
RUN groupadd --system --gid 10001 metrics \
  && useradd  --system --uid 10001 --gid metrics --no-create-home --shell /sbin/nologin metrics
USER metrics
```

**Rationale**:

- **uid 10001** is the OpenShift / common-OSS convention for
  unprivileged service uids; falls outside the default `/etc/passwd`
  range (0-999 reserved for system, 1000+ for humans) and avoids
  collisions with runner-injected uids GitHub Actions uses.
- **No HOME directory** — `--no-create-home` keeps the image
  smaller and prevents accidental temp-file accumulation. chromedp
  writes to `/tmp` (world-writable) when needed.
- **Why not uid 1000** — GitHub Actions runs the container with
  `--user 1001:121` (the `runner` user); using uid 1000 would
  conflict with images run outside Actions context (e.g., local
  `docker run`). uid 10001 is unambiguous.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| Run as `nobody` (uid 65534) | Convention-violating for service workloads; some K8s policies block it. |
| Stick with root (current Dockerfile) | Violates the "Switch to a non-root user once M10's hardening lands" TODO already in the Dockerfile comments; defeats container-hardening best practices. |
| uid 1000 | Conflicts with GitHub Actions' injected runner uid (1001) and many host-user setups. |

**Spec linkage**: FR-005. Edge case: GitHub Action's runner
overrides USER directive when `runs.using: docker` invokes the
container — our `USER metrics` directive is preserved unless the
consumer explicitly sets `runs.docker.username:` (rare).

**Plan-phase risk**: chromedp + chromium-as-non-root requires
disabling chromium's sandbox (`--no-sandbox` flag) — already done
in `internal/render/browser.go` so no Dockerfile-side change is
needed beyond the USER directive.

---

## R-005: Migration guide content scope + structure

**Decision**: `docs/migration-to-go.md` is a **Japanese-language**
top-down migration guide structured in 6 sections (per existing
project doc convention — see
[`docs/design/15-selection-answer.md`](../../docs/design/15-selection-answer.md)
as a structural reference):

1. **概要 (Overview)** — Why this go-port exists, what it offers vs
   upstream, who this guide is for.
2. **採用機能一覧 (Adopted feature matrix)** — Authoritative table
   listing the 21 adopted plugins + 2 adopted templates + the
   adopted output formats, cross-referenced to the source-of-truth
   docs (`docs/design/15-selection-answer.md`).
3. **未対応機能一覧 (Unported feature matrix)** — Mirror table of
   the unported set: M5 web instance / OAuth / insights, M8 social
   plugins (19 plugins), community templates (terminal / markdown /
   community), PDF / Markdown output, with a one-line rationale per
   row referencing the selection-answer doc.
4. **入力互換性 (Input compatibility semantics)** — How `with:`
   inputs map across the boundary: adopted inputs are drop-in
   identical (per constitution principle I); unported inputs are
   silently no-op'd; provides a worked example of a
   `metrics.yml` workflow that uses both adopted + unported inputs
   and shows what the go port does.
5. **移行手順 (Migration steps)** — Step-by-step: (a) pin
   `mjun0812/github-metrics@v1.0.0`, (b) drop unported plugin
   gates, (c) re-run the workflow, (d) compare output against
   pre-migration baseline (DOM-equivalent SVG per constitution
   principle II).
6. **ロールバック (Rollback procedure)** — One-line revert of the
   `uses:` field, zero-cost re-flow: the upstream workflow file
   syntax is unchanged so no migration of inputs/secrets is needed.

The guide is intended to be readable end-to-end in under 10
minutes (spec SC-005). The unported-features rationale links back
to `docs/design/15-selection-answer.md` rather than re-iterating
the per-plugin justification.

**Rationale**:

- **Japanese-first matches project convention** — `docs/design/`,
  `README.md` are Japanese (constitution V "ドキュメント言語").
  Code comments / identifiers remain English. English translation
  can ship as a follow-up patch release if the upstream community
  requests it; not blocking for v1.0.
- **6 sections mirror the upstream migration-guide template** — most
  OSS port projects (Caddy 1→2, Hugo themes 1→2, etc.) use this
  exact structure. Reduces cognitive load for readers coming from
  similar migrations.
- **Authoritative tables, not duplicated rationale** — the
  selection-answer doc owns the *why* per plugin. The migration
  guide owns the *what* and *how*. Avoids two sources of truth.

**Alternatives considered**:

| Option | Rejected because |
|--------|-----------------|
| English-first guide | Conflicts with constitution V "docs are Japanese". Re-evaluate post-v1.0 based on upstream community response. |
| One-page cheatsheet (no narrative) | Insufficient context for users mid-migration who hit unported features; would generate support questions a fuller guide pre-empts. |
| Inline per-plugin migration notes in each plugin's source | Scatters knowledge; hard to discover. The doc-first approach centralizes it. |

**Spec linkage**: FR-008, SC-005. Edge case: a reader with a
workflow that depends on unported plugin output → guide §3 + §4
explicitly walk through the silent-no-op behavior so they can
remove the unported gates before migration without surprise.

**Plan-phase risk**: low. Doc size estimated ~250 lines of
Markdown. Single reviewer (the maintainer); no external dependency.

---

## Summary

All 5 decisions resolved with informed choices. No `NEEDS
CLARIFICATION` carries through to Phase 1. The chosen approaches
align with constitution principles I (input compat untouched),
III (scope discipline guarded by FR-010 + new compliance test),
and V (Japanese docs, English code).
