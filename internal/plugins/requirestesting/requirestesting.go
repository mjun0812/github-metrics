// Package requirestesting provides helpers for per-plugin Requires() drift
// tests. Two assertion styles are offered:
//
//   - AssertExpected: static check — asserts that Plugin.Requires() returns
//     exactly the expected set. Use as the lightweight baseline check for
//     plugins that do not exercise end-to-end Run calls in unit tests.
//
//   - AssertCalledMatchesRequires: dynamic end-to-end check — calls
//     Plugin.Run with a dataprovider.CountingMock and asserts that the set
//     of Provider methods actually invoked equals the declared Requires()
//     set. Fails immediately when a plugin silently starts calling a new
//     Provider method without updating Requires().
package requirestesting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/dataprovider"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// TB is the subset of testing.TB this package needs so the production
// binary does not transitively import the testing package.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// KeySet converts a []plugins.DataKey slice to a set for order-independent
// equality comparisons.
func KeySet(keys []plugins.DataKey) map[plugins.DataKey]struct{} {
	out := make(map[plugins.DataKey]struct{}, len(keys))
	for _, k := range keys {
		out[k] = struct{}{}
	}
	return out
}

// sortedKeys returns the keys of a set as a sorted string slice.
func sortedKeys(s map[plugins.DataKey]struct{}) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, string(k))
	}
	sort.Strings(out)
	return out
}

// diff returns elements in a that are absent from b.
func diff(a, b map[plugins.DataKey]struct{}) map[plugins.DataKey]struct{} {
	out := map[plugins.DataKey]struct{}{}
	for k := range a {
		if _, ok := b[k]; !ok {
			out[k] = struct{}{}
		}
	}
	return out
}

// AssertExpected asserts that p.Requires() returns exactly the expected
// set of DataKeys (order-insensitive). Use for a static declaration check
// without wiring a full Run execution.
func AssertExpected(t TB, p plugins.Plugin, expected []plugins.DataKey) {
	t.Helper()

	want := KeySet(expected)
	got := KeySet(p.Requires())

	missing := diff(want, got)
	extra := diff(got, want)

	if len(missing) == 0 && len(extra) == 0 {
		return
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing from Requires()=%v", sortedKeys(missing)))
	}
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("extra in Requires()=%v", sortedKeys(extra)))
	}
	t.Fatalf(
		"%s.Requires() mismatch:\n  declared=%v\n  expected=%v\n  %s",
		p.Name(),
		sortedKeys(got),
		sortedKeys(want),
		strings.Join(parts, "; "),
	)
}

// AssertCalledMatchesRequires asserts that the set of Provider methods
// recorded by mock after calling Plugin.Run equals the set declared by
// Plugin.Requires(). Call this AFTER running the plugin with mock as the
// provider so the counting is populated.
//
// This is the dynamic drift detector: if a plugin starts calling an
// undeclared Provider method, or stops calling a declared one, this
// assertion fires.
func AssertCalledMatchesRequires(t TB, p plugins.Plugin, mock *dataprovider.CountingMock) {
	t.Helper()

	declared := KeySet(p.Requires())
	actual := mock.CalledKeySet()

	missing := diff(declared, actual)
	extra := diff(actual, declared)

	if len(missing) == 0 && len(extra) == 0 {
		return
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts,
			fmt.Sprintf("declared but NOT called=%v (over-declared)", sortedKeys(missing)))
	}
	if len(extra) > 0 {
		parts = append(parts,
			fmt.Sprintf("called but NOT declared=%v (missing from Requires)", sortedKeys(extra)))
	}
	t.Fatalf(
		"%s Requires() drift detected:\n  declared=%v\n  actually called=%v\n  %s",
		p.Name(),
		sortedKeys(declared),
		sortedKeys(actual),
		strings.Join(parts, "; "),
	)
}
