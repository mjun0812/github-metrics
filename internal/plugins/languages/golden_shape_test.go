package languages_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
)

// TestRun_GoldenShape pins the JSON shape of *languages.Result against
// tests/golden/json/m4/languages.json. The shape (key set, types) must
// stay 1:1 with the upstream data.plugins.languages payload — value
// drift is allowed but key/type drift is not (constitution 原則 II).
func TestRun_GoldenShape(t *testing.T) {
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
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')

	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "languages.json")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if werr := os.WriteFile(gp, got, 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", werr)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("ReadFile %s: %v (run with -update)", gp, err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), string(got))
	}
}
