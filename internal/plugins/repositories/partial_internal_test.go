package repositories

import (
	"testing"
	"time"
)

// TestFormatCreated pins the upstream repositories/index.mjs date logic:
// < 1 day → "N hour(s) ago", < 30 days → "N day(s) ago", else the
// "Mon DD YYYY" absolute label. A zero time yields "" so the caller
// skips the "created" span entirely (#466).
func TestFormatCreated(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, time.October, 26, 12, 0, 0, 0, time.UTC)
	// Absolute-date cases need a reference "now" far enough ahead that the
	// 30-day relative window has elapsed; otherwise they fall into the
	// "N days ago" branch.
	later := time.Date(2024, time.December, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   time.Time
		now  time.Time
		want string
	}{
		{"zero skips span", time.Time{}, now, ""},
		{"hours ago singular", now.Add(-30 * time.Minute), now, "1 hour ago"},
		{"hours ago plural", now.Add(-3 * time.Hour), now, "3 hours ago"},
		{"days ago singular", now.Add(-24 * time.Hour), now, "1 day ago"},
		{"days ago plural", now.Add(-5 * 24 * time.Hour), now, "5 days ago"},
		{"absolute date zero-padded day", time.Date(2024, time.October, 6, 0, 0, 0, 0, time.UTC), later, "Oct 06 2024"},
		{"absolute date two-digit day", time.Date(2024, time.October, 26, 0, 0, 0, 0, time.UTC), later, "Oct 26 2024"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatCreated(tc.in, tc.now); got != tc.want {
				t.Errorf("formatCreated(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
