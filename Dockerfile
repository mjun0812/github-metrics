# syntax=docker/dockerfile:1.7
#
# Multi-stage Dockerfile for the metrics-action binary.
#
# Builder stage compiles a static binary; runtime stage adds chromium
# for chromedp-driven SVG resize / PNG / JPEG rendering (M3) and
# exposes the binary as ENTRYPOINT so `docker run` works as both an
# Action (GITHUB_ACTIONS=true) and a CLI (any args).
#
# Image: ghcr.io/mjun0812/github-metrics:vX.Y.Z (semver) + :latest +
# :sha-<short> per the release pipeline in
# .github/workflows/release.yml.

# -------- Builder stage --------
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git make ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/metrics-action ./cmd/metrics-action

# -------- Runtime stage --------
# chromedp/headless-shell ships a pre-configured chromium suitable for
# headless rendering. We use it as the base so we don't have to manage
# chromium dependencies ourselves.
FROM chromedp/headless-shell:latest AS runtime

# Working directory used by `action.Run` to write the rendered output
# file (default /renders/github-metrics.svg).
WORKDIR /

RUN mkdir -p /renders

COPY --from=builder /out/metrics-action /metrics-action

# Default entrypoint: the metrics-action binary. The runner provides
# INPUT_* env vars in Action mode; CLI mode receives args directly.
ENTRYPOINT ["/metrics-action"]
