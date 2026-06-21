package habits_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/habits"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestHabits_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, habits.Plugin, []plugins.DataKey{})
}
