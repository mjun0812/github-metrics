// Package integration_test — #409 Phase C structural invariants.
//
// After the outer foreignObject was dropped and the templates started
// computing the total height in Go, every generated SVG must be pure,
// self-sized SVG: no `<foreignObject>`, no `height="99999"` placeholder,
// no `#metrics-end` anchor, and a root `<svg height>` that exactly equals
// the sum of the stacked per-section heights.
package integration_test

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"

	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
	_ "github.com/mjun0812/github-metrics/internal/templates/repository"
)

// rootSVGHeightRe captures the height on the outermost <svg> element.
var rootSVGHeightRe = regexp.MustCompile(`^<svg [^>]*\bheight="(\d+)"`)

// sectionSVGHeightRe captures the height of each per-section nested <svg>,
// i.e. the WrapSection block `<g data-section="X" ...><svg ... height="N" ...>`.
// The plugin dispatcher's extra `<g class="plugin-X" ...>` wrapper sits
// outside the data-section group, so this still matches the section svg.
var sectionSVGHeightRe = regexp.MustCompile(`data-section="[^"]*"[^>]*><svg [^>]*\bheight="(\d+)"`)

// assertPhaseCSVG runs the shared structural checks: no foreignObject /
// placeholder / anchor, and root height == sum of section heights.
func assertPhaseCSVG(t *testing.T, svg string) {
	t.Helper()
	if strings.Contains(svg, "<foreignObject") {
		t.Errorf("generated SVG still contains a <foreignObject>")
	}
	if strings.Contains(svg, `height="99999"`) {
		t.Errorf(`generated SVG still contains the height="99999" placeholder`)
	}
	if strings.Contains(svg, `id="metrics-end"`) {
		t.Errorf(`generated SVG still contains the #metrics-end measurement anchor`)
	}

	m := rootSVGHeightRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("could not find root <svg height>; first 120 bytes: %.120s", svg)
	}
	rootH, _ := strconv.Atoi(m[1])

	sum := 0
	sm := sectionSVGHeightRe.FindAllStringSubmatch(svg, -1)
	if len(sm) == 0 {
		t.Fatalf("no data-section blocks found in SVG")
	}
	for _, g := range sm {
		n, _ := strconv.Atoi(g[1])
		sum += n
	}
	if rootH != sum {
		t.Errorf("root <svg height=%d> != sum of %d section heights (%d)", rootH, len(sm), sum)
	}
}

// TestPhaseC_ClassicSVG_SelfSized asserts the classic template emits a
// foreignObject-free, Go-sized SVG whose root height equals the summed
// section heights (header + activity/community + repositories + metadata).
func TestPhaseC_ClassicSVG_SelfSized(t *testing.T) {
	engine.SetVersionForTest(t, "test-version")
	restore := header.SetNowForTest(func() time.Time {
		return time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	})
	t.Cleanup(restore)

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "svg",
		Inputs: map[string]any{
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
			"plugin_base":         "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	assertPhaseCSVG(t, string(res.Output))
}

// TestPhaseC_RepositorySVG_SelfSized asserts the same invariants for the
// repository template (header + introduction + metadata).
func TestPhaseC_RepositorySVG_SelfSized(t *testing.T) {
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "svg",
		Inputs: map[string]any{
			"user":                "octocat",
			"repo":                "hello-world",
			"chrome_header":       "yes",
			"chrome_introduction": "yes",
			"plugin_introduction": "yes",
			"chrome_metadata":     "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	assertPhaseCSVG(t, string(res.Output))
}
