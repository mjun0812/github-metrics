#!/usr/bin/env bash
# scripts/gen-doc-samples.sh — generate the docs/examples/ sample SVG
# set for the README plugins gallery + per-plugin doc pages.
#
# Runs the 19 adopted plugins (mjun0812 user by default) + 10 sub-mode
# variants (achievements compact + languages recent/indepth/details +
# isocalendar full-year + calendar full + stargazers graph + notable
# indepth + habits facts + habits charts) + 1 foundational `plugin-base` render + 1
# classic template sample (`metrics-classic`, base-only upstream-parity
# render — see issue #463) = 30
# user-mode
# logical samples. (`contributors` is NOT rendered in user mode — it is a
# repository-mode-only plugin and would emit an empty card; see issue #448
# and the PLUGINS array note below.) On top of that the repository-mode section renders
# the foundational `plugin-base-repo` chrome, the one repo-context
# plugin that produces a unique panel on its own (`plugin-people-repo`),
# 2 repo-context variants (`plugin-contributors-repo-contributions`
# adds the per-contributor adds/dels columns, `plugin-people-repo-types`
# pins all three people types) and 1 `metrics-repository` composite
# overview = 5 logical repo-mode samples. The 14 user-mode-only plugins
# (achievements/activity/calendar/habits/isocalendar/notable/projects/
# reactions/repositories/sponsors/sponsorships/starlists/stars/topics)
# and the 4 plugins whose repo-mode panel is byte-identical to
# `plugin-base-repo` for ${REPO} (languages/contributors/stargazers/
# traffic) are intentionally NOT rendered as repository-mode samples —
# they would just duplicate `plugin-base-repo`. The user-mode-only
# plugins also emit a WARN log when invoked under `--template
# repository` (see internal/plugins/modegate.go). Grand total: 35
# logical samples (svg + png each).
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
# NOTE: `contributors` is intentionally NOT in this user-mode list. It is a
# repository-mode-only plugin (internal/plugins/contributors/contributors.go
# short-circuits to Skipped when RepoRef() == nil, see
# internal/plugins/modegate.go::RequireRepoMode). Rendering it here produced an
# empty user-mode card (height=8, ~1729 bytes) that was wrongly published as the
# canonical `plugin-contributors.svg` sample (issue #448). The representative
# contributors sample is the repo-mode `plugin-contributors-repo-contributions`
# render below.
PLUGINS=(
	achievements activity calendar habits isocalendar
	notable people projects repositories sponsors
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
	local base="$1"
	shift

	echo "  → ${base}"
	local fmt
	for fmt in svg png; do
		local outfile="${base}.${fmt}"
		local tmp_raw="${WORKDIR}/${outfile}"
		local final="${OUTDIR}/${outfile}"

		local attempt ok=0
		for attempt in 1 2 3; do
			rm -f "$tmp_raw"
			if docker run --rm \
				-e GITHUB_TOKEN \
				-e HTTP_PROXY="" \
				-e HTTPS_PROXY="" \
				-e http_proxy="" \
				-e https_proxy="" \
				-e NO_PROXY="*" \
				-e no_proxy="*" \
				-v "${WORKDIR}:/out" \
				"$IMAGE" \
				--user "$USER" \
				--token-env GITHUB_TOKEN \
				--output "$fmt" \
				--filename "/out/${outfile}" \
				--dryrun \
				"$@" >/dev/null 2>"${WORKDIR}/${outfile}.log" && [[ -s "$tmp_raw" ]]; then
				ok=1
				break
			fi
			[[ "$attempt" -lt 3 ]] && echo "    retry ${attempt}/3 ${fmt} ..."
		done
		if [[ "$ok" -eq 0 ]]; then
			echo "    FAIL ${fmt} (see ${WORKDIR}/${outfile}.log)"
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
	local base="$1"
	shift

	echo "  → ${base}"
	local fmt
	for fmt in svg png; do
		local outfile="${base}.${fmt}"
		local tmp_raw="${WORKDIR}/${outfile}"
		local final="${OUTDIR}/${outfile}"

		local attempt ok=0
		for attempt in 1 2 3; do
			rm -f "$tmp_raw"
			if docker run --rm \
				-e GITHUB_TOKEN \
				-e HTTP_PROXY="" \
				-e HTTPS_PROXY="" \
				-e http_proxy="" \
				-e https_proxy="" \
				-e NO_PROXY="*" \
				-e no_proxy="*" \
				-v "${WORKDIR}:/out" \
				"$IMAGE" \
				--user "$USER" \
				--repo "$REPO" \
				--template repository \
				--token-env GITHUB_TOKEN \
				--output "$fmt" \
				--filename "/out/${outfile}" \
				--dryrun \
				"$@" >/dev/null 2>"${WORKDIR}/${outfile}.log" && [[ -s "$tmp_raw" ]]; then
				ok=1
				break
			fi
			[[ "$attempt" -lt 3 ]] && echo "    retry ${attempt}/3 ${fmt} ..."
		done
		if [[ "$ok" -eq 0 ]]; then
			echo "    FAIL ${fmt} (see ${WORKDIR}/${outfile}.log)"
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

echo "== 16 per-plugin single-panel renders (languages handled separately) =="
for slug in "${PLUGINS[@]}"; do
	base_arg="base="
	if [[ "${slug}" == "traffic" ]]; then
		base_arg="base=repositories"
	fi
	render_one "plugin-${slug}" \
		--template classic \
		--plugin "${base_arg}" \
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

# `reactions` is rendered separately so the sample carries the
# `plugin_reactions_details=percentage` override, producing the
# upstream 8-emoji gauge panel with per-emoji percentages (matching
# docs/reference_examples/metrics.plugin.reactions.svg). The default
# `plugin_reactions_limit` is 200 (see metadata.yml), so the header
# reads "from last <=200 comments".
render_one "plugin-reactions" \
	--template classic \
	--plugin "base=" \
	--plugin "plugin_reactions=yes" \
	--plugin "plugin_reactions_details=percentage"

render_one "plugin-isocalendar-fullyear" \
	--template classic \
	--plugin "base=" \
	--plugin "plugin_isocalendar=yes" \
	--plugin "plugin_isocalendar_duration=full-year"

render_one "plugin-calendar-full" \
	--template classic \
	--plugin "base=" \
	--plugin "plugin_calendar=yes" \
	--plugin "plugin_calendar_limit=0"

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
echo "== classic template overview (upstream-parity base render) =="
# `metrics-classic` is the canonical classic-template sample. It renders
# the classic template with NO extra `plugin_<name>=yes` toggles so the
# output mirrors upstream `lowlighter/metrics` `metrics.classic.svg`,
# which is itself essentially the base header card (the base sections
# header / activity / community / repositories / metadata only).
#
# Why no plugin toggles (issue #463): this sample previously composed 12
# adopted plugins (isocalendar / calendar / languages / activity /
# achievements / notable / repositories / habits / stars / reactions /
# stargazers / traffic) into a single render. That produced a ~3200px
# card that did NOT match upstream's ~200px classic output, so the
# "classic 総合" comparison row in docs/comparison.md was misleading.
# Upstream's classic sample is base-only, so this sample is base-only too;
# the per-plugin gallery samples already demonstrate each adopted
# plugin's panel individually.
#
# This naturally renders close to `plugin-base.svg` (both are the classic
# base chrome). That is expected and correct, since upstream classic is
# also effectively base-only.
#
# CSS is minified automatically: the metadata `optimize` default
# ("css, xml") flows through the render pipeline (see
# internal/engine/dispatch.go::optimizeEnabled), matching upstream's
# single-line minified style block.
render_one "metrics-classic" \
	--template classic \
	--plugin "base=header, repositories"

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
echo "== per-plugin repository-mode renders (only plugins with visible delta) =="
# The repository template's partial order is hard-coded in
# `assets/templates/repository/partials/_.json`. base.runRepository
# pre-populates the chrome (header / introduction / languages /
# activity / contributors) before any plugin partial runs, so those
# sections appear in every sample regardless of the `--plugin X=yes`
# toggles. Empirically, on ${REPO}, only ONE plugin toggle produces
# a stand-alone visible delta from `plugin-base-repo`:
#   - `plugin_people=yes` (adds the `<section data-section="people">`
#     block with default stargazers + watchers)
# The other 18 adopted plugins toggled in repo mode produce SVG
# byte-identical to `plugin-base-repo` on this sample repo (verified
# md5sum) and are therefore NOT emitted as separate samples — they
# would just duplicate `plugin-base-repo` and bloat the docs gallery.
# Two of them surface their value only via variants (people-types and
# contributors-contributions, rendered below).
#
# The 14 user-mode-only plugins (achievements / activity / calendar /
# habits / isocalendar / notable / projects / reactions / repositories /
# sponsors / sponsorships / starlists / stars / topics) additionally
# emit a WARN log when invoked under `--template repository`; the
# guard lives in internal/plugins/modegate.go and the per-plugin Run()
# bodies short-circuit to a descriptive SkippedReason instead of
# silently returning empty data.
render_one_repo "plugin-people-repo" \
	--plugin "base=" \
	--plugin "plugin_people=yes"

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
# partials triggered" case. Eighteen of the 19 adopted plugins produce
# a SVG byte-identical to this sample when rendered alone under
# `--template repository` against ${REPO} — either because their slug
# is absent from `assets/templates/repository/partials/_.json`, or
# because their Run() short-circuits via the mode gate in
# internal/plugins/modegate.go (the 14 user-mode-only plugins), or
# because their repo-mode panel happens to render empty on this
# sample repo (languages / contributors / stargazers / traffic).
# Only `plugin_people=yes` produces a stand-alone delta and is
# emitted separately above. See docs/comparison.md "repository mode
# サンプル一覧" for the parity table.
render_one_repo "plugin-base-repo"

echo
echo "== repository template base overview =="
# `metrics-repository` is the apples-to-apples counterpart of upstream's
# `docs/reference_examples/metrics.repository.svg` (issue #464): a plain
# base repository render with NO plugin toggles. The repository
# `base.header` partial alone draws the upstream chrome — repository
# name + Created / Deployed / disk-usage on the left, the contribution
# mini-calendar / Environments on the right — so the section structure
# matches the upstream reference exactly.
#
# History: this sample previously enabled languages/contributors/people/
# stargazers/activity toggles, producing a tall multi-section composite
# that diverged from the upstream `metrics.repository.svg` reference
# (issue #464). Plugin partials are now gated by `plugin_<slug>` in the
# repository template dispatcher (internal/templates/repository/
# repository.go), so a toggle-free render emits the base chrome only.
# The per-plugin repository-mode samples above (plugin-base-repo,
# plugin-people-repo, plugin-contributors-repo-contributions,
# plugin-people-repo-types) still cover each plugin partial individually.
render_one_repo "metrics-repository"

echo
echo "== Summary =="
# 2 formats (svg + png) per logical sample.
# PLUGINS holds 17 slugs (languages is rendered as a one-off so it can
# pass `plugin_languages_details=bytes-size,percentage`); +3 languages
# entries (default, recent, indepth) +7 parity variants
# (achievements.compact, notable.indepth, habits.facts, habits.charts,
#  languages.details, isocalendar.fullyear, stargazers.graph)
# +1 foundational base render +1 classic template overview
# (multi-plugin composite render).
# Repository mode adds:
#   - 1 foundational `plugin-base-repo` render
#   - 1 stand-alone repo-mode plugin sample (`plugin-people-repo`)
#   - 2 repo-context variants (`plugin-contributors-repo-contributions`,
#     `plugin-people-repo-types`)
#   - 1 `metrics-repository` overview composite
# = 5 repo-mode logical samples. The 14 user-mode-only plugins +
# 4 plugins with byte-identical repo output are intentionally NOT
# emitted as repo-mode samples (see "per-plugin repository-mode
# renders" section above for the rationale).
REPO_LOGICAL=5
TOTAL=$(((${#PLUGINS[@]} + 3 + 9 + 1 + 1 + REPO_LOGICAL) * 2))
OK=$((TOTAL - ${#FAILURES[@]}))
echo "  OK:   ${OK}/${TOTAL}"
if ((${#FAILURES[@]} > 0)); then
	echo "  FAIL: ${#FAILURES[@]}"
	for f in "${FAILURES[@]}"; do
		echo "    - $f"
	done
	echo
	echo "Per-failure logs preserved in: $WORKDIR"
	trap - EXIT # keep workdir for inspection
	exit 1
fi
echo "All ${TOTAL} sample files (svg + png) generated under ${OUTDIR}/"
