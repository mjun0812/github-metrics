package header_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider/dataprovidertest"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestHeader_Requires_Static asserts that header.Plugin.Requires()
// declares exactly [KeyProfile, KeyCommitCalendar]. If a developer
// silently adds a Provider call without updating Requires(), the
// dynamic test below catches the drift.
//
// Mirrors the per-plugin Requires() drift-test pattern introduced by
// #604 (see internal/plugins/achievements/achievements_requires_test.go).
func TestHeader_Requires_Static(t *testing.T) {
	requirestesting.AssertExpected(t, header.Plugin, []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyCommitCalendar,
	})
}

// TestHeader_Requires_Dynamic wires a dataprovidertest.CountingMock as the
// Provider, calls Run, and asserts that the set of Provider methods
// actually invoked equals the set declared by Requires().
//
// CountingMock's default ProfileFn returns a non-nil *plugins.Profile so
// header.Run proceeds past the nil-profile guard and reaches the
// CommitCalendar call. Both KeyProfile and KeyCommitCalendar are
// therefore recorded.
//
// Failure modes:
//   - "declared but NOT called=[profile]" or "[commit_calendar]" — Run
//     stopped calling that Provider method; either revert the regression
//     or drop the key from Requires().
//   - "called but NOT declared=[...]" — Run added a new Provider call;
//     extend Requires() so prefetch and dependency tracking stay correct.
func TestHeader_Requires_Dynamic(t *testing.T) {
	mock := dataprovidertest.NewCountingMock()

	// Auto-enable gate (#640): without chrome_header (or plugin_header)
	// set, header.Run early-returns before touching the Provider.
	pc := mocks.NewPluginContext(t, mocks.WithInputs(map[string]any{
		"user":          "octocat",
		"chrome_header": "yes",
	}))
	pc.Provider = mock

	_, _ = header.Plugin.Run(context.Background(), pc)

	requirestesting.AssertCalledMatchesRequires(t, header.Plugin, mock)
}
