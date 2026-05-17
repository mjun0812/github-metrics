// Package partials hosts the repository-template-specific partials
// the M7 layout needs. They are NOT registered in the global classic
// partials registry — instead the parent repository package consumes
// them through the per-template lookup table in repository.go.
//
// All four functions follow the M2 partial contract (nil-safe,
// XML-escaped, single-line SVG fragment string).
package partials

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	classicpart "github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// BaseHeader renders the repo identity row: owner avatar + owner /
// name + description. Returns "" when data.Repo is nil so the
// templates' dispatch path stays nil-safe.
func BaseHeader(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", nil
	}
	r := pc.Data.Repo
	var b strings.Builder
	b.WriteString(`<section data-section="header" data-template="repository">`)
	b.WriteString(`<h1 class="field">`)
	if r.OwnerAvatar != "" {
		fmt.Fprintf(&b, `<img class="avatar" src=%q width="20" height="20" />`,
			classicpart.EscapeXML(r.OwnerAvatar))
	}
	fmt.Fprintf(&b, `<span>%s/%s</span>`,
		classicpart.EscapeXML(r.Owner), classicpart.EscapeXML(r.Name))
	b.WriteString(`</h1>`)
	if r.Description != "" {
		fmt.Fprintf(&b, `<p class="repo-description">%s</p>`,
			classicpart.EscapeXML(r.Description))
	}
	b.WriteString(`</section>`)
	return b.String(), nil
}

// Introduction surfaces the repo's about text + primary language /
// license badges. Returns "" when data.Repo is nil.
func Introduction(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", nil
	}
	r := pc.Data.Repo
	if r.PrimaryLanguage == "" && r.LicenseName == "" && r.DefaultBranch == "" {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="introduction">`)
	b.WriteString(`<div class="row repo-badges">`)
	if r.PrimaryLanguage != "" {
		colorAttr := ""
		if r.PrimaryLanguageColor != "" {
			colorAttr = fmt.Sprintf(` style="--lang-color:%s"`,
				classicpart.EscapeXML(r.PrimaryLanguageColor))
		}
		fmt.Fprintf(&b, `<span class="badge language"%s>%s</span>`,
			colorAttr, classicpart.EscapeXML(r.PrimaryLanguage))
	}
	if r.LicenseName != "" {
		fmt.Fprintf(&b, `<span class="badge license">%s</span>`,
			classicpart.EscapeXML(r.LicenseName))
	}
	if r.DefaultBranch != "" {
		fmt.Fprintf(&b, `<span class="badge branch">%s</span>`,
			classicpart.EscapeXML(r.DefaultBranch))
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// BaseCommunity renders contributors / stargazers / forks counts.
// Returns "" when data.Repo is nil OR all counts are zero (so empty
// repos do not render a stray empty section).
func BaseCommunity(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", nil
	}
	r := pc.Data.Repo
	if r.Stargazers == 0 && r.Forks == 0 && r.Contributors == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="community">`)
	b.WriteString(`<div class="row community-stats">`)
	fmt.Fprintf(&b, `<span class="stat stargazers">%s stars</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Stargazers))))
	fmt.Fprintf(&b, `<span class="stat forks">%s forks</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Forks))))
	fmt.Fprintf(&b, `<span class="stat contributors">%s contributors</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Contributors))))
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// BaseActivity renders the recent commits / open issues / open PRs
// triple. Returns "" when data.Repo is nil OR the repo is archived
// AND has no activity to show.
func BaseActivity(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", nil
	}
	a := pc.Data.Repo.Activity
	if a.RecentCommits == 0 && a.OpenIssues == 0 && a.OpenPullRequests == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="activity">`)
	b.WriteString(`<div class="row activity-stats">`)
	fmt.Fprintf(&b, `<span class="stat commits">%s commits (30d)</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.RecentCommits))))
	fmt.Fprintf(&b, `<span class="stat issues">%s open issues</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.OpenIssues))))
	fmt.Fprintf(&b, `<span class="stat prs">%s open PRs</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.OpenPullRequests))))
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// Lookup returns the repository-template partial for the given name,
// or nil when none is owned by this package. Plugin partial names
// (`languages`, `activity`, etc.) are intentionally absent — the
// repository template's Run resolves those through the shared classic
// partials registry where the per-plugin packages already register
// them by name.
func Lookup(name string) (templates.PartialFunc, bool) {
	switch name {
	case "base.header":
		return BaseHeader, true
	case "introduction":
		return Introduction, true
	case "base.community":
		return BaseCommunity, true
	case "base.activity":
		return BaseActivity, true
	}
	return nil, false
}

func maxNonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
