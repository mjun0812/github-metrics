package content

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/svgcontent"
)

// contract is one per-plugin semantic requirement set (layer B). It
// pins the *information* a plugin's card must carry, expressed
// independently of any single data value, so it stays valid as the
// underlying GitHub data drifts. Each field is optional; an empty
// slice means "no requirement of this kind".
type contract struct {
	// example is the docs/examples/<file>.svg under test.
	example string
	// issue is the tracking issue this contract pins (for failure
	// messages).
	issue string

	// mustContain: every substring must appear in the visible text
	// (case-insensitive).
	mustContain []string
	// mustMatch: every regexp must match at least minMatch[i] times
	// across the visible text.
	mustMatch []*regexp.Regexp
	minMatch  []int
	// mustNotContain: none of these substrings may appear (catches
	// placeholder / error states leaking into the render).
	mustNotContain []string
	// conditionalMatch: assertions that only apply when a trigger
	// substring is present in the visible text. Used for
	// data-dependent contracts like "PR events must carry diff
	// stats" — vacuously true when the sample window contains no
	// PR events.
	conditionalMatch []conditionalRule
}

// conditionalRule asserts that pattern matches at least minMatch
// times, but only when trigger (case-insensitive substring) is
// present in the visible text. When trigger is absent the rule
// passes vacuously.
type conditionalRule struct {
	trigger  string
	pattern  *regexp.Regexp
	minMatch int
}

// filesChangedRE matches the activity "N files changed" label.
var filesChangedRE = regexp.MustCompile(`(?i)\d+\s+files?\s+changed`)

// contracts enumerates the semantic guarantees per plugin example.
// Keep each entry traceable to its issue so a red test reads as
// "issue #NNN is still unfixed", not an opaque assertion failure.
var contracts = []contract{
	{
		// #465: activity PR events must carry the diff volume
		// (files changed / additions / deletions). The sample
		// window may or may not include a PullRequestEvent (it
		// depends on mjun0812's recent activity), so the check is
		// conditional on "Opened PR" being in the rendered text —
		// otherwise the contract holds vacuously.
		example: "plugin-activity.svg",
		issue:   "#465",
		conditionalMatch: []conditionalRule{
			{trigger: "opened pr", pattern: filesChangedRE, minMatch: 1},
		},
	},
	{
		// #466: each repository card must carry its license, and the
		// upstream layout includes the "created <date>" line.
		example:     "plugin-repositories.svg",
		issue:       "#466",
		mustContain: []string{"license", "created"},
	},
	{
		// #468: the habits card must render the "Language activity"
		// section alongside the indentation / time-of-day facts.
		example:     "plugin-habits.svg",
		issue:       "#468",
		mustContain: []string{"Language activity"},
	},
	{
		// #469: starred repository cards must include license info.
		// The sample's starred set includes MIT-licensed repos
		// (spec-kit, gin), so an MIT label must surface once the
		// license/metadata is wired through.
		example:     "plugin-stars.svg",
		issue:       "#469",
		mustContain: []string{"MIT"},
	},
	{
		// #471: repository-mode contributors must render the real
		// per-contributor commit counts, not the "stats pending"
		// placeholder that appears while /stats/contributors returns
		// 202.
		example:        "plugin-contributors-repo-contributions.svg",
		issue:          "#471",
		mustNotContain: []string{"stats pending"},
	},
}

// TestDOMContracts enforces the per-plugin semantic contracts (B).
// These run with no upstream reference, so they also guard plugins
// (activity, habits) that have no docs/reference_examples fixture.
func TestDOMContracts(t *testing.T) {
	for _, c := range contracts {
		t.Run(c.example, func(t *testing.T) {
			raw := readExample(t, c.example)
			text, err := svgcontent.VisibleText(raw)
			if err != nil {
				t.Fatalf("extract visible text: %v", err)
			}

			for _, sub := range c.mustContain {
				ok, err := svgcontent.Contains(raw, sub)
				if err != nil {
					t.Fatalf("contains check: %v", err)
				}
				if !ok {
					t.Errorf("%s: visible text is missing required marker %q\n  got: %q",
						c.issue, sub, text)
				}
			}

			for i, re := range c.mustMatch {
				want := 1
				if i < len(c.minMatch) {
					want = c.minMatch[i]
				}
				n, err := svgcontent.MatchCount(raw, re)
				if err != nil {
					t.Fatalf("match check: %v", err)
				}
				if n < want {
					t.Errorf("%s: pattern %q matched %d time(s), want >= %d\n  got: %q",
						c.issue, re.String(), n, want, text)
				}
			}

			lower := strings.ToLower(text)
			for _, rule := range c.conditionalMatch {
				if !strings.Contains(lower, strings.ToLower(rule.trigger)) {
					t.Logf("%s: trigger %q absent — skipping conditional pattern %q",
						c.issue, rule.trigger, rule.pattern.String())
					continue
				}
				n, err := svgcontent.MatchCount(raw, rule.pattern)
				if err != nil {
					t.Fatalf("conditional match check: %v", err)
				}
				if n < rule.minMatch {
					t.Errorf("%s: trigger %q present but pattern %q matched %d time(s), want >= %d\n  got: %q",
						c.issue, rule.trigger, rule.pattern.String(), n, rule.minMatch, text)
				}
			}

			for _, sub := range c.mustNotContain {
				ok, err := svgcontent.Contains(raw, sub)
				if err != nil {
					t.Fatalf("not-contains check: %v", err)
				}
				if ok {
					t.Errorf("%s: visible text contains forbidden placeholder %q\n  got: %q",
						c.issue, sub, text)
				}
			}
		})
	}
}
