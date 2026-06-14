package contributors

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

// contributorsOcticon is the upstream `<%- octicon "people" %>` 16x16
// path used in the contributors section header.
const contributorsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5.5 3.5a2 2 0 100 4 2 2 0 000-4zM2 5.5a3.5 3.5 0 115.898 2.549 5.507 5.507 0 013.034 4.084.75.75 0 11-1.482.235 4.001 4.001 0 00-7.9 0 .75.75 0 01-1.482-.236A5.507 5.507 0 013.102 8.05 3.49 3.49 0 012 5.5zM11 4a.75.75 0 100 1.5 1.5 1.5 0 01.666 2.844.75.75 0 00-.416.672v.352a.75.75 0 00.574.73c1.2.289 2.162 1.2 2.522 2.372a.75.75 0 101.434-.44 5.01 5.01 0 00-2.56-3.012A3 3 0 0011 4z"></path></svg>`

// commitsOcticon is the upstream `<%- octicon "git-commit" %>` 16x16 path
// inlined inside each chip's `.contributions` badge, mirroring
// `org_repo/source/templates/repository/partials/contributors.ejs:30`.
const commitsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`

// Partial renders the classic SVG fragment for the contributors plugin.
// Upstream classic does not ship a contributors.ejs — contributors is
// rendered only in the repository template — so we emit a self-contained
// section with header + per-contributor chips. Each chip contains the
// avatar and login; contributions mode adds a right-side commit count
// badge and optional ++/-- diff badge.
//
// Returns "" until contributors.go's Run wires up data (M7 repo-mode
// is the only path that currently populates List; user/org modes stay
// Skipped).
//
// Output structure mirrors upstream
// `org_repo/source/templates/repository/partials/contributors.ejs:25-33`:
//
//	<section data-section="contributors">
//	  <h2 class="field"><svg/>Contributors of ${base}</h2>
//	  <div class="row"><section class="contributors fill-width">
//	    [for each contributor]:
//	      <div class="label" data-login="${login}">
//	        <img class="avatar" src="..." width="22" height="22" alt=""/>
//	        ${login}
//	        [contributions only]: <div class="contributions">${commits} <svg/></div>
//	        [contributions+stats]: <span class="label-right">++A --D</span>
//	      </div>
//	  </section></div>
//	</section>
//
// adds/dels stay in a `<span class="label-right">` because upstream's
// contributors_contributions toggle is not adopted yet; surfacing them as
// a separate badge keeps the upstream `.contributions` badge unchanged.
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

	const avatarSize = 22 // upstream contributors.ejs hard-codes 22x22

	var b strings.Builder
	b.WriteString(`<section data-section="contributors">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sContributors of %s</h2>`,
		contributorsOcticon, partials.EscapeXML(r.Base),
	)
	b.WriteString(`<div class="row"><section class="contributors fill-width">`)
	for _, c := range r.List {
		fmt.Fprintf(
			&b,
			`<div class="label" data-login="%s">`,
			partials.EscapeXML(c.Login),
		)
		if c.AvatarURL != "" {
			fmt.Fprintf(
				&b,
				`<img class="avatar" src="%s" width="%d" height="%d" alt=""/>`,
				partials.EscapeXML(c.AvatarURL), avatarSize, avatarSize,
			)
		}
		// Raw login text (no <span class="login">) — upstream emits the
		// name directly inside the .label div so the .contributors CSS
		// (flex + gap) lays out avatar / name / count consistently.
		b.WriteString(partials.EscapeXML(c.Login))
		if r.Contributions {
			fmt.Fprintf(&b, `<div class="contributions">%d %s</div>`, c.Commits, commitsOcticon)
			if !r.StatsPending {
				fmt.Fprintf(
					&b,
					`<span class="label-right">++%d --%d</span>`,
					c.Additions, c.Deletions,
				)
			}
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
