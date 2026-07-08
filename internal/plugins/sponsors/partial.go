package sponsors

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// heartOcticon is the upstream `<%- octicon "heart" %>` 16x16 path used
// in the sponsors section header. Mirrors EJS line 4.
const heartOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4.25 2.5c-1.336 0-2.75 1.164-2.75 3 0 2.15 1.58 4.144 3.365 5.682A20.565 20.565 0 008 13.393a20.561 20.561 0 003.135-2.211C12.92 9.644 14.5 7.65 14.5 5.5c0-1.836-1.414-3-2.75-3-1.373 0-2.609.986-3.029 2.456a.75.75 0 01-1.442 0C6.859 3.486 5.623 2.5 4.25 2.5zM8 14.25l-.345.666-.002-.001-.006-.003-.018-.01a7.643 7.643 0 01-.31-.17 22.075 22.075 0 01-3.434-2.414C2.045 10.731 0 8.35 0 5.5 0 2.836 2.086 1 4.25 1 5.797 1 7.153 1.802 8 3.02 8.847 1.802 10.203 1 11.75 1 13.914 1 16 2.836 16 5.5c0 2.85-2.045 5.231-3.885 6.818a22.08 22.08 0 01-3.744 2.584l-.018.01-.006.003h-.002L8 14.25zm0 0l.345.666a.752.752 0 01-.69 0L8 14.25z"></path></svg>`

func sponsorsAreFundingVerb(n int) string {
	if n == 1 {
		return " is"
	}
	return "s are"
}

// Partial renders the classic SVG fragment for the sponsors plugin
// matching upstream org_repo/source/templates/classic/partials/sponsors.ejs.
//
// Output (native SVG): a `<section data-section="sponsors">` anchor
// wrapping a nested <svg> with a section header and, per configured
// section, a `<g class="sponsors goal">` box (goal description, progress
// bar, funding-goal text, sponsor avatar grid + past block) or a
// `<g class="sponsors">` About block (header + markdown bio). The
// partial reports its consumed pixel height (#409 Phase B2).
//
// Settings: mjun0812 uses plugin_sponsors_sections: goal, about, list +
// plugin_sponsors_past: yes.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", 0, nil
	}

	title := r.Title
	if title == "" {
		title = "Sponsor Me!"
	}
	user := r.User
	if user == "" {
		user = "this user"
	}
	size := r.Size
	if size == 0 {
		size = 24
	}

	sections := r.Sections
	if len(sections) == 0 {
		sections = []string{"goal", "list", "about"}
	}
	hasGoal := slices.Contains(sections, "goal")
	hasList := slices.Contains(sections, "list")

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(heartOcticon, title)
	body.WriteString(header)

	for _, section := range sections {
		// Upstream skips the standalone "list" iteration when "goal" is
		// also present because "goal" emits the list inline (EJS line 17).
		if hasGoal && hasList && section == "list" {
			continue
		}
		switch section {
		case "goal", "list":
			m, h := renderGoalSection(r, section, user, size, hasList, y)
			body.WriteString(m)
			y += h
		case "about":
			m, h := renderAboutSection(r, y)
			body.WriteString(m)
			y += h
		}
	}

	height := int(y)
	return chrome.WrapSection("sponsors", height, body.String()), height, nil
}

// Native-SVG sponsors geometry. `.sponsors` is inset 13px each side;
// `.sponsors.goal` adds an 8px horizontal / 6px vertical pad and a light
// grey rounded background.
const (
	sponsorInset  = 13.0
	sponsorBoxW   = chrome.CardWidth - 2*sponsorInset
	sponsorPadX   = 8.0
	sponsorPadY   = 6.0
	sponsorContX  = sponsorInset + sponsorPadX
	sponsorContW  = sponsorBoxW - 2*sponsorPadX
	sponsorGoalBG = "#777777" // .sponsors.goal background (12% opacity)
	sponsorText   = "#777777"
	sponsorAvGap  = 2.0 // .sponsors .avatar { margin: 2px }
)

