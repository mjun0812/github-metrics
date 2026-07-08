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
		// v3.0 canonical surface — opt-in only, no default-all
		// fallback (#649 removed the legacy `base`=CSV translator and
		// the absent-input default-all behaviour).
		{"nil", nil, []string{}},
		{"empty", map[string]any{}, []string{}},

		// Legacy `base` input is silently ignored — v3.0 removed the
		// CSV translator (#649 / #652).
		{"legacy-base-csv-ignored", map[string]any{"base": "header,metadata"}, []string{}},

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
			// Every chrome_* key present but set to false → empty set.
			name: "chrome-present-all-false",
			in:   map[string]any{"chrome_header": false, "chrome_metadata": "no"},
			want: []string{},
		},
		{
			// Legacy `base` alongside chrome_* — base is ignored;
			// only chrome_* contributes.
			name: "chrome-wins-over-legacy-base",
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

func TestMetadataFooter(t *testing.T) {
	t.Run("not enabled", func(t *testing.T) {
		got, h := chrome.MetadataFooter(&templates.PartialContext{Inputs: map[string]any{}}, nil, chrome.FooterOpts{})
		if got != "" || h != 0 {
			t.Errorf("expected empty/0, got %q, %d", got, h)
		}
	})

	t.Run("enabled by section", func(t *testing.T) {
		pc := &templates.PartialContext{
			Inputs: map[string]any{},
			Data:   plugins.NewData(),
		}
		got, h := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{})
		if !strings.Contains(got, `data-section="metadata"`) {
			t.Errorf("missing section attr: %q", got)
		}
		if !strings.Contains(got, "mjun0812/github-metrics@") {
			t.Errorf("missing version: %q", got)
		}
		// #409 Phase C: the footer is native SVG (`<text>`), not an HTML
		// `<footer>` element, and self-reports a positive height.
		if !strings.Contains(got, "<text") {
			t.Errorf("footer should render as native SVG <text>: %q", got)
		}
		if strings.Contains(got, "<footer>") {
			t.Errorf("footer must no longer emit an HTML <footer> element: %q", got)
		}
		if h <= 0 {
			t.Errorf("footer height should be positive, got %d", h)
		}
	})

	t.Run("private notice (classic)", func(t *testing.T) {
		data := plugins.NewData()
		data.Account = plugins.AccountUser
		pc := &templates.PartialContext{Inputs: map[string]any{}, Data: data}
		got, _ := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{IncludePrivateNotice: true})
		if !strings.Contains(got, "include private contributions") {
			t.Errorf("missing private notice: %q", got)
		}
	})

	t.Run("no private notice (repository)", func(t *testing.T) {
		data := plugins.NewData()
		data.Account = plugins.AccountUser
		pc := &templates.PartialContext{Inputs: map[string]any{}, Data: data}
		got, _ := chrome.MetadataFooter(pc, map[string]struct{}{"metadata": {}}, chrome.FooterOpts{})
		if strings.Contains(got, "include private contributions") {
			t.Errorf("unexpected private notice: %q", got)
		}
	})

	t.Run("legacy base.metadata input", func(t *testing.T) {
		pc := &templates.PartialContext{
			Inputs: map[string]any{"base.metadata": "yes"},
			Data:   plugins.NewData(),
		}
		got, _ := chrome.MetadataFooter(pc, nil, chrome.FooterOpts{})
		if !strings.Contains(got, `data-section="metadata"`) {
			t.Errorf("legacy input should enable: %q", got)
		}
	})
}
