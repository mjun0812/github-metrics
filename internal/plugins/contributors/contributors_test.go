package contributors_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/contributors"
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

func run(t *testing.T, account plugins.AccountKind) *contributors.Result {
	t.Helper()
	data := plugins.NewData()
	data.Account = account
	pc := &plugins.PluginContext{Data: data, Inputs: map[string]any{}}
	out, err := contributors.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*contributors.Result)
}

func TestRun_UserAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountUser)
	if !r.Skipped {
		t.Errorf("user account should be Skipped in M4; got %+v", r)
	}
}

func TestRun_OrganizationAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountOrganization)
	if !r.Skipped {
		t.Errorf("organization account should be Skipped in M4; got %+v", r)
	}
}

func TestRun_RepositoryAccountSkipped(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountRepository)
	if !r.Skipped {
		t.Errorf("repository account should still be Skipped in M4 (M7 territory); got %+v", r)
	}
}

func TestRun_SkippedReasonNonEmpty(t *testing.T) {
	t.Parallel()
	r := run(t, plugins.AccountUser)
	if r.SkippedReason == "" {
		t.Errorf("SkippedReason should be non-empty for trace logs")
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &contributors.Result{
		Skipped:  true,
		List:     []contributors.Contributor{},
		Sections: []string{},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "contributors.json")
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
