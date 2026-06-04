#!/usr/bin/env bash
#
# release-verify.sh — post-release verification helper for github-metrics M10.
#
# Verifies a published vX.Y.Z release by exercising the 4 SC-001..SC-004
# checks in order:
#   1. Multi-arch Docker manifest (linux/amd64 + linux/arm64) on GHCR.
#   2. SHA256SUMS validates the downloaded binaries.
#   3. cosign keyless OIDC verifies the image manifest.
#   4. action.yml runs.image: reference matches the tag.
#
# Usage:
#   scripts/release-verify.sh <tag>
#
# Example:
#   scripts/release-verify.sh v1.0.0
#
# Each section runs independently; a single failure does not skip
# subsequent sections. The script collects per-section results and
# exits non-zero if any section failed.
#
# Dependencies (must be on PATH):
#   - docker  (section 1)
#   - sha256sum (section 2)
#   - cosign  (section 3) — https://github.com/sigstore/cosign
#   - grep    (section 4)
#   - curl    (section 2 — to download binaries from GitHub Releases)

set -euo pipefail

IMAGE_REPO="ghcr.io/mjun0812/github-metrics"
ORG_REPO="mjun0812/github-metrics"
OIDC_ISSUER="https://token.actions.githubusercontent.com"
CERT_IDENTITY_RE="https://github.com/${ORG_REPO}/.github/workflows/release.yml@refs/tags/v.*"

usage() {
  cat <<EOF
Usage: $(basename "$0") <tag>

Verify a published github-metrics release end-to-end.

Arguments:
  <tag>   The semver release tag (e.g. v1.0.0).

Exits non-zero if any verification section fails.
EOF
}

