package base_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider/dataprovidertest"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/base"
	"github.com/mjun0812/github-metrics/internal/plugins/requirestesting"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestBase_Requires_Static asserts that base.Plugin.Requires() declares
// exactly [KeyProfile, KeyRepositorySummary]. The dynamic test below
// catches the case where Run silently starts calling additional
// Provider methods.
func TestBase_Requires_Static(t *testing.T) {
	requirestesting.AssertExpected(t, base.Plugin, []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyRepositorySummary,
	})
}

// TestBase_Requires_Dynamic wires a CountingMock and asserts the set of
// Provider methods touched by Run equals the declared Requires() set.
func TestBase_Requires_Dynamic(t *testing.T) {
	mock := dataprovidertest.NewCountingMock()

	pc := mocks.NewPluginContext(t)
	pc.Provider = mock

	_, _ = base.Plugin.Run(context.Background(), pc)

	requirestesting.AssertCalledMatchesRequires(t, base.Plugin, mock)
}

// TestRun_NilContextReturnsEmptyResult — base must tolerate the
// unwired ctor used by per-plugin tests that exercise the lookup path
// without a Provider.
func TestRun_NilContextReturnsEmptyResult(t *testing.T) {
	t.Parallel()
	got, err := base.Plugin.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run(nil pc): err=%v", err)
	}
	r, ok := got.(*base.Result)
	if !ok || r == nil {
		t.Fatalf("Run(nil pc): want non-nil *Result, got %T %v", got, got)
	}
	if r.Profile != nil || r.RepositorySummary != nil || r.Error != nil {
		t.Errorf("Run(nil pc): want zero-value Result, got %+v", r)
	}
	// Empty (zero-value) Result is NOT skipped; IsSkipped only reports
	// true for a literal nil receiver. Anchor the contract here so a
	// future change is loud.
	if r.IsSkipped() {
		t.Errorf("IsSkipped on empty Result: got true, want false (only nil receiver should report skipped)")
	}
	var nilResult *base.Result
	if !nilResult.IsSkipped() {
		t.Errorf("IsSkipped on nil receiver: got false, want true")
	}
}

// TestRun_NilProviderReturnsEmptyResult mirrors the header plugin's
// guard: a PluginContext with no Provider must return a non-nil
// zero-value Result and no error so the runner records it without
// crashing.
func TestRun_NilProviderReturnsEmptyResult(t *testing.T) {
	t.Parallel()
	pc := mocks.NewPluginContext(t)
	pc.Provider = nil

	got, err := base.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run(nil Provider): err=%v", err)
	}
	r, ok := got.(*base.Result)
	if !ok || r == nil {
		t.Fatalf("Run(nil Provider): want non-nil *Result, got %T", got)
	}
	if r.Profile != nil || r.RepositorySummary != nil {
		t.Errorf("Run(nil Provider): expected unpopulated Result, got %+v", r)
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

	pc := mocks.NewPluginContext(t)
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

	pc := mocks.NewPluginContext(t)
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

	pc := mocks.NewPluginContext(t)
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

// TestResult_IsSkipped covers the nil-receiver contract used by the
// classic dispatcher's SkippableResult interface.
func TestResult_IsSkipped(t *testing.T) {
	t.Parallel()
	var nilResult *base.Result
	if !nilResult.IsSkipped() {
		t.Errorf("nil *Result must report IsSkipped() = true")
	}
	empty := &base.Result{}
	if empty.IsSkipped() {
		t.Errorf("non-nil empty *Result must NOT report IsSkipped() = true")
	}
}
