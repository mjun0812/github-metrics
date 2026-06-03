// Package content holds the content-level verification suite that
// closes the gap behind issues #463–#472. Unlike the byte-golden
// suite (circular: goldens were generated from the implementation
// under test) and the height/file-size checks (too coarse to notice
// a missing label or a wrong number), these tests parse the rendered
// SVG and assert on the *visible content a viewer actually sees*.
//
// Three layers, increasing specificity:
//
//   - density_test.go    (D) every example SVG must not be an empty
//     card — a cheap blanket safety net for the "rendered nothing"
//     bug class (#472).
//   - dom_contract_test.go (B) per-plugin required semantic elements
//     ("files changed", "License", "Language activity", …), asserted
//     even for plugins that have no upstream reference fixture.
//   - reference_parity_test.go (A) compares our output against the
//     real upstream output vendored under docs/reference_examples/ —
//     the only non-circular ground truth in the repo.
//
// The suite reads the committed artifacts under docs/examples/ and
// docs/reference_examples/ directly, so it gates exactly the files
// the issues name and needs neither chromium nor network.
package content

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the absolute path of the repository root,
// resolved relative to this source file so the suite is
// CWD-independent.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(this), "..", "..")
}

// readExample reads docs/examples/<name> (e.g. "plugin-people.svg").
func readExample(t *testing.T, name string) []byte {
	t.Helper()
	return readRepo(t, filepath.Join("docs", "examples", name))
}

// readReference reads docs/reference_examples/<name>
// (e.g. "metrics.plugin.people.svg").
func readReference(t *testing.T, name string) []byte {
	t.Helper()
	return readRepo(t, filepath.Join("docs", "reference_examples", name))
}

func readRepo(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel)) //nolint:gosec // fixed test-fixture path under the repo
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

// exampleSVGs lists every committed example SVG under docs/examples/.
func exampleSVGs(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "docs", "examples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".svg" {
			out = append(out, e.Name())
		}
	}
	return out
}
