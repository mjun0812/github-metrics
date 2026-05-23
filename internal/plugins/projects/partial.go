package projects

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

// projectsOcticon is the upstream `<%- octicon "project" %>` 16x16 path
// used in the count header (EJS line 4).
const projectsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.75 0A1.75 1.75 0 000 1.75v12.5C0 15.216.784 16 1.75 16h12.5A1.75 1.75 0 0016 14.25V1.75A1.75 1.75 0 0014.25 0H1.75zM1.5 1.75a.25.25 0 01.25-.25h12.5a.25.25 0 01.25.25v12.5a.25.25 0 01-.25.25H1.75a.25.25 0 01-.25-.25V1.75zM11.75 3a.75.75 0 00-.75.75v7.5a.75.75 0 001.5 0v-7.5a.75.75 0 00-.75-.75zm-8.25.75a.75.75 0 011.5 0v5.5a.75.75 0 01-1.5 0v-5.5zM8 3a.75.75 0 00-.75.75v3.5a.75.75 0 001.5 0v-3.5A.75.75 0 008 3z"></path></svg>`

// projectRowOcticon is the upstream `<%- octicon "project-roadmap" %>`-style
// project name icon (EJS line 18).
const projectRowOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M0 3.75C0 2.784.784 2 1.75 2h12.5c.966 0 1.75.784 1.75 1.75v8.5A1.75 1.75 0 0114.25 14H1.75A1.75 1.75 0 010 12.25v-8.5zm1.75-.25a.25.25 0 00-.25.25v8.5c0 .138.112.25.25.25h12.5a.25.25 0 00.25-.25v-8.5a.25.25 0 00-.25-.25H1.75zM3.5 6.25a.75.75 0 01.75-.75h7a.75.75 0 010 1.5h-7a.75.75 0 01-.75-.75zm.75 2.25a.75.75 0 000 1.5h4a.75.75 0 000-1.5h-4z"></path></svg>`

// clockOcticon is the upstream `<%- octicon "clock" %>` icon used in the
// "Updated YYYY-MM-DD" line (EJS line 34).
const clockOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0zM8 0a8 8 0 100 16A8 8 0 008 0zm.5 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 00.471.696l2.5 1a.75.75 0 00.557-1.392L8.5 7.742V4.75z"></path></svg>`

// pluralS mirrors upstream's `s()` helper.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Partial renders the classic SVG fragment for the projects plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/projects.ejs
// for the fields the M4 data model exposes (name, description,
// updatedAt, url). Progress bars + items rows are upstream-only until
// the GraphQL fetch lands (see projects.go Run TODO).
//
// Output structure:
//
//	<section data-section="projects">
//	  <h2 class="field"><svg project/>N Project(s)</h2>
//	  <div class="row"><section class="project">
//	    [for each project]:
//	      <div class="row fill-width"><section>
//	        <div class="field"><svg roadmap/>${name}</div>
//	      </section></div>
//	      [if description]:
//	        <div class="row fill-width"><section>
//	          <div class="field description">${description}</div>
//	        </section></div>
//	      <div class="row"><section>
//	        <div class="field"><svg clock/>Updated ${date}</div>
//	      </section></div>
//	  </section></div>
//	</section>
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
	b.WriteString(`<section data-section="projects">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%s%d Project%s</h2>`,
		projectsOcticon, len(r.List), pluralS(len(r.List)),
	)
	b.WriteString(`<div class="row">`)
	b.WriteString(`<section class="project">`)
	for _, p := range r.List {
		// Project name row.
		b.WriteString(`<div class="row fill-width"><section>`)
		fmt.Fprintf(
			&b,
			`<div class="field">%s<a href="%s">%s</a></div>`,
			projectRowOcticon,
			partials.EscapeXML(p.URL),
			partials.EscapeXML(p.Name),
		)
		b.WriteString(`</section></div>`)

		// Description row (when present).
		if p.Description != "" {
			b.WriteString(`<div class="row fill-width"><section>`)
			fmt.Fprintf(
				&b,
				`<div class="field description">%s</div>`,
				partials.EscapeXML(p.Description),
			)
			b.WriteString(`</section></div>`)
		}

		// Updated row.
		updated := ""
		if !p.UpdatedAt.IsZero() {
			updated = p.UpdatedAt.UTC().Format("2006-01-02")
		}
		b.WriteString(`<div class="row"><section>`)
		fmt.Fprintf(
			&b,
			`<div class="field">%sUpdated %s</div>`,
			clockOcticon, partials.EscapeXML(updated),
		)
		b.WriteString(`</section></div>`)
	}
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
