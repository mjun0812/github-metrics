package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// reposHeaderOcticon is the upstream `<%- octicon "repo" %>`-style 16x16
// path used in the repositories section header (EJS line 4).
const reposHeaderOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M0 2.75A2.75 2.75 0 012.75 0h10.5A2.75 2.75 0 0116 2.75v10.5A2.75 2.75 0 0113.25 16H2.75A2.75 2.75 0 010 13.25V2.75zM2.75 1.5c-.69 0-1.25.56-1.25 1.25v10.5c0 .69.56 1.25 1.25 1.25h10.5c.69 0 1.25-.56 1.25-1.25V2.75c0-.69-.56-1.25-1.25-1.25H2.75z"></path><path d="M8 4a.75.75 0 01.75.75V6.7l1.69-.975a.75.75 0 01.75 1.3L9.5 8l1.69.976a.75.75 0 01-.75 1.298L8.75 9.3v1.951a.75.75 0 01-1.5 0V9.299l-1.69.976a.75.75 0 01-.75-1.3L6.5 8l-1.69-.975a.75.75 0 01.75-1.3l1.69.976V4.75A.75.75 0 018 4z"></path></svg>`

// repoNonForkOcticon is the per-row icon for non-fork repos (EJS line 16).
const repoNonForkOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2 2.5A2.5 2.5 0 014.5 0h8.75a.75.75 0 01.75.75v12.5a.75.75 0 01-.75.75h-2.5a.75.75 0 110-1.5h1.75v-2h-8a1 1 0 00-.714 1.7.75.75 0 01-1.072 1.05A2.495 2.495 0 012 11.5v-9zm10.5-1V9h-8c-.356 0-.694.074-1 .208V2.5a1 1 0 011-1h8zM5 12.25v3.25a.25.25 0 00.4.2l1.45-1.087a.25.25 0 01.3 0L8.6 15.7a.25.25 0 00.4-.2v-3.25a.25.25 0 00-.25-.25h-3.5a.25.25 0 00-.25.25z"></path></svg>`

// repoForkOcticon is the per-row icon for fork repos (EJS line 14).
const repoForkOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`

// rowStarOcticon11 is the smaller 11x11 star icon used in the infos row.
const rowStarOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25z"></path></svg>`

// rowForkOcticon11 is the smaller 11x11 fork icon used in the infos row.
const rowForkOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`

// langDotOcticon returns the per-repo primary-language color dot icon.
// Falls back to grey when no color is set (matches upstream's `#959DA5`).
func langDotOcticon(color string) string {
	if color == "" {
		color = "#959DA5"
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>`,
		partials.EscapeXML(color),
	)
}

// Partial renders the classic SVG fragment for the repositories plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/repositories.ejs.
//
// Output structure:
//
//	<section data-section="repositories">
//	  <h2 class="field"><svg/>Featured repositories</h2>
//	  <div class="row"><section class="largeable-flex-wrap">
//	    [for each repo]:
//	      <div class="row fill-width largeable-width-half">
//	        <section class="repository">
//	          <div class="field"><svg repo|fork/><div class="name">
//	            <span>${nameWithOwner}</span>
//	          </div></div>
//	          [if description]: <div class="field description">${description}</div>
//	          <div class="field infos">
//	            [if language]: <div class="language"><svg dot/>${name}</div>
//	            <div><svg star/>${stars}</div>
//	            <div><svg fork/>${forks}</div>
//	          </div>
//	        </section>
//	      </div>
//	  </section></div>
//	</section>
//
// License / issues / PRs counts from upstream's full info row are
// omitted — our plugins.Repository data model doesn't currently surface
// those fields (would need a follow-up GraphQL fragment).
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Featured) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="repositories">`)
	fmt.Fprintf(&b, `<h2 class="field">%sFeatured repositories</h2>`, reposHeaderOcticon)
	b.WriteString(`<div class="row"><section class="largeable-flex-wrap">`)
	for _, repo := range r.Featured {
		writeRepoCard(&b, repo)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeRepoCard emits one upstream-equivalent `<section class="repository">`
// card for a single repo. Reused for both the Featured + Pinned + Starred
// + Random sections when Run wires those up.
func writeRepoCard(b *strings.Builder, repo plugins.Repository) {
	b.WriteString(`<div class="row fill-width largeable-width-half">`)
	fmt.Fprintf(
		b,
		`<section class="repository" data-stars="%d" data-forks="%d">`,
		repo.Stars, repo.Forks,
	)
	icon := repoNonForkOcticon
	if repo.IsFork {
		icon = repoForkOcticon
	}
	fmt.Fprintf(
		b,
		`<div class="field">%s<div class="name"><span>`,
		icon,
	)
	if repo.URL != "" {
		fmt.Fprintf(
			b,
			`<a href="%s">%s</a>`,
			partials.EscapeXML(repo.URL),
			partials.EscapeXML(repo.NameWithOwner),
		)
	} else {
		b.WriteString(partials.EscapeXML(repo.NameWithOwner))
	}
	b.WriteString(`</span></div></div>`)
	if repo.Description != "" {
		fmt.Fprintf(
			b,
			`<div class="field description">%s</div>`,
			partials.EscapeXML(repo.Description),
		)
	}
	b.WriteString(`<div class="field infos">`)
	if repo.Language != nil && repo.Language.Name != "" {
		fmt.Fprintf(
			b,
			`<div class="language">%s%s</div>`,
			langDotOcticon(repo.Language.Color),
			partials.EscapeXML(repo.Language.Name),
		)
	}
	fmt.Fprintf(b, `<div>%s%d</div>`, rowStarOcticon11, repo.Stars)
	fmt.Fprintf(b, `<div>%s%d</div>`, rowForkOcticon11, repo.Forks)
	b.WriteString(`</div>`)
	b.WriteString(`</section></div>`)
}
