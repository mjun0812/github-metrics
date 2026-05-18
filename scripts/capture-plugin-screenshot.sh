#!/usr/bin/env bash
# scripts/capture-plugin-screenshot.sh — capture a 720x800 PNG screenshot
# of a plugin's rendered SVG, mimicking GitHub's <img src=...svg> render
# path. Used by the 011 per-plugin parity PRs to commit before/after
# visual proof under specs/011-plugin-rendering-parity/plugins/screenshots/.
#
# Spec: specs/011-plugin-rendering-parity/quickstart.md §2, §7
# Data model: E-006 (per-plugin screenshot pair)
#
# Usage:
#   bash scripts/capture-plugin-screenshot.sh <slug> <before|after>
#
# Pre-conditions:
#   - GITHUB_TOKEN exported (classic PAT with read:user + repo scopes)
#   - docker image github-metrics:local present
#       (build with: docker build -f deploy/Dockerfile -t github-metrics:local .)
#   - chromium or Google Chrome installed locally
#       (macOS: /Applications/Google Chrome.app — auto-detected)
#       (Linux: chromium in PATH — auto-detected)
#
# Env overrides:
#   METRICS_DOC_USER   default: mjun0812
#   METRICS_DOC_IMAGE  default: github-metrics:local
#   METRICS_CHROME     override chrome binary path

set -uo pipefail

SLUG="${1:-}"
PHASE="${2:-}"

usage() {
  echo "Usage: $0 <plugin-slug> <before|after>" >&2
  echo "Example: $0 languages before" >&2
  exit 2
}

if [[ -z "$SLUG" || -z "$PHASE" ]]; then
  usage
fi
if [[ "$PHASE" != "before" && "$PHASE" != "after" ]]; then
  echo "FATAL: phase must be 'before' or 'after' (got: $PHASE)" >&2
  usage
fi

USER="${METRICS_DOC_USER:-mjun0812}"
IMAGE="${METRICS_DOC_IMAGE:-github-metrics:local}"

# Pre-conditions ----------------------------------------------------
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "FATAL: GITHUB_TOKEN env var is empty or unset" >&2
  exit 1
fi
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "FATAL: docker image '$IMAGE' not found." >&2
  echo "Build it first: docker build -f deploy/Dockerfile -t github-metrics:local ." >&2
  exit 1
fi

# Detect chromium / chrome binary.
CHROME_BIN="${METRICS_CHROME:-}"
if [[ -z "$CHROME_BIN" ]]; then
  for candidate in \
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    "/Applications/Chromium.app/Contents/MacOS/Chromium" \
    "$(command -v chromium 2>/dev/null || true)" \
    "$(command -v google-chrome 2>/dev/null || true)"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      CHROME_BIN="$candidate"
      break
    fi
  done
fi
if [[ -z "$CHROME_BIN" ]]; then
  echo "FATAL: chromium / Google Chrome not found. Install or set METRICS_CHROME." >&2
  exit 1
fi

OUTDIR="specs/011-plugin-rendering-parity/plugins/screenshots"
mkdir -p "$OUTDIR"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

# 1. Render the plugin SVG via the local docker image. Mirrors the
#    invocation used by scripts/gen-doc-samples.sh for consistency.
echo "1/3 rendering ${SLUG} SVG via docker..."
if ! docker run --rm \
    -e GITHUB_TOKEN \
    -v "${WORKDIR}:/out" \
    "$IMAGE" \
    --user "$USER" \
    --token-env GITHUB_TOKEN \
    --template classic \
    --output svg \
    --filename "/out/${SLUG}.svg" \
    --dryrun \
    --plugin "plugin_${SLUG}=yes" >"${WORKDIR}/render.log" 2>&1
then
  echo "FATAL: docker render failed (see ${WORKDIR}/render.log):" >&2
  cat "${WORKDIR}/render.log" >&2
  trap - EXIT
  exit 1
fi
if [[ ! -s "${WORKDIR}/${SLUG}.svg" ]]; then
  echo "FATAL: SVG file empty / not produced" >&2
  exit 1
fi

# 2. HTML wrapper that mimics GitHub's <img src=...svg> render path
#    (foreignObject contents render the same way as on github.com).
echo "2/3 wrapping SVG in HTML for <img> rendering..."
cat >"${WORKDIR}/wrapper.html" <<EOF
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>${SLUG} screenshot</title>
<style>
  body { margin: 0; padding: 8px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; background: #f6f8fa; }
  img { display: block; max-width: 704px; box-shadow: 0 0 0 1px #d0d7de; }
</style>
</head>
<body>
<img src="file://${WORKDIR}/${SLUG}.svg" alt="${SLUG} sample">
</body>
</html>
EOF

# 3. Headless chromium screenshot at 720x800. --no-sandbox is required
#    when chromium runs as root; harmless otherwise.
echo "3/3 capturing 720x800 screenshot via headless chromium..."
"$CHROME_BIN" \
  --headless=new \
  --disable-gpu \
  --no-sandbox \
  --hide-scrollbars \
  --window-size=720,800 \
  --virtual-time-budget=2000 \
  --screenshot="${WORKDIR}/screenshot.png" \
  "file://${WORKDIR}/wrapper.html" >"${WORKDIR}/chrome.log" 2>&1 \
  || {
    echo "FATAL: chrome screenshot failed (see ${WORKDIR}/chrome.log):" >&2
    cat "${WORKDIR}/chrome.log" >&2
    trap - EXIT
    exit 1
  }
if [[ ! -s "${WORKDIR}/screenshot.png" ]]; then
  echo "FATAL: screenshot file empty / not produced" >&2
  exit 1
fi

FINAL="${OUTDIR}/${SLUG}-${PHASE}.png"
mv "${WORKDIR}/screenshot.png" "$FINAL"
echo "OK: ${FINAL} ($(wc -c <"$FINAL" | tr -d ' ') bytes)"
