package calendar_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestCalendar_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, calendar.Plugin, []plugins.DataKey{plugins.KeyUser})
}
