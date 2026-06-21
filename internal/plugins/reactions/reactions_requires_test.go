package reactions_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/reactions"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestReactions_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, reactions.Plugin, []plugins.DataKey{})
}
