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

// Partial renders the classic SVG fragment for the achievements plugin.
// DOM contract: emits one <svg class="achievement"> per entry with the
// rank surfaced via data-achievement="<rank>".
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
	b.WriteString(`<ul class="achievements-list">`)
	for _, a := range r.List {
		fmt.Fprintf(
			&b,
			`<li class="achievement-entry"><svg class="achievement" data-achievement="%s" data-octicon=":octicon-%s-16:"></svg><span class="achievement-title">%s</span><span class="achievement-value">%d</span></li>`,
			partials.EscapeXML(a.Rank),
			partials.EscapeXML(a.Icon),
			partials.EscapeXML(a.Title),
			a.Value,
		)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
