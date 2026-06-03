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
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
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

// calendar cell geometry mirrors upstream `base.header.ejs`: 11px cells
// laid out at a 15px horizontal pitch.
const (
	calendarCellSize  = 11
	calendarCellPitch = 15
	// emptyCellColor is the GitHub no-contribution color, used as a
	// defensive fallback when the GraphQL `color` field is empty.
	emptyCellColor = "#ebedf0"
)

// BaseHeader renders the upstream `base.header.ejs`-equivalent repo
// chrome: the repository name plus the two-column stats row (Created /
// Deployed / disk-usage on the left, the contribution mini-calendar /
// Environments on the right). Returns "" when data.Repo is nil so the
// dispatch path stays nil-safe. #464.
func BaseHeader(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.Repo == nil {
		return "", nil
	}
	r := pc.Data.Repo
	var b strings.Builder
	b.WriteString(`<section data-section="header" data-template="repository">`)
	b.WriteString(`<h1 class="field">`)
	b.WriteString(octiconPlaceholder)
	fmt.Fprintf(&b, `<span>%s/%s</span>`,
		classicpart.EscapeXML(r.Owner), classicpart.EscapeXML(r.Name))
	b.WriteString(`</h1>`)

	b.WriteString(`<div class="row">`)

	// Left column: Created / Deployed / disk usage.
	b.WriteString(`<section>`)
	if age := format.RelativeAge(r.CreatedAt, nowFunc()); age != "" {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`Created %s</div>`, age)
	}
	fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`Deployed %d time%s</div>`,
		r.Deployments, format.S(int64(r.Deployments), "s"))
	if r.DiskUsageKB > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s used</div>`,
			format.FormatDiskKB(r.DiskUsageKB))
	}
	b.WriteString(`</section>`)

	// Right column: contribution calendar + Environments.
	b.WriteString(`<section>`)
	if row := contributionRow(r.Calendar); row != "" {
		b.WriteString(row)
	}
	fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%d Environment%s</div>`,
		r.Environments, format.S(int64(r.Environments), "s"))
	b.WriteString(`</section>`)

	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// octiconPlaceholder is the 16×16 SVG stub used wherever a real octicon
// path is not substituted. Pinning the intrinsic size keeps the
// `.field` flex row height at ~17px (see the classic partials package
// for the full rationale).
const octiconPlaceholder = `<svg xmlns="http://www.w3.org/2000/svg" class="octicon" width="16" height="16" viewBox="0 0 16 16"/>`

// contributionRow renders the BaseHeader mini contribution calendar as
// a single horizontal row of `class="day"` cells, mirroring upstream
// `base.header.ejs` (last 14 days, oldest → newest). Returns "" when no
// days are present so the block hides for repos with no signal.
func contributionRow(days []plugins.ContributionDay) string {
	if len(days) == 0 {
		return ""
	}
	width := len(days) * calendarCellPitch
	var b strings.Builder
	b.WriteString(`<div class="field calendar" data-block="calendar-grid">`)
	fmt.Fprintf(
		&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="16">`,
		width, calendarCellSize, width,
	)
	b.WriteString(`<g>`)
	for i, d := range days {
		color := emptyCellColor
		if d.Color != "" {
			color = d.Color
		}
		fmt.Fprintf(
			&b,
			`<rect class="day" fill=%q x="%d" y="0" width="%d" height="%d" rx="2" ry="2"/>`,
			color, i*calendarCellPitch, calendarCellSize, calendarCellSize,
		)
	}
	b.WriteString(`</g></svg></div>`)
	return b.String()
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

// TruthyInput reports whether the input keyed by `key` is a truthy
// toggle ("true" / "yes" / "1" / bool true). Mirrors the classic
// dispatcher's gate so the repository template applies identical
// `plugin_<slug>` semantics. #464.
func TruthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "yes" || x == "1"
	default:
		return false
	}
}

// ResolveBaseSections reads the `base` input and returns the set of
// enabled base section names. Mirrors the classic template:
//
//   - input absent → all sections on (default render).
//   - input present but empty string → no base sections (`base=` /
//     per-plugin renders strip the base.header chrome).
//   - input is a CSV → split, trim, lowercase each entry.
//
// #464.
func ResolveBaseSections(in map[string]any) map[string]struct{} {
	const allSections = "header, activity, community, repositories, metadata"
	raw, present := readBaseInput(in)
	if !present {
		raw = allSections
	}
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		s := strings.ToLower(strings.TrimSpace(part))
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}

// readBaseInput extracts the `base` input. Returns (value, true) when
// the key is present even if the value is "" so callers can tell
// "user set base to empty" from "user did not set base".
func readBaseInput(in map[string]any) (string, bool) {
	if in == nil {
		return "", false
	}
	v, ok := in["base"]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case []string:
		return strings.Join(x, ","), true
	case []any:
		parts := make([]string, 0, len(x))
		for _, p := range x {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ","), true
	}
	return "", false
}

// PartialEnabledByBase reports whether the named repository-owned
// partial should render given the resolved base sections. Only
// `base.header` is gated here (mapped to the "header" section); every
// other `_.json` entry is a plugin partial that passes through (its
// `plugin_<slug>` toggle is applied separately by the caller). #464.
func PartialEnabledByBase(name string, sections map[string]struct{}) bool {
	if name == "base.header" {
		_, ok := sections["header"]
		return ok
	}
	return true
}
