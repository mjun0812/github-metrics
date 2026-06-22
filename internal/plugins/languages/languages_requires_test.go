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

// TestLanguagesIndepth_Requires asserts that languages.IndepthPlugin.Requires()
// declares exactly [] (no Provider dependencies). languages-indepth reads
// pc.Data fields populated by base and the languages plugin, not Provider.
func TestLanguagesIndepth_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, languages.IndepthPlugin, []plugins.DataKey{})
}

// TestLanguagesRecent_Requires asserts that languages.RecentPlugin.Requires()
// declares exactly [] (no Provider dependencies). languages-recent fetches
// data through the GitHub REST API directly and does not consume Provider.
func TestLanguagesRecent_Requires(t *testing.T) {
	requirestesting.AssertExpected(t, languages.RecentPlugin, []plugins.DataKey{})
}
