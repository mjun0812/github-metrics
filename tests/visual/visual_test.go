// Package visual hosts the visual regression tests for the adopted
// templates. Each committed golden SVG (tests/golden/classic/*.svg and
// tests/golden/repository/*.svg) is rasterized through the production
// render.Resvg renderer and the decoded PNG is checked against robust,
// environment-independent invariants.
//
// # Why pixel statistics rather than PNG byte goldens
//
// resvg is deterministic for a fixed font file, but the Liberation Sans
// family it maps the SVG font stacks onto ships in different point
// releases across Linux distributions (and thus across a contributor's
// machine and the CI runner). A byte-exact PNG golden would therefore
// flap on font-version drift alone, so it is deliberately not used.
//
// Instead each raster is asserted against three invariants that catch
// the failure classes this suite exists to guard, without depending on
// exact glyph rasterization:
//
//   - decoded dimensions equal the SVG's declared width/height — catches
//     a 1x1 placeholder or a runaway canvas leaking through.
//   - the non-transparent pixel ratio sits inside a wide band — a near-
//     zero ratio means resvg silently skipped every <text> (the font-
//     mapping regression removing the generic-family flags would cause),
//     and a near-total ratio means the card rasterized as one opaque
//     block instead of laid-out content.
//   - the distinct-color count clears a small floor — degenerate single-
//     colour output cannot pass.
//
// The suite skips when the resvg binary is unavailable, so the default
// `go test ./...` run stays green; the CI test-resvg job installs resvg
// and is what actually exercises this path.
package visual

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mjun0812/github-metrics/internal/render"
)

// pixel-statistics bands. The floors/ceilings are intentionally wide:
// the observed non-transparent ratio across the committed goldens spans
// ~6%..56% with 29..900 distinct colours, so these bounds leave ample
// margin for cross-distro font-rasterization differences while still
// failing a blank, all-opaque, or single-colour raster.
const (
	minNonTransparentRatio = 0.02
	maxNonTransparentRatio = 0.90
	minDistinctColors      = 4
)

// rootWidthRe / rootHeightRe pull the numeric width/height out of the
// first (root) <svg> tag. The goldens carry finalized integer
// dimensions (no height="99999" placeholder), so resvg rasterizes to
// exactly these.
var (
	rootWidthRe  = regexp.MustCompile(`<svg\b[^>]*\bwidth="([0-9]+)"`)
	rootHeightRe = regexp.MustCompile(`<svg\b[^>]*\bheight="([0-9]+)"`)
)

// withResvg constructs a *render.Resvg or skips the test when the binary
// is unavailable. Resolution mirrors render.ResvgOpts.normalize
// (METRICS_RESVG_PATH → PATH), so contributors without resvg still get a
// green default run while CI's test-resvg job exercises the real
// rasterization.
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

// goldenDir resolves a project-rooted golden directory by walking up
// from the test's CWD until the path exists.
func goldenDir(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above CWD", rel)
		}
		dir = parent
	}
}

// TestGoldenRasters rasterizes every committed classic and repository
// golden SVG through render.Resvg and asserts the decoded PNG satisfies
// the dimension and pixel-statistics invariants documented on the
// package. It is table-driven over the golden files on disk so a newly
// added golden is covered automatically.
func TestGoldenRasters(t *testing.T) {
	r := withResvg(t)

	var svgs []string
	for _, rel := range []string{"tests/golden/classic", "tests/golden/repository"} {
		matches, err := filepath.Glob(filepath.Join(goldenDir(t, rel), "*.svg"))
		if err != nil {
			t.Fatalf("glob %s: %v", rel, err)
		}
		svgs = append(svgs, matches...)
	}
	if len(svgs) == 0 {
		t.Fatal("no golden SVGs found; expected classic/repository fixtures")
	}

	for _, svgPath := range svgs {
		name := filepath.Base(svgPath)
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(svgPath)
			if err != nil {
				t.Fatalf("read %s: %v", svgPath, err)
			}
			wantW, wantH := rootDims(t, body)

			res, err := r.Resize(context.Background(), string(body), render.ResizeOpts{Convert: "png"})
			if err != nil {
				t.Fatalf("Resize(png): %v", err)
			}
			if !bytes.HasPrefix(res.Body, []byte("\x89PNG")) {
				t.Fatalf("output missing PNG magic number; got %x", res.Body[:min(8, len(res.Body))])
			}
			img, err := png.Decode(bytes.NewReader(res.Body))
			if err != nil {
				t.Fatalf("png.Decode: %v", err)
			}

			bnds := img.Bounds()
			if bnds.Dx() != wantW || bnds.Dy() != wantH {
				t.Errorf("raster dims = %dx%d, want %dx%d (SVG declared size)",
					bnds.Dx(), bnds.Dy(), wantW, wantH)
			}

			total := bnds.Dx() * bnds.Dy()
			nonTransparent := 0
			colors := make(map[uint32]struct{})
			for y := bnds.Min.Y; y < bnds.Max.Y; y++ {
				for x := bnds.Min.X; x < bnds.Max.X; x++ {
					cr, cg, cb, ca := img.At(x, y).RGBA()
					if ca > 0 {
						nonTransparent++
					}
					key := cr>>8<<24 | cg>>8<<16 | cb>>8<<8 | ca>>8
					colors[key] = struct{}{}
				}
			}

			ratio := float64(nonTransparent) / float64(total)
			if ratio < minNonTransparentRatio {
				t.Errorf("non-transparent ratio = %.3f, below floor %.3f — raster is (near-)blank; "+
					"a resvg font-resolution failure would skip every <text>", ratio, minNonTransparentRatio)
			}
			if ratio > maxNonTransparentRatio {
				t.Errorf("non-transparent ratio = %.3f, above ceiling %.3f — raster is (near-)fully opaque "+
					"instead of laid-out content", ratio, maxNonTransparentRatio)
			}
			if len(colors) < minDistinctColors {
				t.Errorf("distinct colors = %d, want >= %d — raster is degenerate", len(colors), minDistinctColors)
			}
		})
	}
}

// rootDims parses the declared width/height of the root <svg> element.
func rootDims(t *testing.T, svg []byte) (int, int) {
	t.Helper()
	w := rootWidthRe.FindSubmatch(svg)
	h := rootHeightRe.FindSubmatch(svg)
	if w == nil || h == nil {
		t.Fatalf("could not parse root <svg> width/height")
	}
	return atoi(t, w[1]), atoi(t, h[1])
}

func atoi(t *testing.T, b []byte) int {
	t.Helper()
	n := 0
	for _, c := range b {
		n = n*10 + int(c-'0')
	}
	return n
}
