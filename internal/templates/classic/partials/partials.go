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

// Lookup returns the registered partial by canonical name (matching the
// strings inside assets/templates/classic/partials/_.json). The classic
// template uses this to drive partial dispatch from data rather than
// hard-coded switch statements.
func Lookup(name string) (templates.PartialFunc, bool) {
	switch name {
	case "base.header":
		return BaseHeader, true
	case "introduction":
		return Introduction, true
	case "base.activity+community":
		return BaseActivityCommunity, true
	case "base.repositories":
		return BaseRepositories, true
	}
	return nil, false
}
