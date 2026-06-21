//go:build chromedp

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/render"

	// Side-effect import: registers classic with templates registry.
	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

// repoRel resolves the path of a project-rooted file by walking up
// from CWD until it finds the entry. Mirrors the helper used by the
// internal/render chromedp tests.
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

// TestComputePNG_E2E runs the full engine.Compute path through a real
// chromedp Browser (not the FakeRenderer) and asserts the produced
// PNG decodes with a sensible bounding box. The expected dimensions
// are loaded from the golden range file so the assertion stays
// permissive against layout drift.
func TestComputePNG_E2E(t *testing.T) {
	engine.SetVersionForTest(t, "test-version")

	// Construct a real Browser. Skip when chromium is unreachable so
	// the test stays green on developer machines that opt into the
	// chromedp tag without installing chromium.
	browser, err := render.New(render.BrowserOpts{})
	if err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	deps.Render = browser

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := engine.Compute(ctx, engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "png",
		// #602: header is now a regular plugin; enable it so the rendered
		// PNG contains the identity block and the height check passes.
		Inputs: map[string]any{"plugin_header": "yes"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Fatalf("MIME = %q, want image/png", res.MIME)
	}
	img, err := png.Decode(bytes.NewReader(res.Output))
	if err != nil {
		t.Fatalf("png.Decode: %v (output length %d)", err, len(res.Output))
	}

	// Load the dimension range golden to keep this assertion stable
	// against layout tweaks while still catching regressions like a
	// 1x1 placeholder leaking through.
	rangeBody, err := os.ReadFile(repoRel(t, "tests/golden/render/chromedp_height_octocat.json"))
	if err != nil {
		t.Fatalf("read range golden: %v", err)
	}
	var rng struct {
		WidthMin  int `json:"width_min"`
		WidthMax  int `json:"width_max"`
		HeightMin int `json:"height_min"`
		HeightMax int `json:"height_max"`
	}
	if jsonErr := json.Unmarshal(rangeBody, &rng); jsonErr != nil {
		// The golden file embeds an _comment field which json.Unmarshal
		// safely ignores; explicit decode failures are unexpected.
		t.Fatalf("decode range golden: %v", jsonErr)
	}
	bnds := img.Bounds()
	if bnds.Dx() < rng.WidthMin || bnds.Dx() > rng.WidthMax {
		t.Errorf("Width = %d, want range [%d, %d]", bnds.Dx(), rng.WidthMin, rng.WidthMax)
	}
	if bnds.Dy() < rng.HeightMin || bnds.Dy() > rng.HeightMax {
		t.Errorf("Height = %d, want range [%d, %d]", bnds.Dy(), rng.HeightMin, rng.HeightMax)
	}
}
