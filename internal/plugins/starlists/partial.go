package starlists

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

// Partial renders the classic SVG fragment for the starlists plugin.
// DOM contract (partial-classic-m4.md §5): emits one <g class="starlist">
// per list with a <text class="starlist-name">.
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
	b.WriteString(`<section data-section="starlists">`)
	b.WriteString(`<ul class="starlists-list">`)
	for _, s := range r.List {
		fmt.Fprintf(
			&b,
			`<li class="starlist-entry"><g class="starlist" data-count="%d"><text class="starlist-name">%s</text>`,
			s.Count, partials.EscapeXML(s.Name),
		)
		if s.Description != "" {
			fmt.Fprintf(
				&b,
				`<text class="starlist-description">%s</text>`,
				partials.EscapeXML(s.Description),
			)
		}
		if len(s.Languages) > 0 {
			b.WriteString(`<g class="starlist-languages">`)
			for _, lang := range s.Languages {
				fmt.Fprintf(
					&b,
					`<text class="starlist-language" data-language="%s" data-size="%d">%s</text>`,
					partials.EscapeXML(lang.Name),
					lang.Size,
					partials.EscapeXML(lang.Name),
				)
			}
			b.WriteString(`</g>`)
		}
		b.WriteString(`</g></li>`)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
