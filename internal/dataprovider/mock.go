package dataprovider

import (
	"context"
	"sync"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// CountingMock implements plugins.Provider. Every method call is recorded
// so tests can assert that the set of Provider methods actually invoked
// during Plugin.Run matches the set declared by Plugin.Requires().
//
// All methods return zero-value non-error responses by default. Use the
// setter fields to override individual return values for plugins that
// branch on the returned data.
type CountingMock struct {
	mu     sync.Mutex
	called map[plugins.DataKey]int

	// Optional overrides. nil means return a zero-value non-error result.
	ProfileFn        func(ctx context.Context) (*plugins.Profile, error)
	UserFn           func(ctx context.Context) (*plugins.User, error)
	OrganizationFn   func(ctx context.Context) (*plugins.Organization, error)
	RepositoriesFn   func(ctx context.Context) ([]plugins.Repository, error)
	CommitCalendarFn func(ctx context.Context) (*plugins.ContributionCalendar, error)
}

// NewCountingMock returns a CountingMock with all counters initialised to
// zero and all optional overrides unset.
func NewCountingMock() *CountingMock {
	return &CountingMock{
		called: make(map[plugins.DataKey]int),
	}
}

// record increments the call counter for key under the mutex.
func (m *CountingMock) record(key plugins.DataKey) {
	m.mu.Lock()
	m.called[key]++
	m.mu.Unlock()
}

// CalledKeys returns the set of DataKeys that were called at least once.
// The returned map is a snapshot; callers may modify it freely.
func (m *CountingMock) CalledKeys() map[plugins.DataKey]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[plugins.DataKey]int, len(m.called))
	for k, v := range m.called {
		out[k] = v
	}
	return out
}

// CalledKeySet returns the set of DataKeys that were called at least once,
// as a map[plugins.DataKey]struct{} for use with requirestesting helpers.
func (m *CountingMock) CalledKeySet() map[plugins.DataKey]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[plugins.DataKey]struct{}, len(m.called))
	for k := range m.called {
		out[k] = struct{}{}
	}
	return out
}

// Profile implements plugins.Provider.
func (m *CountingMock) Profile(ctx context.Context) (*plugins.Profile, error) {
	m.record(plugins.KeyProfile)
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
	m.record(plugins.KeyUser)
	if m.UserFn != nil {
		return m.UserFn(ctx)
	}
	return &plugins.User{Login: "testuser"}, nil
}

// Organization implements plugins.Provider.
func (m *CountingMock) Organization(ctx context.Context) (*plugins.Organization, error) {
	m.record(plugins.KeyOrganization)
	if m.OrganizationFn != nil {
		return m.OrganizationFn(ctx)
	}
	return &plugins.Organization{Login: "testorg"}, nil
}

// Repositories implements plugins.Provider.
func (m *CountingMock) Repositories(ctx context.Context) ([]plugins.Repository, error) {
	m.record(plugins.KeyRepositories)
	if m.RepositoriesFn != nil {
		return m.RepositoriesFn(ctx)
	}
	return []plugins.Repository{}, nil
}

// CommitCalendar implements plugins.Provider.
func (m *CountingMock) CommitCalendar(ctx context.Context) (*plugins.ContributionCalendar, error) {
	m.record(plugins.KeyCommitCalendar)
	if m.CommitCalendarFn != nil {
		return m.CommitCalendarFn(ctx)
	}
	return &plugins.ContributionCalendar{}, nil
}
