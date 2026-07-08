package achievements

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// trophyOcticonPath is the upstream `<%- octicon "trophy" %>` path data.
// Shared between the section header SVG (rendered 16x16 inline with the
// `<h2>`) and the per-achievement badge SVG (rendered 44x44 to fill the
// `.icon` CSS box).
const trophyOcticonPath = `<path fill-rule="evenodd" d="M3.217 6.962A3.75 3.75 0 010 3.25v-.5C0 1.784.784 1 1.75 1h1.356c.228-.585.796-1 1.462-1h6.864a1.57 1.57 0 011.462 1h1.356c.966 0 1.75.784 1.75 1.75v.5a3.75 3.75 0 01-3.217 3.712 5.014 5.014 0 01-2.771 3.117l.144 1.446c.005.05.03.12.114.204.086.087.217.17.373.227.283.103.618.274.89.568.285.31.467.723.467 1.226v.75h1.25a.75.75 0 110 1.5H2.75a.75.75 0 010-1.5H4v-.75c0-.503.182-.916.468-1.226.27-.294.606-.465.889-.568a1.03 1.03 0 00.373-.227c.084-.085.109-.153.114-.204l.144-1.446a5.014 5.014 0 01-2.77-3.117zM3 2.5H1.75a.25.25 0 00-.25.25v.5c0 .98.626 1.813 1.5 2.122V2.5zm4.457 7.97l-.12 1.204c-.093.925-.858 1.47-1.467 1.691a.764.764 0 00-.3.176c-.037.04-.07.093-.07.21v.75h5v-.75c0-.117-.033-.17-.07-.21a.763.763 0 00-.3-.176c-.609-.221-1.374-.766-1.466-1.69l-.12-1.204a5.052 5.052 0 01-1.087 0zM13 5.373V2.5h1.25a.25.25 0 01.25.25v.5A2.25 2.25 0 0113 5.372zM4.5 1.568c0-.037.03-.068.068-.068h6.864c.037 0 .068.03.068.068V5.5a3.5 3.5 0 11-7 0V1.568z"></path>`

// trophyHeaderSVG is the inline 16x16 trophy used in the section header
// `<h2>` (EJS line 4 — upstream renders `<%- octicon "trophy" %>` inline
// with the heading text).
const trophyHeaderSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">` + trophyOcticonPath + `</svg>`

// trophyBadgeSVG is the per-achievement fallback badge. iconForAchievement
// uses this when an achievement id is not registered in iconsByID (e.g.,
// a custom rank table entry without a known upstream icon). Sized 44x44
// to fill the CSS `.achievement .icon` box (Issue #554).
const trophyBadgeSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="44" height="44" aria-hidden="true">` + trophyOcticonPath + `</svg>`

// iconViewBox / iconSize control the per-badge SVG wrapper. Upstream's
// inline icon fragments are authored against a 60x60 viewBox; we
// render them at 44x44 to match the `.achievement .icon` CSS box.
const (
	iconViewBox = "0 0 60 60"
	iconSize    = "44"
)

// iconForAchievement returns the per-badge SVG for the given
// achievement. It wraps the upstream `<g>` fragment in a sized
// `<svg>` and substitutes the rank's hex pair into the `#primary`
// / `#secondary` placeholders. Unknown ids fall back to the trophy
// badge so the slot never renders empty.
func iconForAchievement(a Achievement) string {
	frag, ok := iconsByID[a.ID]
	if !ok {
		return trophyBadgeSVG
	}
	colors, ok := rankColors[a.Rank]
	if !ok {
		// Unknown rank — fall back to the neutral X palette so we
		// never leave the literal "#primary" / "#secondary" tokens
		// in the output.
		colors = rankColors["X"]
	}
	tinted := strings.ReplaceAll(frag, "#primary", colors[0])
	tinted = strings.ReplaceAll(tinted, "#secondary", colors[1])
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="` + iconViewBox +
		`" width="` + iconSize + `" height="` + iconSize +
		`" aria-hidden="true">` + tinted + `</svg>`
}

// rankClass maps the upstream rank tokens (S/A/B/C/X/$) to the CSS
// class suffix the upstream EJS emits (`rank.charAt(0).toLocaleLowerCase()`
// or `secret` for "$"). EJS line 14.
func rankClass(rank string) string {
	if rank == "$" {
		return "secret"
	}
	if rank == "" {
		return ""
	}
	return strings.ToLower(rank[:1])
}

