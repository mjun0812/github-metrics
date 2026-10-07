package golden_test

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/golden"
)

func TestNormalizeSVG_MasksDynamicFooter(t *testing.T) {
	t.Parallel()
	in := []byte(`<svg><text>Last updated 2026-05-18T14:00:00Z</text><text>github-metrics@v1.2.3</text></svg>`)
	out, _ := golden.NormalizeSVG(in)
	s := string(out)
	if !strings.Contains(s, "Last updated __MASKED__") {
		t.Errorf("dynamic timestamp not masked: %q", s)
	}
	if !strings.Contains(s, "github-metrics@__MASKED__") {
		t.Errorf("dynamic version not masked: %q", s)
	}
}
