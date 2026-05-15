package integration_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"

	// Side-effect import: registers the classic template with the
	// templates registry so engine.MustGet("classic") resolves.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// TestComputeSVG_ClassicOctocatGolden drives engine.Compute through the
// classic template against the mocked-GraphQL deps. The produced SVG
// is normalized via NormalizeSVG (attr-sort + whitespace collapse +
// dynamic-footer mask) and MD5-compared to tests/golden/classic/octocat.svg.
//
// The same -update flag declared by output_json_test.go regenerates the
// golden when invoked with go test -update.
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

	goldenPath := filepath.Join("..", "golden", "classic", "octocat.svg")
	if *updateGolden {
		// Persist the raw (un-normalized) SVG so the file remains
		// human-reviewable and so normalization stays a one-step
		// process on the comparison path. Both sides re-normalize at
		// comparison time, which keeps the golden file faithful to
		// the engine's real output rather than an encoder artifact.
		if mkErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750); mkErr != nil {
			t.Fatalf("mkdir golden: %v", mkErr)
		}
		if wErr := os.WriteFile(goldenPath, res.Output, 0o600); wErr != nil {
			t.Fatalf("write golden: %v", wErr)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	wantNorm, err := NormalizeSVG(want)
	if err != nil {
		t.Fatalf("normalize want: %v", err)
	}
	gotNorm, err := NormalizeSVG(res.Output)
	if err != nil {
		t.Fatalf("normalize got: %v", err)
	}
	if md5sum(wantNorm) != md5sum(gotNorm) {
		t.Fatalf("classic SVG diverged from golden\n--- want hash: %s\n--- got hash:  %s\n--- got ---\n%s",
			md5sum(wantNorm), md5sum(gotNorm), string(gotNorm))
	}
}

func md5sum(b []byte) string {
	sum := md5.Sum(b) //nolint:gosec // not security-sensitive; diff hint only
	return hex.EncodeToString(sum[:])
}
