package languages

import (
	"errors"
	"net"
	"testing"

	enrydata "github.com/go-enry/go-enry/v2/data"
)

// TestRecentPluginIdentity pins the trivial plugin getters
// (Name / Metadata) and the typed Result discriminator (IsSkipped) the
// classic dispatcher uses to suppress this plugin's partial when the
// sub-mode is gated off.
func TestRecentPluginIdentity(t *testing.T) {
	t.Parallel()
	if got := RecentPlugin.Name(); got != "languages.recent" {
		t.Errorf("RecentPlugin.Name() = %q, want %q", got, "languages.recent")
	}
	if got := RecentPlugin.Metadata(); got != nil {
		t.Errorf("RecentPlugin.Metadata() = %v, want nil", got)
	}
}

func TestRecentResultIsSkipped(t *testing.T) {
	t.Parallel()
	var nilR *RecentResult
	if nilR.IsSkipped() {
		t.Errorf("nil *RecentResult.IsSkipped() = true, want false")
	}
	if (&RecentResult{Skipped: false}).IsSkipped() {
		t.Errorf("Skipped=false: got true, want false")
	}
	if !(&RecentResult{Skipped: true}).IsSkipped() {
		t.Errorf("Skipped=true: got false, want true")
	}
}

func TestShortSHA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"1234567", "1234567"}, // exactly 7 — unchanged
		{"12345678", "1234567"},
		{"deadbeefcafe", "deadbee"},
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
		{"empty", rawPushPayload{}, []string{}},
		{
			"all populated",
			rawPushPayload{Commits: []rawPushCommit{{SHA: "a"}, {SHA: "b"}, {SHA: "c"}}},
			[]string{"a", "b", "c"},
		},
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

// TestTypeToString pins the enry.Type → category-string mapping the
// recent-mode category filter compares against the user's
// `plugin_languages_recent_categories` input.
func TestTypeToString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   enrydata.Type
		want string
	}{
		{enrydata.TypeProgramming, "programming"},
		{enrydata.TypeMarkup, "markup"},
		{enrydata.TypeProse, "prose"},
		{enrydata.TypeData, "data"},
		{enrydata.TypeUnknown, "unknown"},
		{enrydata.Type(99), "unknown"}, // default branch
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := typeToString(tc.in); got != tc.want {
				t.Errorf("typeToString(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCategoryAllowed covers the "no filter ⇒ accept everything" guard
// and exercises the enry lookup path for known-programming, known-data,
// and unknown language inputs.
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

	t.Run("programming filter accepts Go", func(t *testing.T) {
		t.Parallel()
		if !categoryAllowed("Go", map[string]struct{}{"programming": {}}) {
			t.Errorf("programming filter should accept Go")
		}
	})

	t.Run("programming filter rejects markup language", func(t *testing.T) {
		t.Parallel()
		// HTML is markup, not programming.
		if categoryAllowed("HTML", map[string]struct{}{"programming": {}}) {
			t.Errorf("programming-only filter should reject HTML")
		}
	})

	t.Run("unknown language returns false on populated filter", func(t *testing.T) {
		t.Parallel()
		if categoryAllowed("not-a-real-language-xyz", map[string]struct{}{"programming": {}}) {
			t.Errorf("unknown language with populated filter should be rejected")
		}
	})
}

// TestColorFor covers the override map, the enry fallback, and the
// "no info available" branch.
func TestColorFor(t *testing.T) {
	t.Parallel()

	t.Run("override wins over enry", func(t *testing.T) {
		t.Parallel()
		got := colorFor("Go", map[string]string{"Go": "#deadbe"})
		if got != "#deadbe" {
			t.Errorf("override: got %q, want %q", got, "#deadbe")
		}
	})

	t.Run("falls back to enry for known language", func(t *testing.T) {
		t.Parallel()
		// Go has a canonical color in the enry data file.
		if got := colorFor("Go", nil); got == "" {
			t.Errorf("expected enry color for Go, got empty")
		}
	})

	t.Run("unknown language returns empty", func(t *testing.T) {
		t.Parallel()
		if got := colorFor("not-a-real-language-xyz", nil); got != "" {
			t.Errorf("expected empty color, got %q", got)
		}
	})
}

func TestRecentFetchStatusErrorString(t *testing.T) {
	t.Parallel()
	e := &recentFetchStatusError{status: 503}
	want := "languages.recent: status 503"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// fakeNetError is a net.Error wrapper that lets the test pin the
// Timeout() return for the isTransientFetchError net-error branch.
type fakeNetError struct{ timeout bool }

func (f *fakeNetError) Error() string   { return "fake net error" }
func (f *fakeNetError) Timeout() bool   { return f.timeout }
func (f *fakeNetError) Temporary() bool { return false }

var _ net.Error = (*fakeNetError)(nil)

func TestIsTransientFetchError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"5xx status is transient", &recentFetchStatusError{status: 502}, true},
		{"4xx status is permanent", &recentFetchStatusError{status: 404}, false},
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

// TestParseRecentInputsDefaults pins the defaults applied when the
// input map carries no `plugin_languages_recent_*` keys.
func TestParseRecentInputsDefaults(t *testing.T) {
	t.Parallel()
	in := parseRecentInputs(nil)
	if in.days != 14 {
		t.Errorf("default days = %d, want 14", in.days)
	}
	if in.load != 100 {
		t.Errorf("default load = %d, want 100", in.load)
	}
	if _, ok := in.categories["programming"]; !ok || len(in.categories) != 1 {
		t.Errorf("default categories = %v, want only {programming}", in.categories)
	}
	if len(in.sections) != 1 || in.sections[0] != "most-used" {
		t.Errorf("default sections = %v, want [most-used]", in.sections)
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
