package integration_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/action"
	"github.com/mjun0812/github-metrics/internal/testutil/golden"
)

// TestAction_BannerSnapshot pins the banner format documented as
// SC-003. Uses the M9 shared `golden.Compare` for byte-exact comparison
// (the banner is deterministic — no normalization needed).
//
// The pre-v3.0 "Mode" row was dropped in #646 when the Action / CLI
// dispatch collapsed into a single pipeline; the golden file reflects
// the post-unification shape.
//
// Run with `go test -update ./tests/integration/...` to regenerate.
func TestAction_BannerSnapshot(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	action.PrintBanner(&buf, action.BannerInfo{
		Version:     "v0.0.0-test",
		Template:    "classic",
		Plugins:     []string{"languages", "activity"},
		TokenMasked: "ghp_***masked***",
		GoVersion:   "go1.26.0",
		OSArch:      "linux/amd64",
	})
	got := buf.Bytes()

	golden.Compare(t, got, "action/banner.txt")

	// Semantic guards — the banner MUST surface these fields in plain
	// text so engineers can grep run logs without ANSI / JSON parsing.
	gotStr := string(got)
	for _, must := range []string{
		"metrics-cli — startup banner",
		"Version", "v0.0.0-test",
		"Template", "classic",
		"Plugins", "languages",
		"Token", "ghp_***masked***",
		"go1.26.0", "linux/amd64",
	} {
		if !strings.Contains(gotStr, must) {
			t.Errorf("banner missing %q\nfull output:\n%s", must, gotStr)
		}
	}
}
