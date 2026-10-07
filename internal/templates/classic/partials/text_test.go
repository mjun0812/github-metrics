package partials_test

import (
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
