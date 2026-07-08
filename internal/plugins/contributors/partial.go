package contributors

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// contributorsOcticon is the upstream `<%- octicon "people" %>` 16x16
// path used in the contributors section header.
const contributorsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5.5 3.5a2 2 0 100 4 2 2 0 000-4zM2 5.5a3.5 3.5 0 115.898 2.549 5.507 5.507 0 013.034 4.084.75.75 0 11-1.482.235 4.001 4.001 0 00-7.9 0 .75.75 0 01-1.482-.236A5.507 5.507 0 013.102 8.05 3.49 3.49 0 012 5.5zM11 4a.75.75 0 100 1.5 1.5 1.5 0 01.666 2.844.75.75 0 00-.416.672v.352a.75.75 0 00.574.73c1.2.289 2.162 1.2 2.522 2.372a.75.75 0 101.434-.44 5.01 5.01 0 00-2.56-3.012A3 3 0 0011 4z"></path></svg>`

// commitsOcticon is the upstream `<%- octicon "git-commit" %>` path
// inlined inside each chip's `.contributions` badge, mirroring
// `org_repo/source/templates/repository/partials/contributors.ejs:30`.
// It renders at ~0.8rem (11px, matching `.contributors .contributions
// svg { width/height: .8rem }`) via width/height on the otherwise 16x16
// viewBox.
const commitsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`

// Contributor-chip geometry. Each chip is a `.label` pill: a light-blue
// rounded rect the height of one 22px line, with the 22px circular avatar
// filling its rounded left cap (padding-left:0 in `.contributors .label`)
// and the login in 12px GitHub-blue. In contributions mode a darker
// `.contributions` sub-pill (commit count + git-commit octicon) and a
// `.label-right` diff sub-pill (`++A --D`) trail the login.
const (
	contribAvatar   = 22.0 // upstream contributors.ejs hard-codes 22x22
	contribChipH    = 22.0 // `.label { line-height: 22px }`
	contribInset    = 6.0  // `.contributors { margin-left: 6px }`
	contribChipMarX = 5.0  // `.label { margin: 2px 5px }` horizontal
	contribChipMarY = 2.0  // `.label { margin: 2px 5px }` vertical
	contribAvGap    = 6.0  // `.avatar { margin: 0 6px }` right side
	contribPadR     = 10.0 // `.label { padding: 0 10px }` right side
	contribLoginF   = 12.0 // `.label { font-size: 12px }`
	contribLoginC   = "#0366D6"
	contribChipBG   = "#58A6FF"
	contribChipBGOp = "0.19" // 0x30 / 255

	// `.contributions` / `.label-right` sub-pills: 0.7rem text on a very
	// light blue (`#58A6FF10`) rounded rect; the octicon is 0.8rem blue.
	contribBadgeF    = 10.0 // ~0.7rem
	contribBadgeBG   = "#58A6FF"
	contribBadgeBGOp = "0.0625" // 0x10 / 255
	contribBadgePadX = 7.0      // `padding: 0 7px`
	contribBadgeIcon = 11.0     // ~0.8rem git-commit octicon
	contribBadgeGap  = 4.0      // octicon `margin-left: 4px`
	contribBadgeC    = "#0366D6"

	contribChipPitch = contribChipH + 2*contribChipMarY
	contribBaseline  = 0.32 // baseline offset ratio (mirrors chrome.baselineRatio)
)

// Partial renders the classic SVG fragment for the contributors plugin.
// Upstream classic does not ship a contributors.ejs — contributors is
// rendered only in the repository template — so we emit a self-contained
// section with a header + a wrapping flow of per-contributor chips.
//
// Returns "" until contributors.go's Run wires up data (M7 repo-mode
// is the only path that currently populates List; user/org modes stay
// Skipped).
//
// Output (native SVG, #409 Phase B3): a `<section
// data-section="contributors">` anchor wrapping a nested `<svg>` with a
// section header ("Contributors of ${base}") above `<g data-login>` chips
// laid out left-to-right and wrapped. Each chip is a `.label` pill —
// circular avatar `<image>` + login `<text>` — and, in contributions
// mode, a `.contributions` badge (commit count + git-commit octicon) plus
// a `.label-right` diff badge (`++A --D`). The partial reports its
// consumed pixel height via the Phase B0 API.
//
// adds/dels stay in a `.label-right` badge because upstream's
// contributors_contributions toggle is not adopted yet; surfacing them as
// a separate badge keeps the `.contributions` badge unchanged.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.List) == 0 {
		return "", 0, nil
	}

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(contributorsOcticon, "Contributors of "+r.Base)
	body.WriteString(header)

	showDiff := r.Contributions && !r.StatsPending
	maxRight := float64(chrome.CardWidth) - contribInset

	cx := contribInset + contribChipMarX
	rowTop := y + contribChipMarY
	rows := 1
	for i, c := range r.List {
		w := chipWidth(c, r.Contributions, showDiff)
		if cx+w > maxRight && cx > contribInset+contribChipMarX {
			cx = contribInset + contribChipMarX
			rowTop += contribChipPitch
			rows++
		}
		fmt.Fprintf(&body, `<g data-login="%s" transform="translate(%d,%d)">%s</g>`,
			partials.EscapeXML(c.Login), int(cx), int(rowTop),
			chipMarkup(c, i, r.Contributions, showDiff))
		cx += w + 2*contribChipMarX
	}

	height := int(y) + rows*contribChipPitch + contribChipMarY
	return chrome.WrapSection("contributors", height, body.String()), height, nil
}

