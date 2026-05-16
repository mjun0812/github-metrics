package stargazers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
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

func runWith(t *testing.T, inputs map[string]any) *stargazers.Result {
	t.Helper()
	data := plugins.NewData()
	pc := &plugins.PluginContext{Data: data, Inputs: inputs}
	out, err := stargazers.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.(*stargazers.Result)
}

func TestRun_AlwaysSkippedInM4(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !r.Skipped {
		t.Errorf("M4 stargazers should always be Skipped; got %+v", r)
	}
}

func TestRun_WorldmapNilEvenWhenRequested(t *testing.T) {
	t.Parallel()
	r := runWith(t, map[string]any{"plugin_stargazers_worldmap": true})
	if r.Worldmap != nil {
		t.Errorf("worldmap should remain nil in M4; got %+v", r.Worldmap)
	}
}

func TestRun_WorldmapWarnLogEmitted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pc := &plugins.PluginContext{
		Data:   plugins.NewData(),
		Inputs: map[string]any{"plugin_stargazers_worldmap": true},
		Logger: logger,
	}
	_, _ = stargazers.Plugin.Run(context.Background(), pc)
	if !strings.Contains(buf.String(), "worldmap is not yet implemented") {
		t.Errorf("expected worldmap WARN log; got %q", buf.String())
	}
}

func TestRun_SkippedReasonMentionsRepositoryAccount(t *testing.T) {
	t.Parallel()
	r := runWith(t, nil)
	if !strings.Contains(r.SkippedReason, "repository") {
		t.Errorf("SkippedReason should mention repository account; got %q", r.SkippedReason)
	}
}

func TestRun_GoldenShape(t *testing.T) {
	r := &stargazers.Result{
		Skipped: true,
		List:    []stargazers.Stargazer{},
		Charts:  stargazers.StargazersCharts{Type: "classic", Series: []stargazers.ChartPoint{}},
	}
	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got = append(got, '\n')
	gp := filepath.Join(repoRoot(t), "tests", "golden", "json", "m4", "stargazers.json")
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
