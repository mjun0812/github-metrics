#!/usr/bin/env bash
#
# migrate-from-lowlighter.sh -- rewrite `uses: lowlighter/metrics@<ref>` lines
# in GitHub Actions workflow files to point at `mjun0812/github-metrics@<ref>`.
#
# Optionally strips `with:` keys that gate plugins the Go port has not
# adopted (see docs/migration-to-go.md sections 3.3, 3.4, 3.5). Stripping
# is off by default so the base migration stays purely mechanical -- the
# Go port silently no-ops unknown `plugin_*` keys anyway.
#
# Usage:
#   scripts/migrate-from-lowlighter.sh [OPTIONS] [PATH]
#
# See `--help` for full options.
#

set -euo pipefail

TARGET_REPO="mjun0812/github-metrics"
DEFAULT_PIN="v1"
DEFAULT_PATH=".github/workflows"

# Plugins unported to the Go port. Sourced verbatim from
# docs/migration-to-go.md sections 3.3, 3.4 and 3.5. Keep in sync when
# that doc changes. Order does not matter -- the regex is alternation.
UNPORTED_SLUGS=(
	# 3.3 GitHub data plugins
	lines gists followup discussions skyline support
	# 3.4 community extension plugins
	code introduction licenses
	# 3.5 social / external API plugins
	anilist chess crypto fortune leetcode music nightscout pagespeed poopmap
	posts rss screenshot splatoon stackoverflow steam stock tweets wakatime
	16personalities
)

