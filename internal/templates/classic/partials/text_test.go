package partials_test

import (
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func TestEscapeXML_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "octocat", "octocat"},
		{"lt", "1 < 2", "1 &lt; 2"},
		{"gt", "3 > 2", "3 &gt; 2"},
		{"amp", "a & b", "a &amp; b"},
		{"dquote", `say "hi"`, `say &#34;hi&#34;`},
		{"squote", "it's", "it&#39;s"},
		{
			"all", `<a href="x" title='y'>&amp;</a>`,
			`&lt;a href=&#34;x&#34; title=&#39;y&#39;&gt;&amp;amp;&lt;/a&gt;`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := partials.EscapeXML(tc.in)
			if got != tc.want {
				t.Fatalf("EscapeXML(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEscapeXML_DoubleEscapesAmpersand(t *testing.T) {
	t.Parallel()
	once := partials.EscapeXML("&amp;")
	if !strings.Contains(once, "&amp;amp;") {
		t.Fatalf("EscapeXML re-escapes; callers must not pre-escape; got %q", once)
	}
}

func TestFormatCount_Tiers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1_000, "1k"},
		{1_500, "1.5k"},
		{250, "250"},
		{1_000_000, "1m"},
	}
	for _, tc := range cases {
		got := partials.FormatCount(tc.in)
		if got != tc.want {
			t.Fatalf("FormatCount(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
