package render

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// padding is the internal value the resize JS template consumes.
// `width` and `height` are multipliers (1.0 + relative/100); the
// absolute components are added after the multiplication.
type padding struct {
	width, height                 float64
	absoluteWidth, absoluteHeight float64
}

// trailingPercentRe matches the relative portion at the end of a
// padding spec (e.g. "0 + 6%" -> "6"). The match is anchored to the
// trailing percent sign so absolute-only inputs like "8" leave the
// relative value at 0.
var trailingPercentRe = regexp.MustCompile(`([+-]?[\d.]+)%\s*$`)

// leadingAbsoluteRe matches the absolute portion at the start of the
// padding spec, after the relative component has been stripped.
var leadingAbsoluteRe = regexp.MustCompile(`^\s*([+-]?[\d.]+)`)

// parsePadding turns the upstream-compatible `config.padding` input
// shape into the internal padding struct. Accepts:
//
//   - `nil` / empty slice    → zero defaults (1.0 multiplier, 0 absolute)
//   - one element            → split on "," to extract width/height,
//     fall back to the same value for both
//   - two or more elements   → first = width, second = height
//
// Malformed numeric tokens are logged at debug level and treated as
// zero so a typo in one input does not break rendering for the rest
// of the run (see contracts/svg-resize.md §3 / FR-007 / Edge Cases).
func parsePadding(in []string, log *slog.Logger) padding {
	if log == nil {
		log = slog.Default()
	}

	var w, h string
	switch {
	case len(in) == 0:
		w, h = "", ""
	case len(in) == 1:
		parts := strings.SplitN(in[0], ",", 2)
		w = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			h = strings.TrimSpace(parts[1])
		} else {
			h = w
		}
	case len(in) >= 2:
		w = strings.TrimSpace(in[0])
		h = strings.TrimSpace(in[1])
	}

	rw := relativeOf(w, log)
	rh := relativeOf(h, log)
	aw := absoluteOf(w, log)
	ah := absoluteOf(h, log)

	return padding{
		width:          1 + rw/100,
		height:         1 + rh/100,
		absoluteWidth:  aw,
		absoluteHeight: ah,
	}
}

// relativeOf returns the percent value at the end of `s` (without the
// `%` sign). Returns 0 when no percent is present.
func relativeOf(s string, log *slog.Logger) float64 {
	m := trailingPercentRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
	if err != nil {
		log.Debug("render: padding relative parse failed", "input", s, "err", err)
		return 0
	}
	return v
}

// absoluteOf returns the absolute pixel value at the start of `s`,
// AFTER stripping any trailing percent portion. So "8 + 11%" → 8, and
// "+0.5 + 2%" → 0.5.
func absoluteOf(s string, log *slog.Logger) float64 {
	rest := trailingPercentRe.ReplaceAllString(s, "")
	rest = strings.TrimRight(rest, " \t+\n")
	m := leadingAbsoluteRe.FindStringSubmatch(rest)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		log.Debug("render: padding absolute parse failed", "input", s, "err", err)
		return 0
	}
	return v
}
