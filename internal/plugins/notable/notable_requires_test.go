package notable_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/notable"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestNotable_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, notable.Plugin, []plugins.DataKey{})
}
