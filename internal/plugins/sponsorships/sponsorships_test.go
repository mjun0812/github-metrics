package sponsorships_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
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

func run(t *testing.T, inputs map[string]any) *sponsorships.Result {
	t.Helper()
	pc := &plugins.PluginContext{Data: plugins.NewData(), Inputs: inputs}
	out, err := sponsorships.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*sponsorships.Result)
}

func TestRun_DefaultEmpty(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.Skipped {
		t.Errorf("MVP should return empty non-Skipped Result")
	}
	if len(r.Active) != 0 {
		t.Errorf("Active should be empty in MVP; got %+v", r.Active)
	}
}

func TestRun_PastSection(t *testing.T) {
	t.Parallel()
	r := run(t, map[string]any{"plugin_sponsorships_sections": "active,past"})
	if len(r.Past) != 0 {
		t.Errorf("Past should remain empty in MVP")
	}
}

func TestRun_NoNilFields(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.Active == nil {
		t.Errorf("Active should be non-nil slice (was %v)", r.Active)
	}
}

func TestRun_IsSkippedFalse(t *testing.T) {
	t.Parallel()
	r := run(t, nil)
	if r.IsSkipped() {
		t.Errorf("IsSkipped should be false")
	}
}

func TestRun_NilInputs(t *testing.T) {
	t.Parallel()
	pc := &plugins.PluginContext{Data: plugins.NewData()}
	out, _ := sponsorships.Plugin.Run(context.Background(), pc)
	if out == nil {
		t.Fatalf("Result should not be nil even with nil inputs")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &sponsorships.Result{Active: []sponsorships.Sponsored{}}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "sponsorships.json")
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
