package achievements_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/achievements"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestAchievements_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, achievements.Plugin, []plugins.DataKey{plugins.KeyUser})
}
