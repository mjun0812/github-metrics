package people_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/people"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestPeople_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, people.Plugin, []plugins.DataKey{})
}
