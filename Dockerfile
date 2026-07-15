# syntax=docker/dockerfile:1.7
#
# github-metrics production container (M10 T-126). The image bundles
# the `resvg` rasterizer so the PNG / JPEG output path can run without
# additional setup — METRICS_RESVG_PATH points at the resvg binary.
# #409 Phase D (#694) replaced the chromedp/headless-chromium renderer
# with resvg, removing ~500 MB of chromium from the image and the whole
# class of "distro bumped chromium and headless crashes in a container"
# incidents (the 2026-07-06 bookworm chromium 149→150 SIGTRAP outage).
#
# Multi-arch: built for linux/amd64 + linux/arm64 via `docker buildx`
# in .github/workflows/release.yml. No arch-specific RUN steps; both the
# Go binary and the resvg binary are compiled inside builder stages that
# buildx runs at the target platform (resvg has no prebuilt linux/arm64
# release, so it is built from source with `cargo install`).
#
# Runtime user: `metrics` (uid 10001, gid 10001) — non-root for
# defence-in-depth. resvg is a self-contained subprocess with no sandbox
# or seccomp requirements, so no special flags are needed.
#
# Image size budget: ≤ 500 MB per platform (asserted in CI by the
# docker_smoke gate). The pre-Phase-D budget was ≤ 900 MB, dominated by
# chromium (~500 MB); with chromium gone the runtime is bookworm-slim +
# the ~6 MB resvg binary + fonts (Noto CJK ~80 MB, Noto emoji, Liberation
# — all kept because resvg rasterizes text with the system font database),
# ~200 MB observed.
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

# svg2png rasterizes a standalone metrics SVG to PNG through the same
# internal/render.Resvg path that --output png uses, with zero API calls
# (#527). The regen-doc-samples render job ships it alongside metrics-cli
# so it can fetch each sample's SVG once and derive the PNG locally
# instead of running a second full API fetch. It is a static
# CGO_ENABLED=0 binary (~few MB) so the image-size impact is trivial.
RUN go build -trimpath \
    -ldflags="-s -w" \
    -o /out/svg2png ./internal/tools/svg2png

# --- resvg build stage ----------------------------------------------
# resvg publishes no prebuilt linux/arm64 binary (only linux/x86_64), so
# build it from source with cargo. buildx runs this stage at the target
# platform, giving a native binary for both amd64 and arm64. resvg 0.47
# requires rustc ≥ 1.87; `rust:1-slim-bookworm` tracks the latest 1.x.
# build-essential provides the C compiler a few transitive `cc`-crate
# deps expect. The buildx layer cache amortizes the compile across CI
# runs.
FROM rust:1-slim-bookworm AS resvg-build
RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
RUN cargo install resvg --version 0.47.0 --locked
# Binary lands at $CARGO_HOME/bin/resvg (/usr/local/cargo/bin/resvg).

# --- Runtime stage --------------------------------------------------
FROM debian:bookworm-slim

# Install the font database resvg rasterizes text with, plus CA certs,
# and clean apt metadata in the same layer so the cleanup shrinks the
# image. The Noto CJK set costs ~80 MB but is required for CJK glyph
# rendering in repo / display names; Liberation Sans/Mono back the
# `sans-serif` / `monospace` generics the SVG font stacks resolve to
# (see internal/render/resvg.go). No chromium is installed — #409
# Phase D removed the browser renderer.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        fonts-noto-color-emoji \
        fonts-noto-cjk \
        fonts-liberation \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb

# resvg binary + the env var internal/render.Resvg resolves it from.
COPY --from=resvg-build /usr/local/cargo/bin/resvg /usr/local/bin/resvg
ENV METRICS_RESVG_PATH=/usr/local/bin/resvg

# Binaries land at /usr/local/bin/, owned by root and mode 0755.
COPY --from=build /out/metrics-cli /usr/local/bin/metrics-cli

# svg2png is a developer/CI aid (regen-doc-samples render job, #527), not
# part of the action / CLI contract. It rasterizes via the same resvg
# binary through METRICS_RESVG_PATH.
COPY --from=build /out/svg2png /usr/local/bin/svg2png

# Run as root so `output_action: commit` / `filename:` writes into
# GitHub Actions' `/github/workspace` mount succeed. That directory is
# owned by the runner user (uid 1001) and refuses writes from any other
# uid, so a non-root container UID cannot land the output file (#779).
# `lowlighter/metrics` runs as root for the same reason.
ENTRYPOINT ["/usr/local/bin/metrics-cli"]
