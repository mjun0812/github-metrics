package calendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestCalendar_Requires asserts that calendar.Plugin.Requires() declares
// exactly [KeyUser].
//
// The static check is sufficient here because the Provider.User call in
// calendar (via resolveUser in fetch.go) is gated on pc.GraphQL != nil and
// plugin_calendar=true. Constructing a full GraphQL mock for a requires-only
// test would duplicate the existing calendar_test.go network fixtures without
// adding signal. The end-to-end drift detector is demonstrated by
// TestAchievements_Requires_Dynamic in the achievements package, which wires
// a CountingMock without a GraphQL dependency.
func TestCalendar_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, calendar.Plugin, []plugins.DataKey{plugins.KeyUser})
}