// Achievement-card geometry / colors, mirroring the `.achievement` CSS.
const (
	achMargin      = 4.0  // `.achievement { margin: 4px 0 }`
	achIconMargin  = 4.0  // `.achievement .icon { margin: 0 4px }`
	achIconSize    = 44.0 // `.achievement .icon { width/height: 44px }`
	achTitleFont   = 14.0 // `.achievement .title { font-size: 14px }`
	achTextFont    = 12.0 // `.achievement .text { font-size: 12px }`
	achTextFill    = "#666666"
	achValueFont   = 10.0 // `.achievement .value { font-size: 10px }`
	achValuePadX   = 5.0  // `.achievement .value { padding: 0 5px }`
	achValueHeight = 16.0
	achValueBgOpac = "0.15" // 0x26 / 255
	achValueGap    = 46.0   // `.achievement .value { margin-left: 46px }`
	achPrefixFont  = 10.0   // compact `.title .prefix { font-size: 10px }`
	achCompactCell = 80.0   // compact `.achievement { width: 80px }`
	baselineFrac   = 0.32
)

// achRankColors maps the rank class (rankClass output) to its
// [titleColor, valueBgColor] pair, mirroring the per-rank `.achievement`
// CSS. titleColor sets the title text + the value pill border/text;
// valueBgColor is the translucent value-pill background. Emitted as
// literal fills because resvg does not resolve the class chain.
var achRankColors = map[string][2]string{
	"":       {"#58A6FF", "#58A6FF"},
	"x":      {"#666666", "#B0B0B0"},
	"b":      {"#9D8FFF", "#9E91FF"},
	"a":      {"#D79533", "#E7BD69"},
	"s":      {"#EB355E", "#EB355E"},
	"secret": {"#FF76CD", "#FF79D1"},
}

// achColors returns the title/value colors for a rank token.
func achColors(rank string) (title, valueBg string) {
	c, ok := achRankColors[rankClass(rank)]
	if !ok {
		c = achRankColors[""]
	}
	return c[0], c[1]
}

// ai formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func ai(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// achTitle builds the achievement title string, applying the upstream
// EJS line 35 rule: when a rank prefix is set the title is lowercased and
// the prefix is prepended.
func achTitle(a Achievement) string {
	prefix := rankPrefixes[a.Rank]
	if prefix != "" {
		return prefix + " " + strings.ToLower(a.Title)
	}
	return a.Title
}

// Partial renders the classic SVG fragment for the achievements plugin
// as native SVG (#409 Phase B6). Mirrors upstream
// org_repo/source/templates/classic/partials/achievements.ejs.
//
// Output (native SVG): a `<section data-section="achievements">` anchor
// wrapping a nested `<svg>` with the trophy section header above the
// achievement badges. Two layouts:
//
//   - detailed: a vertical list of horizontal cards (44px rank icon on
//     the left; title + value pill + description on the right);
//   - compact: an 80px-wide badge grid (centered icon with an overlapping
//     value pill and the centered prefix/title below), wrapping across
//     rows.
//
// Each badge keeps its `class="achievement <rank>"` / `data-rank` /
// `data-icon` DOM hooks, and the partial reports the pixel height it
// consumes.
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

	header, hh := chrome.SVGSectionHeader(trophyHeaderSVG, "Achievements")

	var body strings.Builder
	body.WriteString(header)

	var consumed float64
	if r.Display == displayCompact {
		consumed = renderCompact(&body, r.List, hh)
	} else {
		consumed = renderDetailed(&body, r.List, hh)
	}

	height := int(math.Round(hh + consumed))
	return chrome.WrapSection("achievements", height, body.String()), height, nil
}

// renderDetailed lays the achievements out as a vertical list of
// horizontal cards and returns the total height consumed below top.
func renderDetailed(body *strings.Builder, list []Achievement, top float64) float64 {
	const (
		descWidth = 380.0
		descTop   = 2.0
	)
	y := top
	for _, a := range list {
		y += achMargin
		titleColor, valueBg := achColors(a.Rank)

		iconX, iconY := achIconMargin, y
		infoX := iconX + achIconSize + achIconMargin // 52

		var card strings.Builder
		card.WriteString(chrome.SVGIcon(iconX, iconY, "", iconForAchievement(a)))

		title := achTitle(a)
		titleBaseline := iconY + achTitleFont + 2
		card.WriteString(chrome.SVGText(infoX, titleBaseline, title,
			chrome.SVGTextOpts{Size: achTitleFont, Fill: titleColor}))

		// Value pill after the title (`.value { margin-left: 46px }`),
		// vertically centered on the title line.
		titleW := fontmetrics.Width(title, achTitleFont)
		pillX := infoX + titleW + achValueGap
		lineCenter := titleBaseline - achTitleFont*baselineFrac
		card.WriteString(achValuePill(pillX, lineCenter-achValueHeight/2,
			strconv.Itoa(a.Value), titleColor, valueBg))

		infoBottom := titleBaseline
		if a.Description != "" {
			m, dh := chrome.SVGParagraph(infoX, titleBaseline+descTop, descWidth, achTextFont, achTextFill, a.Description)
			card.WriteString(m)
			infoBottom = titleBaseline + descTop + dh
		}

		cardBottom := iconY + achIconSize
		if infoBottom > cardBottom {
			cardBottom = infoBottom
		}
		y = cardBottom + achMargin

		writeAchievementGroup(body, a, card.String())
	}
	return y - top
}

