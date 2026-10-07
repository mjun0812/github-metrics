package languages

import (
	"errors"
	"testing"
)

func TestShortSHA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"12345678", "1234567"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := shortSHA(tc.in); got != tc.want {
				t.Errorf("shortSHA(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCommitSHAs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   rawPushPayload
		want []string
	}{
		{
			"skips empty SHAs",
			rawPushPayload{Commits: []rawPushCommit{{SHA: ""}, {SHA: "a"}, {SHA: ""}, {SHA: "b"}}},
			[]string{"a", "b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.in.commitSHAs()
			if len(got) != len(tc.want) {
				t.Fatalf("len=%d want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestCategoryAllowed covers the "no filter ⇒ accept everything" guard
// and the rejection of unknown languages under a populated filter.
func TestCategoryAllowed(t *testing.T) {
	t.Parallel()

	t.Run("empty filter accepts everything (including unknown)", func(t *testing.T) {
		t.Parallel()
		if !categoryAllowed("Go", nil) {
			t.Errorf("nil filter: Go should pass")
		}
		if !categoryAllowed("not-a-language", map[string]struct{}{}) {
			t.Errorf("empty filter: unknown lang should pass")
		}
	})

	t.Run("unknown language returns false on populated filter", func(t *testing.T) {
		t.Parallel()
		if categoryAllowed("not-a-real-language-xyz", map[string]struct{}{"programming": {}}) {
			t.Errorf("unknown language with populated filter should be rejected")
		}
	})
}

// TestColorFor covers the override map taking precedence over enry.
func TestColorFor(t *testing.T) {
	t.Parallel()

	t.Run("override wins over enry", func(t *testing.T) {
		t.Parallel()
		got := colorFor("Go", map[string]string{"Go": "#deadbe"})
		if got != "#deadbe" {
			t.Errorf("override: got %q, want %q", got, "#deadbe")
		}
	})
}

// fakeNetError is a net.Error wrapper that lets the test pin the
// Timeout() return for the isTransientFetchError net-error branch.
type fakeNetError struct{ timeout bool }

func (f *fakeNetError) Error() string   { return "fake net error" }
func (f *fakeNetError) Timeout() bool   { return f.timeout }
func (f *fakeNetError) Temporary() bool { return false }

func TestIsTransientFetchError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"500 is the lower bound of transient", &recentFetchStatusError{status: 500}, true},
		{"499 is permanent", &recentFetchStatusError{status: 499}, false},
		{"net.Error with Timeout is transient", &fakeNetError{timeout: true}, true},
		{"net.Error without Timeout is not", &fakeNetError{timeout: false}, false},
		{"retryablehttp giving-up string is transient", errors.New("GET /x: giving up after 4 attempts"), true},
		{"unrelated error is not transient", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientFetchError(tc.err); got != tc.want {
				t.Errorf("isTransientFetchError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestParseRecentInputsNegativeLoadFallsBack asserts a non-positive
// plugin_languages_recent_load keeps the default instead of reaching
// fetchPushEvents, where make(..., 0, load) would panic.
func TestParseRecentInputsNegativeLoadFallsBack(t *testing.T) {
	t.Parallel()
	in := parseRecentInputs(map[string]any{
		"plugin_languages_recent_load": -1,
	})
	if in.load != 100 {
		t.Errorf("load = %d, want default 100 for negative input", in.load)
	}
}

func TestParseRecentInputsOverrides(t *testing.T) {
	t.Parallel()
	in := parseRecentInputs(map[string]any{
		"plugin_languages_recent_days":       7,
		"plugin_languages_recent_load":       50,
		"plugin_languages_recent_categories": "Markup, Data",
		"plugin_languages_sections":          "recently-used, most-used",
	})
	if in.days != 7 {
		t.Errorf("days override = %d, want 7", in.days)
	}
	if in.load != 50 {
		t.Errorf("load override = %d, want 50", in.load)
	}
	if _, ok := in.categories["markup"]; !ok {
		t.Errorf("categories should contain markup (lowercased): %v", in.categories)
	}
	if _, ok := in.categories["data"]; !ok {
		t.Errorf("categories should contain data (lowercased): %v", in.categories)
	}
	if len(in.sections) != 2 || in.sections[0] != "recently-used" {
		t.Errorf("sections = %v, want [recently-used, most-used]", in.sections)
	}
}

func TestParseRecentInputsSectionsEmptyStringFallsBack(t *testing.T) {
	t.Parallel()
	in := parseRecentInputs(map[string]any{"plugin_languages_sections": ""})
	if len(in.sections) != 1 || in.sections[0] != "most-used" {
		t.Errorf("empty-string sections should fall back to default, got %v", in.sections)
	}
}
