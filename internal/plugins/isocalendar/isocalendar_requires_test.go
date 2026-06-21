package isocalendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestIsocalendar_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, isocalendar.Plugin, []plugins.DataKey{plugins.KeyUser})
}
