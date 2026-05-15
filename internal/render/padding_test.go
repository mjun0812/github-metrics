package render

import (
	"io"
	"log/slog"
	"math"
	"testing"
)

// quietLogger discards every log line. Tests should not depend on log
// output, but parsePadding deliberately emits debug warnings for
// malformed input — we don't want them in `go test -v` output.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestParsePadding_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       []string
		wantW    float64
		wantH    float64
		wantAW   float64
		wantAH   float64
		approxEq bool
	}{
		{
			name:   "empty slice -> zero defaults",
			in:     nil,
			wantW:  1.0,
			wantH:  1.0,
			wantAW: 0,
			wantAH: 0,
		},
		{
			name:   "empty string slice",
			in:     []string{""},
			wantW:  1.0,
			wantH:  1.0,
			wantAW: 0,
			wantAH: 0,
		},
		{
			name:   "single element applies to both dimensions",
			in:     []string{"0 + 6%"},
			wantW:  1.06,
			wantH:  1.06,
			wantAW: 0,
			wantAH: 0,
		},
		{
			name:   "two-element slice (width, height)",
			in:     []string{"0 + 6%", "8 + 11%"},
			wantW:  1.06,
			wantH:  1.11,
			wantAW: 0,
			wantAH: 8,
		},
		{
			name:   "single comma-separated string (width, height)",
			in:     []string{"8 + 11%, 0 + 6%"},
			wantW:  1.11,
			wantH:  1.06,
			wantAW: 8,
			wantAH: 0,
		},
		{
			name:   "absolute only",
			in:     []string{"+0.5", "1.5%"},
			wantW:  1.0,
			wantH:  1.015,
			wantAW: 0.5,
			wantAH: 0,
		},
		{
			name:   "bogus token -> all zero defaults",
			in:     []string{"bogus"},
			wantW:  1.0,
			wantH:  1.0,
			wantAW: 0,
			wantAH: 0,
		},
		{
			name:   "exotic locale chars",
			in:     []string{"あ%"},
			wantW:  1.0,
			wantH:  1.0,
			wantAW: 0,
			wantAH: 0,
		},
		{
			name:   "negative relative",
			in:     []string{"0 + -5%"},
			wantW:  0.95,
			wantH:  0.95,
			wantAW: 0,
			wantAH: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parsePadding(tc.in, quietLogger())
			if !approxEqual(got.width, tc.wantW) {
				t.Errorf("width = %v, want %v", got.width, tc.wantW)
			}
			if !approxEqual(got.height, tc.wantH) {
				t.Errorf("height = %v, want %v", got.height, tc.wantH)
			}
			if !approxEqual(got.absoluteWidth, tc.wantAW) {
				t.Errorf("absoluteWidth = %v, want %v", got.absoluteWidth, tc.wantAW)
			}
			if !approxEqual(got.absoluteHeight, tc.wantAH) {
				t.Errorf("absoluteHeight = %v, want %v", got.absoluteHeight, tc.wantAH)
			}
		})
	}
}

// TestParsePadding_NoPanicOnControlChars confirms the function is
// crash-safe against pathological inputs that downstream user
// configuration could push through.
func TestParsePadding_NoPanicOnControlChars(t *testing.T) {
	t.Parallel()
	inputs := []string{"\x00\x01\x02", "\t\n\r", "%%%%%", "+++", "---"}
	for _, in := range inputs {
		_ = parsePadding([]string{in}, quietLogger()) // assert: does not panic
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
