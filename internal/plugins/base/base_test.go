package base_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider/dataprovidertest"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// enabledInputs returns the input map used by Run-side tests so the
// chrome auto-enable gate (#640) lets Run reach the Provider calls.
func enabledInputs() map[string]any {
	return map[string]any{
		"user":                "octocat",
		"chrome_activity":     "yes",
		"chrome_community":    "yes",
		"chrome_repositories": "yes",
	}
}

// TestRun_WithoutDependenciesReturnsEmptyResult — a nil PluginContext or a
// PluginContext with no Provider must return a non-nil zero-value Result
// and no error so the runner records it without crashing. The empty
// Result is not skipped; IsSkipped reports true only for a nil receiver.
func TestRun_WithoutDependenciesReturnsEmptyResult(t *testing.T) {
	t.Parallel()
	noProvider := mocks.NewPluginContext(t)
	noProvider.Provider = nil
	for name, pc := range map[string]*plugins.PluginContext{
		"nil pc":       nil,
		"nil Provider": noProvider,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := base.Plugin.Run(context.Background(), pc)
			if err != nil {
				t.Fatalf("Run: err=%v", err)
			}
			r, ok := got.(*base.Result)
			if !ok || r == nil {
				t.Fatalf("want non-nil *Result, got %T %v", got, got)
			}
			if r.Profile != nil || r.RepositorySummary != nil || r.Error != nil {
				t.Errorf("want zero-value Result, got %+v", r)
			}
			if r.IsSkipped() {
				t.Errorf("IsSkipped on empty Result: got true, want false")
			}
		})
	}
	var nilResult *base.Result
	if !nilResult.IsSkipped() {
		t.Errorf("IsSkipped on nil receiver: got false, want true")
	}
}

// TestRun_SuccessPath populates Result from a Provider that returns
// non-empty Profile + RepositorySummary.
func TestRun_SuccessPath(t *testing.T) {
	t.Parallel()
	mock := dataprovidertest.NewCountingMock()
	mock.ProfileFn = func(_ context.Context) (*plugins.Profile, error) {
		return &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "octocat", Commits: 42},
		}, nil
	}
	mock.RepositorySummaryFn = func(_ context.Context) (*plugins.ComputedRepositories, error) {
		return &plugins.ComputedRepositories{Count: 7, Stargazers: 100}, nil
	}

	pc := mocks.NewPluginContext(t, mocks.WithInputs(enabledInputs()))
	pc.Provider = mock

	got, err := base.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: err=%v", err)
	}
	r := got.(*base.Result)
	if r.Profile == nil || r.Profile.User == nil || r.Profile.User.Login != "octocat" {
		t.Errorf("Profile not populated: %+v", r.Profile)
	}
	if r.RepositorySummary == nil || r.RepositorySummary.Count != 7 {
		t.Errorf("RepositorySummary not populated: %+v", r.RepositorySummary)
	}
	if r.Error != nil {
		t.Errorf("unexpected Error: %v", r.Error)
	}
}

// TestRun_ProfileErrorRecordsAndReturnsNilErr — when Provider.Profile
// fails the Result carries the Error and Run still returns nil so the
// runner records the failure without aborting the rest of the pipeline.
func TestRun_ProfileErrorRecordsAndReturnsNilErr(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	mock := dataprovidertest.NewCountingMock()
	mock.ProfileFn = func(_ context.Context) (*plugins.Profile, error) { return nil, sentinel }

	pc := mocks.NewPluginContext(t, mocks.WithInputs(enabledInputs()))
	pc.Provider = mock

	got, err := base.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run must not propagate plugin-local errors, got %v", err)
	}
	r := got.(*base.Result)
	if !errors.Is(r.Error, sentinel) {
		t.Errorf("Result.Error = %v, want wraps sentinel %v", r.Error, sentinel)
	}
	if r.Profile != nil || r.RepositorySummary != nil {
		t.Errorf("Result should be unpopulated on early failure, got %+v", r)
	}
	// #781: the plugin-local failure must also reach the shared Data
	// accumulator so engine.collectPluginErrors surfaces it (logs +
	// plugins_errors_fatal) instead of it living only on Result.Error.
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 || !errors.Is(errs[0], sentinel) {
		t.Errorf("Data.SnapshotErrors() = %v, want one entry wrapping sentinel", errs)
	}
}

// TestRun_RepositorySummaryErrorRecorded — Profile succeeds but
// RepositorySummary fails. Result keeps the Profile and records the
// summary error.
func TestRun_RepositorySummaryErrorRecorded(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("rate limit")
	mock := dataprovidertest.NewCountingMock()
	mock.ProfileFn = func(_ context.Context) (*plugins.Profile, error) {
		return &plugins.Profile{
			Kind: plugins.ProfileKindUser,
			User: &plugins.User{Login: "octocat"},
		}, nil
	}
	mock.RepositorySummaryFn = func(_ context.Context) (*plugins.ComputedRepositories, error) {
		return nil, sentinel
	}

	pc := mocks.NewPluginContext(t, mocks.WithInputs(enabledInputs()))
	pc.Provider = mock

	got, err := base.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run must not propagate plugin-local errors, got %v", err)
	}
	r := got.(*base.Result)
	if r.Profile == nil {
		t.Errorf("expected Profile to be preserved after summary failure")
	}
	if r.RepositorySummary != nil {
		t.Errorf("expected RepositorySummary to be unpopulated, got %+v", r.RepositorySummary)
	}
	if !errors.Is(r.Error, sentinel) {
		t.Errorf("Result.Error = %v, want wraps sentinel %v", r.Error, sentinel)
	}
}
