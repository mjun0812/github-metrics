package stars_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/stars"
)

func TestStars_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, stars.Plugin, []plugins.DataKey{})
}
