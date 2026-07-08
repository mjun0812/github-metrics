package header

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
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
// and produces a native-SVG identity card (avatar / name / counters).
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Profile == nil {
		return "", 0, nil
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
func BasePartial(ctx context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil {
		return "", 0, nil
	}
	if !basePartialEnabled(pc) {
		return "", 0, nil
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
		return "", 0, nil
	}
	prof, err := pc.Provider.Profile(ctx)
	if err != nil || prof == nil {
		return "", 0, nil //nolint:nilerr // partial-local degradation: a Profile fetch failure must not break the static dispatcher; classic continues rendering the rest of the SVG.
	}
	r := &Result{Profile: prof}
	// CommitCalendar is decorative: a fetch failure leaves the
	// contribution row out but keeps the avatar / counters.
	if cal, cErr := pc.Provider.CommitCalendar(ctx); cErr == nil {
		r.CommitCalendar = cal
	}
	return renderResult(r)
}

// Native-SVG header geometry. The title band holds the 20px avatar and
// the 20px bold display name; counters begin below it. The values mirror
// the flex/CSS metrics the HTML `.field` / h1 / `.avatar` rows used so
// the card lines up with the still-HTML partials it coexists with (#409
// Phase B1).
const (
	hdrTitleTop      = 8                              // h1 margin-top
	hdrAvatarSize    = 20                             // .avatar width/height
	hdrAvatarInset   = 11                             // section inset (5) + .avatar margin (6)
	hdrTitleBand     = 24                             // title content band (avatar / 20px name)
	hdrTitleBottom   = hdrTitleTop + hdrTitleBand + 2 // + h1 margin-bottom
	hdrNameX         = 37                             // inset(5)+avatar(6+20+6); aligns with the field text column
	hdrNameNoAvatarX = 5                              // section inset when no avatar precedes the name
	hdrNameFont      = 20                             // h1 font-size
	hdrNameFill      = "#0366d6"
	// hdrAvatarClipID is the clipPath id for the single header avatar.
	// There is exactly one header per card, so a fixed id is safe.
	hdrAvatarClipID = "header-avatar"
)

// renderResult is the shared rendering core used by Partial (plugin
// dispatch path) and BasePartial (static dispatch path). r is assumed
// non-nil with a populated Profile; callers handle the empty cases. It
// returns the native-SVG markup and the pixel height it consumes.
func renderResult(r *Result) (string, int, error) {
	prof := r.Profile
	// header partial only renders for user accounts.
	if prof.Kind != plugins.ProfileKindUser || prof.User == nil {
		return orgPartial(r)
	}

	u := prof.User
	if u.Login == "" && u.Name == "" {
		return "", 0, nil
	}
	display := u.Name
	if display == "" {
		display = u.Login
	}

	// Two-column counters layout mirroring upstream base.header.ejs.
	// Left column: Joined / Followed by / Following.
	left := chrome.NewSVGColumn(0, chrome.CardWidth/2, hdrTitleBottom)
	if age := format.RelativeAge(u.CreatedAt, currentNow()); age != "" {
		left.Field(":octicon-clock:", "Joined GitHub "+age)
	}
	if u.Followers > 0 {
		left.Field(":octicon-people:", fmt.Sprintf("Followed by %s %s",
			partials.FormatCount(int64(u.Followers)), pluralLabel("user", u.Followers)))
	}
	if u.Following > 0 {
		left.Field(":octicon-people:", fmt.Sprintf("Following %s %s",
			partials.FormatCount(int64(u.Following)), pluralLabel("user", u.Following)))
	}

	// Right column: contribution mini calendar + contributed-to count.
	right := chrome.NewSVGColumn(chrome.CardWidth/2, chrome.CardWidth/2, hdrTitleBottom)
	right.Calendar(recentDays(r.CommitCalendar, headerCalendarDays))
	if u.ContributedTo > 0 {
		noun := "repositories"
		if u.ContributedTo == 1 {
			noun = "repository"
		}
		right.Field(":octicon-repo-push:", fmt.Sprintf("Contributed to %s %s",
			partials.FormatCount(int64(u.ContributedTo)), noun))
	}

	countersHeight := left.Height()
	if right.Height() > countersHeight {
		countersHeight = right.Height()
	}
	height := hdrTitleBottom + int(countersHeight)

	var body strings.Builder
	body.WriteString(avatarMarkup(u.AvatarURL, true))
	body.WriteString(nameMarkup(display, u.AvatarURL != ""))
	if !left.Empty() || !right.Empty() {
		body.WriteString(`<g data-block="header-counters">`)
		body.WriteString(left.Markup())
		body.WriteString(right.Markup())
		body.WriteString(`</g>`)
	}
	return wrapHeaderSVG(body.String(), height), height, nil
}

// orgPartial renders the organization account header card (title only —
// no counters).
func orgPartial(r *Result) (string, int, error) {
	if r.Profile == nil || r.Profile.Organization == nil {
		return "", 0, nil
	}
	org := r.Profile.Organization
	if org.Login == "" && org.Name == "" {
		return "", 0, nil
	}
	display := org.Name
	if display == "" {
		display = org.Login
	}

	height := hdrTitleBottom
	var body strings.Builder
	// Organization avatars clip to a 15%-radius rounded square, not a
	// circle (`.organization.avatar { border-radius: 15% }`).
	body.WriteString(avatarMarkup(org.AvatarURL, false))
	body.WriteString(nameMarkup(display, org.AvatarURL != ""))
	return wrapHeaderSVG(body.String(), height), height, nil
}

// avatarMarkup renders the header avatar or "" when no URL is set.
// rounded selects a circular (user) vs rounded-square (org) clip.
func avatarMarkup(url string, rounded bool) string {
	if url == "" {
		return ""
	}
	return chrome.SVGAvatar(hdrAvatarInset, hdrTitleTop+(hdrTitleBand-hdrAvatarSize)/2,
		hdrAvatarSize, url, hdrAvatarClipID, rounded)
}

// nameMarkup renders the h1 display name as bold blue text, shifted
// right past the avatar when one precedes it.
func nameMarkup(display string, hasAvatar bool) string {
	x := float64(hdrNameNoAvatarX)
	if hasAvatar {
		x = hdrNameX
	}
	baseline := float64(hdrTitleTop) + hdrTitleBand/2 + hdrNameFont*0.32
	return chrome.SVGText(x, baseline, display, chrome.SVGTextOpts{
		Size:     hdrNameFont,
		Weight:   fontmetrics.Bold,
		Fill:     hdrNameFill,
		MaxWidth: chrome.CardWidth - x - hdrNameNoAvatarX,
	})
}

// wrapHeaderSVG wraps the header body in a `<g data-section="header">`
// anchor (kept for DOM diffing / the section gate) plus a fixed-height
// nested `<svg>`. Pure SVG since #409 Phase C dropped the outer
// foreignObject; the template stacks it with the other sections via
// `<g transform="translate(0,y)">`.
func wrapHeaderSVG(body string, height int) string {
	return fmt.Sprintf(
		`<g data-section="header"><svg xmlns="http://www.w3.org/2000/svg" width="100%%" height="%d" viewBox="0 0 %d %d">%s</svg></g>`,
		height, chrome.CardWidth, height, body)
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
