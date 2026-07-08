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
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	classicpart "github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// nowFunc is the time source used by BaseHeader's "Created <N> ago"
// relative-age field. Tests swap it via SetNow to keep the rendered
// age deterministic.
var nowFunc = time.Now

// SetNow overrides the BaseHeader clock for tests and returns a restore
// function. Not safe for concurrent use across parallel tests sharing
// the clock.
func SetNow(now func() time.Time) func() {
	prev := nowFunc
	nowFunc = now
	return func() { nowFunc = prev }
}

// Native-SVG repository header geometry. Mirrors the classic user
// header (internal/plugins/header/render.go) but the title band carries
// the :octicon-repo: mark + owner/name instead of an avatar + display
// name. The values follow the flex/CSS metrics the HTML `.field` / h1
// rows used so the card lines up with the still-HTML partials it
// coexists with (#409 Phase B).
const (
	repoHdrTitleTop    = 8                                                                           // h1 margin-top
	repoHdrIconSize    = 16                                                                          // .octicon width/height
	repoHdrTitleBand   = 24                                                                          // title content band (16px icon / 20px name)
	repoHdrTitleBottom = repoHdrTitleTop + repoHdrTitleBand + 2                                      // + h1 margin-bottom
	repoHdrFieldInset  = 5                                                                           // `section > .field { margin-left: 5px }`
	repoHdrIconMargin  = 8                                                                           // `.field svg { margin: 0 8px }`
	repoHdrNameX       = repoHdrFieldInset + repoHdrIconMargin + repoHdrIconSize + repoHdrIconMargin // 37; aligns the name past the octicon
	repoHdrNameFont    = 20                                                                          // h1 font-size
	repoHdrNameFill    = "#0366d6"
	// repoHdrIconFill is the `.field svg` grey fill (`#959da5`); it wins
	// over `h1 svg { fill: currentColor }` on specificity, so the title
	// octicon renders grey like every other field icon.
	repoHdrIconFill = "#959da5"
)

// BaseHeader renders the upstream `base.header.ejs`-equivalent repo
// chrome as native SVG: the repository name plus the two-column stats
// row (Created / Deployed / disk-usage on the left, the contribution
// mini-calendar / Environments on the right). Returns "" when data.Repo
// is nil so the dispatch path stays nil-safe. Mirrors the classic user
// header's native-SVG layout (#409 Phase C prerequisite; #464).
func BaseHeader(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", 0, nil
	}
	r := pc.Data.Repo

	// Left column: Created / Deployed / disk usage.
	left := chrome.NewSVGColumn(0, chrome.CardWidth/2, repoHdrTitleBottom)
	if age := format.RelativeAge(r.CreatedAt, nowFunc()); age != "" {
		left.Field(":octicon-clock:", "Created "+age)
	}
	left.Field(":octicon-rocket:", fmt.Sprintf("Deployed %d time%s",
		r.Deployments, format.S(int64(r.Deployments), "s")))
	if r.DiskUsageKB > 0 {
		left.Field(":octicon-database:", format.FormatDiskKB(r.DiskUsageKB)+" used")
	}

	// Right column: contribution mini calendar + Environments.
	right := chrome.NewSVGColumn(chrome.CardWidth/2, chrome.CardWidth/2, repoHdrTitleBottom)
	right.Calendar(r.Calendar)
	right.Field(":octicon-server:", fmt.Sprintf("%d Environment%s",
		r.Environments, format.S(int64(r.Environments), "s")))

	countersHeight := left.Height()
	if right.Height() > countersHeight {
		countersHeight = right.Height()
	}
	height := repoHdrTitleBottom + int(countersHeight)

	// Title band: grey :octicon-repo: mark + owner/name (20px bold blue).
	iconX := float64(repoHdrFieldInset + repoHdrIconMargin)
	iconY := float64(repoHdrTitleTop + (repoHdrTitleBand-repoHdrIconSize)/2)
	nameBaseline := float64(repoHdrTitleTop) + repoHdrTitleBand/2 + repoHdrNameFont*0.32

	var body strings.Builder
	body.WriteString(chrome.SVGIcon(iconX, iconY, repoHdrIconFill, ":octicon-repo:"))
	body.WriteString(chrome.SVGText(repoHdrNameX, nameBaseline, r.Owner+"/"+r.Name, chrome.SVGTextOpts{
		Size:     repoHdrNameFont,
		Weight:   fontmetrics.Bold,
		Fill:     repoHdrNameFill,
		MaxWidth: chrome.CardWidth - repoHdrNameX - repoHdrFieldInset,
	}))
	body.WriteString(`<g data-block="header-counters">`)
	body.WriteString(left.Markup())
	body.WriteString(right.Markup())
	body.WriteString(`</g>`)

	return wrapRepoHeaderSVG(body.String(), height), height, nil
}

