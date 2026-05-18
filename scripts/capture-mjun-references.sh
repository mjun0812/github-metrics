#!/usr/bin/env bash
# scripts/capture-mjun-references.sh — render the 5 mjun0812 plugin SVGs
# with their exact upstream settings + capture 720x800 screenshots for
# visual comparison against the upstream references at
# /tmp/mjun-ref/metrics_*.svg.
#
# Usage:
#   bash scripts/capture-mjun-references.sh
#
# Output:
#   specs/011-plugin-rendering-parity/plugins/screenshots/mjun-<label>-ours.png
#   docs/.tmp/mjun-render/metrics_<label>.svg

set -uo pipefail

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "FATAL: GITHUB_TOKEN env var unset" >&2
  exit 1
fi
if ! docker image inspect github-metrics:local >/dev/null 2>&1; then
  echo "FATAL: docker image github-metrics:local not found" >&2
  exit 1
fi

CHROME_BIN=""
for cand in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
            "/Applications/Chromium.app/Contents/MacOS/Chromium" \
            "$(command -v chromium 2>/dev/null || true)" \
            "$(command -v google-chrome 2>/dev/null || true)"; do
  if [[ -n "$cand" && -x "$cand" ]]; then
    CHROME_BIN="$cand"
    break
  fi
done
if [[ -z "$CHROME_BIN" ]]; then
  echo "FATAL: chromium not found" >&2
  exit 1
fi

OUTDIR="specs/011-plugin-rendering-parity/plugins/screenshots"
SVGDIR="docs/.tmp/mjun-render"
mkdir -p "$OUTDIR" "$SVGDIR"

render_and_capture() {
  local label="$1"; shift
  local svg_out="${SVGDIR}/metrics_${label}.svg"
  local png_out="${OUTDIR}/mjun-${label}-ours.png"

  echo "=> ${label}"
  if ! docker run --rm \
      -e GITHUB_TOKEN \
      -v "$(pwd)/${SVGDIR}:/out" \
      github-metrics:local \
      --user mjun0812 \
      --token-env GITHUB_TOKEN \
      --template classic \
      --output svg \
      --filename "/out/metrics_${label}.svg" \
      --dryrun \
      "$@" >/dev/null 2>"/tmp/cap-mjun-${label}.log"
  then
    echo "   FAIL docker render"
    return 0
  fi
  if [[ ! -s "$svg_out" ]]; then
    echo "   FAIL no output"
    return 0
  fi
  echo "   svg: $(wc -c <"$svg_out" | tr -d ' ') bytes"

  local wrapper="${SVGDIR}/${label}.html"
  cat >"$wrapper" <<HTML
<!DOCTYPE html><html><head><meta charset="utf-8"><title>${label}</title>
<style>body{margin:0;padding:8px;font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#f6f8fa}
img{display:block;max-width:704px;box-shadow:0 0 0 1px #d0d7de}</style></head>
<body><img src="file://$(pwd)/${svg_out}" alt="${label}"></body></html>
HTML

  "$CHROME_BIN" \
    --headless=new --disable-gpu --no-sandbox --hide-scrollbars \
    --window-size=720,800 --virtual-time-budget=2000 \
    --screenshot="$png_out" \
    "file://$(pwd)/${wrapper}" >/dev/null 2>"/tmp/cap-mjun-${label}-chrome.log" || true
  if [[ -s "$png_out" ]]; then
    echo "   png: $(wc -c <"$png_out" | tr -d ' ') bytes"
  fi
}

echo "=== mjun0812 plugin renders ==="

render_and_capture languages \
  --plugin plugin_languages=yes \
  --plugin plugin_languages_sections=most-used,recently-used \
  --plugin plugin_languages_details=bytes-size,percentage,lines \
  --plugin plugin_languages_recent_load=500 \
  --plugin plugin_languages_recent_days=30

render_and_capture isocalendar \
  --plugin plugin_isocalendar=yes \
  --plugin plugin_isocalendar_duration=full-year

render_and_capture topics \
  --plugin plugin_topics=yes \
  --plugin plugin_topics_limit=15

render_and_capture topics_icons \
  --plugin plugin_topics=yes \
  --plugin plugin_topics_limit=15 \
  --plugin plugin_topics_mode=icons

render_and_capture calendar \
  --plugin plugin_calendar=yes \
  --plugin plugin_calendar_limit=3

render_and_capture sponsors \
  --plugin plugin_sponsors=yes \
  --plugin plugin_sponsors_sections=goal,about,list \
  --plugin plugin_sponsors_past=yes

echo
echo "Done. PNGs under ${OUTDIR}/mjun-*-ours.png"
