package contributors_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/contributors"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestContributors_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, contributors.Plugin, []plugins.DataKey{})
}
