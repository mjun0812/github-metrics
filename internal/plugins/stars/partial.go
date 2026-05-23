package stars

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

// starOcticon is the upstream `<%- octicon "star" %>` 16x16 path used
// in the stars section header (EJS line 4).
const starOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// repoOcticon is the upstream `<%- octicon "repo" %>` icon used as the
// per-repository row icon (EJS line 16 — non-fork branch).
const repoOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2 2.5A2.5 2.5 0 014.5 0h8.75a.75.75 0 01.75.75v12.5a.75.75 0 01-.75.75h-2.5a.75.75 0 110-1.5h1.75v-2h-8a1 1 0 00-.714 1.7.75.75 0 01-1.072 1.05A2.495 2.495 0 012 11.5v-9zm10.5-1V9h-8c-.356 0-.694.074-1 .208V2.5a1 1 0 011-1h8zM5 12.25v3.25a.25.25 0 00.4.2l1.45-1.087a.25.25 0 01.3 0L8.6 15.7a.25.25 0 00.4-.2v-3.25a.25.25 0 00-.25-.25h-3.5a.25.25 0 00-.25.25z"></path></svg>`

// rowStarOcticon is the smaller star icon used on per-row star counts.
const rowStarOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25z"></path></svg>`

// Partial renders the classic SVG fragment for the stars plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/stars.ejs.
//
// Output structure:
//
//	<section data-section="stars">
//	  <h2 class="field"><svg star/>Recently starred repositories</h2>
//	  <div class="row">
//	    <section class="largeable-flex-wrap">
//	      [for each repo]:
//	        <div class="row fill-width largeable-width-half">
//	          <section class="repository">
//	            <div class="field"><svg repo/><div class="name">
//	              <span>${nameWithOwner}</span>
//	              <span>starred ${date}</span>
//	            </div></div>
//	            <div class="field description">${description}</div>
//	            <div class="field infos">
//	              <div><svg star/>${stars}</div>
//	            </div>
//	          </section>
//	        </div>
//	    </section>
//	  </div>
//	</section>
//
// Upstream's full info row (language color, license, forks, issues, PRs)
// is omitted because our StarredRepo data model carries only
// NameWithOwner/Description/Stars/StarredAt — extending it would need a
// follow-up GraphQL fragment.
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
	b.WriteString(`<section data-section="stars">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sRecently starred repositories</h2>`,
		starOcticon,
	)
	b.WriteString(`<div class="row"><section class="largeable-flex-wrap">`)
	for _, s := range r.List {
		date := ""
		if !s.StarredAt.IsZero() {
			date = s.StarredAt.UTC().Format("2006-01-02")
		}
		b.WriteString(`<div class="row fill-width largeable-width-half">`)
		fmt.Fprintf(
			&b,
			`<section class="repository" data-stars="%d"><div class="field">%s<div class="name"><span>%s</span>`,
			s.Stars, repoOcticon, partials.EscapeXML(s.NameWithOwner),
		)
		if date != "" {
			fmt.Fprintf(&b, `<span>starred %s</span>`, partials.EscapeXML(date))
		}
		b.WriteString(`</div></div>`)
		if s.Description != "" {
			fmt.Fprintf(
				&b,
				`<div class="field description">%s</div>`,
				partials.EscapeXML(s.Description),
			)
		}
		fmt.Fprintf(
			&b,
			`<div class="field infos"><div>%s%d</div></div>`,
			rowStarOcticon, s.Stars,
		)
		b.WriteString(`</section></div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
