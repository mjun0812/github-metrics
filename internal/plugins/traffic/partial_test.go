package traffic_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGoldenPartial = flag.Bool("update-partial", false, "update partial golden files in tests/golden/...")

// goldenPath builds the absolute path to a file under tests/golden/.
func goldenPath(t *testing.T, parts ...string) string {
	t.Helper()
	root := repoRoot(t)
	return filepath.Join(append([]string{root, "tests", "golden"}, parts...)...)
}

// TestPartial_Traffic_Skipped verifies a Skipped result yields the
// empty fragment so the section is not rendered at all.
func TestPartial_Traffic_Skipped(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{Skipped: true})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty fragment for Skipped=true; got %q", got)
	}
}

// TestPartial_Traffic_DelimiterAndHideEmpty validates the two issue
// #412 contract bullets at once:
//
//   - the per-repo line contains the visible ": " delimiter between
//     `</span>` and the view count, so long repo names cannot run
//     into the number;
//   - with HideEmpty=true (default), Count==0 entries are filtered
//     out before render.
func TestPartial_Traffic_DelimiterAndHideEmpty(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/beta":  {Count: 50, Uniques: 20},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 150, Uniques: 60},
		HideEmpty: true,
	})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	// Delimiter contract: each per-repo line must be `</span>: ` (the
	// pre-fix output had `</span> ` which read as `foo/bar1234 views`).
	wantDelim := `<span class="repo">octocat/alpha</span>: 100 views (40 unique)`
	if !strings.Contains(got, wantDelim) {
		t.Errorf("expected delimiter line %q in:\n%s", wantDelim, got)
	}
	// HideEmpty: octocat/zero (Count==0) must NOT be rendered.
	if strings.Contains(got, "octocat/zero") {
		t.Errorf("octocat/zero (Count==0) should be filtered when HideEmpty=true:\n%s", got)
	}
	// Aggregate line still emitted.
	if !strings.Contains(got, `<span class="label">150 views (60 unique)</span>`) {
		t.Errorf("aggregate line missing in:\n%s", got)
	}
}

// TestPartial_Traffic_HideEmptyFalse verifies HideEmpty=false preserves
// the legacy behaviour where every repo (including 0-view) is rendered.
func TestPartial_Traffic_HideEmptyFalse(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"octocat/alpha": {Count: 100, Uniques: 40},
			"octocat/zero":  {Count: 0, Uniques: 0},
		},
		Total:     traffic.TrafficView{Count: 100, Uniques: 40},
		HideEmpty: false,
	})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, "octocat/zero") {
		t.Errorf("octocat/zero should be rendered when HideEmpty=false:\n%s", got)
	}
	// Singular form for 0 → "0 views" (pluralS returns "s" for n != 1).
	if !strings.Contains(got, `<span class="repo">octocat/zero</span>: 0 views (0 unique)`) {
		t.Errorf("expected 0-view line for octocat/zero in:\n%s", got)
	}
}

// TestPartial_Traffic_Golden writes the SVG fragment to
// tests/golden/classic/m4/traffic.svg.
func TestPartial_Traffic_Golden(t *testing.T) {
	data := plugins.NewData()
	data.SetPlugin(traffic.Name, &traffic.Result{
		Views: map[string]traffic.TrafficView{
			"mjun0812/alpha": {Count: 1234, Uniques: 456},
			"mjun0812/beta":  {Count: 50, Uniques: 20},
			"mjun0812/gamma": {Count: 1, Uniques: 1},
		},
		Total:     traffic.TrafficView{Count: 1285, Uniques: 477},
		HideEmpty: true,
	})
	pc := &templates.PartialContext{Data: data}
	got, err := traffic.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}

	gp := goldenPath(t, "classic", "m4", "traffic.svg")
	if *updateGoldenPartial {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", werr)
		}
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile %s: %v (run with -update-partial)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
}
