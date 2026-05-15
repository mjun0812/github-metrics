package render

import (
	"errors"
	"strings"
	"testing"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// TestResizeOpts_Normalize_Direct exercises the package-private
// normalize() entry point directly so the rules contract has explicit
// coverage independent of FakeRenderer plumbing.
func TestResizeOpts_Normalize_Direct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   ResizeOpts
		want ResizeOpts
	}{
		{
			name: "zero values map to upstream defaults",
			in:   ResizeOpts{},
			want: ResizeOpts{
				Convert:        "svg",
				ViewportWidth:  defaultViewportWidth,
				ViewportHeight: defaultViewportHeight,
				SettleDelay:    defaultSettleDelay,
			},
		},
		{
			name: "explicit svg preserved",
			in:   ResizeOpts{Convert: "svg"},
			want: ResizeOpts{
				Convert:        "svg",
				ViewportWidth:  defaultViewportWidth,
				ViewportHeight: defaultViewportHeight,
				SettleDelay:    defaultSettleDelay,
			},
		},
		{
			name: "explicit png preserved",
			in:   ResizeOpts{Convert: "png"},
			want: ResizeOpts{
				Convert:        "png",
				ViewportWidth:  defaultViewportWidth,
				ViewportHeight: defaultViewportHeight,
				SettleDelay:    defaultSettleDelay,
			},
		},
		{
			name: "caller-supplied non-zero viewport preserved",
			in:   ResizeOpts{Convert: "svg", ViewportWidth: 1280, ViewportHeight: 720},
			want: ResizeOpts{
				Convert:        "svg",
				ViewportWidth:  1280,
				ViewportHeight: 720,
				SettleDelay:    defaultSettleDelay,
			},
		},
		{
			name: "caller-supplied SettleDelay preserved",
			in:   ResizeOpts{SettleDelay: 100 * time.Millisecond},
			want: ResizeOpts{
				Convert:        "svg",
				ViewportWidth:  defaultViewportWidth,
				ViewportHeight: defaultViewportHeight,
				SettleDelay:    100 * time.Millisecond,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.in.normalize()
			if err != nil {
				t.Fatalf("normalize: unexpected err %v", err)
			}
			if got.Convert != tc.want.Convert {
				t.Errorf("Convert = %q, want %q", got.Convert, tc.want.Convert)
			}
			if got.ViewportWidth != tc.want.ViewportWidth {
				t.Errorf("ViewportWidth = %d, want %d", got.ViewportWidth, tc.want.ViewportWidth)
			}
			if got.ViewportHeight != tc.want.ViewportHeight {
				t.Errorf("ViewportHeight = %d, want %d", got.ViewportHeight, tc.want.ViewportHeight)
			}
			if got.SettleDelay != tc.want.SettleDelay {
				t.Errorf("SettleDelay = %v, want %v", got.SettleDelay, tc.want.SettleDelay)
			}
		})
	}
}

// TestResizeOpts_Normalize_RejectsUnknownConvert ensures bogus
// Convert values surface as *UnsupportedFormatError.
func TestResizeOpts_Normalize_RejectsUnknownConvert(t *testing.T) {
	t.Parallel()
	tests := []string{"webp", "pdf", "tiff", "  svg  "}
	for _, c := range tests {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			_, err := (ResizeOpts{Convert: c}).normalize()
			if err == nil {
				t.Fatalf("expected error for Convert=%q", c)
			}
			var ufe *xerrors.UnsupportedFormatError
			if !errors.As(err, &ufe) {
				t.Errorf("err type = %T, want *UnsupportedFormatError", err)
			}
			if !strings.Contains(err.Error(), c) {
				t.Errorf("err message %q should mention the rejected convert", err.Error())
			}
		})
	}
}

// TestMimeForConvert_PanicsOnUnknown is a defensive check that
// mimeForConvert (an internal helper) refuses to silently return ""
// for callers that bypass normalize. Resize callers should never see
// this; the panic surfaces a programmer bug rather than mis-typed
// MIME output.
func TestMimeForConvert_PanicsOnUnknown(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for unknown convert, got none")
		}
	}()
	_ = mimeForConvert("bogus")
}

// TestQuoteForJS smoke-checks the helper used to embed JSON into the
// chromedp.Evaluate template. The output must (a) be a JS template
// literal, (b) preserve the JSON bytes verbatim, (c) escape any
// embedded backtick / backslash so the surrounding ` quotes survive.
func TestQuoteForJS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: `{"a":1}`, want: "`{\"a\":1}`"},
		{in: "no specials", want: "`no specials`"},
		{in: "back`tick", want: "`back\\`tick`"},
		{in: `back\slash`, want: "`back\\\\slash`"},
	}
	for _, tc := range tests {
		got := quoteForJS([]byte(tc.in))
		if got != tc.want {
			t.Errorf("quoteForJS(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
