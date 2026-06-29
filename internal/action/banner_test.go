package action

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintBanner_AllFieldsRendered(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{
		Version:     "v1.2.3",
		Template:    "classic",
		Plugins:     []string{"languages", "activity", "achievements"},
		TokenMasked: "ghp_***masked***",
		GoVersion:   "go1.26.3",
		OSArch:      "darwin/arm64",
	})
	out := buf.String()
	for _, want := range []string{
		"metrics-cli — startup banner",
		"Version            │ v1.2.3",
		"Template           │ classic",
		"Plugins            │ achievements, activity, languages", // sorted
		"Token              │ ghp_***masked***",
		"Runtime            │ go1.26.3, darwin/arm64",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q in:\n%s", want, out)
		}
	}
}

// TestPrintBanner_NoModeRow pins the v3.0 mode-unification removal
// (#646): the pre-v3.0 "Mode               │ action / cli" row is gone
// because the dispatch was collapsed into a single pipeline. The Mode
// label MUST NOT appear in banner output anymore.
func TestPrintBanner_NoModeRow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{
		Version:     "v1.2.3",
		Template:    "classic",
		Plugins:     []string{"languages"},
		TokenMasked: "ghp_***masked***",
	})
	if strings.Contains(buf.String(), "Mode ") || strings.Contains(buf.String(), "│ action\n") || strings.Contains(buf.String(), "│ cli\n") {
		t.Errorf("banner must not display a Mode row after #646; got:\n%s", buf.String())
	}
}

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

// TestPrintBanner_NoTokenLeak (SC-011 partial) — supplying a raw PAT
// in the *unrelated* fields must not surface it. The TokenMasked
// field is the only allowed token-bearing slot, and callers are
// expected to pass config.Token.String() (which already masks).
func TestPrintBanner_NoTokenLeak(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{
		Version:     "dev",
		Template:    "classic",
		Plugins:     []string{"languages"},
		TokenMasked: "ghp_***masked***",
	})
	if strings.Contains(buf.String(), "ghp_realToken") {
		t.Errorf("raw PAT leaked into banner; got:\n%s", buf.String())
	}
}

// TestPrintBanner_SnapshotShape pins the row count + leading rulers
// so SC-003 (banner format compatibility with upstream Node) has a
// stable surface for snapshot diffing.
func TestPrintBanner_SnapshotShape(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintBanner(&buf, BannerInfo{
		Version:  "v1.0.0",
		Template: "classic",
		Plugins:  []string{"languages"},
	})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// After #646 the banner has at least 6 rows (2 rulers + header +
	// 4 fields: Version, Template, Plugins, Token).
	if len(lines) < 6 {
		t.Errorf("banner should have at least 6 rows (2 rulers + header + 4 fields); got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "─") {
		t.Errorf("first line should be a ruler; got %q", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "─") {
		t.Errorf("last line should be a ruler; got %q", lines[len(lines)-1])
	}
}
