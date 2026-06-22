package people_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/people"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestPeople_Requires asserts that people.Plugin.Requires() declares
// exactly [] (no Provider dependencies). If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestPeople_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, people.Plugin, []plugins.DataKey{})
}
