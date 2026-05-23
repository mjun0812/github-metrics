# Contract: Sample SVG generation

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002, E-003, E-006

Codifies how each sample SVG under `docs/examples/` is generated.
The `scripts/gen-doc-samples.sh` script + the `make docs-samples`
target implement this contract.

## 1. Pre-conditions

| Resource | Where | Why |
|----------|-------|-----|
| `metrics-cli` binary | `make build` produces `bin/metrics-cli`; the script invokes it directly | the rendering engine |
| `GITHUB_TOKEN` env | exported in shell or via direnv | the engine fetches user data from api.github.com |
| chromium | system install (`brew install chromium` on macOS, system package on Linux) — `METRICS_CHROME_PATH` env may point to it | required by chromedp-gated plugins (`topics`, `starlists`) and by the resize / PNG / JPEG path |
| `normalize-svg-stream` tool | `go run ./internal/tools/normalize-svg-stream` (new; thin wrapper around `internal/testutil/golden.NormalizeSVG`) — see §5 | masks dynamic strings (version footer, Last-updated timestamp) |

The script `exit 1`s with an actionable error if any pre-condition
is missing.

## 2. Per-plugin invocation matrix

```sh
# Default plugin samples (single-panel renders).
# SLUG is one of the 19 adopted slugs.
metrics-cli \
  --user "${METRICS_DOC_USER:-mjun0812}" \
  --token-env GITHUB_TOKEN \
  --template classic \
  --output svg \
  --dryrun \
  --filename - \
  --plugin_${SLUG} yes \
  | go run ./internal/tools/normalize-svg-stream \
  > docs/examples/plugin-${SLUG}.svg
```

Sub-mode variants:

```sh
# languages.recent
metrics-cli --user mjun0812 --token-env GITHUB_TOKEN \
  --template classic --output svg --dryrun --filename - \
  --plugin_languages yes \
  --plugin_languages_sections "most-used,recently-used" \
  | normalize-svg-stream \
  > docs/examples/plugin-languages-recent.svg

# languages.indepth
metrics-cli --user mjun0812 --token-env GITHUB_TOKEN \
  --template classic --output svg --dryrun --filename - \
  --plugin_languages yes \
  --plugin_languages_indepth yes \
  --plugin_languages_analysis_timeout 30 \
  | normalize-svg-stream \
  > docs/examples/plugin-languages-indepth.svg
```

## 3. Hero invocations (E-003)

```sh
# Classic hero — all 19 plugins enabled
metrics-cli \
  --user mjun0812 \
  --token-env GITHUB_TOKEN \
  --template classic \
  --output svg \
  --dryrun \
  --filename - \
  --plugin_achievements yes \
  --plugin_activity yes \
  --plugin_calendar yes \
  --plugin_contributors yes \
  --plugin_habits yes \
  --plugin_isocalendar yes \
  --plugin_languages yes \
  --plugin_notable yes \
  --plugin_people yes \
  --plugin_projects yes \
  --plugin_reactions yes \
  --plugin_repositories yes \
  --plugin_sponsors yes \
  --plugin_sponsorships yes \
  --plugin_stargazers yes \
  --plugin_starlists yes \
  --plugin_stars yes \
  --plugin_topics yes \
  --plugin_traffic yes \
  | normalize-svg-stream \
  > docs/examples/hero-classic.svg

# Repository hero — repo-mode-capable subset (M7 contract)
metrics-cli \
  --user mjun0812 \
  --repo github-metrics \
  --token-env GITHUB_TOKEN \
  --template repository \
  --output svg \
  --dryrun \
  --filename - \
  --plugin_languages yes \
  --plugin_projects yes \
  --plugin_stargazers yes \
  --plugin_people yes \
  --plugin_activity yes \
  --plugin_contributors yes \
  --plugin_sponsors yes \
  | normalize-svg-stream \
  > docs/examples/hero-repository.svg
```

## 4. Execution order + total runtime

`scripts/gen-doc-samples.sh` runs the invocations sequentially.
Each call is independent (separate `metrics-cli` process) so a
single plugin failure does not halt the rest — the script collects
errors and exits non-zero at the end if any sample is missing.

Expected per-sample wall-clock:

- chromedp-cold-start: ~5 s amortized once per script run
- GitHub API fetches: 1-3 s per plugin
- SVG render + chromedp resize: 2-5 s per plugin
- Total per plugin: ~5-10 s

Total for 19 + 3 sub-modes + 2 heroes = ~24 invocations ≈
2-4 minutes wall-clock. Within the 5-min SC-004 budget.

## 5. `normalize-svg-stream` tool

A NEW thin Go tool at `internal/tools/normalize-svg-stream/`:

```go
// main.go
package main

import (
    "fmt"
    "io"
    "os"

    "github.com/mjun0812/github-metrics/internal/testutil/golden"
)

func main() {
    raw, err := io.ReadAll(os.Stdin)
    if err != nil {
        fmt.Fprintf(os.Stderr, "normalize-svg-stream: read stdin: %v\n", err)
        os.Exit(1)
    }
    out, err := golden.NormalizeSVG(raw)
    if err != nil {
        fmt.Fprintf(os.Stderr, "normalize-svg-stream: normalize: %v\n", err)
        os.Exit(1)
    }
    if _, err := os.Stdout.Write(out); err != nil {
        fmt.Fprintf(os.Stderr, "normalize-svg-stream: write stdout: %v\n", err)
        os.Exit(1)
    }
}
```

Imports the existing M9 `NormalizeSVG` so committed SVG bytes are
diff-free across regenerations when only dynamic strings change.

Test surface: `main_test.go` verifies stdin → stdout round-trip
preserves semantic content and masks dynamic strings.

## 6. Failure handling

| Failure | Resolution |
|---------|-----------|
| `metrics-cli` exits non-zero for plugin X | Script records "X failed" and continues; final exit status reflects any failures. |
| `GITHUB_TOKEN` missing / invalid | Script `exit 1`s immediately with clear error before invoking metrics-cli. |
| chromium unavailable | chromedp-gated plugins (`topics`, `starlists`) fail with a clear error; other plugins still generate. |
| SVG parse failure inside `normalize-svg-stream` | Script `exit 1`s with the parse error; the partial SVG is NOT written to `docs/examples/` (the redirect happens atomically — write to `.tmp` then `mv`). |

## 7. Idempotency

Re-running `make docs-samples` against the same upstream-data
state + same code state produces **byte-identical** output (the
NormalizeSVG mask suppresses all dynamic noise). Maintainers can
verify with `git status` after a re-run: zero changes if nothing
upstream moved.

## 8. Atomic write

Each sample is written to `docs/examples/plugin-${SLUG}.svg.tmp`
and only `mv`'d into place after the full pipeline succeeds. A
failed pipeline does not corrupt the existing committed sample.
