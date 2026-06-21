package base_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestBase_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, base.Plugin, []plugins.DataKey{})
}
