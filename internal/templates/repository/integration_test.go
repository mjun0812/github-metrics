package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// TestRun_PartialOrder_MatchesUnderscoreJsonIntersection (T025):
// asserts that for the partials owned by this template package
// (base.header, introduction, base.community, base.activity), the
// rendered SVG contains their data-section markers in the order
// declared by `_.json`.
func TestRun_PartialOrder_MatchesUnderscoreJsonIntersection(t *testing.T) {
	t.Parallel()
	d := plugins.NewData()
	d.Account = plugins.AccountRepository
	d.User = &plugins.User{Login: "octocat", AvatarURL: "https://x"}
	d.Repo = &plugins.Repo{
		Owner:                "octocat",
		OwnerAvatar:          "https://x/a.png",
		Name:                 "hello-world",
		Description:          "Test",
		Stargazers:           1,
		Forks:                1,
		Contributors:         1,
		PrimaryLanguage:      "Go",
		PrimaryLanguageColor: "#00ADD8",
		LicenseName:          "MIT",
		DefaultBranch:        "main",
		Activity:             plugins.RepoActivity{RecentCommits: 1, OpenIssues: 1, OpenPullRequests: 1},
	}
	pc := &templates.PartialContext{
		Data: d,
		// #464: `introduction` is an unadopted plugin slug gated by
		// `plugin_introduction`; enable it so this ordering test can assert
		// header → introduction. A plain base render omits introduction
		// entirely (matching upstream's base-only repository output).
		Inputs: map[string]any{"repo": "hello-world", "plugin_introduction": "yes"},
	}
	out, err := Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Order per upstream `_.json` is base.header → introduction → ...
	// (community / activity content comes from plugin partials registered
	// in the global classic partials registry; this test only asserts the
	// per-template base partials' relative ordering).
	want := []string{
		`data-section="header"`,
		`data-section="introduction"`,
	}
	last := 0
	for _, marker := range want {
		idx := strings.Index(out[last:], marker)
		if idx < 0 {
			t.Errorf("missing or out-of-order marker %q after offset %d\n%s", marker, last, out[max0(last-50):min0(last+200, len(out))])
			return
		}
		last += idx + len(marker)
	}
}

func max0(a int) int {
	if a < 0 {
		return 0
	}
	return a
}

func min0(a, b int) int {
	if a < b {
		return a
	}
	return b
}
