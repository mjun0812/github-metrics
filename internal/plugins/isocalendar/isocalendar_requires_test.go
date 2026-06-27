package isocalendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestIsocalendar_Requires asserts that isocalendar.Plugin.Requires()
// declares exactly the Provider methods the plugin calls during Run.
//
// #605: isocalendar.Run reads Provider.CommitCalendar as the fallback
// when fetchWindowedWeeks returns no data (after base deletion this is
// the canonical source instead of pc.Data.Computed.ContributionCalendar).
// Provider.User is still consulted by resolveLogin in fetch.go when the
// windowed-fetch path is enabled.
func TestIsocalendar_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, isocalendar.Plugin, []plugins.DataKey{
		plugins.KeyUser,
		plugins.KeyCommitCalendar,
	})
}
