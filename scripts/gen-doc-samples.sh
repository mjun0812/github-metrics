#!/usr/bin/env bash
# scripts/gen-doc-samples.sh — generate the docs/examples/ sample SVG
# set for the README plugins gallery + per-plugin doc pages.
#
# Runs the 19 adopted plugins (mjun0812 user by default) + 9 sub-mode
# variants (achievements compact + languages recent/indepth/details +
# isocalendar full-year + stargazers graph + notable indepth + habits
# facts + habits charts) + 1 foundational `plugin-base` render + 1
# classic template overview (multi-plugin composite) = 30 user-mode
# logical samples. On top of that the repository-mode section renders
# all 19 adopted plugins against ${REPO} (default
# mjun0812/flash-attention-prebuild-wheels), 2 repo-context variants
# (contributors with contributions stats + people with repo-context
# types), 1 foundational `plugin-base-repo` render, and 1
# `metrics-repository` composite overview = 23 additional repo-mode
# samples. Grand total: 53 logical samples (svg + png each).
# The default `plugin-languages` sample is emitted via a one-off
# `render_one` call (not the PLUGINS loop) so it can pass
# `plugin_languages_details=bytes-size,percentage` alongside the slug
# toggle.
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
#   - GITHUB_TOKEN exported (classic PAT with at minimum read:user + repo;
#     admin on ${REPO} is required for the traffic-repo sample to fetch
#     real views/clones)
#   - github-metrics:local docker image present
#       (build with: docker build -f deploy/Dockerfile -t github-metrics:local .)
#
# Env overrides:
#   METRICS_DOC_USER   default: mjun0812
#   METRICS_DOC_REPO   default: flash-attention-prebuild-wheels
#                      (repository-mode samples render against
#                       ${METRICS_DOC_USER}/${METRICS_DOC_REPO})
#   METRICS_DOC_IMAGE  default: github-metrics:local
#   METRICS_DOC_OUTDIR default: docs/examples

set -uo pipefail

USER="${METRICS_DOC_USER:-mjun0812}"
REPO="${METRICS_DOC_REPO:-flash-attention-prebuild-wheels}"
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
  # User-mode render: `--user ${USER}` is injected automatically. The
  # caller passes `--template classic` (or nothing — classic is the
  # default) and any `--plugin key=value` overrides. Renders BOTH .svg
  # and .png variants under OUTDIR. SVG is piped through
  # normalize-svg-stream for idempotency; PNG is binary deterministic
  # so the docker output is moved directly.
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

