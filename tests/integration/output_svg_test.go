package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
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
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
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
