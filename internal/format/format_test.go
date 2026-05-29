package format_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
)

func TestFormat_BoundaryValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int64
		opts format.Options
		want string
	}{
		{0, format.Options{}, "0"},
		{999, format.Options{}, "999"},
		{1000, format.Options{}, "1k"},
		{1500, format.Options{}, "1.5k"},
		{999999, format.Options{}, "1m"}, // rounds up at thousands boundary
		{1000000, format.Options{}, "1m"},
		{1234567, format.Options{}, "1.2m"},
		{1000000000, format.Options{}, "1b"},
		{1000000000000, format.Options{}, "1t"},
		{-1500, format.Options{}, "-1.5k"},
		{42, format.Options{Sign: true}, "+42"},
		{0, format.Options{Sign: true}, "0"},
		{1000, format.Options{Suffix: " pts"}, "1k pts"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := format.Format(tc.in, tc.opts); got != tc.want {
				t.Fatalf("Format(%d, %+v) = %q, want %q", tc.in, tc.opts, got, tc.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1024 * 1024, "1 MB"},
		{1024*1024 + 512*1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1 GB"},
		{1024 * 1024 * 1024 * 1024, "1 TB"},
		{-2048, "-2 KB"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := format.FormatBytes(tc.in); got != tc.want {
				t.Fatalf("FormatBytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFormatDiskKB anchors #429 Phase 2: the helper must promote KB →
// MB → GB → TB at the 1024-step boundary, keep one fractional digit
// for non-trailing-zero results, and round-trip negatives with a
// leading minus.
func TestFormatDiskKB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int
		want string
	}{
		{0, "0 KB"},
		{1, "1 KB"},
		{500, "500 KB"},
		{1023, "1023 KB"},
		{1024, "1 MB"},
		{1536, "1.5 MB"},
		{1024 * 1024, "1 GB"},
		{1024*1024 + 512*1024, "1.5 GB"},
		{5242880, "5 GB"}, // 5 * 1024 * 1024 KB
		{1024 * 1024 * 1024, "1 TB"},
		{-2048, "-2 MB"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := format.FormatDiskKB(tc.in); got != tc.want {
				t.Fatalf("FormatDiskKB(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatPercentage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		opts format.Options
		want string
	}{
		{0, format.Options{}, "0%"},
		{0.5, format.Options{}, "50%"},
		{0.1234, format.Options{}, "12.3%"},
		{1, format.Options{}, "100%"},
		{1.2, format.Options{}, "100%"}, // clamped
		{-0.1, format.Options{}, "0%"},  // clamped
		{0.25, format.Options{Sign: true}, "+25%"},
		{0.42, format.Options{Suffix: " hit"}, "42% hit"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := format.FormatPercentage(tc.in, tc.opts); got != tc.want {
				t.Fatalf("FormatPercentage(%v, %+v) = %q, want %q", tc.in, tc.opts, got, tc.want)
			}
		})
	}
}

func TestFormatDate_TimezoneSwitch(t *testing.T) {
	t.Parallel()

	// 2025-01-01T00:30:00Z is 2025-01-01 09:30 Asia/Tokyo (+09:00).
	moment := time.Date(2025, 1, 1, 0, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		opts format.DateOptions
		want string
	}{
		{
			name: "default UTC",
			opts: format.DateOptions{},
			want: "2025-01-01",
		},
		{
			name: "explicit UTC layout RFC3339",
			opts: format.DateOptions{Layout: time.RFC3339},
			want: "2025-01-01T00:30:00Z",
		},
		{
			name: "Asia/Tokyo shifts day forward",
			opts: format.DateOptions{Timezone: "Asia/Tokyo", Layout: time.DateTime},
			want: "2025-01-01 09:30:00",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := format.FormatDate(moment, tc.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("FormatDate = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatDate_InvalidTimezoneFallsBackToUTC(t *testing.T) {
	t.Parallel()

	moment := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	got, err := format.FormatDate(moment, format.DateOptions{Timezone: "Not/AZone"})
	if err == nil {
		t.Fatalf("expected error for invalid timezone, got nil")
	}
	if got != "2025-06-01" {
		t.Fatalf("FormatDate fallback = %q, want %q", got, "2025-06-01")
	}
}

func TestEllipsis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"", 5, ""},
		{"abc", 5, "abc"},
		{"abcdef", 3, "abc…"},
		{"αβγδε", 3, "αβγ…"}, // multi-byte respected
		{"abc", 0, "abc"},    // non-positive returns input
		{"abc", -1, "abc"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := format.Ellipsis(tc.in, tc.max); got != tc.want {
				t.Fatalf("Ellipsis(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestS_Pluralization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n      int64
		suffix string
		want   string
	}{
		{1, "s", ""},
		{0, "s", "s"},
		{2, "s", "s"},
		{-1, "s", ""},
		{-2, "s", "s"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := format.S(tc.n, tc.suffix); got != tc.want {
				t.Fatalf("S(%d, %q) = %q, want %q", tc.n, tc.suffix, got, tc.want)
			}
		})
	}
}

// TestRelativeAge anchors #429 Phase 1 "Joined GitHub <age>" label.
// The helper must pick the largest unit (year > month > day) for
// which the elapsed difference has at least one whole unit, plural-
// ising the noun off the count.
func TestRelativeAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero time", time.Time{}, ""},
		{"future falls back to today", now.AddDate(0, 0, 5), "today"},
		{"same day", now, "today"},
		{"18 years", time.Date(2008, 1, 14, 4, 33, 35, 0, time.UTC), "18 years ago"},
		{"1 year", time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC), "1 year ago"},
		{"2 months", time.Date(2025, 11, 14, 0, 0, 0, 0, time.UTC), "2 months ago"},
		{"1 month", time.Date(2025, 12, 14, 0, 0, 0, 0, time.UTC), "1 month ago"},
		{"7 days", now.AddDate(0, 0, -7), "7 days ago"},
		{"1 day", now.AddDate(0, 0, -1), "1 day ago"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := format.RelativeAge(tc.t, now); got != tc.want {
				t.Fatalf("RelativeAge(%v, %v) = %q, want %q", tc.t, now, got, tc.want)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	t.Parallel()

	if got := format.FormatError(nil, format.Options{}); got != "" {
		t.Fatalf("FormatError(nil) = %q, want empty", got)
	}
	err := errors.New("boom")
	if got := format.FormatError(err, format.Options{Suffix: " (retrying)"}); got != "boom (retrying)" {
		t.Fatalf("FormatError = %q, want %q", got, "boom (retrying)")
	}
}
