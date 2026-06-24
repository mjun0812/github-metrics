package header

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// headerCalendarDays is the trailing window of contribution days the
// header mini-calendar renders. 14 mirrors upstream core/index.mjs's
// slice(-14).
const headerCalendarDays = 14

// nowFunc is the time source used by Partial's "Joined GitHub" label.
// Tests overwrite it via SetNowForTest to anchor the rendered string.
var (
	nowMu   sync.RWMutex
	nowFunc = time.Now
)

// SetNowForTest overrides the time source used by Partial. The
// returned function restores the previous value.
func SetNowForTest(now func() time.Time) func() {
	nowMu.Lock()
	prev := nowFunc
	nowFunc = now
	nowMu.Unlock()
	return func() {
		nowMu.Lock()
		nowFunc = prev
		nowMu.Unlock()
	}
}

func currentNow() time.Time {
	nowMu.RLock()
	fn := nowFunc
	nowMu.RUnlock()
	return fn()
}

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Partial renders the classic SVG fragment for the header plugin.
// It reads the Result published by Run under data.Plugins["header"]
// and produces an output byte-equivalent to the legacy BaseHeader
// partial.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Profile == nil {
		return "", nil
	}

	prof := r.Profile
	// header partial only renders for user accounts.
	if prof.Kind != plugins.ProfileKindUser || prof.User == nil {
		return orgPartial(r)
	}

	u := prof.User
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
			partials.EscapeXML(u.AvatarURL))
	}
	fmt.Fprintf(&b, `<span>%s</span>`, partials.EscapeXML(display))
	b.WriteString(`</h1>`)

	// Two-column layout mirroring upstream base.header.ejs.
	// Left column: Joined / Followed by / Following.
	var leftRows []string
	if age := format.RelativeAge(u.CreatedAt, currentNow()); age != "" {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-clock:Joined GitHub %s</div>`, age,
		))
	}
	if u.Followers > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Followed by %s %s</div>`,
			partials.FormatCount(int64(u.Followers)), pluralLabel("user", u.Followers),
		))
	}
	if u.Following > 0 {
		leftRows = append(leftRows, fmt.Sprintf(
			`<div class="field">:octicon-people:Following %s %s</div>`,
			partials.FormatCount(int64(u.Following)), pluralLabel("user", u.Following),
		))
	}

	// Right column: contribution mini calendar + contributed-to count.
	var rightRows []string
	recent := recentDays(r.CommitCalendar, headerCalendarDays)
	if row := chrome.ContributionRow(recent); row != "" {
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
	return b.String(), nil
}

// orgPartial renders the organization account header card.
func orgPartial(r *Result) (string, error) {
	if r.Profile == nil || r.Profile.Organization == nil {
		return "", nil
	}
	org := r.Profile.Organization
	if org.Login == "" && org.Name == "" {
		return "", nil
	}
	display := org.Name
	if display == "" {
		display = org.Login
	}

	var b strings.Builder
	b.WriteString(`<section data-section="header">`)
	b.WriteString(`<h1 class="field">`)
	if org.AvatarURL != "" {
		fmt.Fprintf(&b, `<img class="avatar" src=%q width="20" height="20" />`,
			partials.EscapeXML(org.AvatarURL))
	}
	fmt.Fprintf(&b, `<span>%s</span>`, partials.EscapeXML(display))
	b.WriteString(`</h1>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// recentDays flattens CommitCalendar weeks into a chronological day
// list and returns the trailing n days, mirroring upstream
// core/index.mjs's slice(-14).
func recentDays(cal *plugins.ContributionCalendar, n int) []plugins.ContributionDay {
	if cal == nil {
		return nil
	}
	days := make([]plugins.ContributionDay, 0, len(cal.Weeks)*7)
	for _, w := range cal.Weeks {
		days = append(days, w.Days...)
	}
	if len(days) == 0 {
		return nil
	}
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	return days
}

// pluralLabel appends "s" unless count == 1.
func pluralLabel(singular string, count int) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}
