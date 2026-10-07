// Package dataprovidertest provides test-only helpers for the
// dataprovider package. Keeping these types in a sub-package that no
// production code imports ensures they are not linked into release
// binaries, even though they need to be reused from _test.go files
// across multiple plugin packages (which prevents them from living in
// a single _test.go file).
package dataprovidertest

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// CountingMock implements plugins.Provider for plugin unit tests.
//
// All methods return zero-value non-error responses by default. Use the
// setter fields to override individual return values for plugins that
// branch on the returned data.
type CountingMock struct {
	// Optional overrides. nil means return a zero-value non-error result.
	ProfileFn           func(ctx context.Context) (*plugins.Profile, error)
	UserFn              func(ctx context.Context) (*plugins.User, error)
	OrganizationFn      func(ctx context.Context) (*plugins.Organization, error)
	RepositoriesFn      func(ctx context.Context) ([]plugins.Repository, error)
	RepositorySummaryFn func(ctx context.Context) (*plugins.ComputedRepositories, error)
	CommitCalendarFn    func(ctx context.Context) (*plugins.ContributionCalendar, error)
	RepoFn              func(ctx context.Context) (*plugins.Repo, error)
}

// NewCountingMock returns a CountingMock with all optional overrides unset.
func NewCountingMock() *CountingMock {
	return &CountingMock{}
}

// Profile implements plugins.Provider.
func (m *CountingMock) Profile(ctx context.Context) (*plugins.Profile, error) {
	if m.ProfileFn != nil {
		return m.ProfileFn(ctx)
	}
	// Return a stub user profile so plugins that branch on nil can proceed.
	return &plugins.Profile{
		Kind: plugins.ProfileKindUser,
		User: &plugins.User{Login: "testuser"},
	}, nil
}

// User implements plugins.Provider.
func (m *CountingMock) User(ctx context.Context) (*plugins.User, error) {
	if m.UserFn != nil {
		return m.UserFn(ctx)
	}
	return &plugins.User{Login: "testuser"}, nil
}

// Organization implements plugins.Provider.
func (m *CountingMock) Organization(ctx context.Context) (*plugins.Organization, error) {
	if m.OrganizationFn != nil {
		return m.OrganizationFn(ctx)
	}
	return &plugins.Organization{Login: "testorg"}, nil
}

// Repositories implements plugins.Provider.
func (m *CountingMock) Repositories(ctx context.Context) ([]plugins.Repository, error) {
	if m.RepositoriesFn != nil {
		return m.RepositoriesFn(ctx)
	}
	return []plugins.Repository{}, nil
}

// RepositorySummary implements plugins.Provider.
func (m *CountingMock) RepositorySummary(ctx context.Context) (*plugins.ComputedRepositories, error) {
	if m.RepositorySummaryFn != nil {
		return m.RepositorySummaryFn(ctx)
	}
	return &plugins.ComputedRepositories{}, nil
}

// CommitCalendar implements plugins.Provider.
func (m *CountingMock) CommitCalendar(ctx context.Context) (*plugins.ContributionCalendar, error) {
	if m.CommitCalendarFn != nil {
		return m.CommitCalendarFn(ctx)
	}
	return &plugins.ContributionCalendar{}, nil
}

// Repo implements plugins.Provider.
func (m *CountingMock) Repo(ctx context.Context) (*plugins.Repo, error) {
	if m.RepoFn != nil {
		return m.RepoFn(ctx)
	}
	return nil, nil
}
