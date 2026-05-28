// Package partials hosts the partial functions assembled by the classic
// template. Each partial is a templates.PartialFunc that takes a
// PartialContext and returns the SVG fragment it owns.
//
// Partials follow three rules:
//
//  1. Empty when nil-safe: missing data (e.g. Data.User == nil) must
//     yield "" without panicking.
//  2. XML safety: dynamic strings flow through classic.EscapeXML.
//  3. Magnitude shortening: integer counts flow through
//     classic.FormatCount.
//
// The classic template owns the call order via partials/_.json.
package partials

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
)

// nowFunc is the time source used by BaseHeader's "Joined GitHub <N>
// years ago" label. It defaults to time.Now; tests overwrite it via
// SetNowForTest to anchor the rendered string. Not goroutine-safe;
// production code never reassigns it.
var nowFunc = time.Now

// SetNowForTest overrides the time source used by BaseHeader. The
// returned function restores the previous value.
func SetNowForTest(now func() time.Time) func() {
	prev := nowFunc
	nowFunc = now
	return func() { nowFunc = prev }
}

// octiconPlaceholder is the empty SVG element used wherever a real
// octicon path has not yet been substituted. Pinning width/height/viewBox
// to 16×16 prevents Chrome from falling back to the default SVG viewport
// (300×150 px). Without this, every `.field { display: flex; align-items:
// center }` row would inflate to ~150 px and the base render would be
// ~5.6× taller than upstream. CSS overrides cannot fix this because SVG
// intrinsic size is governed by element attributes, not CSS properties
// (legacy SVG coordinate-system spec). See also: .octicon rule in
// style.css (belt-and-suspenders fallback for very old viewers).
const octiconPlaceholder = `<svg xmlns="http://www.w3.org/2000/svg" class="octicon" width="16" height="16" viewBox="0 0 16 16"/>`

// BaseHeader renders the avatar + display name block at the top of the
// classic SVG. Returns "" when the User payload is absent.
//
// 429 Phase 1: also renders the upstream base.header.ejs sub-row with
// "Joined GitHub <age>", "Followed by N users", and "Following N
// users" when those fields are populated. Each row is rendered with an
// octiconPlaceholder — a 16×16 SVG stub whose intrinsic size matches
// @primer/octicons so the `.field` row height stays at ~17 px.
//
// Earlier behaviour (#419): with no real data the partial emitted an
// empty `<div class="row"><section/><section/></div>` placeholder
// which caused a tall blank band. With the Phase 1 fields populated
// the row carries real content, so the placeholder regression is
// no longer relevant.
func BaseHeader(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.User == nil {
		return "", nil
	}
	u := pc.Data.User
	if u.Login == "" && u.Name == "" {
		return "", nil
	}
	display := u.Name
	if display == "" {
		display = u.Login
	}

	var b strings.Builder
	b.WriteString(`<section data-section="header">`)
	b.WriteString(`<h1 class="field">`)
	if u.AvatarURL != "" {
		fmt.Fprintf(&b, `<img class="avatar" src=%q width="20" height="20" />`,
			EscapeXML(u.AvatarURL))
	}
	fmt.Fprintf(&b, `<span>%s</span>`, EscapeXML(display))
	b.WriteString(`</h1>`)

	// Upstream sub-row: Joined GitHub / Followed by / Following.
	// Each field is suppressed when its source data is the zero value,
	// so empty / freshly-created accounts do not gain blank rows.
	var subRows []string
	if age := format.RelativeAge(u.CreatedAt, nowFunc()); age != "" {
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field">`+octiconPlaceholder+`Joined GitHub %s</div>`, age,
		))
	}
	if u.Followers > 0 {
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field">`+octiconPlaceholder+`Followed by %s %s</div>`,
			FormatCount(int64(u.Followers)), pluralLabel("user", u.Followers),
		))
	}
	if u.Following > 0 {
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field">`+octiconPlaceholder+`Following %s %s</div>`,
			FormatCount(int64(u.Following)), pluralLabel("user", u.Following),
		))
	}
	// 429 Phase 2: "Contributed to N repositories" sourced from
	// user.repositoriesContributedTo.totalCount. Hidden when zero so
	// fresh accounts (no external contributions) do not gain a noise
	// row.
	if u.ContributedTo > 0 {
		noun := "repositories"
		if u.ContributedTo == 1 {
			noun = "repository"
		}
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field">`+octiconPlaceholder+`Contributed to %s %s</div>`,
			FormatCount(int64(u.ContributedTo)), noun,
		))
	}
	if len(subRows) > 0 {
		b.WriteString(`<div class="row" data-block="header-counters">`)
		for _, row := range subRows {
			b.WriteString(row)
		}
		b.WriteString(`</div>`)
	}

	// 429 Phase 3: contribution mini grid embedded in BaseHeader. Renders
	// the trailing 11 weeks of GitHub's contribution calendar as a 11x7
	// SVG inside `<div class="field calendar">`. Hidden when the data
	// payload is empty (fresh account or a GraphQL failure) so the
	// section stays clean for accounts with no signal.
	if grid := contributionGrid(u.RecentContributions); grid != "" {
		b.WriteString(grid)
	}

	b.WriteString(`</section>`)
	return b.String(), nil
}

