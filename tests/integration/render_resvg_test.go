package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render"
)

// withResvg constructs a *render.Resvg or skips the test when the binary
// is unavailable. Resolution mirrors render.ResvgOpts.normalize
// (METRICS_RESVG_PATH → PATH), so the default `go test` run stays green
// on machines without resvg while the CI test-resvg job (which installs
// the binary) exercises the real rasterization.
func withResvg(t *testing.T) *render.Resvg {
	t.Helper()
	if os.Getenv("METRICS_RESVG_PATH") == "" {
		if _, err := exec.LookPath("resvg"); err != nil {
			t.Skip("resvg binary unavailable: set METRICS_RESVG_PATH or add resvg to PATH")
		}
	}
	r, err := render.NewResvg(render.ResvgOpts{})
	if err != nil {
		t.Skipf("resvg unavailable: %v", err)
	}
	return r
}

// repoFile resolves a project-rooted path by walking up from CWD.
func repoFile(t *testing.T, relPath string) string {
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

// TestComputePNG_E2E_Resvg runs the full engine.Compute path through a
// real resvg renderer (not the FakeRenderer) and asserts the produced
// PNG decodes with a sensible bounding box. The expected dimensions come
// from the golden range file so the assertion stays permissive against
// layout drift while catching a 1x1 placeholder leaking through.
func TestComputePNG_E2E_Resvg(t *testing.T) {
	engine.SetVersionForTest(t, "test-version")

	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
	})
	deps.Render = withResvg(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := engine.Compute(ctx, engine.Request{
		Login:    "octocat",
		Template: "classic",
		Format:   "png",
		// #602: header is an opt-in plugin; without plugin_header the PNG
		// collapses to a near-empty placeholder. This range golden expects
		// the identity card to be present.
		Inputs: map[string]any{"plugin_header": "yes"},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Fatalf("MIME = %q, want image/png", res.MIME)
	}
	if !bytes.HasPrefix(res.Output, []byte("\x89PNG")) {
		t.Fatalf("Output missing PNG magic number; got %x", res.Output[:min(8, len(res.Output))])
	}
	img, err := png.Decode(bytes.NewReader(res.Output))
	if err != nil {
		t.Fatalf("png.Decode: %v (output length %d)", err, len(res.Output))
	}

	rangeBody, err := os.ReadFile(repoFile(t, "tests/golden/render/render_height_octocat.json"))
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

// TestRepositoryTemplate_PNG_Resvg exercises the repository template
// through the resvg pipeline and asserts the produced bytes are a
// decodable PNG (magic number + image/png MIME).
func TestRepositoryTemplate_PNG_Resvg(t *testing.T) {
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	deps.Render = withResvg(t)

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "png",
		Inputs: map[string]any{
			"user":                "octocat",
			"repo":                "hello-world",
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Errorf("MIME = %q, want image/png", res.MIME)
	}
	if !bytes.HasPrefix(res.Output, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		t.Fatalf("PNG signature missing; got %x", res.Output[:min(8, len(res.Output))])
	}
	if _, err := png.Decode(bytes.NewReader(res.Output)); err != nil {
		t.Errorf("png.Decode: %v", err)
	}
}

// TestRepositoryTemplate_JPEG_Resvg mirrors the PNG test for the JPEG
// branch (resvg PNG re-encoded to JPEG in Go).
func TestRepositoryTemplate_JPEG_Resvg(t *testing.T) {
	deps, _ := newEngineDeps(t, map[string]string{
		"User":             userOctocat,
		"UserRepositories": userRepositories250,
		"Repository":       repositoryHelloWorld,
	})
	deps.Render = withResvg(t)

	res, err := engine.Compute(context.Background(), engine.Request{
		Login:    "octocat",
		Repo:     "hello-world",
		Account:  plugins.AccountRepository,
		Template: "repository",
		Format:   "jpeg",
		Inputs: map[string]any{
			"user":                "octocat",
			"repo":                "hello-world",
			"chrome_header":       "yes",
			"chrome_activity":     "yes",
			"chrome_community":    "yes",
			"chrome_repositories": "yes",
			"chrome_metadata":     "yes",
		},
	}, deps)
	if err != nil {
		t.Fatalf("Compute(jpeg): %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", res.MIME)
	}
	if len(res.Output) < 2 || res.Output[0] != 0xff || res.Output[1] != 0xd8 {
		t.Fatalf("JPEG SOI marker missing; got %x", res.Output[:min(2, len(res.Output))])
	}
	if _, err := jpeg.Decode(bytes.NewReader(res.Output)); err != nil {
		t.Errorf("jpeg.Decode: %v", err)
	}
}
