// Package requirestesting provides shared helpers for the per-plugin
// _requires_test.go files introduced by #604.
//
// Every adopted plugin owns one _requires_test.go. At minimum it calls
// AssertExpected, which pins Plugin.Requires() against a hand-curated
// expectation slice — a cheap drift guard for the declared contract.
//
// The richer AssertCalledMatchesRequires path (backed by
// dataprovider.CountingMock) additionally drives Plugin.Run and
// asserts the actually-invoked Provider methods match Plugin.Requires().
// It catches the regression AssertExpected cannot: a plugin silently
// starts calling pc.Provider.X without updating Requires(). Adoption
// is incremental — calendar is the proof-of-life caller; other
// plugins migrate as their Run paths become test-friendly (most need
// GraphQL/REST fixture wiring to reach their Provider calls).
package requirestesting

import (
	"sort"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// AssertExpected fails t when p.Requires() differs from want as a set.
// Order and duplicates are ignored — the contract is set equality.
//
// Per-plugin tests use this to lock the declared Requires() against a
// hand-curated expectation table; a future drive-by edit that adds
// (e.g.) plugins.KeyCommitCalendar without updating the test fails
// here. AssertCalledMatchesRequires extends the check to runtime call
// counts via dataprovider.CountingMock.
func AssertExpected(t *testing.T, p plugins.Plugin, want []plugins.DataKey) {
	t.Helper()
	got := p.Requires()
	if !setEqual(got, want) {
		t.Fatalf("%s.Requires() = %v, want %v (set equality)", p.Name(), sortedCopy(got), sortedCopy(want))
	}
}

// AssertCalledMatchesRequires fails t when called (from
// CountingMock.CalledKeys()) is not equal as a set to p.Requires().
// Designed for plugins where the test harness can drive Run far enough
// to invoke every declared Provider method.
func AssertCalledMatchesRequires(t *testing.T, p plugins.Plugin, called []plugins.DataKey) {
	t.Helper()
	declared := p.Requires()
	if !setEqual(called, declared) {
		t.Fatalf("%s: declared Requires()=%v but Provider methods called=%v",
			p.Name(), sortedCopy(declared), sortedCopy(called))
	}
}

func setEqual(a, b []plugins.DataKey) bool {
	if len(a) != len(b) {
		return false
	}
	as := map[plugins.DataKey]struct{}{}
	for _, k := range a {
		as[k] = struct{}{}
	}
	for _, k := range b {
		if _, ok := as[k]; !ok {
			return false
		}
	}
	return len(as) == len(uniqueSet(b))
}

func uniqueSet(s []plugins.DataKey) map[plugins.DataKey]struct{} {
	out := map[plugins.DataKey]struct{}{}
	for _, k := range s {
		out[k] = struct{}{}
	}
	return out
}

func sortedCopy(s []plugins.DataKey) []plugins.DataKey {
	out := make([]plugins.DataKey, len(s))
	copy(out, s)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
