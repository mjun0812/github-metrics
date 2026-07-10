package pluginutil_test

import (
	"reflect"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

func TestTruthy(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{"bool-true", true, true},
		{"bool-false", false, false},
		{"str-true", "true", true},
		{"str-TRUE-trim", "  TRUE  ", true},
		{"str-yes", "yes", true},
		{"str-1", "1", true},
		{"str-no", "no", false},
		{"str-empty", "", false},
		{"int-1", 1, true},
		{"int-0", 0, false},
		{"int64-1", int64(1), true},
		{"float-0.5", 0.5, true},
		{"float-0", 0.0, false},
		{"unknown", []string{"x"}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pluginutil.Truthy(tc.in); got != tc.want {
				t.Errorf("Truthy(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruthyInput(t *testing.T) {
	if pluginutil.TruthyInput(nil, "k") {
		t.Errorf("nil map should not be truthy")
	}
	if pluginutil.TruthyInput(map[string]any{}, "k") {
		t.Errorf("missing key should not be truthy")
	}
	if !pluginutil.TruthyInput(map[string]any{"k": "YES"}, "k") {
		t.Errorf("YES should be truthy")
	}
}

func TestReadInt(t *testing.T) {
	in := map[string]any{"a": 7, "b": int64(8), "c": 9.5, "d": "10", "e": "  11 ", "bad": "x", "obj": struct{}{}}
	for k, want := range map[string]int{"a": 7, "b": 8, "c": 9, "d": 10, "e": 11} {
		if got, ok := pluginutil.ReadInt(in, k); !ok || got != want {
			t.Errorf("ReadInt(%q) = (%d, %v), want (%d, true)", k, got, ok, want)
		}
	}
	if _, ok := pluginutil.ReadInt(in, "missing"); ok {
		t.Errorf("missing key should not parse")
	}
	if _, ok := pluginutil.ReadInt(in, "bad"); ok {
		t.Errorf("non-numeric string should not parse")
	}
	if _, ok := pluginutil.ReadInt(in, "obj"); ok {
		t.Errorf("struct should not parse")
	}
}

func TestReadIntDefault(t *testing.T) {
	in := map[string]any{"a": 7}
	if got := pluginutil.ReadIntDefault(in, "a", 99); got != 7 {
		t.Errorf("got %d, want 7", got)
	}
	if got := pluginutil.ReadIntDefault(in, "missing", 99); got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}

func TestReadBool(t *testing.T) {
	in := map[string]any{"a": true, "b": "yes", "c": "no", "d": "garbage"}
	if !pluginutil.ReadBool(in, "a") {
		t.Errorf("a")
	}
	if !pluginutil.ReadBool(in, "b") {
		t.Errorf("b")
	}
	if pluginutil.ReadBool(in, "c") {
		t.Errorf("c")
	}
	if pluginutil.ReadBool(in, "d") {
		t.Errorf("d")
	}
	if pluginutil.ReadBool(in, "missing") {
		t.Errorf("missing")
	}
}

func TestReadBoolDefault(t *testing.T) {
	in := map[string]any{"a": true, "b": "no", "c": "garbage"}
	if got := pluginutil.ReadBoolDefault(in, "a", false); !got {
		t.Errorf("a")
	}
	if got := pluginutil.ReadBoolDefault(in, "b", true); got {
		t.Errorf("b should be false (explicit 'no')")
	}
	if got := pluginutil.ReadBoolDefault(in, "c", true); !got {
		t.Errorf("c should fall back to default when unparsable")
	}
	if got := pluginutil.ReadBoolDefault(in, "missing", true); !got {
		t.Errorf("missing should fall back to default")
	}
}

func TestReadCSV(t *testing.T) {
	in := map[string]any{
		"slice": []string{"a", " b ", ""},
		"any":   []any{"x", 1, 2.5, true},
		"str":   "p, q ,, r",
		"empty": "",
		"bad":   42,
	}
	got := pluginutil.ReadCSV(in, "slice")
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("slice: got %v", got)
	}
	got = pluginutil.ReadCSV(in, "any")
	if !reflect.DeepEqual(got, []string{"x", "1", "2.5", "true"}) {
		t.Errorf("any: got %v", got)
	}
	got = pluginutil.ReadCSV(in, "str")
	if !reflect.DeepEqual(got, []string{"p", "q", "r"}) {
		t.Errorf("str: got %v", got)
	}
	if got := pluginutil.ReadCSV(in, "empty"); len(got) != 0 {
		t.Errorf("empty: got %v", got)
	}
	if got := pluginutil.ReadCSV(in, "missing"); got != nil {
		t.Errorf("missing: got %v", got)
	}
	if got := pluginutil.ReadCSV(in, "bad"); got != nil {
		t.Errorf("bad type: got %v", got)
	}
}

func TestLoginFromInputs(t *testing.T) {
	if got := pluginutil.LoginFromInputs(nil); got != "" {
		t.Errorf("nil map: got %q", got)
	}
	if got := pluginutil.LoginFromInputs(map[string]any{"user": "octocat"}); got != "octocat" {
		t.Errorf("user wins: got %q", got)
	}
	if got := pluginutil.LoginFromInputs(map[string]any{"login": "fallback"}); got != "fallback" {
		t.Errorf("login fallback: got %q", got)
	}
	if got := pluginutil.LoginFromInputs(map[string]any{"user": "", "login": "fallback"}); got != "fallback" {
		t.Errorf("empty user falls through: got %q", got)
	}
}

func TestExtrasEnabled(t *testing.T) {
	if !pluginutil.ExtrasEnabled(nil, "k") {
		t.Errorf("nil map should enable")
	}
	if !pluginutil.ExtrasEnabled(map[string]any{}, "k") {
		t.Errorf("missing key should enable")
	}
	if pluginutil.ExtrasEnabled(map[string]any{"k": "no"}, "k") {
		t.Errorf("explicit no should disable")
	}
	if !pluginutil.ExtrasEnabled(map[string]any{"k": "yes"}, "k") {
		t.Errorf("explicit yes should enable")
	}
}

func TestPlural(t *testing.T) {
	if pluginutil.Plural(1) != "" {
		t.Errorf("Plural(1)")
	}
	if pluginutil.Plural(0) != "s" {
		t.Errorf("Plural(0)")
	}
	if pluginutil.Plural(2) != "s" {
		t.Errorf("Plural(2)")
	}
}

func TestIsZeroSHA(t *testing.T) {
	if !pluginutil.IsZeroSHA("") {
		t.Errorf("empty")
	}
	if !pluginutil.IsZeroSHA(pluginutil.ZeroSHA) {
		t.Errorf("zero const")
	}
	if pluginutil.IsZeroSHA("deadbeef") {
		t.Errorf("real sha should be false")
	}
}

func TestNextPageFromLink(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"next present", `<https://api.github.com/x?per_page=100&page=2>; rel="next", <https://api.github.com/x?per_page=100&page=9>; rel="last"`, 2},
		{"only last", `<https://api.github.com/x?per_page=100&page=9>; rel="last", <https://api.github.com/x?per_page=100&page=1>; rel="first"`, 0},
		{"per_page not mistaken for page", `<https://api.github.com/x?per_page=100>; rel="next"`, 0},
		{"next with fragment", `<https://api.github.com/x?page=3#frag>; rel="next"`, 3},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := pluginutil.NextPageFromLink(tc.in); got != tc.want {
				t.Errorf("NextPageFromLink(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