// renderCompact lays the achievements out as an 80px-wide badge grid and
// returns the total height consumed below top.
func renderCompact(body *strings.Builder, list []Achievement, top float64) float64 {
	perRow := int(chrome.CardWidth / achCompactCell)
	if perRow < 1 {
		perRow = 1
	}
	// Per-cell vertical budget: top margin + icon + prefix line + title
	// line + bottom margin.
	const cellHeight = achMargin + achIconSize + achPrefixFont + achTitleFont + 8

	for i, a := range list {
		col := i % perRow
		row := i / perRow
		cellX := float64(col) * achCompactCell
		cellTop := top + float64(row)*cellHeight
		cx := cellX + achCompactCell/2
		titleColor, valueBg := achColors(a.Rank)

		var card strings.Builder
		iconX := cellX + (achCompactCell-achIconSize)/2
		iconY := cellTop + achMargin
		card.WriteString(chrome.SVGIcon(iconX, iconY, "", iconForAchievement(a)))

		// Value pill overlapping the lower part of the icon, centered
		// (`.value-wrapper { margin-top: 36px }`).
		pillText := strconv.Itoa(a.Value)
		pillW := fontmetrics.Width(pillText, achValueFont) + 2*achValuePadX
		card.WriteString(achValuePill(cx-pillW/2, iconY+36, pillText, titleColor, valueBg))

		// Prefix (block above the title) + capitalized title, both
		// centered below the icon.
		labelBaseline := iconY + achIconSize + achPrefixFont
		prefix := rankPrefixes[a.Rank]
		title := a.Title
		if prefix != "" {
			card.WriteString(chrome.SVGText(cx, labelBaseline, prefix,
				chrome.SVGTextOpts{Size: achPrefixFont, Fill: titleColor, Anchor: "middle"}))
			title = strings.ToLower(a.Title)
		}
		titleBaseline := labelBaseline + achTitleFont
		card.WriteString(chrome.SVGText(cx, titleBaseline, capitalize(title),
			chrome.SVGTextOpts{Size: achTitleFont, Fill: titleColor, Anchor: "middle", MaxWidth: achCompactCell}))

		writeAchievementGroup(body, a, card.String())
	}

	rows := (len(list) + perRow - 1) / perRow
	return float64(rows) * cellHeight
}

// writeAchievementGroup wraps one badge's native markup in the
// `<g class="achievement <rank>">` anchor that carries the upstream
// data-rank / data-icon DOM hooks.
func writeAchievementGroup(body *strings.Builder, a Achievement, card string) {
	fmt.Fprintf(
		body,
		`<g class="achievement %s" data-rank="%s" data-icon="%s">%s</g>`,
		partials.EscapeXML(rankClass(a.Rank)),
		partials.EscapeXML(a.Rank),
		partials.EscapeXML(a.Icon),
		card,
	)
}

// achValuePill renders one `.value` pill: a fully-rounded rect whose
// top-left is (x, top), filled with the translucent rank color and
// bordered/labeled in the title color. Returns the markup.
func achValuePill(x, top float64, text, titleColor, bgColor string) string {
	w := fontmetrics.Width(text, achValueFont) + 2*achValuePadX
	baseline := top + achValueHeight/2 + achValueFont*baselineFrac
	var b strings.Builder
	fmt.Fprintf(&b,
		`<rect x="%s" y="%s" width="%s" height="%d" rx="%d" ry="%d" fill=%q fill-opacity=%q stroke=%q/>`,
		ai(x), ai(top), ai(w), int(achValueHeight), int(achValueHeight/2), int(achValueHeight/2),
		bgColor, achValueBgOpac, titleColor)
	b.WriteString(chrome.SVGText(x+achValuePadX, baseline, text,
		chrome.SVGTextOpts{Size: achValueFont, Fill: titleColor}))
	return b.String()
}

// capitalize upper-cases the first rune, matching the compact
// `.title { text-transform: capitalize }` rule for the leading word.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
