package stargazers_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
)

func TestStargazers_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, stargazers.Plugin, []plugins.DataKey{})
}
