package traffic_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
)

// TestTraffic_Requires asserts that traffic.Plugin.Requires() declares
// exactly [KeyRepositories]. If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestTraffic_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, traffic.Plugin, []plugins.DataKey{
		plugins.KeyRepositories,
	})
}
