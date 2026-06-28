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
	// `base.header` is the static partial slot fired by the classic
	// dispatcher before plugin partials. Registering BasePartial here
	// (rather than from the partials package's own init seed) wires the
	// header chrome into the top of every classic render whose `base`
	// CSV includes `header`, even when plugin_header is not toggled.
	partials.Register("base.header", BasePartial)
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
	return renderResult(r)
}

// basePartialEnabled is the explicit input-level gate for BasePartial.
// `chrome_header=yes` is the canonical surface (#640); the legacy
// `plugin_base=yes` master switch is honoured as a v2 compat path when
// no chrome_* input is declared, so workflows that have not migrated
// still get the identity card.
func basePartialEnabled(pc *templates.PartialContext) bool {
	if pc == nil {
		return false
	}
	return runEnabledForInputs(pc.Inputs)
}

// runEnabledForInputs reports whether the header plugin should produce
// any output (either as a Run-time fetch or as a static-partial
// render). Shared by header.Run and BasePartial so the two paths stay
// in lockstep.
func runEnabledForInputs(in map[string]any) bool {
	if chrome.TruthyInput(in, "chrome_header") {
		return true
	}
	if chrome.TruthyInput(in, "plugin_"+Name) {
		return true
	}
	if !chrome.AnyChromeInputPresent(in) &&
		chrome.TruthyInput(in, "plugin_base") {
		return true
	}
	return false
}

// BasePartial is the `base.header` static partial entry point. It
// renders the same chrome as Partial but degrades to a Provider fetch
// when the header plugin has not been Run (i.e. when the user enabled
// `chrome_header=yes` without also setting `plugin_header=yes`). All
// failure modes return ("", nil) so the static dispatcher continues
// rendering the rest of the SVG.
//
// Reading `chrome_header` directly here (rather than relying solely on
// classic/repository.partialEnabledByBase) keeps the input contract
// readable when this function is exercised in isolation — and lets
// the legacy `plugin_base=yes` v2 compat path still pull in the
// identity card without forcing callers to also set chrome_header.
func BasePartial(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil {
		return "", nil
	}
	if !basePartialEnabled(pc) {
		return "", nil
	}
	// Prefer a Result already published by the header plugin's Run.
	if pc.Data != nil {
		if raw, ok := pc.Data.GetPlugin(Name); ok && raw != nil {
			if r, ok := raw.(*Result); ok && r != nil && r.Profile != nil {
				return renderResult(r)
			}
		}
	}
	// Lazy path: no plugin result available. Fetch the two profile
	// pieces directly from the Provider. Any Provider failure degrades
	// silently so the rest of the classic SVG still renders.
	if pc.Provider == nil {
		return "", nil
	}
	prof, err := pc.Provider.Profile(ctx)
	if err != nil || prof == nil {
		return "", nil //nolint:nilerr // partial-local degradation: a Profile fetch failure must not break the static dispatcher; classic continues rendering the rest of the SVG.
	}
	r := &Result{Profile: prof}
	// CommitCalendar is decorative: a fetch failure leaves the
	// contribution row out but keeps the avatar / counters.
	if cal, cErr := pc.Provider.CommitCalendar(ctx); cErr == nil {
		r.CommitCalendar = cal
	}
	return renderResult(r)
}

// renderResult is the shared rendering core used by Partial (plugin
// dispatch path) and BasePartial (static dispatch path). r is assumed
// non-nil with a populated Profile; callers handle the empty cases.
func renderResult(r *Result) (string, error) {
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
