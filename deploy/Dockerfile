# syntax=docker/dockerfile:1.7
#
# github-metrics container build (M3 placeholder; M10 T-126 owns the
# production-grade multi-arch finalization). The image bundles
# chromium so the chromedp-backed svg.Resize path can run without
# additional setup — METRICS_CHROME_PATH is pre-set to the package
# binary location.
#
# Build:  docker build -f deploy/Dockerfile -t github-metrics:dev .
# Run:    docker run --rm github-metrics:dev metrics-action --help

# --- Build stage ----------------------------------------------------
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Cache module downloads independently of source edits.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath strips local paths, the ldflags pin the version through
# the linker variable the Makefile uses.
ARG VERSION=dev
RUN go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-action ./cmd/metrics-action \
    && go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-cli ./cmd/metrics-cli

# --- Runtime stage --------------------------------------------------
FROM debian:bookworm-slim

# chromium + fonts are required for the chromedp svg.Resize path.
# The fonts package keeps text rendering close to the upstream Node
# output on the same content.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium \
        fonts-noto-color-emoji \
        fonts-noto-cjk \
        fonts-liberation \
    && rm -rf /var/lib/apt/lists/*

ENV METRICS_CHROME_PATH=/usr/bin/chromium

COPY --from=build /out/metrics-action /usr/local/bin/metrics-action
COPY --from=build /out/metrics-cli    /usr/local/bin/metrics-cli

# Default to the action binary; CLI is reachable via explicit
# invocation. Switch to a non-root user once M10's hardening lands.
ENTRYPOINT ["/usr/local/bin/metrics-action"]
