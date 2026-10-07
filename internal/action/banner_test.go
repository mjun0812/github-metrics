package action

import (
	"bytes"
	"strings"
	"testing"
)

// TestPrintBanner_NoPluginsShowsNone ensures the empty plugin list
// renders as "(none)" rather than an awkward empty cell.
func TestPrintBanner_NoPluginsShowsNone(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{Version: "dev"})
	if !strings.Contains(buf.String(), "Plugins            │ (none)") {
		t.Errorf("empty plugin list should render (none); got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "Token              │ (not provided)") {
		t.Errorf("empty token should render (not provided); got:\n%s", buf.String())
	}
}
