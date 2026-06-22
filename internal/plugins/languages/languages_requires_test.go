package languages_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestLanguages_Requires asserts that languages.Plugin.Requires() declares
// exactly [KeyRepositories]. If a developer silently adds a Provider call
// without updating Requires(), the drift-detector test in calendar
// (TestCalendar_Requires_Dynamic) provides the end-to-end check; this
// test anchors the static declaration.
func TestLanguages_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, languages.Plugin, []plugins.DataKey{
		plugins.KeyRepositories,
	})
}
