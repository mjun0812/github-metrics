// Re-export shim — the canonical NormalizeSVG implementation moved
// to internal/testutil/golden in M9. This file exists so existing
// in-package callers (`output_svg_test.go::TestComputeSVG_ClassicOctocatGolden`)
// keep compiling unmodified until they migrate to
// `golden.CompareSVG`.
package integration_test

import "github.com/mjun0812/github-metrics/internal/testutil/golden"

// NormalizeSVG is re-exported from `internal/testutil/golden` to
// preserve the M2 in-package call sites.
func NormalizeSVG(raw []byte) ([]byte, error) {
	return golden.NormalizeSVG(raw)
}
