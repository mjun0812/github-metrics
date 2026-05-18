# Implementation Plan: M10 — release / Docker distribution

**Branch**: `008-m10-release-distribution` | **Date**: 2026-05-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/008-m10-release-distribution/spec.md`

## Summary

M10 finalizes the release and distribution layer that M6 partially
stood up. The existing `deploy/Dockerfile` is explicitly labeled
`M3 placeholder; M10 T-126 owns the production-grade multi-arch
finalization`; the existing `.github/workflows/release.yml` builds a
single-arch (`linux/amd64`-only) GHCR image and 4 cross-compiled CLI
binaries without checksums or signatures. M10 ships:

- A production-grade `deploy/Dockerfile` — multi-arch buildable
  (linux/amd64 + linux/arm64), non-root runtime user (uid 10001),
  image-size budget ≤ 600 MB per platform verified in CI.
- An extended `release.yml` — `docker buildx` multi-arch push +
  `SHA256SUMS` checksum file + cosign keyless OIDC signing for image
  manifest and every binary; `dry_run=true` mode for pre-tag
  verification.
- A final `action.yml` `docker://ghcr.io/...:<version>` reference so
  consumers pinning to `@vX.Y.Z` get the matching image.
- A new `docs/migration-to-go.md` — upstream → go-port migration
  guide (adopted vs unported feature set, input-compatibility
  semantics, rollback procedure).

Constitution III invariant (no new plugin / template) is preserved
and explicitly tested via a new compliance assertion that the M1-M9
adopted-feature set is unchanged after M10 lands.

## Technical Context

**Language/Version**: Go 1.26 (unchanged; per `go.mod`).

**Primary Dependencies**: existing only at runtime —
`github.com/chromedp/chromedp` + chromium (in-image), stdlib. New
**build-pipeline-only** dependencies:

- `docker/buildx-action@v3` + `docker/setup-qemu-action@v3` — multi-arch
  via QEMU on GitHub-hosted `ubuntu-latest` runner.
- `sigstore/cosign-installer@v3` + cosign CLI — keyless OIDC signing
  against the GitHub Actions OIDC issuer
  (`https://token.actions.githubusercontent.com`).
- No new `go.mod` entries. No new runtime Go dependencies.

**Storage**: N/A. Release artifacts live in GHCR (image) and GitHub
Releases (binaries + checksums + signatures). Cosign signatures are
also written to the public sigstore Rekor transparency log.

**Testing**: existing `make test` / `make lint` continue to pass on
the M10 head commit (FR-010 guard). M10 adds:

- `tests/compliance/compliance_test.go` extension —
  `TestCompliance_M10_PluginTemplateInvariant` re-asserts the M1-M9
  adopted set is unchanged after M10.
- `tests/integration/dockerfile_test.go` — gated behind `//go:build
  docker_smoke` build tag; runs `docker build -f deploy/Dockerfile
  -t metrics-action:m10-smoke .` then `docker run` smoke + image
  size assertion. Optional locally; mandatory on the `release-docker`
  CI job.
- `release.yml` dry-run job — verifies all release-pipeline steps
  (buildx, cross-compile, checksum generation, cosign signing) work
  end-to-end without publishing.

**Target Platform**: container runtime: linux/amd64 + linux/arm64
(M10's expansion from M6's amd64-only). CLI binaries: linux/darwin ×
amd64/arm64 (4 variants; Windows out-of-scope per spec assumption).

**Project Type**: Single-binary Go monorepo (no change from M9).

**Performance Goals** (per spec SC-002 / SC-007):

- `docker pull` of the published image ≤ 60s on 100Mbps connection.
- End-to-end release pipeline runtime (tag push → all artifacts
  published) ≤ 25 minutes per
  [`docs/design/10-testing-deployment.md` §6](../../docs/design/10-testing-deployment.md).

**Constraints**:

- Image size per platform MUST stay ≤ 600 MB (FR-006); release
  workflow asserts this and fails the release on overrun.
- `action.yml` input matrix is frozen — M10 only updates the
  `docker://` image reference line.
- No production Go code in `cmd/` / `internal/` changes (constitution
  I + III).
- `docs/migration-to-go.md` is **Japanese-language** per project doc
  convention (constitution V).
- Cosign keyless OIDC requires the GitHub Actions OIDC token, so
  signing only runs in Actions context (not in local `act` runs).

