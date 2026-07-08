package reactions

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// commentDiscussionOcticon is the upstream `<%- octicon "comment-discussion" %>`
// 16x16 path used in the reactions section header (EJS line 4).
const commentDiscussionOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 2.75a.25.25 0 01.25-.25h8.5a.25.25 0 01.25.25v5.5a.25.25 0 01-.25.25h-3.5a.75.75 0 00-.53.22L3.5 11.44V9.25a.75.75 0 00-.75-.75h-1a.25.25 0 01-.25-.25v-5.5zM1.75 1A1.75 1.75 0 000 2.75v5.5C0 9.216.784 10 1.75 10H2v1.543a1.457 1.457 0 002.487 1.03L7.061 10h3.189A1.75 1.75 0 0012 8.25v-5.5A1.75 1.75 0 0010.25 1h-8.5zM14.5 4.75a.25.25 0 00-.25-.25h-.5a.75.75 0 110-1.5h.5c.966 0 1.75.784 1.75 1.75v5.5A1.75 1.75 0 0114.25 12H14v1.543a1.457 1.457 0 01-2.487 1.03L9.22 12.28a.75.75 0 111.06-1.06l2.22 2.22v-2.19a.75.75 0 01.75-.75h1a.25.25 0 00.25-.25v-5.5z"></path></svg>`

// reactionEmojis mirrors upstream's category-to-emoji map (EJS line 17).
// Order is upstream-preserved; iteration uses the slice below so we keep
// the same display order. The emoji stay literal Unicode: resvg 0.47
// renders Noto Color Emoji (CBDT) correctly via the Liberation → Noto
// fallback in the production bookworm image (verified 2026-07-08, #689).
var reactionEmojis = []struct {
	key   string
	emoji string
}{
	{"HEART", "❤️"},
	{"THUMBS_UP", "👍"},
	{"THUMBS_DOWN", "👎"},
	{"LAUGH", "😄"},
	{"CONFUSED", "😕"},
	{"EYES", "👀"},
	{"ROCKET", "🚀"},
	{"HOORAY", "🎉"},
}

// Reactions gauge / layout geometry. The gauge stays a nested 120-unit
// viewBox `<svg>` (scaled to 50px) so its inline `stroke-dasharray` final
// value and `animation-gauge` class are preserved verbatim — resvg paints
// the static final arc, browsers still animate (#409 decision log).
const (
	reactGaugeSize   = 50.0
	reactCatMargin   = 4.0 // `.categories { margin-top: 4px }`
	reactGaugeColor  = "#58A6FF"
	reactGaugeStroke = 10.0
	reactEmojiFont   = 40.0

	// Detail (`.title nowrap`) text: base svg 14px #777777 with a smaller
	// (`<small>`-like) 11px run for the "%" / parenthesised segment.
	reactTitleFont  = 14.0
	reactTitleSmall = 11.0
	reactTitleFill  = "#777777"
	reactTitleGap   = 4.0
	reactTitleBand  = 18.0
)

// itoa formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func itoa(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// Partial renders the classic SVG fragment for the reactions plugin as
// native SVG (#409 Phase B4). Mirrors upstream
// org_repo/source/templates/classic/partials/reactions.ejs.
//
// Output (native SVG): a `<section data-section="reactions">` anchor
// wrapping a nested `<svg>`. The comment-discussion section header sits
// above a horizontal row of 8 reaction gauges (`justify-content:
// space-around`), each a `<g data-reaction="...">` holding the 50px info
// gauge (base circle, an arc when score > 0, the emoji as centered
// `<text>`) and an optional centered `title nowrap` detail line driven by
// plugin_reactions_details. Returns the markup and the pixel height it
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
	if !ok || r == nil || r.Skipped {
		return "", 0, nil
	}

	header, hh := chrome.SVGSectionHeader(
		commentDiscussionOcticon,
		fmt.Sprintf("Overall users reactions from last %d comment%s", r.Comments, pluginutil.Plural(r.Comments)),
	)

	gaugeTop := hh + reactCatMargin
	gaugeBottom := gaugeTop + reactGaugeSize
	hasDetails := len(r.Details) > 0
	titleBaseline := gaugeBottom + reactTitleGap + reactTitleFont

	n := len(reactionEmojis)
	colW := float64(chrome.CardWidth) / float64(n)

	var body strings.Builder
	body.WriteString(header)
	for i, entry := range reactionEmojis {
		center := colW*float64(i) + colW/2
		writeReactionCategory(&body, center, gaugeTop, titleBaseline, entry.key, entry.emoji, r.List[entry.key], r.Details)
	}

	height := int(gaugeBottom + reactCatMargin)
	if hasDetails {
		height = int(gaugeBottom + reactTitleGap + reactTitleBand)
	}
	return chrome.WrapSection("reactions", height, body.String()), height, nil
}

