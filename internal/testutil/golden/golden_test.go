package golden_test

import (
	"flag"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/golden"
)

// The shared -update flag is declared in
// tests/integration/output_json_test.go. For the package's own
// self-tests we declare a local stub so the helper has something to
// look up. We register at init time to mirror the cross-package
// import order of real tests.
func init() {
	if flag.Lookup("update") == nil {
		flag.Bool("update", false, "regenerate golden test fixtures from current output")
	}
}

// TestCompare_HappyPath_SameBytes invokes golden.Compare against a
// seeded byte-exact fixture under tests/golden/_testutil_selftest/.
// Without this self-exercise, Compare's path-resolution + read +
// byte-diff happy path would only be covered transitively via the
// integration suite.
func TestCompare_HappyPath_SameBytes(t *testing.T) {
	t.Parallel()
	golden.Compare(t, []byte("hello"), "_testutil_selftest/happy.txt")
}

func TestNormalizeSVG_AttrSortAndWhitespaceCollapse(t *testing.T) {
	t.Parallel()
	in := []byte(`<svg b="2" a="1">
  text  with    spaces
</svg>`)
	out, err := golden.NormalizeSVG(in)
	if err != nil {
		t.Fatalf("NormalizeSVG: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `a="1"`) || !strings.Contains(s, `b="2"`) {
		t.Errorf("attributes missing: %q", s)
	}
	if !strings.Contains(s, "text with spaces") {
		t.Errorf("whitespace not collapsed: %q", s)
	}
	// `a` attribute should come before `b` after sorting.
	if strings.Index(s, `a="1"`) > strings.Index(s, `b="2"`) {
		t.Errorf("attributes not alphabetically sorted: %q", s)
	}
}

func TestNormalizeSVG_MasksDynamicFooter(t *testing.T) {
	t.Parallel()
	in := []byte(`<svg><text>Last updated 2026-05-18T14:00:00Z</text><text>github-metrics@v1.2.3</text></svg>`)
	out, _ := golden.NormalizeSVG(in)
	s := string(out)
	if !strings.Contains(s, "Last updated __MASKED__") {
		t.Errorf("dynamic timestamp not masked: %q", s)
	}
	if !strings.Contains(s, "github-metrics@__MASKED__") {
		t.Errorf("dynamic version not masked: %q", s)
	}
}

// TestCompareJSON_KeyOrderingTolerant verifies CompareJSON survives
// JSON encoders that emit keys in different orders. The seed file at
// _testutil_selftest/keyordering.json is `{"a":1,"b":2}` (a-first).
// We pass a b-first input — CompareJSON must reformat both sides
// through MarshalIndent (which sorts map keys alphabetically) so the
// comparison passes despite the divergent input ordering.
func TestCompareJSON_KeyOrderingTolerant(t *testing.T) {
	t.Parallel()
	golden.CompareJSON(t, []byte(`{"b":2,"a":1}`), "_testutil_selftest/keyordering.json")
}
