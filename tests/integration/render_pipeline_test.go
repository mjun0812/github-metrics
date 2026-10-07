package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// TestComputeSVG_OptimizeInputHonored is the regression guard for the
// wiring bug where the upstream `optimize` input (the only form the
// action / CLI loader emits, metadata default "css, xml") was ignored
// because the render dispatch only consulted the `svg.optimize.css`
// boolean. The fix makes [buildPipelineStages] honor the `optimize`
// list, so the css pass MUST run for every shape the input can take —
// a normalized []string, a raw string, and the comma-separated /
// whitespace-padded multi-pass string that arrives straight from
// INPUT_OPTIMIZE. Each must drop the `/* SVG global context */` comment
// that the classic style.css ships with and that survives an
// unoptimized render; the css-only forms must also shrink the output.
func TestComputeSVG_OptimizeInputHonored(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")

	const cssComment = "/* SVG global context */"
	mkReq := func(inputs map[string]any) engine.Request {
		return engine.Request{
			Login:    "octocat",
			Template: "classic",
			Format:   "svg",
			Inputs:   inputs,
		}
	}
	compute := func(t *testing.T, inputs map[string]any) []byte {
		t.Helper()
		deps, _ := newEngineDeps(t, map[string]string{
			"User":             userOctocat,
			"UserRepositories": userRepositories250,
		})
		res, err := engine.Compute(context.Background(), mkReq(inputs), deps)
		if err != nil {
			t.Fatalf("Compute(svg, inputs=%v): %v", inputs, err)
		}
		return res.Output
	}

	// Baseline: no optimize → the style comment is present verbatim and
	// the CSS ships expanded.
	plain := compute(t, nil)
	if !strings.Contains(string(plain), cssComment) {
		t.Fatalf("unoptimized render should retain %q; comment marker missing", cssComment)
	}

	cases := []struct {
		name     string
		optimize any
		// shrinks is asserted only for css-only passes; a pass list that
		// also re-indents via the xml formatter can offset the css size
		// win, so output length is not compared there.
		shrinks bool
	}{
		{"normalized list", []string{"css"}, true},
		{"comma-separated multi-pass", "css,xml", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := compute(t, map[string]any{"optimize": tc.optimize})
			if strings.Contains(string(out), cssComment) {
				t.Errorf("optimize=%#v did not minify CSS: comment %q still present", tc.optimize, cssComment)
			}
			if tc.shrinks && len(out) >= len(plain) {
				t.Errorf("optimize=%#v should shrink output: got %d bytes, baseline %d", tc.optimize, len(out), len(plain))
			}
		})
	}
}
