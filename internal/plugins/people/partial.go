package people

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Partial renders the people fragment. DOM contract: <g class="person">
// + <image class="avatar"> per entry.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", nil
	}
	total := 0
	for _, list := range r.Types {
		total += len(list)
	}
	if total == 0 {
		return "", nil
	}
	// Stable type ordering for deterministic SVG output.
	keys := make([]string, 0, len(r.Types))
	for k := range r.Types {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(`<section data-section="people">`)
	for _, k := range keys {
		list := r.Types[k]
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<ul class="people-group" data-type="%s">`, partials.EscapeXML(k))
		for _, p := range list {
			fmt.Fprintf(&b,
				`<li class="people-entry"><g class="person"><image class="avatar" href="%s" /><span>%s</span></g></li>`,
				partials.EscapeXML(p.AvatarURL), partials.EscapeXML(p.Login))
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</section>`)
	return b.String(), nil
}
