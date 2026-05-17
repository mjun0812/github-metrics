package topics

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

// Partial renders the classic SVG fragment for the topics plugin.
// DOM contract (partial-classic-m4.md §5): emits one
// <g class="topic"> containing a <text class="topic-name"> per topic.
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
	b.WriteString(`<section data-section="topics">`)
	b.WriteString(`<ul class="topics-list">`)
	for _, t := range r.List {
		fmt.Fprintf(
			&b,
			`<li class="topic-entry"><g class="topic" data-topic="%s"><image class="topic-icon" href="%s" width="16" height="16"></image><text class="topic-name">%s</text></g></li>`,
			partials.EscapeXML(t.Name),
			partials.EscapeXML(t.Icon),
			partials.EscapeXML(t.Name),
		)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
