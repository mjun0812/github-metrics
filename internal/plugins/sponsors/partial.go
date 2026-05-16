package sponsors

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

// Partial renders the sponsors fragment. DOM contract: <g class="sponsor"> per entry.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Sponsors) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="sponsors"><ul class="sponsors-list">`)
	for _, s := range r.Sponsors {
		fmt.Fprintf(&b,
			`<li class="sponsor-entry"><g class="sponsor"><span class="sponsor-login">%s</span><span class="sponsor-tier">%s</span></g></li>`,
			partials.EscapeXML(s.Login), partials.EscapeXML(s.Tier))
	}
	b.WriteString(`</ul></section>`)
	return b.String(), nil
}
