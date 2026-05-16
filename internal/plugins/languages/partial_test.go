package languages_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files in tests/golden/...")

// goldenPath builds the absolute path to a file under tests/golden/.
// Test binaries cwd is the package dir; walk up to the repo root.
func goldenPath(t *testing.T, parts ...string) string {
	t.Helper()
	root := repoRoot(t)
	return filepath.Join(append([]string{root, "tests", "golden"}, parts...)...)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", cwd)
	return ""
}

func TestPartial_Languages_Golden(t *testing.T) {
	r := &languages.Result{
		Favorites: []plugins.LanguageStat{
			{Name: "Go", Color: "#00ADD8", Size: 10000, Count: 2, Value: 0.5556},
			{Name: "TypeScript", Color: "#3178c6", Size: 4500, Count: 1, Value: 0.25},
			{Name: "JavaScript", Color: "#f1e05a", Size: 3000, Count: 2, Value: 0.1667},
		},
		Other: plugins.LanguageStat{
			Name: "Other", Color: "#cccccc", Size: 1100, Count: 2, Value: 0.0611,
		},
		Sections: []string{"most-used"},
		Mostly: plugins.LanguageStat{
			Name: "Go", Color: "#00ADD8", Size: 10000, Count: 2, Value: 0.5556,
		},
		Colors: map[string]string{
			"Go":         "#00ADD8",
			"TypeScript": "#3178c6",
			"JavaScript": "#f1e05a",
			"Other":      "#cccccc",
		},
	}
	data := plugins.NewData()
	data.SetPlugin(languages.Name, r)
	pc := &templates.PartialContext{Data: data}

	got, err := languages.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}

	gp := goldenPath(t, "classic", "m4", "languages.svg")
	if *updateGolden {
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
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
	// DOM contract spot-checks (partial-classic-m4.md §5):
	for _, marker := range []string{
		`<g class="languages-progress">`,
		`<rect class="language-bar"`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

func TestPartial_Languages_Skipped(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(languages.Name, &languages.Result{Skipped: true})
	pc := &templates.PartialContext{Data: data}
	got, err := languages.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty fragment for Skipped=true; got %q", got)
	}
}
