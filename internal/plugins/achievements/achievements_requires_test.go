package achievements_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider/dataprovidertest"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/achievements"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestAchievements_Requires_Static asserts that achievements.Plugin.Requires()
// declares exactly [KeyUser]. If a developer silently adds a Provider call
// without updating Requires(), the dynamic test below catches the drift.
func TestAchievements_Requires_Static(t *testing.T) {
	requirestesting.AssertExpected(t, achievements.Plugin, []plugins.DataKey{plugins.KeyUser})
}

// TestAchievements_Requires_Dynamic is the canonical end-to-end drift
// detector for the Requires() system. It wires a
// dataprovidertest.CountingMock as the Provider, calls Run, and asserts
// that the set of Provider methods actually invoked equals the set
// declared by Requires().
//
// achievements.providerHasUser is called unconditionally when the computed
// Data fields are all zero — which they are in a minimal test context — so
// Provider.User() is always exercised, making this plugin the most reliable
// candidate for the dynamic check without requiring a GraphQL or REST mock.
//
// If this test fails with "declared but NOT called=[user]", the plugin has
// stopped calling Provider.User() and Requires() should be updated.
// If it fails with "called but NOT declared=[...]", a new Provider call was
// added to the plugin without updating Requires() — add the missing key.
func TestAchievements_Requires_Dynamic(t *testing.T) {
	mock := dataprovidertest.NewCountingMock()

	// Build a minimal PluginContext: all Data fields are zero so
	// providerHasUser() is invoked to determine whether to skip.
	pc := mocks.NewPluginContext(t)
	pc.Provider = mock

	// Run the plugin. providerHasUser() calls Provider.User() because
	// pc.Provider != nil and Data.Computed is empty.
	_, _ = achievements.Plugin.Run(context.Background(), pc)

	requirestesting.AssertCalledMatchesRequires(t, achievements.Plugin, mock)
}
