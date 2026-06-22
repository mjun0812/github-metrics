package isocalendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestIsocalendar_Requires asserts that isocalendar.Plugin.Requires() declares
// exactly [KeyUser]. If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestIsocalendar_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, isocalendar.Plugin, []plugins.DataKey{
		plugins.KeyUser,
	})
}
