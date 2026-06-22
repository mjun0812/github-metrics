package sponsors_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
)

// TestSponsors_Requires asserts that sponsors.Plugin.Requires() declares
// exactly [KeyUser]. If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestSponsors_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, sponsors.Plugin, []plugins.DataKey{
		plugins.KeyUser,
	})
}
