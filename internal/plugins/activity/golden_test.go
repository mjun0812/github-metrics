package activity_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/activity"
	"github.com/mjun0812/github-metrics/internal/templates"
)

var updateGolden = flag.Bool("update", false, "update golden files in tests/golden/...")

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
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
	t.Fatalf("could not find repo root from %s", cwd)
	return ""
}

func fixedResult() *activity.Result {
	when := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	return &activity.Result{
		Events: []activity.ActivityEvent{
			{Type: "PushEvent", Repo: "octocat/alpha", Date: when, Visibility: "public"},
			{
				Type:       "PullRequestEvent",
				Repo:       "octocat/beta",
				Date:       when.Add(-1 * time.Hour),
				Visibility: "public",
				Files:      &activity.EventFiles{Changed: 2},
				Lines:      &activity.EventLines{Added: 34, Deleted: 5},
			},
		},
		Days: 14,
	}
}

func TestPartial_Activity_Golden(t *testing.T) {
	r := fixedResult()
	data := plugins.NewData()
	data.SetPlugin(activity.Name, r)
	pc := &templates.PartialContext{Data: data}
	got, err := activity.Partial(context.Background(), pc)
	if err != nil {
		t.Fatalf("Partial: %v", err)
	}
	gp := filepath.Join(repoRoot(t), "tests", "golden", "classic", "m4", "activity.svg")
	if *updateGolden {
		if werr := os.MkdirAll(filepath.Dir(gp), 0o755); werr != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if werr := os.WriteFile(gp, []byte(got), 0o644); werr != nil {
			t.Fatalf("WriteFile: %v", err)
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
	// Tier 3 (011) rewrite: each event row is now
	// `<section class="activity" data-type=... data-repo=...>` containing
	// an inline octicon `<svg>` and a `<div class="content">` verb +
	// repo span. The old `class="activity-event"` + `class="octicon"`
	// markers no longer match.
	for _, marker := range []string{
		`class="activity"`,
		`data-type="`,
		`<span class="repo">`,
	} {
		if !strings.Contains(got, marker) {
			t.Errorf("partial missing marker %q in:\n%s", marker, got)
		}
	}
}

func TestRun_GoldenShape_Activity(t *testing.T) {
	r := fixedResult()
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "activity.json")
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
