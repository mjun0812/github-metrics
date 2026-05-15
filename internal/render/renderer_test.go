package render

import (
	"context"
	"errors"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TestFakeRenderer_Resize exercises the FakeRenderer's documented
// behavior for every supported Convert value plus the rejection path.
func TestFakeRenderer_Resize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		convert     string
		wantMIME    string
		wantPrefix  []byte
		wantPassIn  bool // when true, expect Body == input bytes (SVG branch)
		expectError bool
	}{
		{
			name:       "empty convert is treated as svg",
			convert:    "",
			wantMIME:   "image/svg+xml",
			wantPassIn: true,
		},
		{
			name:       "explicit svg returns input bytes",
			convert:    "svg",
			wantMIME:   "image/svg+xml",
			wantPassIn: true,
		},
		{
			name:       "png starts with the PNG magic header",
			convert:    "png",
			wantMIME:   "image/png",
			wantPrefix: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		},
		{
			name:       "jpeg starts with the JPEG SOI marker",
			convert:    "jpeg",
			wantMIME:   "image/jpeg",
			wantPrefix: []byte{0xFF, 0xD8, 0xFF},
		},
		{
			name:        "unsupported convert returns *UnsupportedFormatError",
			convert:     "bogus",
			expectError: true,
		},
	}

	const svgIn = `<svg><g id="metrics-end"/></svg>`
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := &FakeRenderer{}
			res, err := f.Resize(context.Background(), svgIn, ResizeOpts{Convert: tc.convert})

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var ufe *xerrors.UnsupportedFormatError
				if !errors.As(err, &ufe) {
					t.Errorf("error type = %T, want *UnsupportedFormatError", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.MIME != tc.wantMIME {
				t.Errorf("MIME = %q, want %q", res.MIME, tc.wantMIME)
			}
			if tc.wantPassIn {
				if string(res.Body) != svgIn {
					t.Errorf("Body = %q, want passthrough of input", res.Body)
				}
			}
			if len(tc.wantPrefix) > 0 {
				if len(res.Body) < len(tc.wantPrefix) {
					t.Fatalf("Body shorter (%d) than expected magic header (%d)",
						len(res.Body), len(tc.wantPrefix))
				}
				for i, want := range tc.wantPrefix {
					if res.Body[i] != want {
						t.Errorf("Body[%d] = %#x, want %#x", i, res.Body[i], want)
					}
				}
			}
		})
	}
}

// TestFakeRenderer_ErrOnConvert verifies the per-Convert error
// injection map: the configured value short-circuits Resize for the
// matching Convert and is transparent for the other branches.
func TestFakeRenderer_ErrOnConvert(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated chromedp failure")
	f := &FakeRenderer{ErrOnConvert: map[string]error{"png": sentinel}}

	// PNG branch returns the sentinel.
	_, err := f.Resize(context.Background(), "", ResizeOpts{Convert: "png"})
	if !errors.Is(err, sentinel) {
		t.Errorf("PNG path: err = %v, want sentinel", err)
	}

	// JPEG branch is unaffected (no entry in the map).
	res, err := f.Resize(context.Background(), "", ResizeOpts{Convert: "jpeg"})
	if err != nil {
		t.Errorf("JPEG path: unexpected err %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("JPEG path: MIME = %q, want image/jpeg", res.MIME)
	}

	// SVG branch is unaffected.
	res, err = f.Resize(context.Background(), "<svg/>", ResizeOpts{Convert: "svg"})
	if err != nil {
		t.Errorf("SVG path: unexpected err %v", err)
	}
	if string(res.Body) != "<svg/>" {
		t.Errorf("SVG path: Body = %q, want passthrough", res.Body)
	}
}

// TestFakeRenderer_NewFakeRenderer_ZeroValueIsUsable confirms that the
// zero-value FakeRenderer (and the NewFakeRenderer constructor) emit
// sane defaults for Width / Height.
func TestFakeRenderer_NewFakeRenderer_ZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	f := NewFakeRenderer()
	res, err := f.Resize(context.Background(), "<svg/>", ResizeOpts{Convert: "png"})
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if res.Width != 1 || res.Height != 1 {
		t.Errorf("default Width/Height = (%d,%d), want (1,1)", res.Width, res.Height)
	}
}

// TestFakeRenderer_WidthHeightOverride confirms caller-supplied
// dimensions flow through to ResizeResult.
func TestFakeRenderer_WidthHeightOverride(t *testing.T) {
	t.Parallel()

	f := &FakeRenderer{Width: 480, Height: 720}
	res, err := f.Resize(context.Background(), "<svg/>", ResizeOpts{Convert: "png"})
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if res.Width != 480 || res.Height != 720 {
		t.Errorf("Width/Height = (%d,%d), want (480,720)", res.Width, res.Height)
	}
}

// TestResizeOpts_Normalize covers the default-fill behavior callers
// rely on. We exercise normalize() indirectly through FakeRenderer
// because the function is package-private.
func TestResizeOpts_Normalize(t *testing.T) {
	t.Parallel()

	// Zero ViewportWidth/Height/SettleDelay should not error, and
	// the resulting ResizeResult should still come back populated.
	f := &FakeRenderer{}
	res, err := f.Resize(context.Background(), "<svg/>", ResizeOpts{})
	if err != nil {
		t.Fatalf("normalize via zero opts: %v", err)
	}
	if res.MIME != "image/svg+xml" {
		t.Errorf("default Convert mapping: MIME = %q, want image/svg+xml", res.MIME)
	}

	// Confirm UnsupportedFormatError surfaces from normalize.
	_, err = f.Resize(context.Background(), "", ResizeOpts{Convert: "webp"})
	var ufe *xerrors.UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Errorf("normalize bogus convert: err = %T, want *UnsupportedFormatError", err)
	}
}
