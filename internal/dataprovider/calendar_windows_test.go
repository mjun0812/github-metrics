package dataprovider

import (
	"testing"
	"time"
)

// TestCalendarWindows_SundayAlignedNoMidWeekSplit pins the #781 fix for
// the shared trailing-year calendar: every window boundary lands on a
// Sunday (GitHub's week start) so no calendar week is returned as two
// partial weeks in adjacent windows (which the isocalendar/calendar
// CommitCalendar fallback consumes week-by-week and would render broken).
func TestCalendarWindows_SundayAlignedNoMidWeekSplit(t *testing.T) {
	// A Wednesday "now" so the un-snapped trailing-year start would fall
	// mid-week without the Sunday snap.
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	windows := calendarWindows(now)
	if len(windows) < 2 {
		t.Fatalf("expected the trailing year to split into >=2 windows, got %d", len(windows))
	}

	if wd := windows[0][0].Weekday(); wd != time.Sunday {
		t.Errorf("first window starts on %s, want Sunday", wd)
	}
	if h := windows[0][0].Hour(); h != 0 {
		t.Errorf("first window start hour = %d, want 0 (UTC midnight)", h)
	}

	for i, w := range windows {
		start, end := w[0], w[1]
		if !start.Before(end) {
			t.Fatalf("window %d has non-increasing range %v..%v", i, start, end)
		}
		if i > 0 {
			// Interior window starts are Sundays at UTC midnight.
			if start.Weekday() != time.Sunday || start.Hour() != 0 {
				t.Errorf("window %d starts at %v, want a Sunday 00:00 UTC", i, start)
			}
			// Tiling with no gap/overlap.
			prevEnd := windows[i-1][1]
			if !start.Equal(prevEnd.Add(time.Millisecond)) {
				t.Errorf("window %d start %v does not abut previous end %v", i, start, prevEnd)
			}
		}
	}
	if got := windows[len(windows)-1][1]; !got.Equal(now.Add(-time.Millisecond)) {
		t.Errorf("last window ends at %v, want now-1ms (%v)", got, now.Add(-time.Millisecond))
	}
}
