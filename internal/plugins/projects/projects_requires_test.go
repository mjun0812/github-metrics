package projects_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/projects"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

func TestProjects_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, projects.Plugin, []plugins.DataKey{})
}
