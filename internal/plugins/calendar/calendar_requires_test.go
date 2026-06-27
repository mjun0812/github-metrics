package calendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestCalendar_Requires asserts that calendar.Plugin.Requires() declares
// exactly the Provider methods the plugin calls during Run.
//
// #605: calendar.Run reads Provider.CommitCalendar as the fallback when
// fetchYearlyWeeks returns no data (after base deletion this is the
// canonical source instead of pc.Data.Computed.ContributionCalendar).
// Provider.User is still consulted by resolveUser in fetch.go when the
// per-year query path is enabled.
//
// The static check is sufficient here because dynamic invocation also
// depends on pc.GraphQL / plugin_calendar inputs; the end-to-end drift
// detector is exercised by TestAchievements_Requires_Dynamic in the
// achievements package.
func TestCalendar_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, calendar.Plugin, []plugins.DataKey{
		plugins.KeyUser,
		plugins.KeyCommitCalendar,
	})
}
