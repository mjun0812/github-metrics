# Contract: golden.Compare family

**Date**: 2026-05-18 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-004

The shared golden-file workflow. Three comparators (bytes /
SVG-normalized / JSON-normalized) sharing one `-update` flag.

## 1. Public API

```go
package golden

// Compare diffs `got` against the golden file at
// tests/golden/<goldenPath>. On the -update flag (declared in
// tests/integration/output_json_test.go), rewrites the file and
// passes the test. Without -update, fails the test on byte mismatch
// with a first-divergent-offset diff message.
func Compare(t *testing.T, got []byte, goldenPath string)

// CompareSVG applies M2 NormalizeSVG (attribute-sort + whitespace
// collapse + dynamic-footer mask) to BOTH sides before byte-comparing.
// Used for SVG goldens that must tolerate Go-runtime / chromedp
// drift in non-semantic bytes.
func CompareSVG(t *testing.T, got []byte, goldenPath string)

// CompareJSON re-marshals both sides through json.MarshalIndent
// (2-space indent) so per-key whitespace drift between encoders does
// not break the comparison.
func CompareJSON(t *testing.T, got []byte, goldenPath string)
```

## 2. Update flag wiring (R-007)

The shared `-update` flag is declared once in
`tests/integration/output_json_test.go`:

```go
var updateGolden = flag.Bool("update", false,
    "regenerate golden test fixtures from current output")
```

The new `golden.Compare*` family reads `flag.Lookup("update")`
internally — it does NOT declare a parallel flag. Tests in packages
that don't import `tests/integration` still pick up the flag via the
top-level `flag.Parse` Go runs on test init.

If `flag.Lookup("update")` returns nil (extreme isolation scenario),
the helper assumes false (compare mode). This is the safe default —
tests fail loudly if a golden is missing.

## 3. Failure message format (R-003)

On byte-mismatch (or post-normalization mismatch for SVG/JSON), the
helper emits one `t.Errorf` with this exact shape:

```text
golden drift: tests/golden/<goldenPath>
  first divergent byte at offset N (got len=N1, want len=N2)
    got  [N-40:N+40] = <40-byte window of got, \xNN-escaped>
    want [N-40:N+40] = <40-byte window of want, \xNN-escaped>
  (run with -update to seed)
```

The 40-byte windows are clamped to `[0, len(slice))` so offsets near
the start/end of the file don't out-of-range. Non-printable bytes
(< 0x20 or > 0x7E except whitespace) render as `\xNN`.

## 4. Path resolution

`goldenPath` is relative to `<repo-root>/tests/golden/`. The helper
walks the test working directory upward for `go.mod` (same
`mustRepoRoot` pattern E-001 and E-002 use), then joins
`tests/golden/<goldenPath>`. Examples:

| `goldenPath` arg | resolved path |
|------------------|---------------|
| `"json/octocat.json"` | `<repo-root>/tests/golden/json/octocat.json` |
| `"classic/octocat.svg"` | `<repo-root>/tests/golden/classic/octocat.svg` |
| `"repository/octocat_hello-world.svg"` | `<repo-root>/tests/golden/repository/octocat_hello-world.svg` |

## 5. Seed-on-first-run

When the golden file does not exist AND `-update` is set, the
helper creates the parent directory with `os.MkdirAll(dir, 0o750)`
and writes the file with `os.WriteFile(path, got, 0o600)`.

When the golden file does not exist AND `-update` is NOT set, the
helper fails with `t.Fatalf("golden not seeded: %s (run with -update)")`
— the contract is "seeding is explicit, not automatic on first
run" so a typo in `goldenPath` doesn't silently auto-seed a wrong
file.

## 6. SVG normalization (CompareSVG)

The M2 `NormalizeSVG` helper (currently in
`tests/integration/svg_normalize.go`) moves to
`internal/testutil/golden/svg_normalize.go`. A re-export shim stays
at `tests/integration/svg_normalize.go` so existing imports don't
break during the migration phase.

`NormalizeSVG` applies:

1. Sort attributes within each element (chromedp emits attributes
   in dictionary-iteration order, which is non-deterministic).
2. Collapse contiguous whitespace inside text nodes to a single
   space.
3. Mask the dynamic footer (timestamp + version) — replaces
   `generated_at="<ISO-8601>"` and `version="<semver>"` with
   `generated_at="TIMESTAMP"` and `version="VERSION"`.

## 7. JSON normalization (CompareJSON)

`CompareJSON` re-marshals via:

```go
var box any
if err := json.Unmarshal(raw, &box); err != nil { ... }
out, err := json.MarshalIndent(box, "", "  ")
```

This guarantees identical whitespace + key ordering on both sides.
Maps in Go don't preserve key insertion order, but
`json.MarshalIndent` sorts keys alphabetically — so the round-trip
is deterministic.

## 8. Test plan (for the helper itself)

- `internal/testutil/golden/golden_test.go`:
  - `TestCompare_HappyPath_SameBytes`
  - `TestCompare_Drift_EmitsOffsetAndWindow`
  - `TestCompare_MissingFile_NoUpdate_TFatalf`
  - `TestCompare_MissingFile_WithUpdate_Seeds`
  - `TestCompareSVG_NormalizationTolerantToAttrOrder`
  - `TestCompareSVG_NormalizationMasksDynamicFooter`
  - `TestCompareJSON_TolerantToKeyOrderingDrift`
  - `TestCompare_NonprintableBytesEscaped`
