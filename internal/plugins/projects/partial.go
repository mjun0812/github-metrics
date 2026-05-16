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

// Partial renders the projects fragment. Returns "" when Skipped or
// empty so classic.go suppresses the wrapper. DOM contract: <g
// class="project"> per entry.
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
	b.WriteString(`<section data-section="projects"><ul class="projects-list">`)
	for _, p := range r.List {
		fmt.Fprintf(&b,
			`<li class="project-entry"><g class="project"><a href="%s"><span>%s</span></a></g></li>`,
			partials.EscapeXML(p.URL), partials.EscapeXML(p.Name))
	}
	b.WriteString(`</ul></section>`)
	return b.String(), nil
}
