package projects_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/projects"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestProjects_Requires asserts that projects.Plugin.Requires() declares
// exactly [] (no Provider dependencies). If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestProjects_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, projects.Plugin, []plugins.DataKey{})
}
