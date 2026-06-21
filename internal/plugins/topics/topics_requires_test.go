package topics_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/plugins/topics"
)

func TestTopics_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, topics.Plugin, []plugins.DataKey{})
}
