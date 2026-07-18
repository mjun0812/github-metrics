package calendar

import (
	"testing"
	"time"
)

// TestCalendarChunks_InteriorBoundariesAreSundayAligned pins the #781
// fix: a calendar year longer than one chunk is split only on Sunday
// (week-start) boundaries so GitHub never returns the boundary week as
// two partial weeks in adjacent windows (which calendar.go would render
// as a broken mid-year column with the wrong weekday offset).
func TestCalendarChunks_InteriorBoundariesAreSundayAligned(t *testing.T) {
	// A full calendar year starting on a Wednesday (2025-01-01) — the
	// worst case for mid-week drift.
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 12, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)

	chunks := calendarChunks(from, to)
	if len(chunks) < 2 {
		t.Fatalf("expected a full year to split into multiple windows, got %d", len(chunks))
	}

	for i, c := range chunks {
		start, end := c[0], c[1]
		if !start.Before(end) {
			t.Fatalf("chunk %d has non-increasing range %v..%v", i, start, end)
		}
		// The very first chunk starts at the un-snapped year start; every
		// later chunk must begin on a Sunday at UTC midnight.
		if i > 0 {
			if start.Weekday() != time.Sunday || start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
				t.Errorf("chunk %d starts at %v, want a Sunday 00:00 UTC", i, start)
			}
		}
		// Chunks tile [from, to] with no gap and no overlap: each ends 1ms
		// before the next begins.
		if i > 0 {
			prevEnd := chunks[i-1][1]
			if !start.Equal(prevEnd.Add(time.Millisecond)) {
				t.Errorf("chunk %d start %v does not abut previous end %v", i, start, prevEnd)
			}
		}
		// No window exceeds the intended width by more than the snap slack
		// (< 1 week), keeping every request under the node limit.
		if d := end.Sub(start); d > (chunkWindowDays+7)*24*time.Hour {
			t.Errorf("chunk %d spans %v, wider than the configured window", i, d)
		}
	}
	if got := chunks[0][0]; !got.Equal(from) {
		t.Errorf("first chunk start = %v, want the year start %v", got, from)
	}
	if got := chunks[len(chunks)-1][1]; !got.Equal(to) {
		t.Errorf("last chunk end = %v, want the year end %v", got, to)
	}
}
