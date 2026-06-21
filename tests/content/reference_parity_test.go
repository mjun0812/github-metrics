package content

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/testutil/svgcontent"
)

// exampleToReference maps each docs/examples/ artifact to the
// upstream-generated docs/reference_examples/ fixture rendered from
// the SAME account/repo. The reference side is real upstream output
// (see the repo's reference_examples README), making it the only
// non-circular ground truth available to the suite. Entries whose
// reference file is absent are skipped at runtime.
var exampleToReference = map[string]string{
	"metrics-classic.svg":                        "metrics.classic.svg",
	"metrics-repository.svg":                     "metrics.repository.svg",
	"plugin-header.svg":                          "metrics.base.svg",
	"plugin-calendar.svg":                        "metrics.plugin.calendar.svg",
	"plugin-isocalendar.svg":                     "metrics.plugin.isocalendar.svg",
	"plugin-isocalendar-fullyear.svg":            "metrics.plugin.isocalendar.fullyear.svg",
	"plugin-languages.svg":                       "metrics.plugin.languages.svg",
	"plugin-languages-details.svg":               "metrics.plugin.languages.details.svg",
	"plugin-languages-indepth.svg":               "metrics.plugin.languages.indepth.svg",
	"plugin-notable.svg":                         "metrics.plugin.notable.svg",
	"plugin-notable-indepth.svg":                 "metrics.plugin.notable.indepth.svg",
	"plugin-people.svg":                          "metrics.plugin.people.svg",
	"plugin-reactions.svg":                       "metrics.plugin.reactions.svg",
	"plugin-repositories.svg":                    "metrics.plugin.repositories.svg",
	"plugin-sponsors.svg":                        "metrics.plugin.sponsors.svg",
	"plugin-sponsorships.svg":                    "metrics.plugin.sponsorships.svg",
	"plugin-stargazers.svg":                      "metrics.plugin.stargazers.svg",
	"plugin-stargazers-graph.svg":                "metrics.plugin.stargazers.graph.svg",
	"plugin-starlists.svg":                       "metrics.plugin.starlists.svg",
	"plugin-stars.svg":                           "metrics.plugin.stars.svg",
	"plugin-topics.svg":                          "metrics.plugin.topics.svg",
	"plugin-traffic.svg":                         "metrics.plugin.traffic.svg",
	"plugin-contributors-repo-contributions.svg": "metrics.repository.plugin.contributors.svg",
}

func referenceExists(t *testing.T, refName string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(repoRoot(t), "docs", "reference_examples", refName))
	return err == nil
}

var alphaTokenRE = regexp.MustCompile(`^[\p{L}][\p{L}-]{2,}$`)

// TestReferenceTokenCoverage is a discovery aid, not a gate: for each
// example/reference pair it logs the alphabetic labels present in the
// upstream output but missing from ours. It never fails (real data
// drift means the sets are never identical), but the log makes
// content regressions easy to spot during triage — e.g. it surfaces
// "license" and "created" missing from plugin-repositories.svg (#466)
// and "language"/"activity" missing from plugin-habits-related cards.
//
// Run with: go test ./tests/content/ -run TestReferenceTokenCoverage -v
func TestReferenceTokenCoverage(t *testing.T) {
	names := make([]string, 0, len(exampleToReference))
	for ex := range exampleToReference {
		names = append(names, ex)
	}
	sort.Strings(names)

	for _, ex := range names {
		ref := exampleToReference[ex]
		t.Run(ex, func(t *testing.T) {
			if !referenceExists(t, ref) {
				t.Skipf("reference %s absent", ref)
			}
			exTokens := tokenSet(t, readExample(t, ex))
			refTokens := tokenSet(t, readReference(t, ref))

			var missing []string
			for tok := range refTokens {
				if !alphaTokenRE.MatchString(tok) {
					continue // skip numbers / punctuation-only / very short
				}
				if _, ok := exTokens[tok]; !ok {
					missing = append(missing, tok)
				}
			}
			sort.Strings(missing)
			if len(missing) == 0 {
				t.Logf("ok: every alphabetic reference label is present in our output")
				return
			}
			t.Logf("labels in upstream %s but missing from %s (%d):\n  %s",
				ref, ex, len(missing), strings.Join(missing, " "))
		})
	}
}

func tokenSet(t *testing.T, raw []byte) map[string]struct{} {
	t.Helper()
	toks, err := svgcontent.Tokens(raw)
	if err != nil {
		t.Fatalf("tokens: %v", err)
	}
	set := make(map[string]struct{}, len(toks))
	for _, tok := range toks {
		set[tok] = struct{}{}
	}
	return set
}
