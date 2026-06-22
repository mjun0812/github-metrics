package engine

import (
	"testing"
)

// Note: All tests in this file mutate the package-level `version` variable.
// They must NOT be run in parallel with each other or with tests that call
// SetVersionForTest / SetVersion. Sequential execution is intentional.

// TestSetVersionForTest_ChangesAndRestores verifies that SetVersionForTest
// sets the version within the test and restores it after cleanup.
func TestSetVersionForTest_ChangesAndRestores(t *testing.T) {
	// Not parallel: mutates package-level version.
	prev := Version()

	t.Run("inner", func(t *testing.T) {
		// inner is also not parallel — shares the mutation with parent.
		SetVersionForTest(t, "v1.2.3-test")
		if got := Version(); got != "v1.2.3-test" {
			t.Errorf("Version() during test = %q, want %q", got, "v1.2.3-test")
		}
	})

	// After inner completes (and its cleanup ran), version is restored.
	if got := Version(); got != prev {
		t.Errorf("Version() after cleanup = %q, want original %q", got, prev)
	}
}

// TestSetVersionForTest_EmptyString verifies that an empty string is a
// valid version value.
func TestSetVersionForTest_EmptyString(t *testing.T) {
	// Not parallel: mutates package-level version.
	t.Run("inner", func(t *testing.T) {
		SetVersionForTest(t, "")
		if got := Version(); got != "" {
			t.Errorf("Version() = %q, want empty", got)
		}
	})
}

// TestSetVersion_DirectMutation verifies SetVersion persists the value
// and Version reflects it immediately.
func TestSetVersion_DirectMutation(t *testing.T) {
	// Not parallel: directly mutates package-level state. Restore on exit.
	prev := Version()
	t.Cleanup(func() { SetVersion(prev) })

	SetVersion("v9.8.7")
	if got := Version(); got != "v9.8.7" {
		t.Errorf("Version() = %q, want %q", got, "v9.8.7")
	}

	SetVersion("dev")
	if got := Version(); got != "dev" {
		t.Errorf("Version() = %q, want %q", got, "dev")
	}
}

// TestSetVersionForTest_MultipleSubtests verifies each sub-test receives
// an isolated version value and restores it on completion.
func TestSetVersionForTest_MultipleSubtests(t *testing.T) {
	// Not parallel: mutates package-level version sequentially.
	original := Version()

	for _, tc := range []struct {
		name    string
		version string
	}{
		{"alpha", "v0.1.0-alpha"},
		{"beta", "v0.2.0-beta"},
		{"release", "v1.0.0"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Not parallel: sequential sub-tests sharing the package-level var.
			SetVersionForTest(t, tc.version)
			if got := Version(); got != tc.version {
				t.Errorf("Version() = %q, want %q", got, tc.version)
			}
		})
		// Each sub-test's cleanup should have fired by now.
		if got := Version(); got != original {
			t.Errorf("after sub-test %q, Version() = %q, want original %q", tc.name, got, original)
		}
	}
}
