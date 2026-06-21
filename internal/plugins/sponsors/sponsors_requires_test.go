package sponsors_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsors"
)

func TestSponsors_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, sponsors.Plugin, []plugins.DataKey{plugins.KeyUser})
}
