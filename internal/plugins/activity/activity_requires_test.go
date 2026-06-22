package activity_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/activity"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestActivity_Requires asserts that activity.Plugin.Requires() declares
// exactly [] (no Provider dependencies). If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestActivity_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, activity.Plugin, []plugins.DataKey{})
}
