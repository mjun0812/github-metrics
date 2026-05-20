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

// pluralS mirrors upstream's `s()` helper.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Partial renders the classic SVG fragment for the contributors plugin.
// Upstream classic does not ship a contributors.ejs — contributors is
// rendered only in the repository template — so we emit a self-contained
// section with header + per-contributor row (avatar + login + commit
// count + ++/-- diff line counts) using HTML inside <section>.
//
// Returns "" until contributors.go's Run wires up data (M7 repo-mode
// is the only path that currently populates List; user/org modes stay
// Skipped).
//
// Output structure:
//
//	<section data-section="contributors">
//	  <h2 class="field"><svg/>N Contributor(s)</h2>
//	  <div class="row"><section>
//	    [for each contributor]:
//	      <div class="field contributor" data-login="...">
//	        <img class="avatar" src="..."/>
//	        <span class="login">${login}</span>
//	        <span class="commits">${commits} commit(s)</span>
//	        [if additions/deletions]:
//	          <span class="diff">++${additions} --${deletions}</span>
//	      </div>
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

	const avatarSize = 32 // upstream-compatible compact avatar size

	var b strings.Builder
	b.WriteString(`<section data-section="contributors">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%s%d Contributor%s</h2>`,
		contributorsOcticon, len(r.List), pluralS(len(r.List)),
	)
	b.WriteString(`<div class="row"><section>`)
	for _, c := range r.List {
		fmt.Fprintf(
			&b,
			`<div class="field contributor" data-login="%s">`,
			partials.EscapeXML(c.Login),
		)
		if c.AvatarURL != "" {
			fmt.Fprintf(
				&b,
				`<img class="avatar" src="%s" width="%d" height="%d" alt=""/>`,
				partials.EscapeXML(c.AvatarURL), avatarSize, avatarSize,
			)
		}
		fmt.Fprintf(&b, `<span class="login">%s</span>`, partials.EscapeXML(c.Login))
		fmt.Fprintf(
			&b,
			` <span class="commits">%d commit%s</span>`,
			c.Commits, pluralS(c.Commits),
		)
		if c.Additions != 0 || c.Deletions != 0 {
			fmt.Fprintf(
				&b,
				` <span class="diff"><span class="code">++%d --%d</span></span>`,
				c.Additions, c.Deletions,
			)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
