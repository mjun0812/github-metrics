#!/usr/bin/env bash
#
# migrate-from-lowlighter_test.sh -- smoke test for
# scripts/migrate-from-lowlighter.sh.
#
# Standalone bash test (not wired into `make test`). Exercises the
# script against synthesized workflow files in a scratch directory and
# asserts each option's contract from issue #759. On any assertion
# failure the specific case is printed and the process exits 1.
#

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
MIGRATE="${REPO_ROOT}/scripts/migrate-from-lowlighter.sh"

if [[ ! -x "${MIGRATE}" ]]; then
	echo "FAIL: migration script not found or not executable at ${MIGRATE}" >&2
	exit 1
fi

WORK="$(mktemp -d -t migrate-from-lowlighter-test-XXXXXXXX)"
trap 'rm -rf "${WORK}"' EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# --- Case 1: baseline rewrite + .bak backup -------------------------------
c1="${WORK}/case1.yml"
cat >"${c1}" <<'EOF'
name: metrics
on: [push]
jobs:
  metrics:
    runs-on: ubuntu-latest
    steps:
      - uses: lowlighter/metrics@v3.34
        with:
          user: octocat
EOF
"${MIGRATE}" "${c1}" >/dev/null 2>&1 || fail "case1: script exited non-zero"
grep -q "uses: mjun0812/github-metrics@v1" "${c1}" ||
	fail "case1: rewritten uses: line missing"
grep -q "lowlighter/metrics" "${c1}" &&
	fail "case1: original lowlighter/metrics reference still present"
[[ -f "${c1}.bak" ]] || fail "case1: .bak backup not created"
grep -q "uses: lowlighter/metrics@v3.34" "${c1}.bak" ||
	fail "case1: .bak does not contain the original uses: line"

# --- Case 2: --pin accepts both `v1.0.0` and `@v1.0.0` --------------------
for pin in "v1.0.0" "@v1.0.0"; do
	c2="${WORK}/case2_$(printf '%s' "${pin}" | tr -d '@').yml"
	cat >"${c2}" <<'EOF'
      - uses: lowlighter/metrics@v3.34
EOF
	"${MIGRATE}" --pin "${pin}" --no-backup "${c2}" >/dev/null 2>&1 ||
		fail "case2 (${pin}): script exited non-zero"
	grep -q "uses: mjun0812/github-metrics@v1.0.0" "${c2}" ||
		fail "case2 (${pin}): expected @v1.0.0 in output; got: $(cat "${c2}")"
done

# --- Case 3: --dry-run writes nothing and prints diff to stdout ----------
c3="${WORK}/case3.yml"
cat >"${c3}" <<'EOF'
      - uses: lowlighter/metrics@v3.34
EOF
before_content="$(cat "${c3}")"
# On BSD stat (macOS) `-f %m` gives mtime; on GNU stat, `-c %Y`. Try both.
if before_mtime="$(stat -f %m "${c3}" 2>/dev/null)"; then
	:
elif before_mtime="$(stat -c %Y "${c3}" 2>/dev/null)"; then
	:
else
	fail "case3: neither stat -f %m nor stat -c %Y worked"
fi
# Sleep so any mistaken write is detectable via mtime.
sleep 1
dry_stdout="$("${MIGRATE}" --dry-run "${c3}" 2>/dev/null)" ||
	fail "case3: --dry-run exited non-zero"
after_content="$(cat "${c3}")"
if stat -f %m "${c3}" >/dev/null 2>&1; then
	after_mtime="$(stat -f %m "${c3}")"
else
	after_mtime="$(stat -c %Y "${c3}")"
fi
[[ "${before_content}" == "${after_content}" ]] ||
	fail "case3: file content changed under --dry-run"
[[ "${before_mtime}" == "${after_mtime}" ]] ||
	fail "case3: file mtime changed under --dry-run (${before_mtime} -> ${after_mtime})"
[[ ! -f "${c3}.bak" ]] || fail "case3: .bak created under --dry-run"
printf '%s' "${dry_stdout}" | grep -q '^--- a/' ||
	fail "case3: --dry-run did not print a unified diff to stdout"
printf '%s' "${dry_stdout}" | grep -q 'mjun0812/github-metrics@v1' ||
	fail "case3: --dry-run diff missing replacement string"

# --- Case 4: --no-backup skips .bak --------------------------------------
c4="${WORK}/case4.yml"
cat >"${c4}" <<'EOF'
      - uses: lowlighter/metrics@v3.34
EOF
"${MIGRATE}" --no-backup "${c4}" >/dev/null 2>&1 ||
	fail "case4: script exited non-zero"
[[ ! -f "${c4}.bak" ]] || fail "case4: .bak created despite --no-backup"
grep -q "mjun0812/github-metrics@v1" "${c4}" ||
	fail "case4: file was not rewritten"

# --- Case 5: --strip-unported strips unported plugin keys ----------------
c5="${WORK}/case5.yml"
cat >"${c5}" <<'EOF'
      - uses: lowlighter/metrics@v3.34
        with:
          user: octocat
          plugin_languages: yes
          plugin_anilist: yes
          plugin_anilist_medias: anime
EOF
"${MIGRATE}" --strip-unported --no-backup "${c5}" >/dev/null 2>&1 ||
	fail "case5: script exited non-zero"
grep -q "plugin_languages: yes" "${c5}" ||
	fail "case5: adopted plugin_languages key was removed"
grep -q "plugin_anilist:" "${c5}" &&
	fail "case5: plugin_anilist key was not stripped"
grep -q "plugin_anilist_medias" "${c5}" &&
	fail "case5: plugin_anilist_medias sub-key was not stripped"
grep -q "mjun0812/github-metrics@v1" "${c5}" ||
	fail "case5: uses: line was not rewritten"

# --- Case 6: empty directory prints "nothing to migrate" and exits 0 -----
empty="${WORK}/empty_dir"
mkdir -p "${empty}"
c6_stdout_file="${WORK}/case6.stdout"
c6_stderr_file="${WORK}/case6.stderr"
if ! "${MIGRATE}" "${empty}" >"${c6_stdout_file}" 2>"${c6_stderr_file}"; then
	fail "case6: script exited non-zero on empty directory"
fi
grep -q "nothing to migrate" "${c6_stderr_file}" ||
	fail "case6: 'nothing to migrate' message missing (stderr: $(cat "${c6_stderr_file}"))"

# --- Case 6b: directory with a .yml file that has no lowlighter/metrics --
noop_dir="${WORK}/noop_dir"
mkdir -p "${noop_dir}"
cat >"${noop_dir}/other.yml" <<'EOF'
name: other
on: [push]
jobs:
  x:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
c6b_stderr_file="${WORK}/case6b.stderr"
if ! "${MIGRATE}" "${noop_dir}" 2>"${c6b_stderr_file}" >/dev/null; then
	fail "case6b: script exited non-zero on no-match directory"
fi
grep -q "nothing to migrate" "${c6b_stderr_file}" ||
	fail "case6b: 'nothing to migrate' message missing"
[[ ! -f "${noop_dir}/other.yml.bak" ]] ||
	fail "case6b: .bak created despite no matches"

# --- Case 7: non-existent path exits non-zero ----------------------------
if "${MIGRATE}" "${WORK}/does_not_exist.yml" >/dev/null 2>&1; then
	fail "case7: script returned 0 on non-existent path"
fi

echo "OK: all cases passed"