**Scale/Scope**: 4 task families (Dockerfile finalization, release
workflow extension, action.yml refresh, migration guide doc). 1 new
docs file (`docs/migration-to-go.md`, ~250 lines), 1 modified
Dockerfile (~80 lines), 1 modified release.yml (~250 lines after
extension), 1 modified action.yml (1 line — the `image:` reference),
1 modified compliance test (~30 lines), 1 new optional docker-smoke
integration test (~80 lines). Total LOC delta: **~500 lines added /
~50 lines modified**. No Go production code changes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. 入力互換性 (NON-NEGOTIABLE) | ✓ N/A | M10 does NOT touch `action.yml` input names, defaults, or `metadata.yml` semantics. Only the `runs.image:` (or `runs.steps[].image:`) reference line is bumped to the M10-published GHCR tag. Upstream `lowlighter/metrics` input compatibility unchanged. |
| II. 出力契約 (DOM/JSON 単位) | ✓ N/A | No rendered output changes. SVG / JSON byte output identical for the same input — only the distribution pathway is hardened. |
| III. スコープ規律 (採用機能のみ実装) | ✓ PASS (strengthened) | Pure delivery-pipeline finalization. FR-010 + new `TestCompliance_M10_PluginTemplateInvariant` test re-assert that the 21 adopted plugins + 2 adopted templates set is unchanged after M10. No M5 (web) / M8 (social) code lands. |
| IV. テーブルテスト + Golden File | ✓ PASS (extended) | No rendering logic added. The release pipeline itself gets exercised via a `dry_run=true` workflow_dispatch path + a new optional `docker_smoke` integration test that builds + runs the image. Existing M1-M9 goldens stay byte-stable. |
| V. Go 規約と言語ポリシー | ✓ PASS | No Go source code added — only Dockerfile / workflow YAML / Japanese-language docs. The cross-compile pipeline uses standard `go build -trimpath -ldflags="-s -w -X main.version=..."` (per constitution Technical Constraints). gofumpt / golangci-lint baselines unchanged. |

**Result: ALL GATES PASS** — no Complexity Tracking entry needed.

## Project Structure

### Documentation (this feature)

```text
specs/008-m10-release-distribution/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 — R-001..R-005 (cosign signing, buildx QEMU, image-size shrinking, non-root user, migration guide scope)
├── data-model.md        # Phase 1 — E-001..E-004 (release artifact set, Dockerfile spec, release workflow spec, migration guide spec)
├── quickstart.md        # Phase 1 — release maintainer flow (pre-tag dry-run + tag push + verification)
├── contracts/
│   ├── dockerfile.md             # Dockerfile contract: stages, runtime user, env vars, healthcheck, size budget
│   ├── release-workflow.md       # release.yml contract: triggers, jobs, artifact matrix, signing, dry-run
│   ├── migration-guide.md        # docs/migration-to-go.md content contract: required sections, compatibility tables
│   └── action-yml.md             # action.yml image:-reference contract: docker:// scheme + version pinning
├── tasks.md             # Phase 2 (NOT created by /speckit-plan)
└── checklists/
    └── requirements.md  # 16/16 PASS (created in /speckit-specify)
```

### Source Code (repository root)

```text
deploy/
└── Dockerfile                      # MODIFIED: multi-arch friendly (no arch-specific RUN), non-root user (uid 10001), font pruning, size-assertion-friendly layer ordering

.github/workflows/
└── release.yml                     # EXTENDED: docker buildx (linux/amd64,linux/arm64) push + SHA256SUMS file + cosign keyless OIDC signing + size-budget assertion + dry-run mode

scripts/
└── release-verify.sh               # NEW: post-release verification helper — runs `docker manifest inspect`, `sha256sum -c`, `cosign verify` against a tag

action.yml                          # MODIFIED (1 line): runs.image: → docker://ghcr.io/mjun0812/github-metrics:<version> (or kept as ./Dockerfile reference per contracts/action-yml.md decision)

docs/
└── migration-to-go.md              # NEW: ~250-line Japanese-language migration guide

tests/
├── compliance/
│   └── compliance_test.go          # EXTENDED: TestCompliance_M10_PluginTemplateInvariant (re-asserts M1-M9 adopted set unchanged)
└── integration/
    └── dockerfile_test.go          # NEW: //go:build docker_smoke — builds & runs the image, asserts size + `metrics-action --help` exits 0

Makefile                            # EXTENDED: `make docker-smoke` target wraps the new test; `make release-dry-run` triggers release.yml workflow_dispatch with dry_run=true

README.md                           # MODIFIED: updates the `@v0.6.0` reference to `@v1.0.0` once M10 ships (and points to docs/migration-to-go.md for upstream users)
```

**Structure Decision**: Single-binary Go monorepo (unchanged). M10 is
a **delivery-pipeline finalization phase** — no new Go packages, no
new production code. The change surface is YAML (Dockerfile + 2
workflows), Japanese-language documentation, and one optional
build-tag-gated integration test. The new `scripts/release-verify.sh`
is a portable shell helper used both manually (by maintainers
post-tag) and in CI (consumed by the new docker-smoke test).

## Complexity Tracking

*No violations — all 5 constitution gates pass. This section is omitted
per template guidance.*
