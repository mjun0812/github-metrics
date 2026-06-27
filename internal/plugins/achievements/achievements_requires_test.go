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
// declares exactly the Provider methods the plugin calls during Run. If a
// developer silently adds or removes a Provider call without updating
// Requires(), the dynamic test below catches the drift.
//
// #605: after base deletion the achievements plugin reads
// Provider.{User,Repositories,RepositorySummary} via the effectiveData
// helper instead of pc.Data.Computed.* directly.
func TestAchievements_Requires_Static(t *testing.T) {
	requirestesting.AssertExpected(t, achievements.Plugin, []plugins.DataKey{
		plugins.KeyUser,
		plugins.KeyRepositories,
		plugins.KeyRepositorySummary,
	})
}

// TestAchievements_Requires_Dynamic is the canonical end-to-end drift
// detector for the Requires() system. It wires a
// dataprovidertest.CountingMock as the Provider, calls Run, and asserts
// that the set of Provider methods actually invoked equals the set
// declared by Requires().
//
// effectiveData() calls Provider.User / Repositories / RepositorySummary
// unconditionally — they are the canonical source the rankTable funcs
// read after #605 removed the base plugin populating pc.Data.Computed.*.
//
// If this test fails with "declared but NOT called=[...]", the plugin
// has stopped calling Provider for that key and Requires() should drop
// it. If it fails with "called but NOT declared=[...]", a new Provider
// call was added without updating Requires() — add the missing key.
func TestAchievements_Requires_Dynamic(t *testing.T) {
	mock := dataprovidertest.NewCountingMock()

	pc := mocks.NewPluginContext(t)
	pc.Provider = mock

	_, _ = achievements.Plugin.Run(context.Background(), pc)

	requirestesting.AssertCalledMatchesRequires(t, achievements.Plugin, mock)
}
