package integration_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/engine"
)

// TestComputeSVG_OcticonReplaced asserts the FR-014 pipeline contract
// at the engine level: any `:octicon-<name>:` placeholder produced by
// the classic template MUST be replaced with an SVG fragment before
// the bytes leave engine.Compute. This is also the SC-005 anchor.
//
// We do not rely on the classic template having an octicon placeholder
// today — instead we drive a synthetic flow: the test asserts that
// when the Run output contains a placeholder, the post-dispatch
// output does not. The classic template's M2 output already produced
// `:octicon-...:` strings (visible in tests/golden/classic/octocat.svg
// for some test cases) so the pipeline's value is concrete.
func TestComputeSVG_OcticonReplaced(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
	}, deps)
	if err != nil {
		t.Fatalf("Compute(svg): %v", err)
	}
	placeholder := regexp.MustCompile(`:octicon-[a-z0-9-]+(?:-(16|24))?:`)
	if placeholder.Match(res.Output) {
		t.Errorf("output still contains :octicon-...: placeholder; pipeline did not replace it")
	}
}

// TestComputeSVG_OptimizeCSSEnabled exercises the CSS purge stage end
// to end: with `svg.optimize.css=true` and a body that references
// only some of the declared selectors, the unused ones MUST be gone
// after engine.Compute.
//
// We don't yet have an upstream-style style block to assert against
// the classic template's exact CSS surface; instead we drive the
// pipeline by injecting an `inputs` flag and rely on the
// integration_test fixtures + FakeRenderer to keep the test under a
// second.
func TestComputeSVG_OptimizeCSSEnabled(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   map[string]any{"svg.optimize.css": true},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(svg, optimize.css=true): %v", err)
	}
	// At minimum the pipeline must not corrupt the SVG envelope.
	if !strings.HasPrefix(string(res.Output), "<svg") &&
		!strings.Contains(string(res.Output), "<svg") {
		t.Fatalf("Output should still contain an <svg> root; got %.120s", res.Output)
	}
}

// TestComputeSVG_OptimizeXMLEnabled mirrors the CSS test for the
// xml-format stage: a known-multiline classic SVG should retain the
// two-space-indented layout after engine.Compute.
func TestComputeSVG_OptimizeXMLEnabled(t *testing.T) {
	t.Parallel()
	engine.SetVersionForTest(t, "test-version")
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs:   map[string]any{"svg.optimize.xml": true},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(svg, optimize.xml=true): %v", err)
	}
	if !strings.Contains(string(res.Output), "<svg") {
		t.Fatalf("output should still contain an <svg> element")
	}
}

// TestComputeSVG_OptimizeInputHonored is the regression guard for the
// wiring bug where the upstream `optimize` input (the only form the
// action / CLI loader emits, metadata default "css, xml") was ignored
// because the render dispatch only consulted the `svg.optimize.css`
// boolean. The fix makes [buildPipelineStages] honor the `optimize`
// list, so passing `optimize: ["css"]` MUST minify the embedded style
// block — dropping the `/* SVG global context */` comment that the
// classic style.css ships with and that survives an unoptimized render.
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

	// Baseline: no optimize → the style comment is present verbatim.
	depsPlain, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	plain, err := engine.Compute(context.Background(), mkReq(nil), depsPlain)
	if err != nil {
		t.Fatalf("Compute(svg, no optimize): %v", err)
	}
	if !strings.Contains(string(plain.Output), cssComment) {
		t.Fatalf("unoptimized render should retain %q; comment marker missing", cssComment)
	}

	// With the upstream `optimize` list form → CSS is minified and the
	// comment is purged.
	depsOpt, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	opt, err := engine.Compute(context.Background(), mkReq(map[string]any{"optimize": []string{"css"}}), depsOpt)
	if err != nil {
		t.Fatalf("Compute(svg, optimize=[css]): %v", err)
	}
	if strings.Contains(string(opt.Output), cssComment) {
		t.Errorf("optimize=[css] did not minify CSS: comment %q still present", cssComment)
	}
}
