package render

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// resvgFixtureSVG is a small self-contained native SVG with an explicit
// height. resvg needs no layout measurement, so this rasterizes to a
// deterministic 40x30 image.
const resvgFixtureSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><rect width="40" height="30" fill="#ff0000"/></svg>`

// withResvg constructs a Resvg or skips when the binary is unavailable.
// Resolution mirrors ResvgOpts.normalize (METRICS_RESVG_PATH → PATH) so
// contributors can point the tests at a local build.
func withResvg(t *testing.T) *Resvg {
	t.Helper()
	if os.Getenv(resvgPathEnv) == "" {
		if _, err := exec.LookPath("resvg"); err != nil {
			t.Skip("resvg binary unavailable: set METRICS_RESVG_PATH or add resvg to PATH")
		}
	}
	r, err := NewResvg(ResvgOpts{})
	if err != nil {
		t.Skipf("resvg unavailable: %v", err)
	}
	return r
}

// TestResvg_PNG_Decodable confirms the PNG branch returns bytes with a
// valid PNG magic number that decode to the fixture's dimensions.
func TestResvg_PNG_Decodable(t *testing.T) {
	r := withResvg(t)

	res, err := r.Resize(context.Background(), resvgFixtureSVG, ResizeOpts{Convert: "png"})
	if err != nil {
		t.Fatalf("Resize(png): %v", err)
	}
	if res.MIME != "image/png" {
		t.Errorf("MIME = %q, want image/png", res.MIME)
	}
	if !bytes.HasPrefix(res.Body, []byte("\x89PNG")) {
		t.Errorf("Body missing PNG magic number; got %x", res.Body[:min(8, len(res.Body))])
	}
	img, err := png.Decode(bytes.NewReader(res.Body))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 40 || got.Dy() != 30 {
		t.Errorf("image bounds = %v, want 40x30", got)
	}
	if res.Width != 40 || res.Height != 30 {
		t.Errorf("Width/Height = %d/%d, want 40/30", res.Width, res.Height)
	}
}

// TestResvg_JPEG_Decodable mirrors the PNG test for the re-encoded JPEG
// branch (resvg cannot emit JPEG directly).
func TestResvg_JPEG_Decodable(t *testing.T) {
	r := withResvg(t)

	res, err := r.Resize(context.Background(), resvgFixtureSVG, ResizeOpts{Convert: "jpeg"})
	if err != nil {
		t.Fatalf("Resize(jpeg): %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", res.MIME)
	}
	if !bytes.HasPrefix(res.Body, []byte("\xff\xd8")) {
		t.Errorf("Body missing JPEG magic number; got %x", res.Body[:min(2, len(res.Body))])
	}
	img, err := jpeg.Decode(bytes.NewReader(res.Body))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 40 || got.Dy() != 30 {
		t.Errorf("image bounds = %v, want 40x30", got)
	}
	if res.Width != 40 || res.Height != 30 {
		t.Errorf("Width/Height = %d/%d, want 40/30", res.Width, res.Height)
	}
}

// TestResvg_SVG_PassThrough confirms the svg branch returns the input
// verbatim without invoking the resvg subprocess.
func TestResvg_SVG_PassThrough(t *testing.T) {
	r := withResvg(t)

	res, err := r.Resize(context.Background(), resvgFixtureSVG, ResizeOpts{Convert: "svg"})
	if err != nil {
		t.Fatalf("Resize(svg): %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Errorf("MIME = %q, want image/svg+xml", res.MIME)
	}
	if string(res.Body) != resvgFixtureSVG {
		t.Errorf("Body = %q, want verbatim input", res.Body)
	}
}

// TestApplyPadding covers the pure-Go canvas expansion: trivial padding
// is a no-op that reports the intrinsic dims, and a non-trivial spec
// rewrites width/height/viewBox by width*mult+abs / height*mult+abs
// (ceil) while keeping the viewBox origin so content is not scaled.
func TestApplyPadding(t *testing.T) {
	const in = `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="128" viewBox="0 0 480 128"><rect/></svg>`

	t.Run("trivial padding is a no-op", func(t *testing.T) {
		out, w, h := applyPadding(in, parsePadding(nil, nil))
		if out != in {
			t.Errorf("body rewritten for empty padding:\n got %q", out)
		}
		if w != 480 || h != 128 {
			t.Errorf("dims = %dx%d, want 480x128", w, h)
		}
	})

	t.Run("non-trivial padding expands the canvas", func(t *testing.T) {
		// height "10 + 25%": mult 1.25, abs 10 -> ceil(128*1.25+10)=170.
		// width "0": unchanged -> 480.
		out, w, h := applyPadding(in, parsePadding([]string{"0, 10 + 25%"}, nil))
		if w != 480 || h != 170 {
			t.Errorf("dims = %dx%d, want 480x170", w, h)
		}
		for _, want := range []string{`width="480"`, `height="170"`, `viewBox="0 0 480 170"`} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q\n got %q", want, out)
			}
		}
		if !strings.Contains(out, "<rect/>") {
			t.Errorf("padding must not touch the SVG body; got %q", out)
		}
	})

	t.Run("unparseable root dims return input unchanged", func(t *testing.T) {
		const noDims = `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`
		out, w, h := applyPadding(noDims, parsePadding([]string{"0, 20"}, nil))
		if out != noDims || w != 0 || h != 0 {
			t.Errorf("expected unchanged/zero dims, got %q %dx%d", out, w, h)
		}
	})
}

// TestNewResvg_MissingBinary confirms a non-existent ExecPath fails
// fast with a typed *InputError rather than silently degrading.
func TestNewResvg_MissingBinary(t *testing.T) {
	_, err := NewResvg(ResvgOpts{ExecPath: "/nonexistent/path/to/resvg"})
	if err == nil {
		t.Fatal("NewResvg: expected error for missing binary, got nil")
	}
	var inputErr *xerrors.InputError
	if !errors.As(err, &inputErr) {
		t.Errorf("error = %v, want *xerrors.InputError", err)
	}
}
