# Contract: .golangci.yml extension (FR-011)

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../research.md](../research.md) R-004

The M9 lint additions that extend the existing
`.golangci.yml` (v2 schema, pinned to golangci-lint v2.12.2 per
`.github/workflows/go-ci.yml`).

## 1. Linter additions

The following enabled set MUST stay in `.golangci.yml`:

```yaml
linters:
  default: none
  enable:
    # existing (M1-M7)
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - nilerr
    - gocritic
    - revive
    - prealloc
    - unparam
    - gosec
```

The M9 changes operate on the `settings:` block to **explicitly
enable** the rule subsets that M6/M7 self-reviews surfaced
manually:

```yaml
  settings:
    # ... existing govet, etc. ...

    staticcheck:
      checks:
        - "all"          # default
        - "QF*"          # explicit: De Morgan (QF1001), tagged switch (QF1003), strings.ReplaceAll (QF1004)

    gosec:
      excludes:
        # G115 (integer overflow) — keep enabled; M7 had one false-positive
        # we fixed at the source rather than excluding.
      severity: low

    gocritic:
      enabled-checks:
        - ifElseChain    # tagged-switch suggestion (complements staticcheck QF1003)
        - rangeValCopy   # heap-alloc avoidance
        - hugeParam      # large struct pass-by-value warning
      disabled-checks:
        - paramTypeCombine  # noisy false-positives on test setup funcs
```

## 2. Acceptance criteria

A diff that introduces:

| Pattern | Lint output expected |
|---------|---------------------|
| `if !(a && b) { ... }` (De Morgan candidate) | `staticcheck QF1001: could apply De Morgan's law` |
| `switch { case path == "/": ... case path == "/x": ... }` | `staticcheck QF1003: could use tagged switch on path` |
| `strings.Replace(s, "a", "b", -1)` | `staticcheck QF1004: could use strings.ReplaceAll` |
| `int(uint32(big))` (integer overflow) | `gosec G115: integer overflow conversion` |
| `if x == "a" { ... } else if x == "b" { ... } else { ... }` | `gocritic ifElseChain: rewrite if-else to switch` |

Each of these has been triggered organically in past M6/M7 PRs; M9
makes the trigger automatic instead of relying on manual self-review.

## 3. CI integration (FR-011)

`.github/workflows/go-ci.yml::lint` already runs:

```yaml
- uses: golangci/golangci-lint-action@v8
  with:
    version: v2.12.2
```

The action reads `.golangci.yml` automatically. M9 adds NO new CI
step — only the `.golangci.yml` settings get extended.

## 4. Per-file exemptions

When a lint hit is intentional (e.g., the M6
`internal/action/output_action.go::SetOutput` `nolint:gosec`
exemption for the env-controlled file path), the exemption MUST be
in-line via `//nolint:<lint>` comment + a one-line rationale, NOT
via global exclusion in `.golangci.yml`. Global exclusions are
reserved for false-positives that affect the entire codebase.

## 5. Migration impact

The lint additions may surface existing-code hits. M9 fixes them
inline as part of the lint-config commit (Phase 4 per research
R-008). Expected hit count: 3-10 (most M6/M7 manual fixes already
applied during PR self-review).

## 6. Test plan

The lint config has no traditional unit tests. Verification path:

- Run `golangci-lint run --timeout=10m ./...` locally before/after
  the `.golangci.yml` change.
- Confirm zero pre-existing hits remain after fix-up.
- Push a deliberate-violation branch (separate experiment branch,
  not the M9 PR) and confirm CI flags the specific lint name.
