package repositories

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

// Partial renders the classic SVG fragment for the repositories plugin.
// DOM contract: emits <a class="repository"> per featured repository.
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
	b.WriteString(`<ul class="repositories-list">`)
	for _, repo := range r.Featured {
		fmt.Fprintf(
			&b,
			`<li class="repository-entry"><a class="repository" href="%s" data-stars="%d" data-forks="%d">%s</a></li>`,
			partials.EscapeXML(repo.URL),
			repo.Stars,
			repo.Forks,
			partials.EscapeXML(repo.NameWithOwner),
		)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
