package header_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestHeader_RequiresDeclaration pins header.Plugin.Requires() against
// the hand-curated expectation slice. A future edit that drops either
// KeyProfile or KeyCommitCalendar (or adds a third key) fails here.
func TestHeader_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, header.Plugin, []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyCommitCalendar,
	})
}

// TestHeader_CalledMatchesRequires drives header.Plugin.Run end-to-end
// through dataprovider.CountingMock and asserts the actual set of
// Provider methods invoked during Run equals header.Plugin.Requires().
// This is the runtime drift check the AssertExpected tautology cannot
// perform: if a future edit adds e.g. pc.Provider.User(ctx) inside
// header.Run without bumping Requires(), this test fails.
func TestHeader_CalledMatchesRequires(t *testing.T) {
	mock := dataprovider.NewCountingMock()
	mock.StubProfile = &plugins.Profile{
		Kind: plugins.ProfileKindUser,
		User: &plugins.User{Login: "octocat"},
	}
	mock.StubCommitCalendar = &plugins.ContributionCalendar{}

	pc := mocks.NewPluginContext(
		t,
		mocks.WithInputs(map[string]any{
			"user":          "octocat",
			"plugin_header": "yes",
		}),
	)
	pc.Provider = mock

	if _, err := header.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("header.Plugin.Run: %v", err)
	}

	requirestesting.AssertCalledMatchesRequires(t, header.Plugin, mock.CalledKeys())
}
