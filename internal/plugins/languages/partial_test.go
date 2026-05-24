package languages_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
	// DOM contract spot-checks (base markers + parity additions):
	for _, marker := range []string{
		// v1.0.0 byte-frozen markers (preserved for backward compat)
		`<g class="languages-progress">`,
		`<rect class="language-bar"`,
		// 011 parity additions:
		`<h2 class="field">`, // count header (FR-001 row #2)
		`Language`,           // count text "N Language(s)"
		`<h3 class="field">Most used languages</h3>`,            // section sub-header (row #3)
		`<svg class="bar" xmlns="http://www.w3.org/2000/svg"`,   // bar wrapped in <svg> (row #8, FR-002 bare-<g> fix)
		`<title>Languages distribution</title>`,                 // a11y title (row #16, Q1 verbatim preservation)
		`role="img" aria-label="Languages distribution"`,        // a11y attrs (Q1 verbatim)
		`<div class="field center horizontal-wrap fill-width">`, // per-language color-dot list (row #13)
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

// TestPartial_Languages_Recent verifies the recently-used section is
// emitted as a <g class="languages-recent"> when languages.recent has a
// non-skipped result.
func TestPartial_Languages_Recent(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(languages.Name, &languages.Result{
		Favorites: []plugins.LanguageStat{{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1}},
		Sections:  []string{"most-used", "recently-used"},
		Mostly:    plugins.LanguageStat{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1},
		Colors:    map[string]string{"Go": "#00ADD8"},
	})
	data.SetPlugin(languages.RecentName, &languages.RecentResult{
		Favorites: []plugins.LanguageStat{{Name: "Python", Color: "#3572A5", Size: 200, Value: 1}},
		Days:      7,
		Load:      50,
		Repos:     []string{"octocat/py"},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := languages.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, `<g class="languages-recent"`) {
		t.Errorf("missing recently-used marker in:\n%s", got)
	}
	if !strings.Contains(got, `data-language="Python"`) {
		t.Errorf("missing Python data attr in:\n%s", got)
	}
}

// TestPartial_Languages_NoDuplicateMaskID guards against the regression
// where both most-used and recently-used bars emitted the same
// `<mask id="languages-bar">`, which made the second bar inherit the
// first one's clip on standards-compliant renderers (invalid SVG).
func TestPartial_Languages_NoDuplicateMaskID(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(languages.Name, &languages.Result{
		Favorites: []plugins.LanguageStat{{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1}},
		Sections:  []string{"most-used", "recently-used"},
		Mostly:    plugins.LanguageStat{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1},
		Colors:    map[string]string{"Go": "#00ADD8"},
	})
	data.SetPlugin(languages.RecentName, &languages.RecentResult{
		Favorites: []plugins.LanguageStat{{Name: "Python", Color: "#3572A5", Size: 200, Value: 1}},
		Days:      7,
		Load:      50,
		Repos:     []string{"octocat/py"},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := languages.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	idRe := regexp.MustCompile(`<mask id="([^"]+)"`)
	matches := idRe.FindAllStringSubmatch(got, -1)
	seen := map[string]int{}
	for _, m := range matches {
		seen[m[1]]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("mask id %q appears %d times in single SVG (duplicate ids are invalid):\n%s", id, n, got)
		}
	}
	if seen["languages-bar-most"] != 1 {
		t.Errorf("expected exactly one languages-bar-most mask, got %d", seen["languages-bar-most"])
	}
	if seen["languages-bar-recent"] != 1 {
		t.Errorf("expected exactly one languages-bar-recent mask, got %d", seen["languages-bar-recent"])
	}
}

// TestPartial_Languages_Indepth verifies the indepth section is emitted
// as a <g class="languages-indepth"> when languages.indepth has totals.
func TestPartial_Languages_Indepth(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.SetPlugin(languages.Name, &languages.Result{
		Favorites: []plugins.LanguageStat{{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1}},
		Sections:  []string{"most-used"},
		Mostly:    plugins.LanguageStat{Name: "Go", Color: "#00ADD8", Size: 100, Value: 1},
		Colors:    map[string]string{"Go": "#00ADD8"},
	})
	data.SetPlugin(languages.IndepthName, &languages.IndepthResult{
		Total:    languages.LanguageBytes{Bytes: map[string]int64{"Go": 5000, "Rust": 2500}},
		Analyzed: []string{"octocat/svc"},
	})
	pc := &templates.PartialContext{Data: data}
	got, err := languages.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	if !strings.Contains(got, `<g class="languages-indepth">`) {
		t.Errorf("missing indepth marker in:\n%s", got)
	}
	if !strings.Contains(got, `data-bytes="5000"`) {
		t.Errorf("missing Go bytes attr in:\n%s", got)
	}
	// Sort: Go (5000) should come before Rust (2500). The data-language
	// attribute also appears in the standard "most-used" <rect> output
	// above the indepth section, so we must scope the comparison to the
	// indepth <g class="indepth-language"> entries only. Using the
	// dedicated `indepth-language` class via regexp guarantees we are
	// matching the indepth ordering instead of the most-used favorites.
	indepthRe := regexp.MustCompile(`<text class="indepth-language" data-language="([^"]+)"`)
	matches := indepthRe.FindAllStringSubmatch(got, -1)
	indepthOrder := make([]string, 0, len(matches))
	for _, m := range matches {
		indepthOrder = append(indepthOrder, m[1])
	}
	wantOrder := []string{"Go", "Rust"}
	if !reflect.DeepEqual(indepthOrder, wantOrder) {
		t.Errorf("indepth ordering = %v, want %v in:\n%s", indepthOrder, wantOrder, got)
	}
}
