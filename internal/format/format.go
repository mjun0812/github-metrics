// Package format provides display helpers for rendering numbers, byte
// counts, percentages, timestamps and error messages. The functions match
// the semantics of the upstream lowlighter/metrics formatter helpers but
// are written from scratch in Go.
package format

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Options controls number formatting.
type Options struct {
	// Sign forces a leading '+' on positive numbers when true.
	Sign bool
	// Suffix is appended to the formatted string after the magnitude unit.
	Suffix string
}

// Format formats n with k / m / b / t magnitude suffixes. Values below 1000
// are emitted as integers. Values above are rounded to one decimal place
// using banker-friendly truncation (math.Floor on the absolute value).
//
//	0       -> "0"
//	999     -> "999"
//	1000    -> "1k"
//	1500    -> "1.5k"
//	1000000 -> "1m"
func Format(n int64, opts Options) string {
	sign := ""
	abs := n
	if n < 0 {
		sign = "-"
		abs = -n
	} else if n > 0 && opts.Sign {
		sign = "+"
	}

	val := float64(abs)
	// Thresholds are rounding-aware: any value that would round to >= 1000
	// at the lower tier is promoted to the next tier. e.g. 999999 rounds to
	// 1000k under naive bucketing, so we promote it to 1m.
	const (
		thrK = 999.5            // promote to "k" once one decimal would carry
		thrM = 999_500.0        // 999_999 rounds to 1.0m
		thrB = 999_500_000.0
		thrT = 999_500_000_000.0
	)
	var (
		out  string
		unit string
	)
	switch {
	case val >= thrT:
		out = trimDecimal(val/1_000_000_000_000, 1)
		unit = "t"
	case val >= thrB:
		out = trimDecimal(val/1_000_000_000, 1)
		unit = "b"
	case val >= thrM:
		out = trimDecimal(val/1_000_000, 1)
		unit = "m"
	case val >= thrK:
		out = trimDecimal(val/1_000, 1)
		unit = "k"
	default:
		out = fmt.Sprintf("%d", int64(val))
	}
	return sign + out + unit + opts.Suffix
}

// trimDecimal renders f with at most digits fractional digits and strips
// trailing zeros and a dangling decimal point.
func trimDecimal(f float64, digits int) string {
	s := strconvFloat(f, digits)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func strconvFloat(f float64, digits int) string {
	// Round half away from zero to match human expectations: 1.55 → 1.6.
	pow := math.Pow(10, float64(digits))
	rounded := math.Floor(f*pow+0.5) / pow
	return fmt.Sprintf("%.*f", digits, rounded)
}

// FormatBytes formats a byte count using binary (1024-based) magnitudes:
// "B", "KB", "MB", "GB", "TB". One decimal place is kept for non-zero
// fractional parts.
func FormatBytes(n int64) string {
	const unit = 1024.0
	abs := n
	sign := ""
	if n < 0 {
		sign = "-"
		abs = -n
	}
	if abs < int64(unit) {
		return fmt.Sprintf("%s%d B", sign, abs)
	}
	val := float64(abs)
	suffixes := []string{"KB", "MB", "GB", "TB"}
	idx := -1
	for i := 0; i < len(suffixes); i++ {
		if val < unit {
			break
		}
		val /= unit
		idx = i
	}
	if idx < 0 {
		return fmt.Sprintf("%s%d B", sign, abs)
	}
	return sign + trimDecimal(val, 1) + " " + suffixes[idx]
}

// FormatPercentage renders 0..1 as a percentage with no fractional digits,
// or 0..100 when Options.Sign asserts the value is already in percent units
// (caller convention). To keep the API focused we always treat n as a
// fraction in [0, 1]; values above 1 are clamped via math.Min only when
// they exceed 100 (i.e. clearly out of range).
func FormatPercentage(n float64, opts Options) string {
	pct := n * 100
	if pct > 100 && pct <= 100.0+1e-9 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	sign := ""
	if opts.Sign && n > 0 {
		sign = "+"
	}
	return sign + trimDecimal(pct, 1) + "%" + opts.Suffix
}

// DateOptions controls FormatDate output.
type DateOptions struct {
	// Layout is a Go reference time layout. Defaults to time.DateOnly.
	Layout string
	// Timezone is an IANA timezone name. Empty value uses UTC.
	Timezone string
}

// FormatDate renders t in the requested timezone and layout.
// On an unknown timezone the function falls back to UTC and signals the
// failure via the returned error; the formatted string is still produced.
func FormatDate(t time.Time, opts DateOptions) (string, error) {
	layout := opts.Layout
	if layout == "" {
		layout = time.DateOnly
	}
	loc, err := loadLocation(opts.Timezone)
	if err != nil {
		return t.UTC().Format(layout), err
	}
	return t.In(loc).Format(layout), nil
}

func loadLocation(name string) (*time.Location, error) {
	if name == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC, err
	}
	return loc, nil
}

// Ellipsis truncates s to at most maxRunes runes, appending a single
// U+2026 (…) when truncation occurs. maxRunes <= 0 returns s unchanged.
func Ellipsis(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// S returns "" when n == 1 and suffix otherwise.
// Useful for "1 item" vs "2 items" pluralization.
func S(n int64, suffix string) string {
	if n == 1 || n == -1 {
		return ""
	}
	return suffix
}

// FormatError returns a user-facing string for err. nil yields "".
// When opts.Suffix is non-empty it is appended (no separator).
func FormatError(err error, opts Options) string {
	if err == nil {
		return ""
	}
	return err.Error() + opts.Suffix
}
