package achievements

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// trophyOcticon is the upstream `<%- octicon "trophy" %>` 16x16 path used
// in the achievements section header (EJS line 4).
const trophyOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M3.217 6.962A3.75 3.75 0 010 3.25v-.5C0 1.784.784 1 1.75 1h1.356c.228-.585.796-1 1.462-1h6.864a1.57 1.57 0 011.462 1h1.356c.966 0 1.75.784 1.75 1.75v.5a3.75 3.75 0 01-3.217 3.712 5.014 5.014 0 01-2.771 3.117l.144 1.446c.005.05.03.12.114.204.086.087.217.17.373.227.283.103.618.274.89.568.285.31.467.723.467 1.226v.75h1.25a.75.75 0 110 1.5H2.75a.75.75 0 010-1.5H4v-.75c0-.503.182-.916.468-1.226.27-.294.606-.465.889-.568a1.03 1.03 0 00.373-.227c.084-.085.109-.153.114-.204l.144-1.446a5.014 5.014 0 01-2.77-3.117zM3 2.5H1.75a.25.25 0 00-.25.25v.5c0 .98.626 1.813 1.5 2.122V2.5zm4.457 7.97l-.12 1.204c-.093.925-.858 1.47-1.467 1.691a.764.764 0 00-.3.176c-.037.04-.07.093-.07.21v.75h5v-.75c0-.117-.033-.17-.07-.21a.763.763 0 00-.3-.176c-.609-.221-1.374-.766-1.466-1.69l-.12-1.204a5.052 5.052 0 01-1.087 0zM13 5.373V2.5h1.25a.25.25 0 01.25.25v.5A2.25 2.25 0 0113 5.372zM4.5 1.568c0-.037.03-.068.068-.068h6.864c.037 0 .068.03.068.068V5.5a3.5 3.5 0 11-7 0V1.568z"></path></svg>`

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

// Partial renders the classic SVG fragment for the achievements plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/achievements.ejs.
//
// Output structure:
//
//	<section data-section="achievements">
//	  <h2 class="field"><svg trophy/>Achievements</h2>
//	  <div class="row">
//	    <section class="achievements largeable-flex-wrap">
//	      [for each achievement]:
//	        <div class="achievement ${rank-class} largeable-width-half">
//	          <div class="icon"><svg/>...</div>
//	          <div class="info">
//	            <div class="title">${title}</div>
//	            <div class="text">${description}</div>
//	          </div>
//	        </div>
//	    </section>
//	  </div>
//	</section>
//
// The upstream icon is a per-achievement inline SVG path (`<%- icon %>`
// unescaped); our data model only carries the icon NAME (`Icon` field).
// Until the achievements package ships the per-rank SVG paths, the
// `<div class="icon">` block holds the trophy octicon as a placeholder
// (still renders, no bare elements).
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.List) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="achievements">`)
	fmt.Fprintf(&b, `<h2 class="field">%sAchievements</h2>`, trophyOcticon)
	b.WriteString(`<div class="row">`)
	b.WriteString(`<section class="achievements largeable-flex-wrap">`)
	for _, a := range r.List {
		fmt.Fprintf(
			&b,
			`<div class="achievement %s largeable-width-half" data-rank="%s" data-icon="%s">`,
			partials.EscapeXML(rankClass(a.Rank)),
			partials.EscapeXML(a.Rank),
			partials.EscapeXML(a.Icon),
		)
		// Icon — trophy octicon placeholder until per-achievement icons
		// land. Rendering as inline SVG (not the data-octicon placeholder
		// that doesn't resolve in our pipeline).
		fmt.Fprintf(&b, `<div class="icon">%s</div>`, trophyOcticon)
		// Info block: title + text.
		b.WriteString(`<div class="info">`)
		fmt.Fprintf(
			&b,
			`<div class="title"><span class="prefix">%s</span><span class="value">%d</span></div>`,
			partials.EscapeXML(a.Title),
			a.Value,
		)
		if a.Description != "" {
			fmt.Fprintf(
				&b,
				`<div class="text">%s</div>`,
				partials.EscapeXML(a.Description),
			)
		}
		b.WriteString(`</div>`) // .info
		b.WriteString(`</div>`) // .achievement
	}
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