// renderGoalSection renders the "goal"/"list" box: an optional goal
// description + progress bar, the funding-goal text line, and (when the
// list is configured) the sponsor avatar grid plus past block. Returns
// the markup and the height consumed.
func renderGoalSection(r *Result, section, user string, size int, hasList bool, top float64) (string, float64) {
	boxTop := top + 4
	innerY := boxTop + sponsorPadY

	var inner strings.Builder
	if section == "goal" && r.Goal != nil {
		if r.Goal.Description != "" {
			m, dh := renderMarkdownSVG(r.Goal.Description, sponsorContX, innerY, sponsorContW)
			inner.WriteString(m)
			innerY += dh + 2
		}
		progress := r.Goal.Progress
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		inner.WriteString(progressBar(sponsorContX, innerY+4, sponsorContW, progress))
		innerY += 16
	}

	// Funding-goal text: left "N sponsors are funding user's work" (only
	// when there are active sponsors, per upstream) + right goal title.
	baseline := innerY + 12
	if r.Count.Active.Total > 0 {
		inner.WriteString(chrome.SVGText(sponsorContX, baseline,
			fmt.Sprintf("%d sponsor%s funding %s's work",
				r.Count.Active.Total, sponsorsAreFundingVerb(r.Count.Active.Total), user),
			chrome.SVGTextOpts{Size: 12, Fill: sponsorText, MaxWidth: sponsorContW * 0.65}))
	}
	if section == "goal" && r.Goal != nil && r.Goal.Title != "" {
		inner.WriteString(chrome.SVGText(sponsorContX+sponsorContW, baseline, r.Goal.Title,
			chrome.SVGTextOpts{Size: 12, Fill: sponsorText, Anchor: "end", MaxWidth: sponsorContW * 0.4}))
	}
	innerY += 18

	if section == "list" || hasList {
		specs := make([]chrome.SVGAvatarSpec, 0, len(r.Sponsors))
		for _, s := range r.Sponsors {
			specs = append(specs, chrome.SVGAvatarSpec{URL: s.Avatar, IsOrg: s.Type == "organization"})
		}
		m, ah := chrome.SVGAvatarGrid(sponsorContX, innerY, sponsorContW, float64(size), sponsorAvGap, "sponsor-av", specs)
		inner.WriteString(m)
		innerY += ah
		if r.PastIncluded && r.Count.Past.Total > 0 {
			innerY += 8
			inner.WriteString(chrome.SVGText(sponsorContX, innerY+12,
				fmt.Sprintf("%d sponsor%s helped %s in the past",
					r.Count.Past.Total, pluginutil.Plural(r.Count.Past.Total), user),
				chrome.SVGTextOpts{Size: 12, Fill: sponsorText, MaxWidth: sponsorContW}))
			innerY += 18
			pastSize := float64(size) * 0.8
			pspecs := make([]chrome.SVGAvatarSpec, 0, len(r.Past))
			for _, s := range r.Past {
				pspecs = append(pspecs, chrome.SVGAvatarSpec{URL: s.Avatar, IsOrg: s.Type == "organization"})
			}
			m2, ah2 := chrome.SVGAvatarGrid(sponsorContX, innerY, sponsorContW, pastSize, sponsorAvGap, "sponsor-pav", pspecs)
			inner.WriteString(m2)
			innerY += ah2
		}
	}

	innerY += sponsorPadY
	boxH := innerY - boxTop
	out := fmt.Sprintf(
		`<g class="sponsors goal"><rect x="%d" y="%d" width="%d" height="%d" rx="5" ry="5" fill=%q fill-opacity="0.12"/>%s</g>`,
		int(sponsorInset), int(boxTop), int(sponsorBoxW), int(boxH), sponsorGoalBG, inner.String())
	return out, (innerY - top) + 2
}

// progressBar renders the sponsorship goal bar (filled #ec6cb9 over a
// #d1d5da track) as a positioned nested `<svg>` masked to rounded ends.
func progressBar(x, y, w float64, progress int) string {
	filled := float64(progress) / 100.0 * w
	return fmt.Sprintf(
		`<svg class="bar" x="%d" y="%d" width="%d" height="8" role="img" aria-label="Sponsorship goal progress"><title>Sponsorship goal progress</title><mask id="sponsors-goal-bar"><rect x="0" y="0" width="%d" height="8" fill="white" rx="5"/></mask><rect mask="url(#sponsors-goal-bar)" x="0" y="0" width="%d" height="8" fill="#d1d5da"/><rect mask="url(#sponsors-goal-bar)" x="0" y="0" width="%.2f" height="8" fill="#ec6cb9"/></svg>`,
		int(x), int(y), int(w), int(w), int(w), filled)
}

// renderAboutSection renders the "about" bio: an "About Me" sub-header
// and the markdown body. Returns the markup and the height consumed.
func renderAboutSection(r *Result, top float64) (string, float64) {
	contentW := float64(chrome.CardWidth) - 2*sponsorInset
	y := top + 4
	var inner strings.Builder
	// "About Me" sub-header (h2, no octicon).
	inner.WriteString(chrome.SVGText(sponsorInset, y+18, "About Me",
		chrome.SVGTextOpts{Size: 16, Fill: "#0366d6"}))
	y += 26
	if r.About != "" {
		inner.WriteString(`<g class="markdown">`)
		m, dh := renderMarkdownSVG(r.About, sponsorInset, y, contentW)
		inner.WriteString(m)
		inner.WriteString(`</g>`)
		y += dh
	} else {
		inner.WriteString(`<g class="markdown"></g>`)
	}
	return fmt.Sprintf(`<g class="sponsors">%s</g>`, inner.String()), (y - top) + 2
}
