package notable_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/notable"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found")
	return ""
}

func runWith(t *testing.T, inputs map[string]any) *notable.Result {
	t.Helper()
	data := plugins.NewData()
	pc := &plugins.PluginContext{Data: data, Inputs: inputs}
	out, err := notable.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*notable.Result)
}

func TestRun_SkippedInM4(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !r.Skipped {
		t.Errorf("notable should be Skipped in M4")
	}
}

func TestRun_SkippedWithFilterTrue(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_notable_filter": true})
	if !r.Skipped {
		t.Errorf("filter=true still Skipped")
	}
}

func TestRun_SkippedWithIndepthTrue(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_notable_indepth": true})
	if !r.Skipped {
		t.Errorf("indepth=true still Skipped")
	}
}

func TestRun_EmptyList(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if len(r.List) != 0 {
		t.Errorf("List should be empty in M4; got %+v", r.List)
	}
}

// Spec 013: with GraphQL client unavailable (the M4 test path), Run
// returns Skipped + "GraphQL client unavailable". The original
// "follow-up" message was M4 baseline language; 013 replaces it with
// the precise gating reason.
func TestRun_SkippedReasonExplainsDeferral(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !strings.Contains(r.SkippedReason, "GraphQL") {
		t.Errorf("SkippedReason should mention GraphQL; got %q", r.SkippedReason)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &notable.Result{Skipped: true, List: []notable.NotableContrib{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "notable.json")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile: %v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
	}
}
