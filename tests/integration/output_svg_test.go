package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"

	// Side-effect import: registers the classic template with the
	// templates registry so engine.MustGet("classic") resolves.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// TestComputeSVG_ClassicOctocatGolden drives engine.Compute through
// the classic template against the mocked-GraphQL deps and compares
// the resulting SVG to tests/golden/classic/octocat.svg via the M9
// shared `golden.CompareSVG` helper (which normalizes both sides
// per the M2 NormalizeSVG rules + emits first-divergent-offset
// diff messages on mismatch).
func TestComputeSVG_ClassicOctocatGolden(t *testing.T) {
	engine.SetVersionForTest(t, "test-version")
	// Anchor BaseHeader's "Joined GitHub <age>" label so the golden
	// SVG stays stable across days. octocat.createdAt is 2008-01-14;
	// freezing now() to 2026-01-14 gives a clean "18 years ago".
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	t.Cleanup(restore)

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		// #602: header is now a regular plugin; enable it so the
		// golden classic render still includes the identity block.
		Inputs: map[string]any{"plugin_header": "yes"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Fatalf("MIME = %q, want image/svg+xml", res.MIME)
	}
	if !strings.HasPrefix(string(res.Output), "<svg") {
		t.Fatalf("Output should start with <svg; got %.80s", string(res.Output))
	}

	golden.CompareSVG(t, res.Output, "classic/octocat.svg")
}
