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
// users" when those fields are populated. Each row field carries a
// :octicon-<name>: text marker so the ReplaceOcticons() pipeline step
// can inject the real SVG path.
//
// 493: Restructured to use the upstream two-column layout:
// left <section> holds joined/followers/following, right <section>
// holds the contribution calendar and contributed-to count.
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

	// 493: Two-column upstream layout:
	//   Left  <section> — joined/followers/following fields.
	//   Right <section> — contribution calendar + contributed-to count.
	// This mirrors upstream base.header.ejs which uses
	// `<div class="row"><section/><section/></div>` so that the
	// `.row section { flex: 1 1 0 }` CSS rule activates the side-by-side
	// layout. When either column is fully empty it is omitted so the
	// section stays compact.

	// Left column: Joined / Followed by / Following.
	var leftRows []string
	if age := format.RelativeAge(u.CreatedAt, nowFunc()); age != "" {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-clock:Joined GitHub %s</div>`, age,
		))
	}
	if u.Followers > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Followed by %s %s</div>`,
			FormatCount(int64(u.Followers)), pluralLabel("user", u.Followers),
		))
	}
	if u.Following > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Following %s %s</div>`,
			FormatCount(int64(u.Following)), pluralLabel("user", u.Following),
		))
	}

	// Right column: contribution mini calendar + contributed-to count.
	// 429 Phase 3: calendar rendered as a single horizontal SVG row.
	// 429 Phase 2: "Contributed to N repositories".
	var rightRows []string
	if row := contributionRow(u.RecentContributions); row != "" {
		rightRows = append(rightRows, row)
	}
	if u.ContributedTo > 0 {
		noun := "repositories"
		if u.ContributedTo == 1 {
			noun = "repository"
		}
		rightRows = append(rightRows, fmt.Sprintf(
			`<div class="field">:octicon-repo-push:Contributed to %s %s</div>`,
			FormatCount(int64(u.ContributedTo)), noun,
		))
	}

	if len(leftRows) > 0 || len(rightRows) > 0 {
		b.WriteString(`<div class="row" data-block="header-counters">`)
		if len(leftRows) > 0 {
			b.WriteString(`<section>`)
			for _, row := range leftRows {
				b.WriteString(row)
			}
			b.WriteString(`</section>`)
		}
		if len(rightRows) > 0 {
			b.WriteString(`<section>`)
			for _, row := range rightRows {
				b.WriteString(row)
			}
			b.WriteString(`</section>`)
		}
		b.WriteString(`</div>`)
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
	calendarCellSize = 11
	// calendarCellPitch is the per-day horizontal step. Upstream
	// `base.header.ejs` lays each cell at `x = index*15`, so the 11px
	// cell sits in a 15px slot (4px gap).
	calendarCellPitch = 15
)

// emptyCellColor is the canonical GitHub no-contribution color used for
// padding (e.g. when the trailing week of a fresh account has < 7 days)
// and as a defensive fallback when the GraphQL `color` field is empty.
const emptyCellColor = "#ebedf0"

// contributionRow renders the BaseHeader mini contribution calendar as
// a single horizontal row of day cells embedded in an HTML container so
// the existing `.calendar.field` CSS rule (margin-left/top tweak)
// applies. This mirrors upstream `base.header.ejs`, which lays the last
// 14 days out left-to-right (oldest -> newest).
//
// Each cell is a `class="day"` rect whose `fill` carries the
// GitHub-supplied hex so plain renderers (no CSS) draw the correct
// color; the `.calendar .day` CSS rule adds the cell outline. Returns
// "" when no days are present so the partial hides the block.
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

// BaseActivityCommunity renders the Activity + Community-stats two-column
// block, mirroring upstream `base.activity+community.ejs`.
//
// The Activity column shows the lifetime contribution aggregates
// (commits, PRs reviewed, PRs opened, issues opened, issue comments)
// sourced from Data.User — the base plugin populates these from the
// always-fetched User query's `contributionsCollection.*` aggregate
// fields plus `issueComments.totalCount` (#442), so the section renders
// for a plain `base` run without requiring an indepth-dependent plugin.
//
// The Community column shows the user-relationship counters
// (organizations, following, sponsoring, starred, watching) also
// sourced from Data.User. Per upstream every community row renders
// unconditionally — including "Sponsoring 0 repositories" — so the
// column reflects the GitHub profile exactly.
//
// Each column is independently gated by the resolved `base` sections:
// the Activity column only appears when `activity` is enabled, the
// Community column only when `community` is enabled. The classic
// dispatcher already skips this partial entirely when neither flag is
// set, so reaching here means at least one column is active.
//
// Per #419 the partial returns "" (no wrapper at all) when neither
// active column has any data — i.e. the base plugin has not populated
// the User contribution/community counters (the gen-doc-samples base
// sample, or a degraded GraphQL response). That keeps a base-only
// render from emitting an empty `<section
// data-section="activity-community">` band that previously inflated
// the card height.
func BaseActivityCommunity(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil || pc.Data.User == nil {
		return "", nil
	}
	u := pc.Data.User

	activityOn := baseSectionEnabled(pc.Inputs, "activity")
	communityOn := baseSectionEnabled(pc.Inputs, "community")

	hasActivity := activityOn && (u.Commits > 0 || u.PullRequestsReviewed > 0 ||
		u.PullRequestsOpened > 0 || u.IssuesOpened > 0 || u.IssueComments > 0)
	// Community renders rows unconditionally (upstream shows "Sponsoring
	// 0 repositories"), so the presence test keys off any populated
	// community-or-following/watching counter to distinguish a real run
	// from an unpopulated fixture.
	hasCommunity := communityOn && (u.Organizations > 0 || u.Following > 0 ||
		u.Sponsoring > 0 || u.Starred > 0 || u.Watching > 0)
	if !hasActivity && !hasCommunity {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="activity-community">`)
	b.WriteString(`<section class="row">`)

	if hasActivity {
		b.WriteString(`<section data-block="activity">`)
		b.WriteString(`<h2 class="field">:octicon-graph:Activity</h2>`)
		fmt.Fprintf(&b, `<div class="field">:octicon-git-commit:%s %s</div>`,
			FormatCount(int64(u.Commits)), pluralLabel("Commit", u.Commits))
		fmt.Fprintf(&b, `<div class="field">:octicon-code-review:%s %s reviewed</div>`,
			FormatCount(int64(u.PullRequestsReviewed)), pluralLabel("Pull request", u.PullRequestsReviewed))
		fmt.Fprintf(&b, `<div class="field">:octicon-git-pull-request:%s %s opened</div>`,
			FormatCount(int64(u.PullRequestsOpened)), pluralLabel("Pull request", u.PullRequestsOpened))
		fmt.Fprintf(&b, `<div class="field">:octicon-issue-opened:%s %s opened</div>`,
			FormatCount(int64(u.IssuesOpened)), pluralLabel("Issue", u.IssuesOpened))
		fmt.Fprintf(&b, `<div class="field">:octicon-comment:%s issue %s</div>`,
			FormatCount(int64(u.IssueComments)), pluralLabel("comment", u.IssueComments))
		b.WriteString(`</section>`)
	}

	if hasCommunity {
		b.WriteString(`<section data-block="community">`)
		b.WriteString(`<h2 class="field">:octicon-organization:Community stats</h2>`)
		fmt.Fprintf(&b, `<div class="field">:octicon-organization:Member of %s %s</div>`,
			FormatCount(int64(u.Organizations)), pluralLabel("organization", u.Organizations))
		fmt.Fprintf(&b, `<div class="field">:octicon-people:Following %s %s</div>`,
			FormatCount(int64(u.Following)), pluralLabel("user", u.Following))
		fmt.Fprintf(&b, `<div class="field">:octicon-heart:Sponsoring %s %s</div>`,
			FormatCount(int64(u.Sponsoring)), pluralRepositoryLabel(u.Sponsoring))
		fmt.Fprintf(&b, `<div class="field">:octicon-star:Starred %s %s</div>`,
			FormatCount(int64(u.Starred)), pluralRepositoryLabel(u.Starred))
		fmt.Fprintf(&b, `<div class="field">:octicon-eye:Watching %s %s</div>`,
			FormatCount(int64(u.Watching)), pluralRepositoryLabel(u.Watching))
		b.WriteString(`</section>`)
	}

	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// pluralRepositoryLabel renders "repository" / "repositories" matching
// upstream's `s(count, "y")` helper used by the Community-stats rows.
func pluralRepositoryLabel(count int) string {
	if count == 1 {
		return "repository"
	}
	return "repositories"
}

// baseSectionEnabled reports whether the named base section (e.g.
// "activity" / "community") is enabled by the resolved `base` input.
// When the `base` input is absent every section is enabled (mirrors the
// classic dispatcher's resolveBaseSections default). An explicit empty
// string disables all sections.
func baseSectionEnabled(in map[string]any, section string) bool {
	raw, present := readBaseInputValue(in)
	if !present {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(part), section) {
			return true
		}
	}
	return false
}

// readBaseInputValue extracts the `base` input as a CSV string.
// Returns (value, true) when the key is present even if the value is ""
// so callers can distinguish "unset" (render all) from "explicit empty"
// (render none). Mirrors classic.readBaseInput; kept local to avoid a
// partials→classic import cycle.
func readBaseInputValue(in map[string]any) (string, bool) {
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
	fmt.Fprintf(&b, `<div class="field">:octicon-repo:%s repositories</div>`,
		FormatCount(int64(r.Count)))
	fmt.Fprintf(&b, `<div class="field">:octicon-star:%s stargazers</div>`,
		FormatCount(int64(r.Stargazers)))
	fmt.Fprintf(&b, `<div class="field">:octicon-git-branch:%s forks</div>`,
		FormatCount(int64(r.Forks)))
	if r.Releases > 0 {
		fmt.Fprintf(&b, `<div class="field">:octicon-tag:%s %s</div>`,
			FormatCount(int64(r.Releases)), pluralLabel("release", r.Releases))
	}
	if r.Packages > 0 {
		fmt.Fprintf(&b, `<div class="field">:octicon-package:%s %s</div>`,
			FormatCount(int64(r.Packages)), pluralLabel("package", r.Packages))
	}
	if r.DiskUsage > 0 {
		fmt.Fprintf(&b, `<div class="field">:octicon-database:%s used</div>`,
			format.FormatDiskKB(r.DiskUsage))
	}
	// 442: "Watching N repositories" moved to the Community-stats column
	// (base.activity+community.ejs) to match upstream's layout — only the
	// "N sponsors" (sponsorshipsAsMaintainer) counter belongs to the
	// repositories row per upstream base.repositories.ejs.
	if u := pc.Data.User; u != nil {
		if u.SponsorshipsAsMaintainer > 0 {
			fmt.Fprintf(&b, `<div class="field">:octicon-heart:%s %s</div>`,
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
		`<div class="field">:octicon-law:%s</div>`,
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
