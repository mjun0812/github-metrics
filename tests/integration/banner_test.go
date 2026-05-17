package integration_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/action"
)

// Reuse the existing -update flag declared in output_json_test.go so
// the same `go test -update ./tests/integration/...` pattern applies.
var _ = flag.Lookup("update")

// TestAction_BannerSnapshot pins the banner format documented as
// SC-003. We compare semantic content (key lines must be present) so
// timestamps / Go version drift doesn't churn the golden, but the
// full byte snapshot is also written + re-checked to guard layout
// regressions.
//
// Run with `go test -update ./tests/integration/...` to regenerate.
func TestAction_BannerSnapshot(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	action.PrintBanner(&buf, action.BannerInfo{
		Version:     "v0.0.0-test",
		Mode:        "action",
		Template:    "classic",
		Plugins:     []string{"languages", "activity"},
		TokenMasked: "ghp_***masked***",
		GoVersion:   "go1.26.0",
		OSArch:      "linux/amd64",
	})
	got := buf.String()

	goldenPath := filepath.Join("..", "golden", "action", "banner.txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("updated golden:", goldenPath)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to seed)", err)
	}
	if got != string(want) {
		t.Errorf("banner snapshot drift; len got=%d want=%d", len(got), len(want))
	}

	// Semantic guards — the banner MUST surface these fields in plain
	// text so engineers can grep run logs without ANSI / JSON parsing.
	for _, must := range []string{
		"metrics-action — startup banner",
		"Version", "v0.0.0-test",
		"Mode", "action",
		"Template", "classic",
		"Plugins", "languages",
		"Token", "ghp_***masked***",
		"go1.26.0", "linux/amd64",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("banner missing %q\nfull output:\n%s", must, got)
		}
	}
}
