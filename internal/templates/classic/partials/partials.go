// Package partials hosts the partial functions assembled by the classic
// template. Each partial is a templates.PartialFunc that takes a
// PartialContext and returns the SVG fragment it owns. The contract is
// documented in specs/002-output-classic-json/contracts/classic-template.md.
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
	b.WriteString(`<div class="row"><section></section><section></section></div>`)
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

// BaseActivityCommunity renders the outer two-column scaffold that
// hydrates with activity + community panes once M4 plugins land. M2
// emits the section unconditionally so DOM ordering matches upstream.
func BaseActivityCommunity(_ context.Context, _ *templates.PartialContext) (string, error) {
	var b strings.Builder
	b.WriteString(`<section data-section="activity-community">`)
	b.WriteString(`<section class="row">`)
	b.WriteString(`<section data-block="activity"></section>`)
	b.WriteString(`<section data-block="community"></section>`)
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
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
