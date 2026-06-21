package dataprovider

import (
	"context"
	"sync"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// CountingMock is a test-only plugins.Provider implementation that
// records which methods were invoked. Per-plugin _requires_test.go
// files build one, run Plugin.Run against a PluginContext wired with
// it, then compare CalledKeys() against Plugin.Requires() so a plugin
// that silently adds a Provider call without updating Requires() fails
// CI.
//
// The mock satisfies plugins.Provider with stubbed return values
// (configurable via the Stub fields). Concurrent calls are safe.
type CountingMock struct {
	// StubProfile, StubUser, etc. are the values returned by each
	// method. Leave at the zero value to return nil with a nil error.
	StubProfile        *plugins.Profile
	StubUser           *plugins.User
	StubOrganization   *plugins.Organization
	StubRepositories   []plugins.Repository
	StubCommitCalendar *plugins.ContributionCalendar

	// ErrProfile, ErrUser, etc. let a test force one method to fail
	// (e.g., to verify the plugin's error-handling branch). nil =
	// success.
	ErrProfile        error
	ErrUser           error
	ErrOrganization   error
	ErrRepositories   error
	ErrCommitCalendar error

	mu     sync.Mutex
	called map[plugins.DataKey]int
}

// NewCountingMock returns a CountingMock ready for use. Stub values
// default to nil; callers that need a non-nil payload assign the
// matching Stub field before invoking Plugin.Run.
func NewCountingMock() *CountingMock {
	return &CountingMock{called: map[plugins.DataKey]int{}}
}

// Profile implements plugins.Provider. Records a call against
// plugins.KeyProfile.
func (m *CountingMock) Profile(_ context.Context) (*plugins.Profile, error) {
	m.record(plugins.KeyProfile)
	return m.StubProfile, m.ErrProfile
}

// User implements plugins.Provider. Records a call against
// plugins.KeyUser.
func (m *CountingMock) User(_ context.Context) (*plugins.User, error) {
	m.record(plugins.KeyUser)
	return m.StubUser, m.ErrUser
}

// Organization implements plugins.Provider. Records a call against
// plugins.KeyOrganization.
func (m *CountingMock) Organization(_ context.Context) (*plugins.Organization, error) {
	m.record(plugins.KeyOrganization)
	return m.StubOrganization, m.ErrOrganization
}

// Repositories implements plugins.Provider. Records a call against
// plugins.KeyRepositories.
func (m *CountingMock) Repositories(_ context.Context) ([]plugins.Repository, error) {
	m.record(plugins.KeyRepositories)
	return m.StubRepositories, m.ErrRepositories
}

// CommitCalendar implements plugins.Provider. Records a call against
// plugins.KeyCommitCalendar.
func (m *CountingMock) CommitCalendar(_ context.Context) (*plugins.ContributionCalendar, error) {
	m.record(plugins.KeyCommitCalendar)
	return m.StubCommitCalendar, m.ErrCommitCalendar
}

// CalledKeys returns the set of DataKey values whose corresponding
// method was invoked at least once. The slice is sorted-equivalent for
// stable comparisons via reflect.DeepEqual after caller-side sort, but
// most callers should pass it to KeySet for set semantics.
func (m *CountingMock) CalledKeys() []plugins.DataKey {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]plugins.DataKey, 0, len(m.called))
	for k := range m.called {
		out = append(out, k)
	}
	return out
}

// CallCount returns the number of times key's method was invoked. Zero
// when the method was never called.
func (m *CountingMock) CallCount(key plugins.DataKey) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called[key]
}

// Reset clears the call counters so a single CountingMock can be
// reused across multiple Plugin.Run invocations.
func (m *CountingMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = map[plugins.DataKey]int{}
}

func (m *CountingMock) record(key plugins.DataKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.called == nil {
		m.called = map[plugins.DataKey]int{}
	}
	m.called[key]++
}

// KeySet converts a slice of DataKey values into a set for order-
// independent equality checks. Duplicate keys collapse.
func KeySet(keys []plugins.DataKey) map[plugins.DataKey]struct{} {
	out := make(map[plugins.DataKey]struct{}, len(keys))
	for _, k := range keys {
		out[k] = struct{}{}
	}
	return out
}

// AssertRequiresMatchesActual returns "" when declared and actual cover
// the same DataKey set, or a human-readable mismatch description
// otherwise. Per-plugin requires_test.go files call this to keep the
// per-test boilerplate to one line.
func AssertRequiresMatchesActual(declared, actual []plugins.DataKey) string {
	d := KeySet(declared)
	a := KeySet(actual)
	if len(d) != len(a) {
		return formatMismatch(d, a)
	}
	for k := range d {
		if _, ok := a[k]; !ok {
			return formatMismatch(d, a)
		}
	}
	return ""
}

func formatMismatch(declared, actual map[plugins.DataKey]struct{}) string {
	missing := diffSet(declared, actual)
	extra := diffSet(actual, declared)
	return "Requires() mismatch:" +
		"\n  declared=" + keysSlice(declared) +
		"\n  actual=" + keysSlice(actual) +
		"\n  declared-but-not-called=" + keysSlice(missing) +
		"\n  called-but-not-declared=" + keysSlice(extra)
}

func diffSet(a, b map[plugins.DataKey]struct{}) map[plugins.DataKey]struct{} {
	out := map[plugins.DataKey]struct{}{}
	for k := range a {
		if _, ok := b[k]; !ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func keysSlice(s map[plugins.DataKey]struct{}) string {
	if len(s) == 0 {
		return "[]"
	}
	// Stable order for readable diff. tiny n; bubble-sort is fine.
	ks := make([]plugins.DataKey, 0, len(s))
	for k := range s {
		ks = append(ks, k)
	}
	for i := 0; i < len(ks); i++ {
		for j := i + 1; j < len(ks); j++ {
			if ks[j] < ks[i] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	out := "["
	for i, k := range ks {
		if i > 0 {
			out += ","
		}
		out += string(k)
	}
	return out + "]"
}
