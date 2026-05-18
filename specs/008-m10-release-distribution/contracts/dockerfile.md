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
| Size budget | ≤ 900 MB per platform | v1.0 escalation from the original 600 MB target — see Note below |

> **Note on size budget**: The original 600 MB target (specs FR-006 v1)
> was based on lowlighter/metrics's Node + Puppeteer image size. CI
> measurement on GitHub-hosted `ubuntu-latest` (2026-05-18) showed the
> bookworm-slim + chromium + Noto CJK + Noto Emoji + Liberation
> combination at ~834 MB amd64. Per spec assumption "If the chromium +
> fonts combination cannot fit within 600 MB at build time, the plan
> phase will surface this as an explicit trade-off", the budget is
> raised to 900 MB for v1.0. Dropping `fonts-noto-cjk` (~80 MB savings)
> was rejected because it breaks CJK repo-name / display-name
> rendering. Post-v1.0 optimization candidates: switch runtime base to
> `chromedp/headless-shell` (~250 MB), or split CJK fonts into an
> opt-in variant tag.

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

**Total runtime image floor (measured on GitHub-hosted ubuntu-latest, 2026-05-18)**: ~834 MB amd64. arm64 build under QEMU emulation yields a comparable size. The per-package estimates above sum to ~575 MB but the actual chromium + transitive apt dependencies (libgtk-3, libnss3, libcups2, etc.) bloat the runtime considerably on x86_64 Debian bookworm; the 900 MB budget per §1 leaves ~66 MB headroom on this measurement.

## 4. Size budget enforcement

The `docker-smoke` job in `.github/workflows/release.yml` runs the
`docker_smoke`-tagged test which performs the size assertion:

```go
if sizeBytes > dockerSmokeSizeBudgetBytes /* = 900 MB */ {
    t.Errorf("image size %d bytes (%d MB) exceeds budget 900 MB (FR-006)", ...)
}
```

If size exceeds budget, `docker-smoke` fails; `release-docker` and
`release-binary` have `needs: docker-smoke` so they do not start and
no `docker push` runs. Recovery options (in priority order): (1)
drop `fonts-noto-cjk` to save ~80 MB; (2) switch runtime base to
`chromedp/headless-shell` (~600 MB savings, but loses CJK fonts);
(3) further raise the budget via plan-phase decision.

## 5. Verification

The new `tests/integration/dockerfile_test.go` (build tag
`docker_smoke`) builds the image, runs `docker run --rm
$IMAGE metrics-action --help`, and asserts:

- exit code 0
- stdout contains `Usage:` or `metrics-action` prefix
- `docker image inspect` size ≤ 900 MB per platform (per FR-006 escalation — see §1 Note)

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
