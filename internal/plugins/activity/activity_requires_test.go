package activity_test

import (
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/activity"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
)

// TestActivity_RequiresDeclaration locks activity's declared Provider
// data sources. A drive-by addition of pc.Provider.X(ctx) in activity
// must update this expectation (and ideally a CountingMock-driven
// runtime check too).
func TestActivity_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, activity.Plugin, []plugins.DataKey{})
}
