package reactions

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// commentDiscussionOcticon is the upstream `<%- octicon "comment-discussion" %>`
// 16x16 path used in the reactions section header (EJS line 4).
const commentDiscussionOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 2.75a.25.25 0 01.25-.25h8.5a.25.25 0 01.25.25v5.5a.25.25 0 01-.25.25h-3.5a.75.75 0 00-.53.22L3.5 11.44V9.25a.75.75 0 00-.75-.75h-1a.25.25 0 01-.25-.25v-5.5zM1.75 1A1.75 1.75 0 000 2.75v5.5C0 9.216.784 10 1.75 10H2v1.543a1.457 1.457 0 002.487 1.03L7.061 10h3.189A1.75 1.75 0 0012 8.25v-5.5A1.75 1.75 0 0010.25 1h-8.5zM14.5 4.75a.25.25 0 00-.25-.25h-.5a.75.75 0 110-1.5h.5c.966 0 1.75.784 1.75 1.75v5.5A1.75 1.75 0 0114.25 12H14v1.543a1.457 1.457 0 01-2.487 1.03L9.22 12.28a.75.75 0 111.06-1.06l2.22 2.22v-2.19a.75.75 0 01.75-.75h1a.25.25 0 00.25-.25v-5.5z"></path></svg>`

// reactionEmojis mirrors upstream's category-to-emoji map (EJS line 19).
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
// Two paths:
//
//  1. When `r.Details` (the per-category breakdown) is populated, render
//     the 8-category emoji panel matching upstream lines 17-50.
//
//  2. Otherwise, fall back to the aggregate-only summary using the
//     {Issues, Comments, Discussions, Total} counts our data model
//     surfaces today: a single field showing "N reactions" along with
//     per-source breakdown. The fallback is HTML inside <section> (no
//     bare <text> outside <svg> — that's the bug 011 set out to fix).
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || r.Total == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="reactions">`)
	// Header — "Overall users reactions from last N comments".
	commentCount := r.Comments
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sOverall users reactions from last %d comment%s</h2>`,
		commentDiscussionOcticon, commentCount, pluralS(commentCount),
	)
	b.WriteString(`<div class="row"><section>`)

	if len(r.Details) > 0 {
		writeReactionCategories(&b, r.Details, r.Total)
	} else {
		// Aggregate fallback — three fields (issues / comments / discussions)
		// + a total.
		b.WriteString(`<div class="row fill-width"><section class="categories">`)
		writeCountField(&b, "Issues", r.Issues)
		writeCountField(&b, "Comments", r.Comments)
		writeCountField(&b, "Discussions", r.Discussions)
		writeCountField(&b, "Total reactions", r.Total)
		b.WriteString(`</section></div>`)
	}

	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeReactionCategories emits the 8-category emoji panel (EJS lines
// 18-50). Each category renders as a `<div class="category column">`
// with the emoji and its count/percentage.
func writeReactionCategories(b *strings.Builder, details map[string]int, total int) {
	if total <= 0 {
		total = 1
	}
	b.WriteString(`<div class="row fill-width"><section class="categories">`)
	for _, entry := range reactionEmojis {
		count := details[entry.key]
		pct := float64(count) / float64(total) * 100
		fmt.Fprintf(
			b,
			`<div class="category column" data-reaction="%s"><div class="emoji">%s</div><span class="title nowrap">%d<small> (%.0f%%)</small></span></div>`,
			partials.EscapeXML(entry.key), entry.emoji, count, pct,
		)
	}
	b.WriteString(`</section></div>`)

	// Detail rows for any extra non-canonical categories present in
	// Details map but not in the 8-category set. Sorted alphabetically
	// for deterministic output.
	extras := make([]string, 0)
	canonical := map[string]bool{}
	for _, e := range reactionEmojis {
		canonical[e.key] = true
	}
	for k := range details {
		if !canonical[k] {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		b.WriteString(`<div class="row fill-width"><section class="categories extras">`)
		for _, k := range extras {
			fmt.Fprintf(
				b,
				`<div class="category column" data-reaction="%s"><span class="title nowrap">%s: %d</span></div>`,
				partials.EscapeXML(k), partials.EscapeXML(k), details[k],
			)
		}
		b.WriteString(`</section></div>`)
	}
}

// writeCountField emits one HTML `<div class="field">` row with a label
// and a count — used by the aggregate fallback when Details is empty.
func writeCountField(b *strings.Builder, label string, n int) {
	fmt.Fprintf(
		b,
		`<div class="field"><span class="label">%s</span><span class="value">%d</span></div>`,
		partials.EscapeXML(label), n,
	)
}

// pluralS mirrors upstream's `s()` helper.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
