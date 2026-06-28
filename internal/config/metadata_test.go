package config_test

import (
	"bytes"
	"log/slog"
	"os"
	"sort"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
)

// adoptedPlugins is the source of truth for the M1 plugin set; it must
// stay in sync with scripts/sync-assets.sh and
// docs/design/15-selection-answer.md §6.4.
var adoptedPlugins = []string{
	"base",
	"core",
	"header",
	"languages", "activity", "achievements", "repositories",
	"isocalendar", "calendar", "habits", "stars", "topics", "starlists",
	"people", "notable", "contributors", "reactions", "projects",
	"sponsors", "sponsorships", "stargazers", "traffic",
}

var adoptedTemplates = []string{"classic", "repository"}

func TestLoad_AllAdoptedPluginsLoadFromAssets(t *testing.T) {
	t.Parallel()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up two levels (internal/config → repo root) to find assets/.
	assetsRoot := bytes.NewBufferString("") // placeholder for diagnostics
	_ = assetsRoot
	fsys := os.DirFS(root + "/../../assets")

	ml, err := config.LoadMetadata(fsys, config.LoadOptions{})
	if err != nil {
		t.Fatalf("Load(assets): %v", err)
	}

	got := ml.PluginNames()
	want := append([]string(nil), adoptedPlugins...)
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Fatalf("PluginNames mismatch\n got: %v\nwant: %v", got, want)
	}

	gotT := ml.TemplateNames()
	wantT := append([]string(nil), adoptedTemplates...)
	sort.Strings(wantT)
	if !equalStrings(gotT, wantT) {
		t.Fatalf("TemplateNames mismatch\n got: %v\nwant: %v", gotT, wantT)
	}

	for _, name := range adoptedPlugins {
		pm := ml.Plugins[name]
		if pm == nil {
			t.Fatalf("plugin %s missing", name)
		}
		if pm.Name == "" {
			t.Errorf("plugin %s: empty Name", name)
		}
	}

	for _, name := range adoptedTemplates {
		tm := ml.Templates[name]
		if tm == nil {
			t.Fatalf("template %s missing", name)
		}
		if tm.Name == "" {
			t.Errorf("template %s: empty Name", name)
		}
		if len(tm.Formats) == 0 {
			t.Errorf("template %s: empty Formats", name)
		}
	}
}

func TestLoad_LoadsActionAndVersion(t *testing.T) {
	t.Parallel()

	mem := fstest.MapFS{
		"plugins/base/metadata.yml":      {Data: []byte("name: base plugin\ncategory: core\n")},
		"templates/classic/metadata.yml": {Data: []byte("name: classic\nformats:\n  - svg\n")},
		"action.yml": {Data: []byte(`
name: metrics-action
description: GitHub Action
author: mjun0812
inputs:
  base:
    description: Base content
    default: header, activity
  user:
    description: GitHub login
    required: true
`)},
		"version.txt": {Data: []byte("upstream:3.35.0-beta\n")},
	}

	ml, err := config.LoadMetadata(mem, config.LoadOptions{
		ActionPath:  "action.yml",
		VersionPath: "version.txt",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ml.Action == nil {
		t.Fatalf("Action not parsed")
	}
	if ml.Action.Name != "metrics-action" {
		t.Errorf("Action.Name = %q", ml.Action.Name)
	}
	if ml.Action.Inputs["user"].Required != true {
		t.Errorf("Action.Inputs[user].Required = %v, want true",
			ml.Action.Inputs["user"].Required)
	}
	if ml.Package == nil || ml.Package.UpstreamVersion != "upstream:3.35.0-beta" {
		t.Errorf("Package.UpstreamVersion = %+v", ml.Package)
	}
}

func TestLoad_WarnsAndDropsUnknownInputType(t *testing.T) {
	t.Parallel()

	mem := fstest.MapFS{
		"plugins/sample/metadata.yml": {Data: []byte(`
name: sample
inputs:
  good:
    type: string
    default: a
  weird:
    type: unobtainium
    default: x
`)},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	ml, err := config.LoadMetadata(mem, config.LoadOptions{Logger: logger})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	pm := ml.Plugins["sample"]
	if _, ok := pm.Inputs["weird"]; ok {
		t.Errorf("unknown input was kept; should be dropped")
	}
	if _, ok := pm.Inputs["good"]; !ok {
		t.Errorf("known input was dropped")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"input":"weird"`)) {
		t.Errorf("warn log missing for unknown input: %s", buf.String())
	}
}

func TestLoad_MetadataLoadsUnder200ms(t *testing.T) {
	t.Parallel()

	if raceEnabled {
		t.Skip("race detector overhead invalidates the 200ms budget (SC-003); covered by the non-race test jobs")
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	fsys := os.DirFS(root + "/../../assets")

	start := time.Now()
	if _, err := config.LoadMetadata(fsys, config.LoadOptions{}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("metadata load took %v, want < 200ms (SC-003)", elapsed)
	}
	t.Logf("metadata load elapsed: %v", elapsed)
}

func TestExtras(t *testing.T) {
	t.Parallel()

	mem := fstest.MapFS{
		"plugins/scope/metadata.yml": {Data: []byte(`
name: scope
inputs:
  input_a:
    type: string
    extras: ["metrics.cpu.overuse"]
  input_b:
    type: string
`)},
	}
	ml, err := config.LoadMetadata(mem, config.LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !ml.Extras("scope", "metrics.cpu.overuse") {
		t.Errorf("Extras did not find feature")
	}
	if ml.Extras("scope", "metrics.api.overuse") {
		t.Errorf("Extras matched a feature that is not declared")
	}
	if ml.Extras("unknown", "anything") {
		t.Errorf("Extras matched on a non-existent scope")
	}
}

func TestLoad_RejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	mem := fstest.MapFS{
		"plugins/broken/metadata.yml": {Data: []byte("name: [unterminated")},
	}
	if _, err := config.LoadMetadata(mem, config.LoadOptions{}); err == nil {
		t.Fatalf("Load returned no error on malformed YAML")
	}
}