// wrapRepoHeaderSVG wraps the repository header body in a
// `<g data-section="header" data-template="repository">` anchor (kept
// for the section gate / DOM diffing) plus a fixed-height nested `<svg>`.
// Pure SVG since #409 Phase C dropped the outer foreignObject; mirrors
// the classic header's wrapHeaderSVG but carries the repository
// `data-template` hook.
func wrapRepoHeaderSVG(body string, height int) string {
	return fmt.Sprintf(
		`<g data-section="header" data-template="repository"><svg xmlns="http://www.w3.org/2000/svg" width="100%%" height="%d" viewBox="0 0 %d %d">%s</svg></g>`,
		height, chrome.CardWidth, height, body,
	)
}

// Native-SVG introduction-badges geometry. Mirrors `.repo-badges .badge`
// (14px inherited body text on a row, 14px inter-badge margin) with the
// language badge carrying a 10px `--lang-color` dot (#409 Phase C: the
// dot color is inlined as a literal fill since resvg does not resolve CSS
// variables).
const (
	introInset    = 5.0               // left inset (matches other rows)
	introTop      = 2.0               // small top margin above the badge row
	introRowH     = chrome.FieldPitch // one field-row worth of height
	introFont     = 14.0              // inherited body font-size
	introFill     = "#777777"         // inherited body color
	introBadgeGap = 14.0              // `.badge { margin-right: 14px }`
	introDotR     = 5.0               // 10px `::before` dot radius
	introDotGap   = 5.0               // dot `margin-right: 5px`
	introDotFill  = "#959da5"         // `--lang-color` fallback
)

// Introduction surfaces the repo's primary language / license / default
// branch badges as a native-SVG row (#409 Phase C). Returns "" / 0 when
// data.Repo is nil or no badge field is set.
func Introduction(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", 0, nil
	}
	r := pc.Data.Repo
	if r.PrimaryLanguage == "" && r.LicenseName == "" && r.DefaultBranch == "" {
		return "", 0, nil
	}

	rowCenter := introTop + introRowH/2
	baseline := rowCenter + introFont*0.32

	var b strings.Builder
	cx := introInset
	if r.PrimaryLanguage != "" {
		dotFill := introDotFill
		if r.PrimaryLanguageColor != "" {
			dotFill = r.PrimaryLanguageColor
		}
		fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="%d" fill=%q/>`,
			int(cx+introDotR), int(rowCenter), int(introDotR), dotFill)
		cx += 2*introDotR + introDotGap
		b.WriteString(chrome.SVGText(cx, baseline, r.PrimaryLanguage,
			chrome.SVGTextOpts{Size: introFont, Fill: introFill}))
		cx += fontmetrics.Width(r.PrimaryLanguage, introFont) + introBadgeGap
	}
	if r.LicenseName != "" {
		b.WriteString(chrome.SVGText(cx, baseline, r.LicenseName,
			chrome.SVGTextOpts{Size: introFont, Fill: introFill}))
		cx += fontmetrics.Width(r.LicenseName, introFont) + introBadgeGap
	}
	if r.DefaultBranch != "" {
		b.WriteString(chrome.SVGText(cx, baseline, r.DefaultBranch,
			chrome.SVGTextOpts{Size: introFont, Fill: introFill}))
	}

	height := int(introTop + introRowH)
	return chrome.WrapSection("introduction", height, b.String()), height, nil
}

// BaseCommunity renders contributors / stargazers / forks counts.
// Returns "" when data.Repo is nil OR all counts are zero (so empty
// repos do not render a stray empty section).
func BaseCommunity(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", 0, nil
	}
	r := pc.Data.Repo
	if r.Stargazers == 0 && r.Forks == 0 && r.Contributors == 0 {
		return "", 0, nil
	}
	var b strings.Builder
	b.WriteString(`<g data-section="community">`)
	b.WriteString(`<div class="row community-stats">`)
	fmt.Fprintf(&b, `<span class="stat stargazers">%s stars</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Stargazers))))
	fmt.Fprintf(&b, `<span class="stat forks">%s forks</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Forks))))
	fmt.Fprintf(&b, `<span class="stat contributors">%s contributors</span>`,
		classicpart.FormatCount(int64(maxNonNegative(r.Contributors))))
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), 0, nil
}

// BaseActivity renders the recent commits / open issues / open PRs
// triple. Returns "" when data.Repo is nil OR the repo is archived
// AND has no activity to show.
func BaseActivity(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", 0, nil
	}
	a := pc.Data.Repo.Activity
	if a.RecentCommits == 0 && a.OpenIssues == 0 && a.OpenPullRequests == 0 {
		return "", 0, nil
	}
	var b strings.Builder
	b.WriteString(`<g data-section="activity">`)
	b.WriteString(`<div class="row activity-stats">`)
	fmt.Fprintf(&b, `<span class="stat commits">%s commits (30d)</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.RecentCommits))))
	fmt.Fprintf(&b, `<span class="stat issues">%s open issues</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.OpenIssues))))
	fmt.Fprintf(&b, `<span class="stat prs">%s open PRs</span>`,
		classicpart.FormatCount(int64(maxNonNegative(a.OpenPullRequests))))
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), 0, nil
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
