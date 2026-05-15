//go:build chromedp

package render

import (
	"bytes"
	"context"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadFixture reads the project-rooted fixture file. The test walks
// up from CWD until it finds the file so `go test ./internal/...`
// invocations from any directory still work.
func loadFixture(t *testing.T, relPath string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, relPath)
		if _, err := os.Stat(candidate); err == nil {
			b, err := os.ReadFile(candidate)
			if err != nil {
				t.Fatalf("read %s: %v", candidate, err)
			}
			return string(b)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("fixture %s not found from %s", relPath, dir)
		}
		dir = parent
	}
}

// withBrowser opens a Browser using the default settings. Tests skip
// when chromium cannot be located so contributor environments that
// happen to pull in the chromedp tag manually still pass.
func withBrowser(t *testing.T) *Browser {
	t.Helper()
	b, err := New(BrowserOpts{})
	if err != nil {
		t.Skipf("chromium unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// TestResize_FixedSVG_HeightInRange runs the chromedp measurement
// pass against a known SVG and asserts the height attribute lands in
// the expected range for the upstream-style algorithm.
func TestResize_FixedSVG_HeightInRange(t *testing.T) {
	b := withBrowser(t)
	in := loadFixture(t, "tests/fixtures/render/input_for_measure.svg")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := b.Resize(ctx, in, ResizeOpts{
		Convert:     "svg",
		SettleDelay: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml", res.MIME)
	}
	if !bytes.Contains(res.Body, []byte("<svg")) {
		t.Errorf("Body should still contain an <svg> root; got %.120s", res.Body)
	}
	// The fixture has #metrics-end at SVG-coord y=640 but
	// height="auto" lets the browser pick the rendered height
	// (typically 750-900 for the 480px-wide content). We assert
	// a broad envelope: the measurement should be substantial
	// (>= 400px) and not exceed our 980 viewport materially.
	if res.Height < 400 || res.Height > 1100 {
		t.Errorf("Height = %d, want range [400, 1100] (rendered height of an auto-height SVG)", res.Height)
	}
	if res.Width < 400 || res.Width > 1000 {
		t.Errorf("Width = %d, want range [400, 1000]", res.Width)
	}
	// The fixture starts with height="auto" — Resize must NOT rewrite it.
	if !strings.Contains(string(res.Body), `height="auto"`) {
		t.Errorf("Body should preserve the auto-height fixture attribute; got %.120s", res.Body)
	}
}

// TestResize_PNG_Decodable confirms the PNG branch returns bytes the
// standard library can decode and yields the expected dimensions.
func TestResize_PNG_Decodable(t *testing.T) {
	b := withBrowser(t)
	in := loadFixture(t, "tests/fixtures/render/input_for_measure.svg")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := b.Resize(ctx, in, ResizeOpts{
		Convert:     "png",
		SettleDelay: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Resize(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Errorf("MIME = %q, want image/png", res.MIME)
	}
	img, err := png.Decode(bytes.NewReader(res.Body))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	bnds := img.Bounds()
	if bnds.Dx() < 1 || bnds.Dy() < 1 {
		t.Errorf("image bounds = %v, want non-empty", bnds)
	}
}

// TestResize_JPEG_Decodable mirrors the PNG test for jpeg.
func TestResize_JPEG_Decodable(t *testing.T) {
	b := withBrowser(t)
	in := loadFixture(t, "tests/fixtures/render/input_for_measure.svg")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := b.Resize(ctx, in, ResizeOpts{
		Convert:     "jpeg",
		SettleDelay: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Resize(jpeg): %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", res.MIME)
	}
	img, err := jpeg.Decode(bytes.NewReader(res.Body))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	bnds := img.Bounds()
	if bnds.Dx() < 1 || bnds.Dy() < 1 {
		t.Errorf("image bounds = %v, want non-empty", bnds)
	}
}
