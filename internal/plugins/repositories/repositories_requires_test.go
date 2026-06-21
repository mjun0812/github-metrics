package repositories_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/repositories"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestRepositories_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, repositories.Plugin,
		[]plugins.DataKey{plugins.KeyUser, plugins.KeyRepositories})
}
