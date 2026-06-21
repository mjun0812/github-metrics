package header

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// nowFunc is the time source used by Partial's "Joined GitHub <N>
// years ago" label. It defaults to time.Now; tests override via
// SetNowForTest to anchor the rendered string. Not goroutine-safe;
// production code never reassigns it.
var nowFunc = time.Now

// SetNowForTest overrides the time source used by Partial. The returned
// function restores the previous value.
func SetNowForTest(now func() time.Time) func() {
	prev := nowFunc
	nowFunc = now
	return func() { nowFunc = prev }
}

// Partial renders the classic SVG fragment for the header plugin. It
// mirrors upstream `assets/templates/classic/partials/header.ejs` and
// the legacy in-tree BaseHeader: avatar + display name, joined-GitHub
// age, follower / following counters, the trailing two-week commit
// calendar, and a "contributed to N repositories" row.
//
// Returns "" when the Profile is absent or the plugin Result is missing
// — empty returns are the canonical Skipped signal at the partial layer.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || r.Profile == nil {
		return "", nil
	}

	switch r.Profile.Kind {
	case plugins.ProfileKindUser:
		return renderUser(r.Profile.User, r.Calendar), nil
	case plugins.ProfileKindOrganization:
		return renderOrganization(r.Profile.Organization), nil
	default:
		return "", nil
	}
}

// renderUser produces the user-account flavour of the header card. It
// mirrors the upstream user branch of `base.header.ejs`.
func renderUser(u *plugins.User, calendar []plugins.ContributionDay) string {
	if u == nil {
		return ""
	}
	if u.Login == "" && u.Name == "" {
		return ""
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
			partials.EscapeXML(u.AvatarURL))
	}
	fmt.Fprintf(&b, `<span>%s</span>`, partials.EscapeXML(display))
	b.WriteString(`</h1>`)

	// Two-column layout matching upstream `<div class="row">
	// <section/><section/></div>`. Left column carries the identity
	// rows; right column carries the mini calendar + contributed-to.
	var leftRows []string
	if age := format.RelativeAge(u.CreatedAt, nowFunc()); age != "" {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-clock:Joined GitHub %s</div>`, age,
		))
	}
	if u.Followers > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Followed by %s %s</div>`,
			partials.FormatCount(int64(u.Followers)), pluralUser(u.Followers),
		))
	}
	if u.Following > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Following %s %s</div>`,
			partials.FormatCount(int64(u.Following)), pluralUser(u.Following),
		))
	}

	var rightRows []string
	if row := chrome.ContributionRow(calendar); row != "" {
		rightRows = append(rightRows, row)
	}
	if u.ContributedTo > 0 {
		noun := "repositories"
		if u.ContributedTo == 1 {
			noun = "repository"
		}
		rightRows = append(rightRows, fmt.Sprintf(
			`<div class="field">:octicon-repo-push:Contributed to %s %s</div>`,
			partials.FormatCount(int64(u.ContributedTo)), noun,
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
	return b.String()
}

// renderOrganization produces the organization-account flavour of the
// header card. Mirrors the upstream `else if (account === "organization")`
// branch of `base.header.ejs`.
func renderOrganization(o *plugins.Organization) string {
	if o == nil {
		return ""
	}
	if o.Login == "" && o.Name == "" {
		return ""
	}
	display := o.Name
	if display == "" {
		display = o.Login
	}

	var b strings.Builder
	b.WriteString(`<section data-section="header">`)
	b.WriteString(`<h1 class="field">`)
	if o.AvatarURL != "" {
		fmt.Fprintf(&b, `<img class="avatar organization" src=%q width="20" height="20" />`,
			partials.EscapeXML(o.AvatarURL))
	}
	fmt.Fprintf(&b, `<span>%s</span>`, partials.EscapeXML(display))
	b.WriteString(`</h1>`)

	if o.MembersCount > 0 {
		b.WriteString(`<div class="row" data-block="header-counters">`)
		b.WriteString(`<section>`)
		fmt.Fprintf(&b, `<div class="field">:octicon-people:%s %s</div>`,
			partials.FormatCount(int64(o.MembersCount)), pluralMember(o.MembersCount))
		b.WriteString(`</section>`)
		b.WriteString(`</div>`)
	}

	b.WriteString(`</section>`)
	return b.String()
}

func pluralUser(n int) string {
	if n == 1 {
		return "user"
	}
	return "users"
}

func pluralMember(n int) string {
	if n == 1 {
		return "member"
	}
	return "members"
}
