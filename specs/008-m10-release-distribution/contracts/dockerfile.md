# Contract: Production Dockerfile

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002

The runtime image consumed by both `uses: mjun0812/github-metrics@vX.Y.Z`
Action invocations and direct `docker run ghcr.io/...` invocations.
Replaces the M3-placeholder Dockerfile per T-126.

## 1. Build invariants

| Property | Value | Rationale |
|----------|-------|-----------|
| Build base | `golang:1.26-bookworm` | matches `go.mod` `go` directive (constitution V) |
| Runtime base | `debian:bookworm-slim` | smallest debian variant with full apt; required for chromium availability (research R-003) |
| Stages | 2 (`build`, `runtime`) | minimizes runtime image (no Go toolchain shipped) |
| BUILD ARG `VERSION` | required, no default | release.yml passes the tag; build fails fast if unset rather than silently shipping `version=dev` |
| go build flags | `-trimpath -ldflags="-s -w -X main.version=${VERSION}"` | per constitution Technical Constraints |
| Multi-arch | `linux/amd64` + `linux/arm64` via buildx | required by FR-001 |
| `apt-get clean` + `rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb` | mandatory in the same RUN as install | shrinks layers per R-003 |

## 2. Runtime environment

| Property | Value | Rationale |
|----------|-------|-----------|
| `ENTRYPOINT` | `["/usr/local/bin/metrics-action"]` | default action invocation |
| `USER` | `metrics` (uid 10001, gid 10001) | FR-005 non-root; uid choice per R-004 |
| `WORKDIR` | (none / inherited) | Action invokes with `--workdir $GITHUB_WORKSPACE` injected by the runner |
| `ENV METRICS_CHROME_PATH` | `/usr/bin/chromium` | chromedp resize path uses this |
| Binary mode | 0755, owned by root | non-root can execute, cannot modify |
| HEALTHCHECK | NONE | Action container is per-invocation; no consumer |

## 3. apt package set

| Package | Purpose | Approx size |
|---------|---------|-------------|
| `ca-certificates` | TLS for GitHub API calls | ~1 MB |
| `chromium` | chromedp resize/PNG/JPEG path | ~200 MB |
| `fonts-noto-color-emoji` | emoji glyph support in rendered SVG | ~30 MB |
| `fonts-noto-cjk` | CJK glyph support (escalation lever — see R-003) | ~80 MB |
| `fonts-liberation` | sans/serif/mono Latin fallback | ~5 MB |

**Total runtime image floor**: ~575 MB amd64 / ~600 MB arm64.

## 4. Size budget enforcement

Release workflow asserts post-build:

```bash
size_bytes=$(docker image inspect "${IMAGE_REF}" --format '{{.Size}}')
size_mb=$((size_bytes / 1024 / 1024))
if [ "$size_mb" -gt 600 ]; then
  echo "::error::Image size ${size_mb} MB exceeds 600 MB budget"
  exit 1
fi
```

If size exceeds budget, the release fails and no `docker push`
runs. Recovery options (in priority order): (1) drop
`fonts-noto-cjk` to save ~80 MB; (2) escalate the budget via
plan-phase decision.

## 5. Verification

The new `tests/integration/dockerfile_test.go` (build tag
`docker_smoke`) builds the image, runs `docker run --rm
$IMAGE metrics-action --help`, and asserts:

- exit code 0
- stdout contains `Usage:` or `metrics-action` prefix
- `docker image inspect` size ≤ 600 MB per platform

The local helper `make docker-smoke` wraps the test for
developer self-service. The release workflow `release-docker`
job runs it as a gate before signing.

## 6. Non-functional notes

- **Why no HEALTHCHECK**: GitHub Actions invokes the container
  with `--entrypoint /usr/local/bin/metrics-action $INPUT_*`
  and reaps it after each invocation. There is no long-running
  process to health-check. Adding HEALTHCHECK would slow cold
  starts without benefit.
- **Why bookworm-slim, not bookworm-distroless**: distroless has
  no apt, so we cannot install chromium + fonts. Multi-stage
  copying from a debian stage would erase any size win and add
  complexity.
- **Why uid 10001**: OpenShift / Kubernetes / OSS-service
  convention; safe outside the 0-999 system range and the
  1000-1999 GitHub Actions runner range.
