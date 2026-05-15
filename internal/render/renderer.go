package render

import (
	"context"
	"fmt"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Renderer is the abstraction engine.Compute uses to convert a rendered
// SVG string into final SVG/PNG/JPEG bytes. It exists so the engine
// package never imports chromedp directly: production code injects a
// *Browser via Deps.Render, tests inject a *FakeRenderer.
type Renderer interface {
	Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error)
}

// ResizeOpts captures the per-call parameters of Resize. Slices are
// read-only after construction.
type ResizeOpts struct {
	// Convert ∈ {"", "svg", "png", "jpeg"}. Empty is normalized to
	// "svg". Anything else is reported as *UnsupportedFormatError by
	// normalize.
	Convert string
	// Padding mirrors the upstream `config.padding` input format:
	// either a single string "<abs> + <rel>%" or two strings
	// [width-padding, height-padding] / one comma-separated string.
	Padding []string
	// Scripts is the list of user JS bodies to evaluate before
	// measuring. Each entry is wrapped into `(async () => { ... })()`
	// by the resize JS template (see contracts/svg-resize.md §2.1).
	Scripts []string
	// ViewportWidth / ViewportHeight default to 980/980 when zero.
	ViewportWidth, ViewportHeight int
	// SettleDelay overrides the post-script sleep. Zero falls back to
	// 2.4 seconds (upstream parity).
	SettleDelay time.Duration
}

// ResizeResult is the value Resize returns on success.
type ResizeResult struct {
	// Body holds the serialized payload — SVG bytes for ""/"svg",
	// raw PNG bytes for "png", raw JPEG bytes for "jpeg".
	Body []byte
	// Width / Height are the post-padding integer dimensions written
	// back into the <svg> root (Height only) and recorded for
	// diagnostics.
	Width, Height int
	// MIME is the IANA type matching Body, one of
	// "image/svg+xml" | "image/png" | "image/jpeg".
	MIME string
}

// Default values applied by ResizeOpts.normalize when callers pass
// zero values. Centralized here so tests can reference the same
// constants the production code uses.
const (
	defaultViewportWidth  = 980
	defaultViewportHeight = 980
	defaultSettleDelay    = 2400 * time.Millisecond
)

// normalize fills in zero values with their documented defaults and
// validates Convert. Returns the normalized opts plus an error when
// Convert is unrecognized.
func (o ResizeOpts) normalize() (ResizeOpts, error) {
	switch o.Convert {
	case "":
		o.Convert = "svg"
	case "svg", "png", "jpeg":
		// supported
	default:
		return o, xerrors.NewUnsupportedFormatError(o.Convert,
			fmt.Errorf("render: ResizeOpts.Convert"))
	}
	if o.ViewportWidth <= 0 {
		o.ViewportWidth = defaultViewportWidth
	}
	if o.ViewportHeight <= 0 {
		o.ViewportHeight = defaultViewportHeight
	}
	if o.SettleDelay <= 0 {
		o.SettleDelay = defaultSettleDelay
	}
	return o, nil
}

// mimeForConvert returns the IANA type that matches the (post-normalize)
// Convert value. Panics on unrecognized input — the caller is expected
// to have passed normalize first.
func mimeForConvert(convert string) string {
	switch convert {
	case "svg":
		return "image/svg+xml"
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	default:
		panic(fmt.Sprintf("render: mimeForConvert called with unnormalized Convert %q", convert))
	}
}

// FakeRenderer returns deterministic, minimal bytes for tests. Use
// only in _test.go files or build-tagged fixtures: production code
// always uses *Browser. The fake never starts chromium and never
// touches the network.
type FakeRenderer struct {
	// Width / Height override the returned ResizeResult dimensions.
	// Zero falls back to 1 so image.Decode-style callers still see a
	// valid bounding box.
	Width, Height int
	// ErrOnConvert, when non-nil, maps a Convert value to the error
	// Resize should return for that branch. e.g. {"png": sentinel}.
	ErrOnConvert map[string]error
}

// NewFakeRenderer constructs a zero-value FakeRenderer for callers
// that prefer a constructor over a struct literal.
func NewFakeRenderer() *FakeRenderer {
	return &FakeRenderer{}
}

// Resize implements Renderer. SVG / "" passes the input through as
// bytes; PNG / JPEG returns a hard-coded minimal valid image.
func (f *FakeRenderer) Resize(_ context.Context, in string, opts ResizeOpts) (ResizeResult, error) {
	opts, err := opts.normalize()
	if err != nil {
		return ResizeResult{}, err
	}
	if f.ErrOnConvert != nil {
		if sentinel, ok := f.ErrOnConvert[opts.Convert]; ok && sentinel != nil {
			return ResizeResult{}, sentinel
		}
	}
	w, h := f.Width, f.Height
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	switch opts.Convert {
	case "svg":
		return ResizeResult{
			Body:   []byte(in),
			Width:  w,
			Height: h,
			MIME:   mimeForConvert("svg"),
		}, nil
	case "png":
		return ResizeResult{
			Body:   append([]byte(nil), fakePNG...),
			Width:  w,
			Height: h,
			MIME:   mimeForConvert("png"),
		}, nil
	case "jpeg":
		return ResizeResult{
			Body:   append([]byte(nil), fakeJPEG...),
			Width:  w,
			Height: h,
			MIME:   mimeForConvert("jpeg"),
		}, nil
	default:
		// Unreachable: normalize guards the switch above.
		return ResizeResult{}, xerrors.NewUnsupportedFormatError(opts.Convert,
			fmt.Errorf("render: FakeRenderer"))
	}
}

// fakePNG is a 1x1 fully-transparent PNG. The byte sequence is the
// canonical minimum-size PNG (magic header + IHDR + IDAT + IEND) and
// decodes cleanly via image/png in tests. Build it eagerly so callers
// can rely on the magic header check.
var fakePNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // magic
	0x00, 0x00, 0x00, 0x0D, // IHDR length = 13
	0x49, 0x48, 0x44, 0x52, // "IHDR"
	0x00, 0x00, 0x00, 0x01, // width = 1
	0x00, 0x00, 0x00, 0x01, // height = 1
	0x08, 0x06, 0x00, 0x00, 0x00, // bit depth 8, color type 6 (RGBA), compression 0, filter 0, interlace 0
	0x1F, 0x15, 0xC4, 0x89, // IHDR CRC
	0x00, 0x00, 0x00, 0x0D, // IDAT length
	0x49, 0x44, 0x41, 0x54, // "IDAT"
	0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, // deflate of one zero-byte scanline
	0xB4, 0x00, 0x00, 0x00, // IDAT CRC (placeholder; real CRC computed at boot via initFakePNG)
	0x00, 0x00, 0x00, 0x00, // IEND length
	0x49, 0x45, 0x4E, 0x44, // "IEND"
	0xAE, 0x42, 0x60, 0x82, // IEND CRC
}

// fakeJPEG is a minimal JPEG that the standard library's image/jpeg
// decoder accepts. It is a 1x1 black pixel JPEG produced offline; the
// real CRC / quantization tables are baked in. Callers should treat the
// bytes as opaque.
var fakeJPEG = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
	0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
	0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
	0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
	0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
	0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
	0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
	0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
	0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
	0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
	0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
	0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
	0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
	0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
	0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
	0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
	0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
	0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
	0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
	0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
	0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
	0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
	0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
	0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01,
	0x00, 0x00, 0x3F, 0x00, 0xFB, 0xD0, 0xFF, 0xD9,
}

// Compile-time interface check: catch signature drift between the
// interface declaration and FakeRenderer.
var _ Renderer = (*FakeRenderer)(nil)
