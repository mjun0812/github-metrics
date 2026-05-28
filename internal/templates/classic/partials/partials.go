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

// BaseHeader renders the avatar + display name block at the top of the
// classic SVG. Returns "" when the User payload is absent.
//
// 429 Phase 1: also renders the upstream base.header.ejs sub-row with
// "Joined GitHub <age>", "Followed by N users", and "Following N
// users" when those fields are populated. Each row is rendered with a
// neutral `<svg class="octicon"></svg>` placeholder — the icon path
// shapes are left for the Phase 2 / Phase 3 work that brings the
// contribution-calendar grid and the upstream-equivalent octicons.
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
			`<div class="field"><svg class="octicon"></svg>Joined GitHub %s</div>`, age,
		))
	}
	if u.Followers > 0 {
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field"><svg class="octicon"></svg>Followed by %s %s</div>`,
			FormatCount(int64(u.Followers)), pluralLabel("user", u.Followers),
		))
	}
	if u.Following > 0 {
		subRows = append(subRows, fmt.Sprintf(
			`<div class="field"><svg class="octicon"></svg>Following %s %s</div>`,
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
			`<div class="field"><svg class="octicon"></svg>Contributed to %s %s</div>`,
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

	b.WriteString(`</section>`)
	return b.String(), nil
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
	b.WriteString(`<h2 class="field"><svg class="octicon"></svg>Activity</h2>`)
	if c.TotalCommits > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s</div>`,
			FormatCount(int64(c.TotalCommits)), pluralLabel("Commit", c.TotalCommits))
	}
	if c.TotalPullRequests > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s opened</div>`,
			FormatCount(int64(c.TotalPullRequests)), pluralLabel("Pull request", c.TotalPullRequests))
	}
	if c.TotalIssues > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s opened</div>`,
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
	fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s repositories</div>`,
		FormatCount(int64(r.Count)))
	fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s stargazers</div>`,
		FormatCount(int64(r.Stargazers)))
	fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s forks</div>`,
		FormatCount(int64(r.Forks)))
	if r.Releases > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s</div>`,
			FormatCount(int64(r.Releases)), pluralLabel("release", r.Releases))
	}
	if r.Packages > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s</div>`,
			FormatCount(int64(r.Packages)), pluralLabel("package", r.Packages))
	}
	if r.DiskUsage > 0 {
		fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s used</div>`,
			format.FormatDiskKB(r.DiskUsage))
	}
	if u := pc.Data.User; u != nil {
		if u.Watching > 0 {
			noun := "repositories"
			if u.Watching == 1 {
				noun = "repository"
			}
			fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>Watching %s %s</div>`,
				FormatCount(int64(u.Watching)), noun)
		}
		if u.SponsorshipsAsMaintainer > 0 {
			fmt.Fprintf(&b, `<div class="field"><svg class="octicon"></svg>%s %s</div>`,
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
// inside the partial at the top three entries upstream
// `base.repositories.ejs` displays. The data model carries up to 5
// (see internal/plugins/base.licensePreferenceTopN) so future
// renderers can opt into a longer breakdown without re-aggregating.
const licensePreferenceTopRender = 3

// licensePreferenceRow renders the "License preference: A 60% / B 20% /
// C 10%" field when at least one license bucket exists. Returns "" so
// callers can skip emitting the row entirely. Percentages are rounded
// to whole numbers — the upstream label trims fractional digits, and
// any drift below 1 percentage point would not be visually meaningful.
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
		parts = append(parts, fmt.Sprintf("%s %d%%", EscapeXML(s.Name), pct))
	}
	return fmt.Sprintf(
		`<div class="field"><svg class="octicon"></svg>License preference: %s</div>`,
		strings.Join(parts, " / "),
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
