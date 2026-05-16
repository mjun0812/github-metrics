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

// Partial renders the stars fragment. DOM contract: <a
// class="starred-repo"> per entry.
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
	b.WriteString(`<section data-section="stars"><ul class="stars-list">`)
	for _, s := range r.List {
		fmt.Fprintf(&b,
			`<li class="star-entry"><a class="starred-repo" data-stars="%d">%s</a></li>`,
			s.Stars, partials.EscapeXML(s.NameWithOwner))
	}
	b.WriteString(`</ul></section>`)
	return b.String(), nil
}
