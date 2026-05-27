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

	"github.com/mjun0812/github-metrics/internal/templates"
)

// BaseHeader renders the avatar + display name block at the top of the
// classic SVG. Returns "" when the User payload is absent.
//
// Upstream's base.header.ejs also renders a two-column sub-row with
// "Joined GitHub", followed-by count, contribution-calendar swatches,
// and "Contributed to N repositories". Those rely on user fields the
// Go base plugin does not yet populate (registration date, followers,
// repositoriesContributedTo, computed.calendar). Until those land,
// emitting the empty placeholder `<div class="row"><section/><section/></div>`
// produced a visually broken header (the avatar floated alone with
// large unused vertical space below). #419 dropped the placeholder
// so the header section is dense again. When the missing data fields
// land, this partial should grow back to emit the populated sub-row.
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
	b.WriteString(`</div></section>`)
	return b.String(), nil
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
