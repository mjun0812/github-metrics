package repositories_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/repositories"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestRepositories_Requires asserts that repositories.Plugin.Requires() declares
// exactly [KeyRepositories, KeyUser]. If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestRepositories_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, repositories.Plugin, []plugins.DataKey{
		plugins.KeyRepositories,
		plugins.KeyUser,
	})
}
