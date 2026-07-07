// Package fontmetrics measures text width and greedy-wraps text using
// an embedded TrueType font, so Go render code can compute layout
// (e.g. partial height) for native SVG output without a browser.
//
// The embedded font is Liberation Sans (Regular/Bold), a
// metrics-compatible substitute for Arial released under the SIL Open
// Font License 1.1 — see fonts/LICENSE.txt. Because the font actually
// used to rasterize a given SVG may differ (system font stacks vary),
// measurements here are an approximation good enough for allocating
// layout space, not pixel-perfect typesetting.
package fontmetrics

import (
	"embed"
	"fmt"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/LiberationSans-Regular.ttf fonts/LiberationSans-Bold.ttf
var fontFS embed.FS

// fallbackAdvanceRatio approximates, as a fraction of sizePx, the
// advance width used for runes with no glyph in the embedded font
// (e.g. emoji, most CJK — Liberation Sans only covers Latin/Cyrillic/
// Greek). Measurement is the goal here, not correctness for every
// script, so an absent glyph falls back to this fixed advance instead
// of erroring or measuring as zero-width. Roughly half an em matches
// Liberation Sans' average Latin glyph advance.
const fallbackAdvanceRatio = 0.5

// Weight selects which embedded font face to measure with.
type Weight int

const (
	Regular Weight = iota
	Bold
)

var (
	loadOnce sync.Once
	loadErr  error
	fonts    [2]*sfnt.Font

	// sfnt.Buffer is not safe for concurrent use; pool it rather than
	// allocate one per call.
	bufferPool = sync.Pool{New: func() any { return new(sfnt.Buffer) }}
)

func load() {
	loadOnce.Do(func() {
		fonts[Regular], loadErr = parseEmbedded("fonts/LiberationSans-Regular.ttf")
		if loadErr != nil {
			return
		}
		fonts[Bold], loadErr = parseEmbedded("fonts/LiberationSans-Bold.ttf")
	})
}

func parseEmbedded(path string) (*sfnt.Font, error) {
	raw, err := fontFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fontmetrics: read %s: %w", path, err)
	}
	f, err := sfnt.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("fontmetrics: parse %s: %w", path, err)
	}
	return f, nil
}

// Width returns the rendered width in pixels of text at sizePx using
// the Regular weight.
func Width(text string, sizePx float64) float64 {
	return WidthWeight(text, sizePx, Regular)
}

// WidthBold returns the rendered width in pixels of text at sizePx
// using the Bold weight.
func WidthBold(text string, sizePx float64) float64 {
	return WidthWeight(text, sizePx, Bold)
}

// WidthWeight returns the rendered width in pixels of text at sizePx
// using the given weight. It never errors: if the embedded font fails
// to parse (which should not happen — the font is compiled in) it
// returns 0, and individual runes without a glyph fall back to
// fallbackAdvanceRatio so one exotic character doesn't invalidate the
// whole measurement.
func WidthWeight(text string, sizePx float64, weight Weight) float64 {
	if text == "" || sizePx <= 0 {
		return 0
	}
	load()
	if loadErr != nil {
		return 0
	}
	f := fonts[weight]

	buf := bufferPool.Get().(*sfnt.Buffer)
	defer bufferPool.Put(buf)

	ppem := fixed.Int26_6(math.Round(sizePx * 64))
	fallback := fixed.Int26_6(math.Round(sizePx * fallbackAdvanceRatio * 64))

	var width fixed.Int26_6
	for _, r := range text {
		idx, err := f.GlyphIndex(buf, r)
		if err != nil || idx == 0 {
			// idx == 0 is sfnt's .notdef / "rune not in cmap" result.
			width += fallback
			continue
		}
		adv, err := f.GlyphAdvance(buf, idx, ppem, font.HintingNone)
		if err != nil {
			width += fallback
			continue
		}
		width += adv
	}
	return float64(width) / 64
}

// Wrap greedily word-wraps text into lines no wider than maxWidth
// pixels at sizePx, measured with the Regular weight. Words are never
// split: a single word wider than maxWidth still occupies its own
// line (which will overflow maxWidth) rather than being broken mid-
// word. Whitespace runs collapse to single spaces, matching how the
// wrapped lines are expected to be re-joined for display.
func Wrap(text string, sizePx, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	spaceWidth := Width(" ", sizePx)

	out := make([]string, 0, 1)
	line := words[0]
	lineWidth := Width(line, sizePx)

	for _, w := range words[1:] {
		wWidth := Width(w, sizePx)
		if lineWidth+spaceWidth+wWidth <= maxWidth {
			line += " " + w
			lineWidth += spaceWidth + wWidth
			continue
		}
		out = append(out, line)
		line = w
		lineWidth = wWidth
	}
	return append(out, line)
}
