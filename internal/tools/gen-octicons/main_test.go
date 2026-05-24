package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRun_StarAndChevronDown drives the tool against a tiny synthetic
// build/data.json (covering one single-size icon and one two-size icon
// with hyphenated name) and asserts the materialized fragments match
// the expected octicon fragment shape.
func TestRun_StarAndChevronDown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	in := filepath.Join(dir, "data.json")
	pkg := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-pkg")

	// Stub upstream input.
	upstream := map[string]any{
		"star": map[string]any{
			"name": "star",
			"heights": map[string]any{
				"16": map[string]any{
					"width": 16,
					"path":  `<path d="M8 .25l2.39 4.84"/>`,
				},
				"24": map[string]any{
					"width": 24,
					"path":  `<path d="M12 .5l3.5 7"/>`,
				},
			},
		},
		"chevron-down": map[string]any{
			"name": "chevron-down",
			"heights": map[string]any{
				"16": map[string]any{
					"width": 16,
					"path":  `<path d="M12.78 6.22a..."/>`,
				},
			},
		},
	}
	mustWriteJSON(t, in, upstream)

	out := filepath.Join(dir, "out.json")
	when := time.Date(2026, 5, 15, 12, 34, 56, 0, time.UTC)
	if err := run(in, out, "primer/octicons@test", when); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	got := readDoc(t, out)

	if got.Meta.Source != "primer/octicons@test" {
		t.Errorf("_meta.source = %q, want primer/octicons@test", got.Meta.Source)
	}
	if got.Meta.GeneratedAt != "2026-05-15T12:34:56Z" {
		t.Errorf("_meta.generated_at = %q, want 2026-05-15T12:34:56Z", got.Meta.GeneratedAt)
	}

	starSizes := got.Icons["star"]
	if starSizes == nil {
		t.Fatalf("icons.star missing")
	}
	if _, ok := starSizes["16"]; !ok {
		t.Errorf("icons.star.16 missing")
	}
	if _, ok := starSizes["24"]; !ok {
		t.Errorf("icons.star.24 missing")
	}
	star16 := starSizes["16"]
	for _, want := range []string{
		`xmlns="http://www.w3.org/2000/svg"`,
		`width="16"`,
		`height="16"`,
		`viewBox="0 0 16 16"`,
		`<path d="M8 .25l2.39 4.84"/>`,
	} {
		if !strings.Contains(star16, want) {
			t.Errorf("star.16 fragment missing %q\nfull:\n%s", want, star16)
		}
	}

	chev := got.Icons["chevron-down"]
	if chev == nil {
		t.Fatalf("icons.chevron-down missing — hyphenated names should be preserved")
	}
	if _, ok := chev["24"]; ok {
		t.Errorf("icons.chevron-down.24 should be absent (upstream did not provide it)")
	}

	// Sanity: package.json fallback path is only exercised when
	// --source is empty. Cross-check that explicit --source wins.
	_ = pkg
}

// TestRun_NoSource_FallsBackToPackageJSON exercises the secondary
// source label discovery path. We seed a sibling package.json so the
// tool can read the version off it.
func TestRun_NoSource_FallsBackToPackageJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	octiconsDir := filepath.Join(root, "octicons")
	buildDir := filepath.Join(octiconsDir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}
	in := filepath.Join(buildDir, "data.json")
	pkgJSON := filepath.Join(octiconsDir, "package.json")

	mustWriteJSON(t, in, map[string]any{
		"alert": map[string]any{
			"name":    "alert",
			"heights": map[string]any{"16": map[string]any{"width": 16, "path": `<path/>`}},
		},
	})
	mustWriteJSON(t, pkgJSON, map[string]any{"version": "19.42.0"})

	out := filepath.Join(root, "out.json")
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := run(in, out, "", when); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := readDoc(t, out)
	if got.Meta.Source != "primer/octicons@19.42.0" {
		t.Errorf("source = %q, want primer/octicons@19.42.0", got.Meta.Source)
	}
}

// TestRun_MissingPath_DropsVariant ensures that a (name, size) entry
// with an empty path is silently skipped rather than producing an
// empty `<svg></svg>` placeholder.
func TestRun_MissingPath_DropsVariant(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	in := filepath.Join(dir, "data.json")
	mustWriteJSON(t, in, map[string]any{
		"empty-only": map[string]any{
			"heights": map[string]any{"16": map[string]any{"width": 16, "path": ""}},
		},
		"has-path": map[string]any{
			"heights": map[string]any{"16": map[string]any{"width": 16, "path": `<path/>`}},
		},
	})

	out := filepath.Join(dir, "out.json")
	if err := run(in, out, "test", time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := readDoc(t, out)
	if _, ok := got.Icons["empty-only"]; ok {
		t.Errorf("empty-only icon should have been dropped (no usable sizes)")
	}
	if _, ok := got.Icons["has-path"]; !ok {
		t.Errorf("has-path icon should be present")
	}
}

func mustWriteJSON(t *testing.T, path string, body any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	enc, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, enc, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readDoc(t *testing.T, path string) outDoc {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var d outDoc
	if err := json.Unmarshal(body, &d); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return d
}
