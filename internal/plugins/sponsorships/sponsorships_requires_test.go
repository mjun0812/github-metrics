package sponsorships_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
)

func TestSponsorships_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, sponsorships.Plugin, []plugins.DataKey{})
}
