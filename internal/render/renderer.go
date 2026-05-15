package render

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

// fakePNG is a 1x1 fully-transparent PNG, generated at package init
// via image/png.Encode so the CRC32 checksums (IHDR / IDAT / IEND)
// are correct. The previous hand-rolled byte literal had a
// placeholder IDAT CRC that broke png.Decode; we now lean on the
// stdlib encoder to guarantee a byte stream callers can decode.
var fakePNG = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 0})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// Encode of a 1x1 RGBA image cannot fail in practice; if it
		// does we have a programmer bug, so panic at package init.
		panic(fmt.Sprintf("render: cannot build fakePNG: %v", err))
	}
	return buf.Bytes()
}()

// fakeJPEG is a minimal 1x1 JPEG, generated at package init via
// image/jpeg.Encode so the quantization tables / Huffman tables / SOS
// marker are all valid. Same rationale as fakePNG above: the stdlib
// encoder is the authoritative source of well-formed bytes.
var fakeJPEG = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		panic(fmt.Sprintf("render: cannot build fakeJPEG: %v", err))
	}
	return buf.Bytes()
}()

// Compile-time interface check: catch signature drift between the
// interface declaration and FakeRenderer.
var _ Renderer = (*FakeRenderer)(nil)