// writeReactionCategory emits one `<g data-reaction>` gauge column
// centered on centerX, mirroring EJS lines 18-54. The gauge keeps its
// class names for the browser stylesheet/animation and carries literal
// stroke/fill presentation attributes so resvg (which does not resolve
// the CSS `currentColor` class chain) renders the same blue arc.
func writeReactionCategory(b *strings.Builder, centerX, gaugeTop, titleBaseline float64, key, emoji string, react Reaction, details []string) {
	gx := centerX - reactGaugeSize/2
	fmt.Fprintf(b, `<g data-reaction="%s">`, partials.EscapeXML(key))
	fmt.Fprintf(
		b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" width="50" height="50" x="%s" y="%s" class="gauge info">`,
		itoa(gx), itoa(gaugeTop),
	)
	fmt.Fprintf(
		b,
		`<circle class="gauge-base" r="53" cx="60" cy="60" fill="none" stroke="%s" stroke-width="%d" stroke-opacity="0.2" stroke-linecap="round"></circle>`,
		reactGaugeColor, int(reactGaugeStroke),
	)
	if react.Score > 0 {
		fmt.Fprintf(
			b,
			`<circle class="gauge-arc" transform="rotate(-90 60 60)" r="53" cx="60" cy="60" fill="none" stroke="%s" stroke-width="%d" stroke-linecap="round" stroke-dasharray="%s 329"></circle>`,
			reactGaugeColor, int(reactGaugeStroke), formatDash(react.Score*329),
		)
	}
	fmt.Fprintf(
		b,
		`<text x="60" y="60" dominant-baseline="central" text-anchor="middle" font-size="%d" fill="%s">%s</text>`,
		int(reactEmojiFont), reactGaugeColor, emoji,
	)
	b.WriteString(`</svg>`)

	if len(details) > 0 {
		fmt.Fprintf(
			b,
			`<text x="%s" y="%s" text-anchor="middle" font-size="%d" fill="%s" class="title nowrap">`,
			itoa(centerX), itoa(titleBaseline), int(reactTitleFont), reactTitleFill,
		)
		writeDetail(b, details[0], react)
		if len(details) > 1 {
			b.WriteString(`<tspan font-size="11">(`)
			writeDetail(b, details[1], react)
			b.WriteString(`)</tspan>`)
		}
		b.WriteString(`</text>`)
	}
	b.WriteString(`</g>`)
}

// writeDetail renders a single detail field (count or percentage)
// mirroring EJS lines 38-49. The "%" is a smaller `<tspan>`, standing in
// for the HTML `<small>`.
func writeDetail(b *strings.Builder, kind string, react Reaction) {
	switch kind {
	case "count":
		fmt.Fprintf(b, `%d`, react.Value)
	case "percentage":
		fmt.Fprintf(b, `%d<tspan font-size="11">%%</tspan>`, int(math.Round(react.Score*100)))
	}
}

// formatDash formats a gauge-arc dash length, trimming trailing zeros so
// integral values render as "329" rather than "329.000000" while
// fractional values keep enough precision to be visually distinct.
func formatDash(v float64) string {
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
