# syntax=docker/dockerfile:1.7
#
# github-metrics production container (M10 T-126). The image bundles
# chromium so the chromedp-backed svg.Resize / PNG / JPEG path can run
# without additional setup — METRICS_CHROME_PATH points at the system
# chromium binary.
#
# Multi-arch: built for linux/amd64 + linux/arm64 via `docker buildx`
# in .github/workflows/release.yml. No arch-specific RUN steps; the Go
# binary is compiled inside the builder stage which buildx runs at the
# target platform.
#
# Runtime user: `metrics` (uid 10001, gid 10001) — non-root for
# defence-in-depth. chromedp launches chromium with --no-sandbox per
# internal/render/browser.go which is required when running as a
# non-root user without seccomp profiles.
#
# Image size budget: ≤ 900 MB per platform (asserted in CI by the
# docker_smoke gate). The v1.0 escalation from 600 MB → 900 MB is
# documented in research.md R-003 §"Plan-phase risk": the
# bookworm-slim + chromium + Noto CJK fonts combination measures
# ~830 MB on GitHub-hosted ubuntu-latest runners. CJK fonts cost
# ~80 MB but breaking CJK repo-name rendering was rejected for v1.0.
#
# Build (local):  docker build -t github-metrics:dev .
# Run   (local):  docker run --rm github-metrics:dev metrics-cli --help

# --- Build stage ----------------------------------------------------
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Cache module downloads independently of source edits.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath strips local paths; ldflags pin the version into
# main.version (matches Makefile build flags + constitution Technical
# Constraints). CGO_ENABLED=0 keeps the binary statically linked so
# the runtime stage does not need glibc-compat headaches when arm64
# emulation is in play.
ARG VERSION=dev
ENV CGO_ENABLED=0
RUN go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-cli ./cmd/metrics-cli

# --- Runtime stage --------------------------------------------------
FROM debian:bookworm-slim

# Install chromium + fonts and clean apt metadata in the same layer
# so the cleanup actually shrinks the image. The Noto CJK font set
# costs ~80 MB but is required for CJK glyph rendering in repo /
# display names; if the per-platform size budget tightens, this is
# the first lever per research.md R-003.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium \
        fonts-noto-color-emoji \
        fonts-noto-cjk \
        fonts-liberation \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb

ENV METRICS_CHROME_PATH=/usr/bin/chromium

# Binaries land at /usr/local/bin/, owned by root and mode 0755 so
# the non-root runtime user can execute but not modify.
COPY --from=build /out/metrics-cli /usr/local/bin/metrics-cli

# Non-root user. uid 10001 sits outside the system range (0-999) and
# the GitHub Actions runner range (1000-1999), avoiding collisions
# when the container is invoked outside Actions context.
RUN groupadd --system --gid 10001 metrics \
    && useradd  --system --uid 10001 --gid metrics --no-create-home --shell /sbin/nologin metrics

USER metrics

ENTRYPOINT ["/usr/local/bin/metrics-cli"]
