package traffic_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/traffic"
)

func TestTraffic_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, traffic.Plugin, []plugins.DataKey{plugins.KeyRepositories})
}
