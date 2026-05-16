package languages

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

const partialBarWidth = 460

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Partial renders the classic SVG fragment for the languages plugin.
// Returns "" when the result is missing or skipped — classic.go's
// dispatcher then suppresses the wrapper entirely (contract §6).
//
// DOM contract (partial-classic-m4.md §5): emits a single
// <g class="languages-progress"> containing one <rect class="language-bar">
// per favorite + an "Other" rect when applicable.
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
	bars := append([]plugins.LanguageStat(nil), r.Favorites...)
	if r.Other.Size > 0 {
		bars = append(bars, r.Other)
	}
	if len(bars) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="languages">`)
	fmt.Fprintf(&b, `<g class="languages-progress">`)
	offset := 0.0
	for _, lang := range bars {
		width := lang.Value * partialBarWidth
		if width <= 0 {
			continue
		}
		fmt.Fprintf(&b,
			`<rect class="language-bar" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
			offset, width, partials.EscapeXML(colorOrDefault(lang.Color)), partials.EscapeXML(lang.Name))
		offset += width
	}
	b.WriteString(`</g>`)
	b.WriteString(`<ul class="languages-list">`)
	for _, lang := range bars {
		fmt.Fprintf(
			&b,
			`<li class="language-entry" data-language="%s"><span class="language-name">%s</span> <span class="language-value">%.1f%%</span></li>`,
			partials.EscapeXML(lang.Name),
			partials.EscapeXML(lang.Name),
			lang.Value*100,
		)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

func colorOrDefault(c string) string {
	if c == "" {
		return "#cccccc"
	}
	return c
}
