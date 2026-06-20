package reactions

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
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
// the same display order.
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

// Partial renders the classic SVG fragment for the reactions plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/reactions.ejs.
//
// It always renders the 8-category emoji gauge panel: one
// `<div class="category column">` per reaction with a `<svg class="gauge
// info">` circle (plus a `gauge-arc` when score > 0), the emoji as a
// `<text>` inside the gauge, and an optional `title nowrap` detail span
// driven by plugin_reactions_details.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="reactions">`)
	// Header — "Overall users reactions from last N comments".
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sOverall users reactions from last %d comment%s</h2>`,
		commentDiscussionOcticon, r.Comments, pluginutil.Plural(r.Comments),
	)
	b.WriteString(`<div class="row"><section>`)
	b.WriteString(`<div class="row fill-width"><section class="categories">`)
	for _, entry := range reactionEmojis {
		writeReactionCategory(&b, entry.key, entry.emoji, r.List[entry.key], r.Details)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeReactionCategory emits one `<div class="category column">` gauge
// for a single reaction content, mirroring EJS lines 18-54.
func writeReactionCategory(b *strings.Builder, key, emoji string, react Reaction, details []string) {
	fmt.Fprintf(b, `<div class="category column" data-reaction="%s">`, partials.EscapeXML(key))
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" width="50" height="50" class="gauge info">`)
	b.WriteString(`<circle class="gauge-base" r="53" cx="60" cy="60"></circle>`)
	if react.Score > 0 {
		fmt.Fprintf(
			b,
			`<circle class="gauge-arc" transform="rotate(-90 60 60)" r="53" cx="60" cy="60" stroke-dasharray="%s 329"></circle>`,
			formatDash(react.Score*329),
		)
	}
	fmt.Fprintf(b, `<text x="60" y="60" dominant-baseline="central">%s</text>`, emoji)
	b.WriteString(`</svg>`)

	if len(details) > 0 {
		b.WriteString(`<span class="title nowrap">`)
		writeDetail(b, details[0], react)
		if len(details) > 1 {
			b.WriteString(`<small>(`)
			writeDetail(b, details[1], react)
			b.WriteString(`)</small>`)
		}
		b.WriteString(`</span>`)
	}
	b.WriteString(`</div>`)
}

// writeDetail renders a single detail field (count or percentage)
// mirroring EJS lines 38-49.
func writeDetail(b *strings.Builder, kind string, react Reaction) {
	switch kind {
	case "count":
		fmt.Fprintf(b, `%d`, react.Value)
	case "percentage":
		fmt.Fprintf(b, `%d<small>%%</small>`, int(math.Round(react.Score*100)))
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
