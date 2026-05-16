package sponsorships

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

// Partial renders the sponsorships fragment. DOM contract: <g
// class="sponsored"> per active entry.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Active) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="sponsorships"><ul class="sponsorships-list">`)
	for _, s := range r.Active {
		fmt.Fprintf(&b,
			`<li class="sponsorship-entry"><g class="sponsored"><span>%s</span></g></li>`,
			partials.EscapeXML(s.Login))
	}
	b.WriteString(`</ul></section>`)
	return b.String(), nil
}
