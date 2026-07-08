package fontmetrics

import (
	"strings"
	"testing"
)

// approxEqual reports whether got is within tolPx of want. Widths are
// deterministic given the embedded font bytes, but the tolerance keeps
// the test from being brittle across golang.org/x/image version bumps
// that might tweak rounding.
func approxEqual(t *testing.T, got, want, tolPx float64) {
	t.Helper()
	if diff := got - want; diff < -tolPx || diff > tolPx {
		t.Errorf("got %.4f, want %.4f +/- %.2f", got, want, tolPx)
	}
}

func TestWidth_KnownStrings(t *testing.T) {
	// Golden values captured from this package's own implementation
	// against the embedded Liberation Sans Regular/Bold fonts at 14px.
	cases := []struct {
		text   string
		sizePx float64
		weight Weight
		want   float64
	}{
		{"Joined GitHub 8 years ago", 14, Regular, 165.70},
		{"Hello", 14, Regular, 31.89},
		{"Hello, World!", 14, Regular, 80.11},
		{"A", 14, Regular, 9.34},
		{"The quick brown fox jumps over the lazy dog", 14, Regular, 276.92},
		{"Hello", 14, Bold, 34.22},
	}
	for _, c := range cases {
		got := WidthWeight(c.text, c.sizePx, c.weight)
		approxEqual(t, got, c.want, 1.0)
	}
}

func TestWidth_Empty(t *testing.T) {
	if got := Width("", 14); got != 0 {
		t.Errorf("Width(\"\", 14) = %v, want 0", got)
	}
}

func TestWidth_ZeroOrNegativeSize(t *testing.T) {
	if got := Width("Hello", 0); got != 0 {
		t.Errorf("Width(text, 0) = %v, want 0", got)
	}
	if got := Width("Hello", -5); got != 0 {
		t.Errorf("Width(text, -5) = %v, want 0", got)
	}
}

// TestWidth_Monotonicity verifies that appending characters never
// shrinks the measured width, and that a strict superset of visible
// text is strictly wider.
func TestWidth_Monotonicity(t *testing.T) {
	prefixes := []string{"H", "He", "Hel", "Hell", "Hello", "Hello,", "Hello, W", "Hello, World!"}
	prev := 0.0
	for _, p := range prefixes {
		w := Width(p, 14)
		if w <= prev {
			t.Errorf("Width(%q) = %.4f, want > previous width %.4f", p, w, prev)
		}
		prev = w
	}
}

// TestWidth_ScaleLinearity checks that doubling sizePx roughly doubles
// the measured width (within the fixed-point rounding that per-glyph
// advances accumulate).
func TestWidth_ScaleLinearity(t *testing.T) {
	text := "Joined GitHub 8 years ago"
	w14 := Width(text, 14)
	w28 := Width(text, 28)
	w56 := Width(text, 56)

	approxEqual(t, w28, w14*2, w14*0.02)
	approxEqual(t, w56, w14*4, w14*0.02)
}

// TestWidth_BoldWiderThanRegular checks that, for ordinary Latin text,
// the Bold face is measured no narrower than Regular — bold glyphs
// have equal or larger advances in Liberation Sans.
func TestWidth_BoldWiderThanRegular(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	reg := Width(text, 16)
	bold := WidthBold(text, 16)
	if bold <= reg {
		t.Errorf("WidthBold(%q) = %.4f, want > Width(...) = %.4f", text, bold, reg)
	}
}

// TestWidth_MissingGlyphFallback documents and verifies the fallback
// behavior for runes absent from Liberation Sans (e.g. emoji): rather
// than erroring or measuring as zero-width, each such rune is charged
// a fixed fallback advance (fallbackAdvanceRatio * sizePx). This keeps
// Width usable as a layout approximation even for content GitHub may
// render with emoji.
func TestWidth_MissingGlyphFallback(t *testing.T) {
	const sizePx = 14.0
	const emoji = "\U0001F600" // grinning face, not in Liberation Sans

	got := Width(emoji, sizePx)
	want := sizePx * fallbackAdvanceRatio
	if got != want {
		t.Errorf("Width(emoji) = %.4f, want exactly the fallback advance %.4f", got, want)
	}

	// Two missing-glyph runes should charge the fallback twice, not
	// collapse to a single advance or zero.
	got2 := Width(emoji+emoji, sizePx)
	approxEqual(t, got2, want*2, 0.01)
}

func TestWrap_NoLineExceedsMaxWidth(t *testing.T) {
	const sizePx = 14.0
	const maxWidth = 150.0
	text := "Joined GitHub 8 years ago and made many contributions since then"

	lines := Wrap(text, sizePx, maxWidth)
	if len(lines) < 2 {
		t.Fatalf("Wrap produced %d line(s), want multiple for a long input", len(lines))
	}
	for _, l := range lines {
		words := strings.Fields(l)
		if len(words) > 1 && Width(l, sizePx) > maxWidth {
			t.Errorf("line %q width %.2f exceeds maxWidth %.2f", l, Width(l, sizePx), maxWidth)
		}
	}
}

func TestWrap_WordsPreserved(t *testing.T) {
	text := "Joined GitHub 8 years ago and made many contributions since then"
	lines := Wrap(text, 14, 150)

	rejoined := make([]string, 0, len(strings.Fields(text)))
	for _, l := range lines {
		rejoined = append(rejoined, strings.Fields(l)...)
	}
	got := strings.Join(rejoined, " ")
	want := strings.Join(strings.Fields(text), " ")
	if got != want {
		t.Errorf("Wrap reordered/dropped words:\n got  %q\n want %q", got, want)
	}
}

func TestWrap_EmptyString(t *testing.T) {
	if lines := Wrap("", 14, 150); len(lines) != 0 {
		t.Errorf("Wrap(\"\", ...) = %v, want empty", lines)
	}
	if lines := Wrap("   ", 14, 150); len(lines) != 0 {
		t.Errorf("Wrap(whitespace-only, ...) = %v, want empty", lines)
	}
}

func TestWrap_SingleWordWiderThanMaxWidth(t *testing.T) {
	// A single "word" (no spaces) wider than maxWidth must still be
	// returned intact on its own line rather than being split
	// mid-word or dropped.
	longWord := strings.Repeat("supercalifragilisticexpialidocious", 3)
	lines := Wrap(longWord, 14, 10)
	if len(lines) != 1 {
		t.Fatalf("Wrap(overlong word) produced %d line(s), want 1", len(lines))
	}
	if lines[0] != longWord {
		t.Errorf("Wrap(overlong word) = %q, want unchanged %q", lines[0], longWord)
	}
}

func TestWrap_MaxWidthLargerThanText(t *testing.T) {
	text := "Hello, World!"
	lines := Wrap(text, 14, 100000)
	if len(lines) != 1 || lines[0] != text {
		t.Errorf("Wrap(short text, huge maxWidth) = %v, want single unchanged line", lines)
	}
}