// Contribution mini-grid geometry. Mirrors upstream's 11x11 cell size
// with a 2px gap, giving a 13px column / row pitch. The SVG is sized
// just large enough to bound the 11x7 cell array; the partial omits
// itself entirely when the underlying data is empty so the SVG never
// reserves blank space.
const (
	calendarCellSize  = 11
	calendarCellGap   = 2
	calendarCellPitch = calendarCellSize + calendarCellGap
	calendarColumns   = 11
	calendarRows      = 7
	calendarWidth     = (calendarColumns-1)*calendarCellPitch + calendarCellSize
	calendarHeight    = (calendarRows-1)*calendarCellPitch + calendarCellSize
)

// emptyCellColor is the canonical GitHub no-contribution color used for
// padding (e.g. when the trailing week of a fresh account has < 7 days)
// and as a defensive fallback when the GraphQL `color` field is empty.
const emptyCellColor = "#ebedf0"

// contributionGrid renders the BaseHeader mini contribution grid as a
// fragment of SVG embedded in an HTML container so the existing
// `.calendar.field` CSS rule (margin-left/top tweak) applies.
//
// Each cell is tagged `calendar-graph-day-<level>` so themed CSS
// overrides (`--color-calendar-graph-day-Ln-bg`) still work, and the
// `fill` attribute carries the GitHub-supplied hex so plain renderers
// (no CSS) draw the correct color too. Returns "" when no weeks are
// present.
func contributionGrid(weeks []plugins.ContributionWeek) string {
	if len(weeks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="field calendar" data-block="calendar-grid">`)
	fmt.Fprintf(
		&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`,
		calendarWidth, calendarHeight, calendarWidth, calendarHeight,
	)
	b.WriteString(`<g>`)
	for col, week := range weeks {
		if col >= calendarColumns {
			break
		}
		for row := 0; row < calendarRows; row++ {
			x := col * calendarCellPitch
			y := row * calendarCellPitch
			color := emptyCellColor
			level := 0
			if row < len(week.Days) {
				d := week.Days[row]
				if d.Color != "" {
					color = d.Color
				}
				level = format.ContributionLevel(d.ContributionCount, d.Color)
			}
			fmt.Fprintf(
				&b,
				`<rect class="calendar-graph-day-%d" fill=%q x="%d" y="%d" width="%d" height="%d" rx="2" ry="2"/>`,
				level, color, x, y, calendarCellSize, calendarCellSize,
			)
		}
	}
	b.WriteString(`</g></svg></div>`)
	return b.String()
}

// Introduction is a stub: the introduction plugin lands in M4. Until
// then the partial returns "" so the partial dispatch order stays
// stable.
func Introduction(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	if _, ok := pc.Data.GetPlugin("introduction"); !ok {
		return "", nil
	}
	// Plugin will populate this in M4; for now we keep the structure
	// addressable but empty so the DOM does not gain a stray section.
	return "", nil
}

// BaseActivityCommunity renders the activity + community two-column
// block. The activity column shows aggregate counters (commits, issues,
// pull requests) sourced from Data.Computed; the community column is
// kept as a reserved placeholder for the user-relationship counters
// (organizations, sponsoring, watching) that the upstream
// base.activity+community.ejs partial renders — those Go-side data
// fields (followers, sponsorshipsAsSponsor, watching, ...) are not yet
// populated by the base plugin.
//
// Per #419 the partial now returns "" (no wrapper at all) when no
// activity counter has a value, so a base-only render does not emit
// an empty `<section data-section="activity-community">` with nothing
// inside. The previous behaviour produced a tall empty band that made
// the resulting SVG look broken (huge whitespace) and inflated the
// rendered size because the section reserved space the CSS treated
// as a real row.
func BaseActivityCommunity(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	c := pc.Data.Computed
	hasActivity := c.TotalCommits > 0 || c.TotalIssues > 0 || c.TotalPullRequests > 0
	if !hasActivity {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="activity-community">`)
	b.WriteString(`<section class="row">`)
	b.WriteString(`<section data-block="activity">`)
	b.WriteString(`<h2 class="field">` + octiconPlaceholder + `Activity</h2>`)
	if c.TotalCommits > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s</div>`,
			FormatCount(int64(c.TotalCommits)), pluralLabel("Commit", c.TotalCommits))
	}
	if c.TotalPullRequests > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s opened</div>`,
			FormatCount(int64(c.TotalPullRequests)), pluralLabel("Pull request", c.TotalPullRequests))
	}
	if c.TotalIssues > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s opened</div>`,
			FormatCount(int64(c.TotalIssues)), pluralLabel("Issue", c.TotalIssues))
	}
	b.WriteString(`</section>`)
	b.WriteString(`<section data-block="community"></section>`)
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// pluralLabel adds a trailing "s" to the singular when count != 1.
// Mirrors upstream `s(count)` template helper used by
// base.activity+community.ejs.
func pluralLabel(singular string, count int) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}