usage() {
	cat <<EOF
Usage: $(basename "$0") [OPTIONS] [PATH]

Rewrite \`uses: lowlighter/metrics@<ref>\` lines in GitHub Actions workflow
files to \`uses: ${TARGET_REPO}@<ref>\`. Preserves leading indentation and
trailing comments. Optionally strips \`with:\` keys that gate unported
plugins (see docs/migration-to-go.md sections 3.3-3.5).

Arguments:
  PATH                A workflow file, or a directory. Default:
                      \`${DEFAULT_PATH}\`. When PATH is a directory, only
                      \`*.yml\` and \`*.yaml\` files are considered.

Options:
  --pin <ref>         Replacement ref (default: \`${DEFAULT_PIN}\`).
                      Accepts both \`v1.0.0\` and \`@v1.0.0\`.
  --dry-run           Print a unified diff of the changes and exit
                      without writing anything.
  --no-backup         Do not create a \`<file>.bak\` backup before
                      overwriting.
  --strip-unported    Also remove \`with:\` keys that gate plugins the Go
                      port has not adopted. Removes both
                      \`plugin_<slug>:\` and \`plugin_<slug>_<subkey>:\`
                      lines. Off by default.
  -h, --help          Show this help and exit.

Exit codes:
  0    Success (including "nothing to migrate").
  >0   PATH does not exist, argument error, or I/O error.
EOF
}

PIN="${DEFAULT_PIN}"
DRY_RUN=0
NO_BACKUP=0
STRIP_UNPORTED=0
TARGET_PATH=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--pin)
		if [[ $# -lt 2 ]]; then
			echo "error: --pin requires an argument" >&2
			exit 2
		fi
		PIN="$2"
		shift 2
		;;
	--pin=*)
		PIN="${1#--pin=}"
		shift
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--no-backup)
		NO_BACKUP=1
		shift
		;;
	--strip-unported)
		STRIP_UNPORTED=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	--)
		shift
		break
		;;
	-*)
		echo "error: unknown option: $1" >&2
		usage >&2
		exit 2
		;;
	*)
		if [[ -n "${TARGET_PATH}" ]]; then
			echo "error: only one positional argument is allowed (got '${TARGET_PATH}' and '$1')" >&2
			exit 2
		fi
		TARGET_PATH="$1"
		shift
		;;
	esac
done

if [[ -n "${1:-}" ]]; then
	if [[ -n "${TARGET_PATH}" ]]; then
		echo "error: only one positional argument is allowed" >&2
		exit 2
	fi
	TARGET_PATH="$1"
fi

if [[ -z "${TARGET_PATH}" ]]; then
	TARGET_PATH="${DEFAULT_PATH}"
fi

# Normalize `--pin` so we accept both `v1.0.0` and `@v1.0.0`.
PIN="${PIN#@}"
if [[ -z "${PIN}" ]]; then
	echo "error: --pin cannot be empty" >&2
	exit 2
fi

REPLACEMENT="${TARGET_REPO}@${PIN}"

if [[ ! -e "${TARGET_PATH}" ]]; then
	echo "error: path does not exist: ${TARGET_PATH}" >&2
	exit 1
fi

# Collect target files. A directory expands to *.yml + *.yaml only; a
# single file is used as-is (no extension filter, matching the issue's
# "workflow file OR a directory" contract).
files=()
if [[ -d "${TARGET_PATH}" ]]; then
	while IFS= read -r -d '' f; do
		files+=("$f")
	done < <(find "${TARGET_PATH}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0 | LC_ALL=C sort -z)
elif [[ -f "${TARGET_PATH}" ]]; then
	files=("${TARGET_PATH}")
else
	echo "error: not a regular file or directory: ${TARGET_PATH}" >&2
	exit 1
fi

# Build regexes.
# uses_re captures:
#   [1] the leading portion up to (and including) the opening quote slot
#       -- indent, optional list dash, `uses:` and its trailing space
#   [3] optional opening quote (" or ')
#   [4] optional closing quote (must mirror opening for well-formed YAML,
#       but we do not enforce that here)
#   [5] optional trailing whitespace + comment
uses_re='^([[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*)(["'"'"']?)lowlighter/metrics@[^[:space:]"'"'"'#]+(["'"'"']?)([[:space:]].*)?$'

strip_re=""
if [[ ${STRIP_UNPORTED} -eq 1 ]]; then
	slug_alt="$(
		IFS='|'
		printf '%s' "${UNPORTED_SLUGS[*]}"
	)"
	# Matches `plugin_<slug>:` OR `plugin_<slug>_<subkey>:` at any indent.
	# The `_<subkey>` group is `_` followed by one or more identifier chars;
	# this stops `plugin_lines:` from also matching `plugin_linesoftext:`.
	strip_re="^[[:space:]]*plugin_(${slug_alt})(_[A-Za-z0-9_]+)?:"
fi

total_files=0
total_files_rewritten=0
total_uses_rewrites=0
total_keys_stripped=0

for file in ${files[@]+"${files[@]}"}; do
	total_files=$((total_files + 1))

	if [[ ! -r "${file}" ]]; then
		echo "error: cannot read: ${file}" >&2
		exit 1
	fi

	original="$(cat -- "${file}")"

	new_lines=()
	file_uses_rewrites=0
	file_keys_stripped=0

	while IFS= read -r line || [[ -n "${line}" ]]; do
		if [[ "${line}" =~ ${uses_re} ]]; then
			prefix="${BASH_REMATCH[1]}"
			qopen="${BASH_REMATCH[3]}"
			qclose="${BASH_REMATCH[4]}"
			trailing="${BASH_REMATCH[5]-}"
			new_lines+=("${prefix}${qopen}${REPLACEMENT}${qclose}${trailing}")
			file_uses_rewrites=$((file_uses_rewrites + 1))
			continue
		fi
		if [[ -n "${strip_re}" && "${line}" =~ ${strip_re} ]]; then
			file_keys_stripped=$((file_keys_stripped + 1))
			continue
		fi
		new_lines+=("${line}")
	done <<<"${original}"

	# Re-assemble. `printf '%s\n'` always ends with a newline, matching how
	# `cat` read the file (which trims exactly the final newline if present).
	# Guard against the "all lines stripped" case: bash 3.2 (macOS default)
	# errors on `${arr[@]}` for an empty array under `set -u`.
	new_content="$(printf '%s\n' ${new_lines[@]+"${new_lines[@]}"})"

	total_uses_rewrites=$((total_uses_rewrites + file_uses_rewrites))
	total_keys_stripped=$((total_keys_stripped + file_keys_stripped))

	if [[ "${original}" == "${new_content}" ]]; then
		continue
	fi

	total_files_rewritten=$((total_files_rewritten + 1))

	if [[ ${DRY_RUN} -eq 1 ]]; then
		# `diff -u` returns 1 when files differ -- we expect that here, so
		# swallow the exit status. `--label` gives readable a/b headers.
		diff -u --label "a/${file}" --label "b/${file}" \
			<(printf '%s\n' "${original}") \
			<(printf '%s\n' "${new_content}") || true
		continue
	fi

	if [[ ${NO_BACKUP} -eq 0 ]]; then
		cp -- "${file}" "${file}.bak"
	fi

	# Write via a sibling temp file + mv so we do not truncate the target
	# if the write is interrupted mid-stream.
	tmp="$(mktemp -- "${file}.tmp.XXXXXX")"
	printf '%s\n' "${new_content}" >"${tmp}"
	mv -- "${tmp}" "${file}"
done

if [[ ${total_uses_rewrites} -eq 0 && ${total_keys_stripped} -eq 0 ]]; then
	echo "no lowlighter/metrics uses: found -- nothing to migrate" >&2
	exit 0
fi

printf '%d files scanned, %d rewritten, %d keys stripped\n' \
	"${total_files}" "${total_files_rewritten}" "${total_keys_stripped}" >&2
