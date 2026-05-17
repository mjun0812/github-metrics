package golden

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Compare diffs `got` against the golden file at
// tests/golden/<goldenPath>. On the `-update` flag (declared in
// tests/integration/output_json_test.go and shared by all callers),
// rewrites the golden file and passes. Without -update, fails the
// test on byte mismatch with a first-divergent-offset diff window.
func Compare(t *testing.T, got []byte, goldenPath string) {
	t.Helper()
	abs := resolveGoldenPath(t, goldenPath)
	if updateFlag() {
		writeGolden(t, abs, got)
		return
	}
	want, err := os.ReadFile(abs) //nolint:gosec // operator-controlled path under tests/golden/
	if err != nil {
		t.Fatalf("golden not seeded: %s (run with -update)", goldenPath)
	}
	if diffMessage(got, want); !equalBytes(got, want) {
		t.Errorf("%s", buildDriftMessage(goldenPath, got, want))
	}
}

// CompareSVG applies NormalizeSVG to both sides before byte-diffing.
// Used for SVG goldens that must tolerate Go-runtime / chromedp drift
// in non-semantic bytes (attribute order, whitespace inside text
// nodes, dynamic footer fragments).
func CompareSVG(t *testing.T, got []byte, goldenPath string) {
	t.Helper()
	abs := resolveGoldenPath(t, goldenPath)
	if updateFlag() {
		// Persist the raw (un-normalized) SVG so the golden file
		// stays human-reviewable. Normalization runs at comparison
		// time, so the comparison invariant holds regardless.
		writeGolden(t, abs, got)
		return
	}
	want, err := os.ReadFile(abs) //nolint:gosec // operator-controlled path under tests/golden/
	if err != nil {
		t.Fatalf("golden not seeded: %s (run with -update)", goldenPath)
	}
	gotNorm, err := NormalizeSVG(got)
	if err != nil {
		t.Fatalf("normalize got: %v", err)
	}
	wantNorm, err := NormalizeSVG(want)
	if err != nil {
		t.Fatalf("normalize want: %v", err)
	}
	if !equalBytes(gotNorm, wantNorm) {
		t.Errorf("%s", buildDriftMessage(goldenPath, gotNorm, wantNorm))
	}
}

// CompareJSON re-marshals both sides through json.MarshalIndent with
// the standard 2-space indent so per-key whitespace drift between
// encoders does not break the comparison.
func CompareJSON(t *testing.T, got []byte, goldenPath string) {
	t.Helper()
	abs := resolveGoldenPath(t, goldenPath)
	if updateFlag() {
		writeJSONGolden(t, abs, got)
		return
	}
	want, err := os.ReadFile(abs) //nolint:gosec // operator-controlled path under tests/golden/
	if err != nil {
		t.Fatalf("golden not seeded: %s (run with -update)", goldenPath)
	}
	gotPretty, err := reformatJSON(got)
	if err != nil {
		t.Fatalf("reformat got: %v", err)
	}
	wantPretty, err := reformatJSON(want)
	if err != nil {
		t.Fatalf("reformat want: %v", err)
	}
	if !equalBytes(gotPretty, wantPretty) {
		t.Errorf("%s", buildDriftMessage(goldenPath, gotPretty, wantPretty))
	}
}

// updateFlag reads the project-wide -update flag declared in
// tests/integration/output_json_test.go. Returns false when the flag
// is not registered (extreme isolation case — fail safe to compare
// mode so tests still detect drift).
func updateFlag() bool {
	f := flag.Lookup("update")
	if f == nil {
		return false
	}
	getter, ok := f.Value.(flag.Getter)
	if !ok {
		return false
	}
	v, ok := getter.Get().(bool)
	if !ok {
		return false
	}
	return v
}

// resolveGoldenPath joins the goldenPath against <repo-root>/tests/golden/.
func resolveGoldenPath(t *testing.T, goldenPath string) string {
	t.Helper()
	root, err := goldenRoot()
	if err != nil {
		t.Fatalf("golden: locate repo root: %v", err)
	}
	return filepath.Join(root, "tests", "golden", goldenPath)
}