// BaseRepositories renders the count / stargazers / forks row. Returns
// "" when Repositories.Count is zero so the section disappears for
// fresh accounts.
//
// 429 Phase 1: also surfaces "N watching" and "N sponsors" sourced from
// Data.User.{Watching,SponsorshipsAsMaintainer}. Both labels are
// suppressed when their counter is zero so accounts with no
// watch/sponsorship activity do not gain noise rows.
//
// 429 Phase 2: surfaces "N releases", "N packages", "<disk> used", and
// the License-preference top-3 row sourced from
// Data.Computed.Repositories.{Releases,Packages,DiskUsage,
// LicensePreference}. Each label is hidden when its counter is zero
// (or the license slice is empty), so the partial stays dense on
// accounts that lack the corresponding signal.
func BaseRepositories(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	r := pc.Data.Computed.Repositories
	if r.Count == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="repositories">`)
	b.WriteString(`<div class="row">`)
	fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s repositories</div>`,
		FormatCount(int64(r.Count)))
	fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s stargazers</div>`,
		FormatCount(int64(r.Stargazers)))
	fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s forks</div>`,
		FormatCount(int64(r.Forks)))
	if r.Releases > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s</div>`,
			FormatCount(int64(r.Releases)), pluralLabel("release", r.Releases))
	}
	if r.Packages > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s</div>`,
			FormatCount(int64(r.Packages)), pluralLabel("package", r.Packages))
	}
	if r.DiskUsage > 0 {
		fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s used</div>`,
			format.FormatDiskKB(r.DiskUsage))
	}
	if u := pc.Data.User; u != nil {
		if u.Watching > 0 {
			noun := "repositories"
			if u.Watching == 1 {
				noun = "repository"
			}
			fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`Watching %s %s</div>`,
				FormatCount(int64(u.Watching)), noun)
		}
		if u.SponsorshipsAsMaintainer > 0 {
			fmt.Fprintf(&b, `<div class="field">`+octiconPlaceholder+`%s %s</div>`,
				FormatCount(int64(u.SponsorshipsAsMaintainer)),
				pluralLabel("sponsor", u.SponsorshipsAsMaintainer))
		}
	}
	if row := licensePreferenceRow(r.LicensePreference); row != "" {
		b.WriteString(row)
	}
	b.WriteString(`</div></section>`)
	return b.String(), nil
}

// licensePreferenceTopRender caps the License-preference labels shown
// inside the partial at the top three entries. The data model carries
// up to 5 (see internal/plugins/base.licensePreferenceTopN) so future
// renderers (e.g. a richer dashboard view) can opt into a longer
// breakdown without re-aggregating. The classic field uses a compact
// notation tuned for the 480px upstream-parity canvas width, so even
// three entries fit on one line for typical license names.
const licensePreferenceTopRender = 3

// licenseShortName collapses GitHub's verbose `licenseInfo.name`
// labels into the SPDX-flavoured short form upstream renders for
// chips/badges. Falls back to the original string when no rule applies
// so unknown licenses still surface (just longer).
//
// Examples (input -> output):
//   - "MIT License"                                 -> "MIT"
//   - "Apache License 2.0"                          -> "Apache-2.0"
//   - "BSD 3-Clause \"New\" or \"Revised\" License" -> "BSD-3-Clause"
//   - "GNU General Public License v3.0"             -> "GPL-3.0"
//   - "GNU Lesser General Public License v2.1"      -> "LGPL-2.1"
//   - "Mozilla Public License 2.0"                  -> "MPL-2.0"
//   - "The Unlicense"                               -> "Unlicense"
func licenseShortName(name string) string {
	switch {
	case name == "MIT License":
		return "MIT"
	case name == "ISC License":
		return "ISC"
	case name == "The Unlicense":
		return "Unlicense"
	case name == "Do What The F*ck You Want To Public License":
		return "WTFPL"
	case name == "Creative Commons Zero v1.0 Universal":
		return "CC0-1.0"
	case strings.HasPrefix(name, "Apache License"):
		return "Apache-" + strings.TrimSpace(strings.TrimPrefix(name, "Apache License"))
	case strings.HasPrefix(name, "BSD 2-Clause"):
		return "BSD-2-Clause"
	case strings.HasPrefix(name, "BSD 3-Clause"):
		return "BSD-3-Clause"
	case strings.HasPrefix(name, "BSD 4-Clause"):
		return "BSD-4-Clause"
	case strings.HasPrefix(name, "GNU Affero General Public License"):
		rest := strings.TrimSpace(strings.TrimPrefix(name, "GNU Affero General Public License"))
		return "AGPL-" + strings.TrimPrefix(rest, "v")
	case strings.HasPrefix(name, "GNU Lesser General Public License"):
		rest := strings.TrimSpace(strings.TrimPrefix(name, "GNU Lesser General Public License"))
		return "LGPL-" + strings.TrimPrefix(rest, "v")
	case strings.HasPrefix(name, "GNU General Public License"):
		rest := strings.TrimSpace(strings.TrimPrefix(name, "GNU General Public License"))
		return "GPL-" + strings.TrimPrefix(rest, "v")
	case strings.HasPrefix(name, "Mozilla Public License"):
		rest := strings.TrimSpace(strings.TrimPrefix(name, "Mozilla Public License"))
		return "MPL-" + rest
	case strings.HasPrefix(name, "Eclipse Public License"):
		rest := strings.TrimSpace(strings.TrimPrefix(name, "Eclipse Public License"))
		return "EPL-" + strings.TrimPrefix(rest, "v")
	case strings.HasPrefix(name, "Boost Software License"):
		return "BSL-1.0"
	}
	return name
}

// licensePreferenceRow renders the "MIT 50% · Apache-2.0 22% · BSD-3-Clause 6%"
// field when at least one license bucket exists. Returns "" so callers
// can skip emitting the row entirely. Percentages are rounded to whole
// numbers — the upstream label trims fractional digits, and any drift
// below 1 percentage point would not be visually meaningful.
//
// License names are normalised through licenseShortName so the row fits
// on a single line at the 480px upstream-parity canvas width: the
// previous verbose format ("License preference: MIT License 50% /
// Apache License 2.0 22% / BSD 3-Clause \"New\" or \"Revised\" License
// 6%") rendered ~758px wide and forced horizontal overflow.
func licensePreferenceRow(shares []plugins.LicenseShare) string {
	if len(shares) == 0 {
		return ""
	}
	limit := len(shares)
	if limit > licensePreferenceTopRender {
		limit = licensePreferenceTopRender
	}
	parts := make([]string, 0, limit)
	for _, s := range shares[:limit] {
		// Round to nearest whole percent (banker-friendly via +0.5).
		pct := int(s.Percent + 0.5)
		parts = append(parts, fmt.Sprintf("%s %d%%", EscapeXML(licenseShortName(s.Name)), pct))
	}
	return fmt.Sprintf(
		`<div class="field">`+octiconPlaceholder+`%s</div>`,
		strings.Join(parts, " · "),
	)
}

// registry maps partial names (e.g. "base.header" or "plugin.languages")
// to their PartialFunc implementations. Populated by init() in this
// package for M2 base.* partials, and by per-plugin packages
// (internal/plugins/<name>/) via the Register entry point added in M4
// for M4 plugin partials.
var registry = map[string]templates.PartialFunc{}

func init() {
	Register("base.header", BaseHeader)
	Register("introduction", Introduction)
	Register("base.activity+community", BaseActivityCommunity)
	Register("base.repositories", BaseRepositories)
}

// Register adds a PartialFunc under the given name. Subsequent calls
// with the same name overwrite the previous registration — this is the
// expected behavior for both the M2 init-based base partial setup and
// the M4 plugin partial registration path where each plugin package
// registers itself at process start. Not goroutine-safe; intended to
// run from init() only.
func Register(name string, fn templates.PartialFunc) {
	registry[name] = fn
}

// Lookup returns the registered partial by canonical name. M2 base.*
// partials are registered during package init(); M4 plugin partials
// register themselves via Register from their owning plugin package's
// init(). Returns (nil, false) for unknown names; the classic template
// treats that as a contract failure for _.json entries and as a silent
// skip for M4 plugin partials still in flight (US1/US2/US3 land
// incrementally).
func Lookup(name string) (templates.PartialFunc, bool) {
	fn, ok := registry[name]
	return fn, ok
}
