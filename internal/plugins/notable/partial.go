package notable

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

// notableOcticon is the upstream `<%- octicon "rocket" %>` 16x16 path
// used in the notable section header (EJS line 4).
const notableOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M11.696 2.704a.5.5 0 11.708.707l-1.395 1.396a.5.5 0 01-.708-.707l1.395-1.396zM10.5 7a.5.5 0 100-1 .5.5 0 000 1zm-7 .5a.5.5 0 100-1 .5.5 0 000 1zm5.5 5.5a.5.5 0 100-1 .5.5 0 000 1zm-.5-9.5a.5.5 0 100 1 .5.5 0 000-1zm2.65 6.85l1.394 1.395a.5.5 0 11-.707.707l-1.395-1.394a.5.5 0 01.708-.708zM4.61 11.39a.5.5 0 11.706-.708L3.92 12.077a.5.5 0 11-.707-.707L4.61 11.39zM3.214 3.21a.5.5 0 01.707-.707l1.396 1.395a.5.5 0 01-.708.707L3.215 3.211z"></path></svg>`

// Partial renders the classic SVG fragment for the notable plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/notable.ejs.
//
// Returns "" while Run is unwired (Skipped or empty List). When data
// arrives, emits a header (rocket octicon + "Notable contributions"
// + count) followed by per-contribution rows with login, repo, title,
// and an optional `<span class="indepth">` indepth badge.
//
// Output structure:
//
//	<section data-section="notable">
//	  <h2 class="field"><svg/>Notable contributions (N)</h2>
//	  <div class="row"><section>
//	    [for each contrib]:
//	      <div class="field">
//	        <span class="login">${login}</span>
//	        <span class="repo">${repo}</span>
//	        <span class="title">${title}</span>
//	        [if indepth]: <span class="indepth">indepth</span>
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

	var b strings.Builder
	b.WriteString(`<section data-section="notable">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sNotable contributions (%d)</h2>`,
		notableOcticon, len(r.List),
	)
	b.WriteString(`<div class="row"><section>`)
	for _, c := range r.List {
		b.WriteString(`<div class="field">`)
		fmt.Fprintf(&b, `<span class="login">%s</span>`, partials.EscapeXML(c.Login))
		if c.Repo != "" {
			fmt.Fprintf(&b, ` in <span class="repo">%s</span>`, partials.EscapeXML(c.Repo))
		}
		if c.Title != "" {
			fmt.Fprintf(&b, ` <span class="title">%s</span>`, partials.EscapeXML(c.Title))
		}
		if c.Indepth {
			b.WriteString(` <span class="indepth">indepth</span>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