// goldenRoot walks up from the working directory until it finds
// go.mod. Cached after first call.
var (
	goldenRootCache string
	goldenRootOnce  sync.Once
)

func goldenRoot() (string, error) {
	var initErr error
	goldenRootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			initErr = err
			return
		}
		for {
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				goldenRootCache = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				initErr = os.ErrNotExist
				return
			}
			dir = parent
		}
	})
	return goldenRootCache, initErr
}

func writeGolden(t *testing.T, abs string, got []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("golden mkdir: %v", err)
	}
	if err := os.WriteFile(abs, got, 0o600); err != nil {
		t.Fatalf("golden write: %v", err)
	}
	t.Logf("golden updated: %s (%d bytes)", abs, len(got))
}

// writeJSONGolden is writeGolden + json pretty-print so seeded JSON
// goldens are human-reviewable.
func writeJSONGolden(t *testing.T, abs string, got []byte) {
	t.Helper()
	pretty, err := reformatJSON(got)
	if err != nil {
		t.Fatalf("golden json reformat: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("golden mkdir: %v", err)
	}
	if err := os.WriteFile(abs, append(pretty, '\n'), 0o600); err != nil {
		t.Fatalf("golden write: %v", err)
	}
	t.Logf("golden updated: %s (%d bytes)", abs, len(pretty))
}

// reformatJSON parses then re-marshals via json.MarshalIndent with
// the project's standard 2-space indent. Trailing newline is dropped
// so the writeJSONGolden path can append exactly one.
func reformatJSON(raw []byte) ([]byte, error) {
	var box any
	if err := json.Unmarshal(raw, &box); err != nil {
		return nil, fmt.Errorf("reformat json: %w", err)
	}
	out, err := json.MarshalIndent(box, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffMessage is reserved for future stderr-side reporting — kept
// callable so the Compare body can be extended without breaking the
// signature.
func diffMessage(_, _ []byte) {}

// buildDriftMessage assembles the user-facing failure message per
// contracts/golden-compare.md §3. Returns a multi-line string.
func buildDriftMessage(goldenPath string, got, want []byte) string {
	offset := firstDivergentOffset(got, want)
	var b strings.Builder
	fmt.Fprintf(&b, "golden drift: tests/golden/%s\n", goldenPath)
	fmt.Fprintf(&b, "  first divergent byte at offset %d (got len=%d, want len=%d)\n",
		offset, len(got), len(want))
	fmt.Fprintf(&b, "    got  [%d:%d] = %s\n",
		clampLo(offset-40), clampHi(offset+40, len(got)),
		escapeWindow(got, offset-40, offset+40))
	fmt.Fprintf(&b, "    want [%d:%d] = %s\n",
		clampLo(offset-40), clampHi(offset+40, len(want)),
		escapeWindow(want, offset-40, offset+40))
	fmt.Fprintf(&b, "  (run with -update to seed)")
	return b.String()
}

func firstDivergentOffset(a, b []byte) int {
	min := len(a)
	if len(b) < min {
		min = len(b)
	}
	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min
}

func clampLo(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func clampHi(n, length int) int {
	if n > length {
		return length
	}
	if n < 0 {
		return 0
	}
	return n
}

// escapeWindow renders bytes[lo:hi] with non-printable bytes as \xNN
// escapes. Bounds are clamped to [0, len(bytes)].
func escapeWindow(b []byte, lo, hi int) string {
	lo = clampLo(lo)
	hi = clampHi(hi, len(b))
	if lo >= hi {
		return "(empty)"
	}
	var out strings.Builder
	for _, c := range b[lo:hi] {
		switch {
		case c == '\n':
			out.WriteString(`\n`)
		case c == '\r':
			out.WriteString(`\r`)
		case c == '\t':
			out.WriteString(`\t`)
		case c >= 0x20 && c <= 0x7E:
			out.WriteByte(c)
		default:
			fmt.Fprintf(&out, `\x%02x`, c)
		}
	}
	return out.String()
}
