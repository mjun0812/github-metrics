#!/usr/bin/env bash
# scripts/gen-doc-samples.sh — generate the docs/examples/ sample SVG
# set for the README plugins gallery + per-plugin doc pages.
#
# Runs the 19 adopted plugins (mjun0812 user by default) + 9 sub-mode
# variants (achievements compact + languages recent/indepth/details +
# isocalendar full-year + stargazers graph + notable indepth + habits
# facts + habits charts) + 1 foundational `base` render + 1 classic
# template overview (multi-plugin composite) = 30 logical samples
# (svg + png each). The default `plugin-languages` sample is emitted
# via a one-off `render_one` call (not the PLUGINS loop) so it can
# pass `plugin_languages_details=bytes-size,percentage` alongside the
# slug toggle.
#
# `core` is excluded from the foundational set: it implements
# configuration parsing and the parallel plugin runner and has no
# standalone visual output — see docs/plugins/core.md.
#
# Pipeline per file:
#   1. docker run github-metrics:local → writes /out/<file>.svg
#   2. go run ./internal/tools/normalize-svg-stream < tmp → mask dynamic
#   3. mv into docs/examples/ atomically
#
# Pre-conditions:
#   - GITHUB_TOKEN exported (classic PAT with at minimum read:user + repo)
#   - github-metrics:local docker image present
#       (build with: docker build -f deploy/Dockerfile -t github-metrics:local .)
#
# Env overrides:
#   METRICS_DOC_USER   default: mjun0812
#   METRICS_DOC_IMAGE  default: github-metrics:local
#   METRICS_DOC_OUTDIR default: docs/examples

set -uo pipefail

USER="${METRICS_DOC_USER:-mjun0812}"
IMAGE="${METRICS_DOC_IMAGE:-github-metrics:local}"
OUTDIR="${METRICS_DOC_OUTDIR:-docs/examples}"

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
mkdir -p "$OUTDIR"

# Work dir for volume mount + intermediate files.
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

# Adopted plugin slugs (mirror of tests/compliance/compliance_test.go
# adoptedM4Plugins minus base/core). `languages` is intentionally
# omitted here and rendered via a one-off `render_one` call below so
# the default sample can also carry the
# `plugin_languages_details=bytes-size,percentage` override (the
# generic loop only emits `plugin_<slug>=yes`).
PLUGINS=(
  achievements activity calendar contributors habits isocalendar
  notable people projects reactions repositories sponsors
  sponsorships stargazers starlists stars topics traffic
)

# Track failures so the script can exit non-zero at the end.
FAILURES=()

render_one() {
  # render_one <basename-without-ext> <extra-args...>
  # Renders BOTH .svg and .png variants under OUTDIR. SVG is piped
  # through normalize-svg-stream for idempotency; PNG is binary
  # deterministic so the docker output is moved directly.
  local base="$1"; shift

  echo "  → ${base}"
  local fmt
  for fmt in svg png; do
    local outfile="${base}.${fmt}"
    local tmp_raw="${WORKDIR}/${outfile}"
    local final="${OUTDIR}/${outfile}"

    if ! docker run --rm \
        -e GITHUB_TOKEN \
        -v "${WORKDIR}:/out" \
        "$IMAGE" \
        --user "$USER" \
        --token-env GITHUB_TOKEN \
        --output "$fmt" \
        --filename "/out/${outfile}" \
        --dryrun \
        "$@" >/dev/null 2>"${WORKDIR}/${outfile}.log"
    then
      echo "    FAIL ${fmt} (see ${WORKDIR}/${outfile}.log)"
      FAILURES+=("$outfile")
      continue
    fi
    if [[ ! -s "$tmp_raw" ]]; then
      echo "    FAIL ${fmt} (no output file)"
      FAILURES+=("$outfile")
      continue
    fi
    if [[ "$fmt" == "svg" ]]; then
      local tmp_norm="${WORKDIR}/${outfile}.norm"
      if ! go run ./internal/tools/normalize-svg-stream <"$tmp_raw" >"$tmp_norm" 2>"${WORKDIR}/${outfile}.norm.log"; then
        echo "    FAIL ${fmt} (normalize)"
        FAILURES+=("$outfile")
        continue
      fi
      mv "$tmp_norm" "$final"
    else
      mv "$tmp_raw" "$final"
    fi
    echo "    OK ${fmt} ($(wc -c <"$final" | tr -d ' ') bytes)"
  done
}

echo "== 18 per-plugin single-panel renders (languages handled separately) =="
for slug in "${PLUGINS[@]}"; do
  render_one "plugin-${slug}" \
    --template classic \
    --plugin "base=" \
    --plugin "plugin_${slug}=yes"
done

echo
echo "== languages default + 2 sub-mode variants =="
# All three carry `plugin_languages_details=bytes-size,percentage` so
# the rendered cards show per-language bytes and percentage next to
# the colored bar. The dedicated `plugin-languages-details` variant
# below adds `lines` on top for the "full numeric column" demo.
render_one "plugin-languages" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_details=bytes-size,percentage"

render_one "plugin-languages-recent" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_sections=recently-used" \
  --plugin "plugin_languages_recent_load=300" \
  --plugin "plugin_languages_recent_days=30" \
  --plugin "plugin_languages_details=bytes-size,percentage"

