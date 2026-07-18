package header_test

import (
	"context"
	"errors"
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

// TestHeader_ProviderErrorRecordedOnData verifies a plugin-local Provider
// failure is threaded onto the shared Data accumulator (#781) so
// engine.collectPluginErrors surfaces it in the run log and honours
// plugins_errors_fatal, instead of it living only on Result.Error.
func TestHeader_ProviderErrorRecordedOnData(t *testing.T) {
	sentinel := errors.New("resource limit")
	mock := dataprovidertest.NewCountingMock()
	mock.ProfileFn = func(context.Context) (*plugins.Profile, error) { return nil, sentinel }

	pc := mocks.NewPluginContext(t, mocks.WithInputs(map[string]any{
		"user":          "octocat",
		"chrome_header": "yes",
	}))
	pc.Provider = mock

	got, err := header.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run must not propagate plugin-local errors, got %v", err)
	}
	r := got.(*header.Result)
	if !errors.Is(r.Error, sentinel) {
		t.Errorf("Result.Error = %v, want wraps sentinel", r.Error)
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 || !errors.Is(errs[0], sentinel) {
		t.Errorf("Data.SnapshotErrors() = %v, want one entry wrapping sentinel", errs)
	}
}
