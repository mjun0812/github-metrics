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
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
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
	if row := chrome.ContributionRow(u.RecentContributions); row != "" {
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
		fmt.Fprintf(&b, `<div class="field">:octicon-git-commit:%d %s</div>`,
			u.Commits, pluralLabel("Commit", u.Commits))
		fmt.Fprintf(&b, `<div class="field">:octicon-code-review:%d %s reviewed</div>`,
			u.PullRequestsReviewed, pluralLabel("Pull request", u.PullRequestsReviewed))
		fmt.Fprintf(&b, `<div class="field">:octicon-git-pull-request:%d %s opened</div>`,
			u.PullRequestsOpened, pluralLabel("Pull request", u.PullRequestsOpened))
		fmt.Fprintf(&b, `<div class="field">:octicon-issue-opened:%d %s opened</div>`,
			u.IssuesOpened, pluralLabel("Issue", u.IssuesOpened))
		fmt.Fprintf(&b, `<div class="field">:octicon-comment:%d issue %s</div>`,
			u.IssueComments, pluralLabel("comment", u.IssueComments))
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

func repositoryWord(count int) string {
	if count == 1 {
		return "Repository"
	}
	return "Repositories"
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
	fmt.Fprintf(&b, `<h2 class="field">:octicon-repo:%d %s</h2>`,
		r.Count, repositoryWord(r.Count))
	b.WriteString(`<div class="row">`)
	b.WriteString(`<section>`)
	if top := licensePreferenceTop(r.LicensePreference); top != "" {
		fmt.Fprintf(&b, `<div class="field">:octicon-law:Prefers %s license</div>`,
			EscapeXML(top))
	}
	fmt.Fprintf(&b, `<div class="field">:octicon-tag:%d %s</div>`,
		r.Releases, pluralLabel("Release", r.Releases))
	fmt.Fprintf(&b, `<div class="field">:octicon-package:%d %s</div>`,
		r.Packages, pluralLabel("Package", r.Packages))
	if r.DiskUsage > 0 {
		fmt.Fprintf(&b, `<div class="field">:octicon-database:%s used</div>`,
			format.FormatDiskKB(r.DiskUsage))
	}
	b.WriteString(`</section><section>`)
	sponsors := 0
	if u := pc.Data.User; u != nil {
		sponsors = u.SponsorshipsAsMaintainer
	}
	fmt.Fprintf(&b, `<div class="field">:octicon-heart:%d %s</div>`,
		sponsors, pluralLabel("Sponsor", sponsors))
	fmt.Fprintf(&b, `<div class="field">:octicon-star:%d %s</div>`,
		r.Stargazers, pluralLabel("Stargazer", r.Stargazers))
	fmt.Fprintf(&b, `<div class="field">:octicon-repo-forked:%d %s</div>`,
		r.Forks, pluralLabel("Forker", r.Forks))
	fmt.Fprintf(&b, `<div class="field">:octicon-eye:%d %s</div>`,
		r.Watchers, pluralLabel("Watcher", r.Watchers))
	if views := trafficViews(pc); views > 0 {
		fmt.Fprintf(&b, `<div class="field">:octicon-book:%s views in last two weeks</div>`,
			FormatCount(int64(views)))
	}
	b.WriteString(`</section></div></section>`)
	return b.String(), nil
}

type trafficViewTotaler interface {
	TotalViews() int
}

func trafficViews(pc *templates.PartialContext) int {
	if pc == nil || pc.Data == nil {
		return 0
	}
	raw, ok := pc.Data.GetPlugin("traffic")
	if !ok || raw == nil {
		return 0
	}
	if sr, ok := raw.(interface{ IsSkipped() bool }); ok && sr.IsSkipped() {
		return 0
	}
	if r, ok := raw.(trafficViewTotaler); ok {
		return r.TotalViews()
	}
	return 0
}

func licensePreferenceTop(shares []plugins.LicenseShare) string {
	if len(shares) == 0 {
		return ""
	}
	return licenseShortName(shares[0].Name)
}

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
