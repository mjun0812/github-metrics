package starlists_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/starlists"
)

func TestStarlists_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, starlists.Plugin, []plugins.DataKey{plugins.KeyRepositories})
}
