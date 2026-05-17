package languages

import (
	"context"
	"fmt"
	"sort"
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
	if len(bars) == 0 && !hasRecentSection(pc) {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="languages">`)
	if len(bars) > 0 {
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
	}
	writeRecentSection(&b, pc)
	writeIndepthSection(&b, pc)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// hasRecentSection reports whether the recently-used or indepth side
// blocks have anything to render — used to keep the wrapping <section>
// from being emitted when ALL three blocks are empty.
func hasRecentSection(pc *templates.PartialContext) bool {
	if pc == nil || pc.Data == nil {
		return false
	}
	if raw, ok := pc.Data.GetPlugin(RecentName); ok {
		if r, ok := raw.(*RecentResult); ok && r != nil && !r.Skipped && len(r.Favorites) > 0 {
			return true
		}
	}
	if raw, ok := pc.Data.GetPlugin(IndepthName); ok {
		if r, ok := raw.(*IndepthResult); ok && r != nil && !r.Skipped && len(r.Total.Bytes) > 0 {
			return true
		}
	}
	return false
}

// writeRecentSection emits the <g class="languages-recent"> block when
// the languages.recent plugin returned a non-skipped result with at
// least one favorite. Silently no-ops otherwise.
func writeRecentSection(b *strings.Builder, pc *templates.PartialContext) {
	if pc == nil || pc.Data == nil {
		return
	}
	raw, ok := pc.Data.GetPlugin(RecentName)
	if !ok || raw == nil {
		return
	}
	r, ok := raw.(*RecentResult)
	if !ok || r == nil || r.Skipped {
		return
	}
	bars := append([]plugins.LanguageStat(nil), r.Favorites...)
	if r.Other.Size > 0 {
		bars = append(bars, r.Other)
	}
	if len(bars) == 0 {
		return
	}
	fmt.Fprintf(b, `<g class="languages-recent" data-days="%d">`, r.Days)
	offset := 0.0
	for _, lang := range bars {
		width := lang.Value * partialBarWidth
		if width <= 0 {
			continue
		}
		fmt.Fprintf(b,
			`<rect class="language-bar-recent" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
			offset, width, partials.EscapeXML(colorOrDefault(lang.Color)), partials.EscapeXML(lang.Name))
		offset += width
	}
	b.WriteString(`</g>`)
}

// writeIndepthSection emits the <g class="languages-indepth"> block
// when the languages.indepth plugin returned a non-skipped result with
// at least one totalled language. Silently no-ops otherwise.
func writeIndepthSection(b *strings.Builder, pc *templates.PartialContext) {
	if pc == nil || pc.Data == nil {
		return
	}
	raw, ok := pc.Data.GetPlugin(IndepthName)
	if !ok || raw == nil {
		return
	}
	r, ok := raw.(*IndepthResult)
	if !ok || r == nil || r.Skipped {
		return
	}
	if len(r.Total.Bytes) == 0 {
		return
	}
	// Stable ordering — biggest language first, then alpha tie-break.
	type entry struct {
		name  string
		bytes int64
	}
	entries := make([]entry, 0, len(r.Total.Bytes))
	for name, byteCount := range r.Total.Bytes {
		entries = append(entries, entry{name: name, bytes: byteCount})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].bytes != entries[j].bytes {
			return entries[i].bytes > entries[j].bytes
		}
		return entries[i].name < entries[j].name
	})
	b.WriteString(`<g class="languages-indepth">`)
	for _, e := range entries {
		fmt.Fprintf(b,
			`<text class="indepth-language" data-language="%s" data-bytes="%d">%s</text>`,
			partials.EscapeXML(e.name), e.bytes, partials.EscapeXML(e.name))
	}
	b.WriteString(`</g>`)
}

func colorOrDefault(c string) string {
	if c == "" {
		return "#cccccc"
	}
	return c
}
