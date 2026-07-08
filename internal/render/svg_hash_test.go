package render

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TestHash_EmptyString covers the empty-input contract: blank-only
// input returns ("", nil) per docs/design/13-appendix.md §H.
func TestHash_EmptyString(t *testing.T) {
	t.Parallel()
	tests := []string{"", "   ", "\n\t  \r\n"}
	for _, in := range tests {
		got, err := Hash(in)
		if err != nil {
			t.Errorf("Hash(%q) err = %v, want nil", in, err)
		}
		if got != "" {
			t.Errorf("Hash(%q) = %q, want empty", in, got)
		}
	}
}

// TestHash_NoSVGRoot ensures non-SVG documents surface as
// *xerrors.InputError so callers can detect malformed input via
// errors.As.
func TestHash_NoSVGRoot(t *testing.T) {
	t.Parallel()
	got, err := Hash(`<html><body>no svg here</body></html>`)
	if got != "" {
		t.Errorf("Hash = %q, want empty", got)
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Errorf("err type = %T, want *xerrors.InputError", err)
	}
}

// TestHash_FooterRemoved is the SC-004 anchor: two SVGs whose only
// DOM difference lives inside the first <footer> MUST produce the
// same hash.
func TestHash_FooterRemoved(t *testing.T) {
	t.Parallel()
	a := `<svg xmlns="http://www.w3.org/2000/svg"><g class="body"/><footer>generated 2026-01-01</footer></svg>`
	b := `<svg xmlns="http://www.w3.org/2000/svg"><g class="body"/><footer>generated 2026-05-15</footer></svg>`
	ha, err := Hash(a)
	if err != nil {
		t.Fatalf("Hash(a): %v", err)
	}
	hb, err := Hash(b)
	if err != nil {
		t.Fatalf("Hash(b): %v", err)
	}
	if ha != hb {
		t.Errorf("Hash(a)=%q != Hash(b)=%q (only footer differs)", ha, hb)
	}
	if len(ha) != 32 {
		t.Errorf("Hash length = %d, want 32 hex chars", len(ha))
	}
}

// TestHash_MetadataSectionRemoved is the #409 Phase C anchor: the footer
// is now a native-SVG `<g data-section="metadata">` block (no HTML
// `<footer>`), so two SVGs whose only difference is the timestamp-bearing
// metadata group MUST still hash identically (data-changed detection).
func TestHash_MetadataSectionRemoved(t *testing.T) {
	t.Parallel()
	a := `<svg xmlns="http://www.w3.org/2000/svg"><g data-section="header"><text>x</text></g><g data-section="metadata"><svg height="21"><text>Last updated 2026-01-01T00:00:00Z with x@v1</text></svg></g></svg>`
	b := `<svg xmlns="http://www.w3.org/2000/svg"><g data-section="header"><text>x</text></g><g data-section="metadata"><svg height="21"><text>Last updated 2026-05-15T12:34:56Z with x@v1</text></svg></g></svg>`
	ha, err := Hash(a)
	if err != nil {
		t.Fatalf("Hash(a): %v", err)
	}
	hb, err := Hash(b)
	if err != nil {
		t.Fatalf("Hash(b): %v", err)
	}
	if ha != hb {
		t.Errorf("Hash(a)=%q != Hash(b)=%q (only metadata timestamp differs)", ha, hb)
	}
}

// TestHash_DOMDifference asserts non-footer DOM changes produce
// different hashes (the prior test would be meaningless without this
// complement).
func TestHash_DOMDifference(t *testing.T) {
	t.Parallel()
	a := `<svg xmlns="http://www.w3.org/2000/svg"><g class="x"/></svg>`
	b := `<svg xmlns="http://www.w3.org/2000/svg"><g class="y"/></svg>`
	ha, _ := Hash(a)
	hb, _ := Hash(b)
	if ha == hb {
		t.Errorf("hashes should differ; got %q for both", ha)
	}
}

// TestHash_MultipleFooters verifies the "only the first <footer> is
// stripped" rule.
func TestHash_MultipleFooters(t *testing.T) {
	t.Parallel()
	a := `<svg xmlns="http://www.w3.org/2000/svg"><footer>first-A</footer><footer>second</footer></svg>`
	b := `<svg xmlns="http://www.w3.org/2000/svg"><footer>first-B</footer><footer>second</footer></svg>`
	ha, _ := Hash(a)
	hb, _ := Hash(b)
	if ha != hb {
		t.Errorf("only the first <footer> should be removed; got %q vs %q", ha, hb)
	}

	c := `<svg xmlns="http://www.w3.org/2000/svg"><footer>first</footer><footer>second-1</footer></svg>`
	d := `<svg xmlns="http://www.w3.org/2000/svg"><footer>first</footer><footer>second-2</footer></svg>`
	hc, _ := Hash(c)
	hd, _ := Hash(d)
	if hc == hd {
		t.Errorf("second footer is load-bearing; hashes should differ but got %q for both", hc)
	}
}

// TestHash_GoldenOctocat anchors the M3 implementation against the M2
// classic golden so a refactor of the impl detail (parser, encoder,
// whitespace handling) does not silently drift the output. The
// expected hex lives in tests/golden/render/octocat_svg_hash.txt;
// run `go test -run TestHash_GoldenOctocat -update ./internal/render`
// to regenerate after intentional changes.
//
// Note: this golden is coupled to tests/golden/classic/octocat.svg.
// Regenerating that file MUST also regenerate octocat_svg_hash.txt.
func TestHash_GoldenOctocat(t *testing.T) {
	t.Parallel()
	classicPath := repoRel(t, "tests/golden/classic/octocat.svg")
	svg, err := os.ReadFile(classicPath)
	if err != nil {
		t.Fatalf("read %s: %v", classicPath, err)
	}
	got, err := Hash(string(svg))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// Derive the golden path from the M2 SVG path: replace
	// "tests/golden/classic/octocat.svg" with
	// "tests/golden/render/octocat_svg_hash.txt". This lets the
	// UPDATE_GOLDEN branch create the file on first run without
	// requiring repoRel to find a not-yet-existing target.
	goldenDir := filepath.Join(filepath.Dir(filepath.Dir(classicPath)), "render")
	goldenPath := filepath.Join(goldenDir, "octocat_svg_hash.txt")
	if envIsSet("UPDATE_GOLDEN") {
		if mkErr := os.MkdirAll(goldenDir, 0o750); mkErr != nil {
			t.Fatalf("mkdir golden dir: %v", mkErr)
		}
		if writeErr := os.WriteFile(goldenPath, []byte(got+"\n"), 0o600); writeErr != nil {
			t.Fatalf("write golden: %v", writeErr)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with UPDATE_GOLDEN=1 go test ...)", goldenPath, err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Errorf("Hash drift\n got:  %s\n want: %s\nregenerate with UPDATE_GOLDEN=1 go test ./internal/render -run TestHash_GoldenOctocat",
			got, strings.TrimSpace(string(want)))
	}
}

// envIsSet reports whether the named env var is set to a non-empty
// value. Pulled into a helper so the assertion stays readable when
// other golden tests adopt the same mechanism.
func envIsSet(key string) bool {
	v, ok := os.LookupEnv(key)
	return ok && v != ""
}

// repoRel resolves a project-rooted relative path by walking upward
// from CWD. Mirrors the helper in svg_resize_chromedp_test.go (which
// lives behind a build tag, so we duplicate here for the non-tag
// test file).
func repoRel(t *testing.T, relPath string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, relPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above CWD %s", relPath, dir)
		}
		dir = parent
	}
}