if [[ $# -ne 1 ]]; then
  usage >&2
  exit 1
fi

TAG="$1"

if [[ ! "${TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "::error::tag '${TAG}' does not match v<MAJOR>.<MINOR>.<PATCH>" >&2
  exit 1
fi

WORKDIR="$(mktemp -d -t metrics-release-verify-XXXXXXXX)"
trap 'rm -rf "${WORKDIR}"' EXIT

# Per-section result tracking. 0 = OK, non-zero = FAIL.
RC_MANIFEST=0
RC_SHA256=0
RC_COSIGN=0
RC_ACTIONYML=0

echo "==> release-verify.sh ${TAG}"
echo "    image:    ${IMAGE_REPO}:${TAG}"
echo "    workdir:  ${WORKDIR}"
echo

# -------------------------------------------------------------------
# 1. docker manifest — assert multi-arch (linux/amd64 + linux/arm64)
# -------------------------------------------------------------------
echo "==> [1/4] docker manifest inspect (SC-001)"
if ! command -v docker >/dev/null 2>&1; then
  echo "    SKIP: docker not on PATH"
  RC_MANIFEST=2
else
  if ! manifest_json="$(docker manifest inspect "${IMAGE_REPO}:${TAG}" 2>&1)"; then
    echo "    FAIL: docker manifest inspect failed"
    echo "    ${manifest_json}"
    RC_MANIFEST=1
  else
    have_amd64="$(printf '%s' "${manifest_json}" | grep -c '"architecture": "amd64"' || true)"
    have_arm64="$(printf '%s' "${manifest_json}" | grep -c '"architecture": "arm64"' || true)"
    if [[ "${have_amd64}" -ge 1 && "${have_arm64}" -ge 1 ]]; then
      echo "    OK: manifest lists linux/amd64 and linux/arm64"
    else
      echo "    FAIL: expected both amd64 + arm64 platforms in manifest list"
      echo "    found: amd64=${have_amd64} arm64=${have_arm64}"
      RC_MANIFEST=1
    fi
  fi
fi
echo

# -------------------------------------------------------------------
# 2. SHA256SUMS provenance + binary integrity (SC-003)
#
# Two-layer defence:
#   (a) Filename allowlist regex — every filename field in SHA256SUMS
#       MUST match the canonical metrics-cli binary shape. Rejects
#       crafted filenames containing '/', '..', or any characters
#       that could be passed to `curl -o` to traverse paths.
#       Always active, no cosign dependency.
#   (b) Cosign keyless OIDC verification of SHA256SUMS itself via
#       sign-blob bundle. Establishes provenance: SHA256SUMS was
#       produced by this repo's release.yml workflow, not by an
#       attacker. Active when cosign is on PATH; if cosign is
#       missing, a WARN is emitted but the section still runs the
#       layer-(a) defence + binary checksum match.
# -------------------------------------------------------------------
echo "==> [2/4] SHA256SUMS provenance + binary integrity (SC-003)"

# Canonical binary-name shape — see contracts/release-workflow.md §2.2
# (matrix: linux|darwin × amd64|arm64; semver tag including optional
# prerelease suffix). Update if the matrix expands in a future release.
FILENAME_RE='^metrics-cli_v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?_(linux|darwin)_(amd64|arm64)$'

if ! command -v curl >/dev/null 2>&1 || ! command -v sha256sum >/dev/null 2>&1; then
  echo "    SKIP: curl or sha256sum not on PATH"
  RC_SHA256=2
else
  cd "${WORKDIR}"
  base_url="https://github.com/${ORG_REPO}/releases/download/${TAG}"

  if ! curl -sSfL -o SHA256SUMS "${base_url}/SHA256SUMS"; then
    echo "    FAIL: cannot download SHA256SUMS from ${base_url}/SHA256SUMS"
    RC_SHA256=1
  else
    # 2a. Verify SHA256SUMS provenance via cosign keyless OIDC. Without
    # this, an attacker who can tamper with one Release asset could
    # publish crafted SHA256SUMS lines and recompute the hashes to
    # match attacker-controlled binaries.
    if command -v cosign >/dev/null 2>&1; then
      if ! curl -sSfL -o SHA256SUMS.cosign.bundle \
            "${base_url}/SHA256SUMS.cosign.bundle"; then
        echo "    FAIL: cannot download SHA256SUMS.cosign.bundle — release artifact missing"
        RC_SHA256=1
      elif ! verify_blob_out="$(cosign verify-blob \
            --bundle SHA256SUMS.cosign.bundle \
            --certificate-identity-regexp "${CERT_IDENTITY_RE}" \
            --certificate-oidc-issuer "${OIDC_ISSUER}" \
            SHA256SUMS 2>&1)"; then
        echo "    FAIL: cosign verify-blob on SHA256SUMS failed — file may be tampered"
        echo "    ${verify_blob_out}"
        RC_SHA256=1
      else
        echo "    OK: SHA256SUMS cosign signature verified (provenance established)"
      fi
    else
      echo "    WARN: cosign not on PATH — SHA256SUMS provenance NOT verified."
      echo "          Filename allowlist + sha256sum -c will still run; install cosign for full provenance."
    fi

    # 2b. Filename allowlist + binary download + sha256sum -c.
    # Only proceed if SHA256SUMS provenance is OK or cosign was absent.
    if [[ "${RC_SHA256}" -ne 1 ]]; then
      download_ok=1
      # Use the trailing-newline-tolerant form: `read` returns
      # non-zero on EOF without a final newline, which under
      # `set -e` would silently drop the last entry. The
      # `|| [[ -n "${line}" ]]` clause catches that record.
      while IFS= read -r line || [[ -n "${line}" ]]; do
        [[ -z "${line}" ]] && continue
        # SHA256SUMS canonical line: "<64-hex><sp><sp>?<filename>".
        # awk extracts the last whitespace-separated field; sha256sum
        # emits 2-space separation but binary mode prepends a `*`.
        # We accept both via the regex match below.
        sha_field="$(printf '%s' "${line}" | awk '{print $1}')"
        filename="$(printf '%s' "${line}" | awk '{print $NF}')"
        # Strip a leading '*' (sha256sum binary-mode marker).
        filename="${filename#\*}"

        if [[ ! "${sha_field}" =~ ^[0-9a-f]{64}$ ]]; then
          echo "    FAIL: SHA256SUMS entry has malformed hash field — refusing to proceed"
          download_ok=0
          continue
        fi
        # Path-traversal guard: only canonical filenames pass.
        if [[ ! "${filename}" =~ ${FILENAME_RE} ]]; then
          printf "    FAIL: SHA256SUMS entry contains non-canonical filename %q — refusing (path-traversal guard)\n" "${filename}"
          download_ok=0
          continue
        fi
        if ! curl -sSfL -o "${filename}" "${base_url}/${filename}"; then
          echo "    FAIL: cannot download binary ${filename}"
          download_ok=0
        fi
      done < SHA256SUMS

      if [[ "${download_ok}" -eq 1 ]]; then
        if sha256sum -c SHA256SUMS >/dev/null 2>&1; then
          echo "    OK: all SHA256SUMS entries match"
        else
          echo "    FAIL: sha256sum -c reported mismatches"
          sha256sum -c SHA256SUMS || true
          RC_SHA256=1
        fi
      else
        RC_SHA256=1
      fi
    fi
  fi
  cd - >/dev/null
fi
echo

# -------------------------------------------------------------------
# 3. cosign verify — keyless OIDC against the image manifest
# -------------------------------------------------------------------
echo "==> [3/4] cosign verify image (SC-004)"
if ! command -v cosign >/dev/null 2>&1; then
  echo "    SKIP: cosign not on PATH (install via 'go install github.com/sigstore/cosign/v2/cmd/cosign@latest')"
  RC_COSIGN=2
else
  if cosign verify "${IMAGE_REPO}:${TAG}" \
      --certificate-identity-regexp "${CERT_IDENTITY_RE}" \
      --certificate-oidc-issuer "${OIDC_ISSUER}" >/dev/null 2>&1; then
    echo "    OK: image signature verified against ${CERT_IDENTITY_RE}"
  else
    echo "    FAIL: cosign verify failed"
    cosign verify "${IMAGE_REPO}:${TAG}" \
      --certificate-identity-regexp "${CERT_IDENTITY_RE}" \
      --certificate-oidc-issuer "${OIDC_ISSUER}" || true
    RC_COSIGN=1
  fi
fi
echo

# -------------------------------------------------------------------
# 4. action.yml grep — confirm the runs.image: line points at TAG
# -------------------------------------------------------------------
echo "==> [4/4] action.yml runs.image: matches tag (FR-007)"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$(dirname "$0")/..")"
ACTION_YML="${REPO_ROOT}/action.yml"
if [[ ! -f "${ACTION_YML}" ]]; then
  echo "    FAIL: action.yml not found at ${ACTION_YML}"
  RC_ACTIONYML=1
elif grep -E "image:\s*'docker://${IMAGE_REPO}:${TAG}'" "${ACTION_YML}" >/dev/null; then
  echo "    OK: action.yml references ${IMAGE_REPO}:${TAG}"
else
  echo "    FAIL: action.yml does not reference ${IMAGE_REPO}:${TAG}"
  grep -E "^\s*image:" "${ACTION_YML}" || true
  RC_ACTIONYML=1
fi
echo

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
summarize() {
  case "$1" in
    0) printf '✓ OK\n' ;;
    1) printf '✗ FAIL\n' ;;
    2) printf '○ SKIP\n' ;;
    *) printf '? UNKNOWN\n' ;;
  esac
}

echo "===== Summary ====="
printf '  [1/4] docker manifest   : %s\n' "$(summarize ${RC_MANIFEST})"
printf '  [2/4] sha256sum -c      : %s\n' "$(summarize ${RC_SHA256})"
printf '  [3/4] cosign verify     : %s\n' "$(summarize ${RC_COSIGN})"
printf '  [4/4] action.yml grep   : %s\n' "$(summarize ${RC_ACTIONYML})"

# Any FAIL (rc=1) is a hard failure. SKIP (rc=2) is tolerated since
# missing tooling is the caller's responsibility, not the release's.
if (( RC_MANIFEST == 1 || RC_SHA256 == 1 || RC_COSIGN == 1 || RC_ACTIONYML == 1 )); then
  exit 1
fi
exit 0