render_one "plugin-languages-indepth" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_indepth=yes" \
  --plugin "plugin_languages_analysis_timeout=30" \
  --plugin "plugin_languages_details=bytes-size,percentage"

echo
echo "== other upstream-parity sub-mode variants =="
# Only variants where the Go implementation produces output that visibly
# differs from the plain plugin (for the sample user) are kept here.
# Intentionally omitted:
#   - calendar.full (plugin_calendar_limit=0), repositories.pinned
#       Go accepts the option but the output is byte-identical to the plain
#       plugin for this sample user's data.
#   - topics.icons, starlists.languages, sponsors.full
#       sample user has no data, so the card renders empty.
#   - contributors.contributions, people.repository
#       repository-template territory; not rendered in user-mode samples.
#   - stargazers.worldmap (Google Maps API, backlog),
#     stargazers.chartist (deprecated alias, byte-identical to graph).
render_one "plugin-achievements-compact" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_achievements=yes" \
  --plugin "plugin_achievements_display=compact"

render_one "plugin-notable-indepth" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_notable=yes" \
  --plugin "plugin_notable_indepth=yes" \
  --plugin "plugin_notable_repositories=yes"

render_one "plugin-languages-details" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_details=bytes-size,percentage,lines"

render_one "plugin-isocalendar-fullyear" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_isocalendar=yes" \
  --plugin "plugin_isocalendar_duration=full-year"

render_one "plugin-stargazers-graph" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_stargazers=yes" \
  --plugin "plugin_stargazers_charts_type=graph"

render_one "plugin-habits-facts" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_habits=yes" \
  --plugin "plugin_habits_facts=yes" \
  --plugin "plugin_habits_charts=no"

render_one "plugin-habits-charts" \
  --template classic \
  --plugin "base=" \
  --plugin "plugin_habits=yes" \
  --plugin "plugin_habits_facts=no" \
  --plugin "plugin_habits_charts=yes"

echo
echo "== classic template overview (multi-plugin composite) =="
# `metrics-classic` is the cross-plugin overview sample: the upstream
# `metrics.classic.svg` reference (docs/original_examples/) shows what
# a user gets when they enable a typical README plugin set on top of
# the classic template, but the per-plugin samples in this directory
# only convey each plugin in isolation. This render combines the
# plugins that produce non-empty output for `mjun0812` so docs viewers
# can preview the assembled card.
#
# Plugin selection rationale:
#   - Included: isocalendar, calendar, languages, activity, achievements,
#     notable, repositories, habits, stars, reactions, stargazers,
#     traffic. These all have real data on mjun0812 and mirror
#     upstream's typical classic showcase.
#   - Excluded (empty data on the sample user): contributors, projects,
#     sponsors, sponsorships, starlists, topics — see docs/comparison.md
#     for the per-plugin empty-card note.
#   - Excluded (oversized): people. The `plugin-people.svg` sample is
#     5.8 MB on its own and would dominate the composite.
#   - Excluded (repo-template territory or deprecated/backlog): all the
#     variants already filtered out of the per-plugin sub-mode section
#     above.
render_one "metrics-classic" \
  --template classic \
  --plugin "base=header, activity, community, repositories, metadata" \
  --plugin "plugin_isocalendar=yes" \
  --plugin "plugin_calendar=yes" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_details=bytes-size,percentage" \
  --plugin "plugin_activity=yes" \
  --plugin "plugin_achievements=yes" \
  --plugin "plugin_achievements_display=compact" \
  --plugin "plugin_notable=yes" \
  --plugin "plugin_repositories=yes" \
  --plugin "plugin_habits=yes" \
  --plugin "plugin_stars=yes" \
  --plugin "plugin_reactions=yes" \
  --plugin "plugin_stargazers=yes" \
  --plugin "plugin_traffic=yes"

echo
echo "== foundational base render =="
# `base` is the upstream-special plugin that draws the user/org header
# card every other plugin sits on top of. We render it on its own so
# docs/plugins/base.md has a visual reference. Default `base` value
# is "header, activity, community, repositories, metadata"; passing
# no plugin_<name>=yes toggle means only the base sections render.
#
# `core` is intentionally absent here — it has no standalone visual
# output (configuration + parallel runner only). See
# docs/plugins/core.md.
render_one "plugin-base" \
  --template classic

echo
echo "== Summary =="
# 2 formats (svg + png) per logical sample.
# PLUGINS holds 18 slugs (languages is rendered as a one-off so it can
# pass `plugin_languages_details=bytes-size,percentage`); +3 languages
# entries (default, recent, indepth) +7 parity variants
# (achievements.compact, notable.indepth, habits.facts, habits.charts,
#  languages.details, isocalendar.fullyear, stargazers.graph)
# +1 foundational base render +1 classic template overview composite.
TOTAL=$(( (${#PLUGINS[@]} + 3 + 7 + 1 + 1) * 2 ))
OK=$((TOTAL - ${#FAILURES[@]}))
echo "  OK:   ${OK}/${TOTAL}"
if (( ${#FAILURES[@]} > 0 )); then
  echo "  FAIL: ${#FAILURES[@]}"
  for f in "${FAILURES[@]}"; do
    echo "    - $f"
  done
  echo
  echo "Per-failure logs preserved in: $WORKDIR"
  trap - EXIT  # keep workdir for inspection
  exit 1
fi
echo "All ${TOTAL} sample files (svg + png) generated under ${OUTDIR}/"
