package calendar_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/calendar"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

func TestCalendar_RequiresDeclaration(t *testing.T) {
	requirestesting.AssertExpected(t, calendar.Plugin, []plugins.DataKey{plugins.KeyUser})
}

// TestCalendar_CalledMatchesRequires drives calendar.Plugin.Run end-to-end
// through dataprovider.CountingMock and asserts the actual set of Provider
// methods invoked during Run equals calendar.Plugin.Requires(). This is the
// runtime drift check the AssertExpected tautology cannot perform: if a
// future edit adds e.g. pc.Provider.CommitCalendar(ctx) inside calendar
// without bumping Requires(), this test fails. It is also the proof-of-life
// for the requirestesting + CountingMock infrastructure that the rest of
// the adopted plugin tree will adopt incrementally.
//
// The Run path here intentionally bails out at fetchYearlyWeeks's
// CreatedAt.IsZero() guard so no GraphQL traffic is exercised; the only
// observable Provider call is the resolveUser-backed Provider.User().
func TestCalendar_CalledMatchesRequires(t *testing.T) {
	mock := dataprovider.NewCountingMock()
	mock.StubUser = &plugins.User{Login: "octocat"} // CreatedAt left zero on purpose

	mux := mocks.NewGraphQLMux(t)
	pc := mocks.NewPluginContext(
		t,
		mocks.WithGraphQL(mux),
		mocks.WithInputs(map[string]any{
			"user":            "octocat",
			"plugin_calendar": "yes",
		}),
	)
	pc.Provider = mock

	if _, err := calendar.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("calendar.Plugin.Run: %v", err)
	}

	requirestesting.AssertCalledMatchesRequires(t, calendar.Plugin, mock.CalledKeys())
}
