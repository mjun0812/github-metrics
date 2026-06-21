package languages_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/languages"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestLanguages_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, languages.Plugin, []plugins.DataKey{plugins.KeyRepositories})
}

func TestLanguagesRecent_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, languages.RecentPlugin, []plugins.DataKey{})
}

func TestLanguagesIndepth_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, languages.IndepthPlugin, []plugins.DataKey{})
}
