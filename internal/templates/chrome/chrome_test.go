package chrome_test

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
)

func TestTruthyInput(t *testing.T) {
	in := map[string]any{
		"a": true, "b": "true", "c": "yes", "d": "1",
		"e": "false", "f": false, "g": 1,
	}
	for k, want := range map[string]bool{"a": true, "b": true, "c": true, "d": true, "e": false, "f": false, "g": false} {
		if got := chrome.TruthyInput(in, k); got != want {
			t.Errorf("%s: got %v, want %v", k, got, want)
		}
	}
	if chrome.TruthyInput(in, "missing") {
		t.Errorf("missing should be false")
	}
	if chrome.TruthyInput(nil, "k") {
		t.Errorf("nil map should be false")
	}
}

func TestResolveBaseSections(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want []string
	}{
		// Legacy fallbacks (defensive — action layer normally pre-translates).
		{"absent", nil, []string{"header", "activity", "community", "repositories", "metadata"}},
		{"empty-string", map[string]any{"base": ""}, []string{}},
		{"csv", map[string]any{"base": "header, metadata"}, []string{"header", "metadata"}},
		{"slice", map[string]any{"base": []string{"header", "Metadata"}}, []string{"header", "metadata"}},
		{"slice-any", map[string]any{"base": []any{"header", "Metadata"}}, []string{"header", "metadata"}},

		// Canonical chrome_* path (#640).
		{
			name: "chrome-header-only",
			in:   map[string]any{"chrome_header": "yes"},
			want: []string{"header"},
		},
		{
			name: "chrome-multi",
			in:   map[string]any{"chrome_header": true, "chrome_repositories": "yes"},
			want: []string{"header", "repositories"},
		},
		{
			// All present but all false — explicit "no chrome" intent;
			// the legacy default-all fallback MUST NOT fire.
			name: "chrome-present-all-false",
			in:   map[string]any{"chrome_header": false, "chrome_metadata": "no"},
			want: []string{},
		},
		{
			// chrome_* wins when both inputs are present together.
			name: "chrome-wins-over-base",
			in: map[string]any{
				"chrome_header": "yes",
				"base":          "metadata,activity",
			},
			want: []string{"header"},
		},
		{
			name: "introduction-via-chrome",
			in:   map[string]any{"chrome_introduction": "yes"},
			want: []string{"introduction"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chrome.ResolveBaseSections(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("size: got %v, want %v", got, tc.want)
			}
			for _, k := range tc.want {
				if _, ok := got[k]; !ok {
					t.Errorf("missing %q in %v", k, got)
				}
			}
		})
	}
}

func TestAnyChromeInputPresent(t *testing.T) {
	t.Parallel()
	if chrome.AnyChromeInputPresent(nil) {
		t.Errorf("nil map should not be present")
	}
	if chrome.AnyChromeInputPresent(map[string]any{"plugin_header": "yes"}) {
		t.Errorf("non-chrome key should not register")
	}
	if !chrome.AnyChromeInputPresent(map[string]any{"chrome_header": false}) {
		t.Errorf("chrome_header (any value) should register")
	}
	if !chrome.AnyChromeInputPresent(map[string]any{"chrome_introduction": "no"}) {
		t.Errorf("chrome_introduction should register")
	}
}

func TestChromeSectionInputKey(t *testing.T) {
	t.Parallel()
	if got := chrome.ChromeSectionInputKey("header"); got != "chrome_header" {
		t.Errorf("ChromeSectionInputKey(header) = %q, want chrome_header", got)
	}
}

func TestReadBaseInput(t *testing.T) {
	if _, ok := chrome.ReadBaseInput(nil); ok {
		t.Errorf("nil map should not be present")
	}
	if _, ok := chrome.ReadBaseInput(map[string]any{}); ok {
		t.Errorf("missing key should not be present")
	}
	if v, ok := chrome.ReadBaseInput(map[string]any{"base": ""}); !ok || v != "" {
		t.Errorf("empty string: got (%q, %v)", v, ok)
	}
	if v, ok := chrome.ReadBaseInput(map[string]any{"base": []string{"a", "b"}}); !ok || v != "a,b" {
		t.Errorf("slice: got (%q, %v)", v, ok)
	}
}

func TestMetadataFooter(t *testing.T) {
	t.Run("not enabled", func(t *testing.T) {
		got := chrome.MetadataFooter(&templates.PartialContext{Inputs: map[string]any{}}, nil, chrome.FooterOpts{})
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("enabled by section", func(t *testing.T) {
		pc := &templates.PartialContext{
			Inputs: map[string]any{},
			Data:   plugins.NewData(),
		}
		got := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{})
		if !strings.Contains(got, `data-section="metadata"`) {
			t.Errorf("missing section attr: %q", got)
		}
		if !strings.Contains(got, "mjun0812/github-metrics@") {
			t.Errorf("missing version: %q", got)
		}
		if !strings.Contains(got, "<footer>") || !strings.Contains(got, "</footer>") {
			t.Errorf("missing footer tags: %q", got)
		}
	})

	t.Run("private notice (classic)", func(t *testing.T) {
		data := plugins.NewData()
		data.Account = plugins.AccountUser
		pc := &templates.PartialContext{Inputs: map[string]any{}, Data: data}
		got := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{IncludePrivateNotice: true})
		if !strings.Contains(got, "include private contributions") {
			t.Errorf("missing private notice: %q", got)
		}
	})

	t.Run("no private notice (repository)", func(t *testing.T) {
		data := plugins.NewData()
		data.Account = plugins.AccountUser
		pc := &templates.PartialContext{Inputs: map[string]any{}, Data: data}
		got := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{})
		if strings.Contains(got, "include private contributions") {
			t.Errorf("unexpected private notice: %q", got)
		}
	})

	t.Run("legacy base.metadata input", func(t *testing.T) {
		pc := &templates.PartialContext{
			Inputs: map[string]any{"base.metadata": "yes"},
			Data:   plugins.NewData(),
		}
		got := chrome.MetadataFooter(pc, nil, chrome.FooterOpts{})
		if !strings.Contains(got, `data-section="metadata"`) {
			t.Errorf("legacy input should enable: %q", got)
		}
	})
}

func TestContributionRow(t *testing.T) {
	if got := chrome.ContributionRow(nil); got != "" {
		t.Errorf("empty days should render nothing: %q", got)
	}
	days := []plugins.ContributionDay{
		{Color: "#216e39"},
		{Color: ""}, // falls back to empty-cell color
	}
	got := chrome.ContributionRow(days)
	if !strings.Contains(got, "#216e39") {
		t.Errorf("missing day color: %q", got)
	}
	if !strings.Contains(got, "#ebedf0") {
		t.Errorf("missing empty fallback color: %q", got)
	}
	if !strings.Contains(got, `data-block="calendar-grid"`) {
		t.Errorf("missing data-block: %q", got)
	}
}