// badgeText returns the "++A --D" diff label for a contributor.
func badgeText(c Contributor) string {
	return fmt.Sprintf("++%d --%d", c.Additions, c.Deletions)
}

// chipWidth computes the pixel width one contributor chip occupies,
// mirroring chipMarkup's cursor math so the flow can wrap before drawing.
func chipWidth(c Contributor, contributions, showDiff bool) float64 {
	x := contribAvatar + contribAvGap + fontmetrics.Width(c.Login, contribLoginF)
	if !contributions {
		return x + contribPadR
	}
	x += contribBadgeGap + contributionsBadgeWidth(c)
	if showDiff {
		x += contribBadgeGap + diffBadgeWidth(c)
	}
	return x + contribBadgePadX
}

func contributionsBadgeWidth(c Contributor) float64 {
	count := fmt.Sprintf("%d", c.Commits)
	return 2*contribBadgePadX + fontmetrics.Width(count, contribBadgeF) + contribBadgeGap + contribBadgeIcon
}

func diffBadgeWidth(c Contributor) float64 {
	return 2*contribBadgePadX + fontmetrics.Width(badgeText(c), contribBadgeF)
}

// chipMarkup renders one contributor chip's inner SVG at a local origin
// (the enclosing `<g data-login transform>` positions it). It draws the
// pill background, the circular avatar, the login, and — in contributions
// mode — the `.contributions` and `.label-right` sub-pills.
func chipMarkup(c Contributor, idx int, contributions, showDiff bool) string {
	chipW := chipWidth(c, contributions, showDiff)
	baseline := contribChipH/2 + contribLoginF*contribBaseline

	var b strings.Builder
	fmt.Fprintf(&b,
		`<rect x="0" y="0" width="%d" height="%d" rx="%d" ry="%d" fill=%q fill-opacity=%q/>`,
		int(chipW), int(contribChipH), int(contribChipH/2), int(contribChipH/2), contribChipBG, contribChipBGOp)

	if c.AvatarURL != "" {
		b.WriteString(chrome.SVGAvatar(0, 0, contribAvatar, c.AvatarURL,
			fmt.Sprintf("contributor-av-%d", idx), true))
	}

	loginX := contribAvatar + contribAvGap
	b.WriteString(chrome.SVGText(loginX, baseline, c.Login,
		chrome.SVGTextOpts{Size: contribLoginF, Fill: contribLoginC}))

	if !contributions {
		return b.String()
	}

	x := loginX + fontmetrics.Width(c.Login, contribLoginF) + contribBadgeGap
	b.WriteString(badgeMarkup(x, contributionsBadgeWidth(c), func(inner *strings.Builder) {
		count := fmt.Sprintf("%d", c.Commits)
		inner.WriteString(chrome.SVGText(x+contribBadgePadX, baseline, count,
			chrome.SVGTextOpts{Size: contribBadgeF, Fill: contribBadgeC}))
		iconX := x + contribBadgePadX + fontmetrics.Width(count, contribBadgeF) + contribBadgeGap
		iconY := (contribChipH - contribBadgeIcon) / 2
		inner.WriteString(chrome.SVGIcon(iconX, iconY, contribBadgeC, commitsOcticon))
	}, "contributions"))
	x += contributionsBadgeWidth(c) + contribBadgeGap

	if showDiff {
		b.WriteString(badgeMarkup(x, diffBadgeWidth(c), func(inner *strings.Builder) {
			inner.WriteString(chrome.SVGText(x+contribBadgePadX, baseline, badgeText(c),
				chrome.SVGTextOpts{Size: contribBadgeF, Fill: contribBadgeC}))
		}, "label-right"))
	}

	return b.String()
}

// badgeMarkup wraps a sub-pill: a very light blue rounded rect at x of
// the given width plus the body drawn by fill, tagged with class so the
// `.contributions` / `.label-right` DOM hooks survive.
func badgeMarkup(x, w float64, fill func(*strings.Builder), class string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<g class=%q>`, class)
	fmt.Fprintf(&b,
		`<rect x="%d" y="0" width="%d" height="%d" rx="%d" ry="%d" fill=%q fill-opacity=%q/>`,
		int(x), int(w), int(contribChipH), int(contribChipH/2), int(contribChipH/2), contribBadgeBG, contribBadgeBGOp)
	fill(&b)
	b.WriteString(`</g>`)
	return b.String()
}