render_one_repo() {
  # render_one_repo <basename-without-ext> <extra-args...>
  # Repository-mode render: `--user ${USER} --repo ${REPO} --template
  # repository` are injected automatically. The caller passes
  # `--plugin key=value` overrides (typically `--plugin base=` to
  # suppress the default base sections + `--plugin plugin_<slug>=yes`
  # to enable a single plugin). Output handling is identical to
  # render_one (svg + png, normalize-svg-stream for svg).
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
        --repo "$REPO" \
        --template repository \
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
echo "== 19 per-plugin repository-mode renders =="
# Every adopted plugin is rendered once more under
# `--template repository --repo ${REPO}`.
#
# IMPORTANT — what the repository template actually renders:
# The repository template's partial order is hard-coded in
# `assets/templates/repository/partials/_.json`. base.runRepository
# pre-populates the chrome (header / introduction / languages /
# activity / contributors) before any plugin partial runs, so those
# sections appear in every sample regardless of the `--plugin X=yes`
# toggles. Empirically, on ${REPO}, only TWO plugin toggles produce
# a visible delta from `plugin-base-repo`:
#   - `plugin_people=yes` (adds the `<section data-section="people">`
#     block with default stargazers + watchers)
#   - `plugin_contributors_contributions=yes` (extends the chrome's
#     contributors rows with adds/dels columns)
# All other 17 adopted plugins toggled in repo mode produce SVG
# byte-identical to `plugin-base-repo`. This is upstream parity (the
# upstream `metrics.repository.svg` shows the same partial set) — not
# a Go-side regression. The per-plugin samples are kept anyway so the
# docs gallery has a 1:1 mapping with the user-mode set; the
# meaningful repo-mode deltas live in the variant + composite samples
# below.
PLUGINS_REPO=(
  achievements activity calendar contributors habits isocalendar
  languages notable people projects reactions repositories sponsors
  sponsorships stargazers starlists stars topics traffic
)
for slug in "${PLUGINS_REPO[@]}"; do
  render_one_repo "plugin-${slug}-repo" \
    --plugin "base=" \
    --plugin "plugin_${slug}=yes"
done

echo
echo "== repository-mode sub-mode variants =="
# `contributors.contributions` surfaces per-contributor adds/deletes
# via the REST /stats/contributors endpoint (cf. #393, #424). The
# plain `plugin-contributors-repo` sample already covers commit
# counts; this variant demonstrates the additions/deletions columns.
render_one_repo "plugin-contributors-repo-contributions" \
  --plugin "base=" \
  --plugin "plugin_contributors=yes" \
  --plugin "plugin_contributors_contributions=yes"

# `people.repository`: in repository mode the default people types
# auto-switch to stargazers + watchers (see
# internal/plugins/people/people.go::Run). This variant pins the
# full repo-context set (stargazers + watchers + contributors) so
# every supported repo-mode type renders in one card.
render_one_repo "plugin-people-repo-types" \
  --plugin "base=" \
  --plugin "plugin_people=yes" \
  --plugin "plugin_people_types=stargazers,watchers,contributors"

echo
echo "== repository foundational base render =="
# `plugin-base-repo` renders the repository template chrome on its
# own (no plugin toggles). It is the canonical reference for the
# "what does the repository template look like with no plugin
# partials triggered" case — the 12 adopted plugins whose slug is
# absent from `assets/templates/repository/partials/_.json`
# (achievements / calendar / habits / isocalendar / notable /
# reactions / repositories / sponsorships / stars / starlists /
# topics / traffic) all produce a SVG that is byte-identical to
# this sample when rendered under `--template repository`. See
# docs/comparison.md "repository mode サンプル一覧" for the parity
# table.
render_one_repo "plugin-base-repo"

echo
echo "== repository template overview (multi-plugin composite) =="
# `metrics-repository` mirrors the role of `metrics-classic` for the
# repository template: a single composite card showing every partial
# whose toggle has visible effect on ${REPO}.
#
# Plugin selection rationale:
#   - Included: `plugin_people` (adds the people section with 3
#     repo-context types) and `plugin_contributors_contributions`
#     (extends the chrome contributors rows with adds/dels columns).
#     The other toggles (languages/activity/stargazers) are passed for
#     documentation intent — they are no-ops on top of the chrome but
#     make the recipe self-explanatory.
#   - Excluded: every toggle that does not change the output on this
#     sample repo (see the per-plugin comment above). Including them
#     would not affect the rendered bytes.
render_one_repo "metrics-repository" \
  --plugin "plugin_languages=yes" \
  --plugin "plugin_languages_details=bytes-size,percentage" \
  --plugin "plugin_contributors=yes" \
  --plugin "plugin_contributors_contributions=yes" \
  --plugin "plugin_people=yes" \
  --plugin "plugin_people_types=stargazers,watchers,contributors" \
  --plugin "plugin_stargazers=yes" \
  --plugin "plugin_stargazers_charts_type=graph" \
  --plugin "plugin_activity=yes"

echo
echo "== Summary =="
# 2 formats (svg + png) per logical sample.
# PLUGINS holds 18 slugs (languages is rendered as a one-off so it can
# pass `plugin_languages_details=bytes-size,percentage`); +3 languages
# entries (default, recent, indepth) +7 parity variants
# (achievements.compact, notable.indepth, habits.facts, habits.charts,
#  languages.details, isocalendar.fullyear, stargazers.graph)
# +1 foundational base render +1 classic template overview composite.
# Repository mode adds 19 per-plugin repo renders + 2 repo variants
# (contributors.contributions, people.repo-context types) + 1
# foundational `plugin-base-repo` render + 1 `metrics-repository`
# overview composite.
TOTAL=$(( (${#PLUGINS[@]} + 3 + 7 + 1 + 1 + ${#PLUGINS_REPO[@]} + 2 + 1 + 1) * 2 ))
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
