package starlists

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// listOcticon is the upstream `<%- octicon "list-unordered" %>` 16x16
// path used in the count header (EJS line 4).
const listOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.75 2.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-6.5zm4 5a.75.75 0 000 1.5h7.5a.75.75 0 000-1.5h-7.5zm0 5a.75.75 0 000 1.5h7.5a.75.75 0 000-1.5h-7.5zM3 8a1 1 0 11-2 0 1 1 0 012 0zm-1 6a1 1 0 100-2 1 1 0 000 2z"></path><path d="M13.314 4.918L11.07 2.417A.25.25 0 0111.256 2h4.488a.25.25 0 01.186.417l-2.244 2.5a.25.25 0 01-.372 0z"></path></svg>`

// perListOcticon is the per-starlist header octicon (EJS line 18).
const perListOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2 4a1 1 0 100-2 1 1 0 000 2zm3.75-1.5a.75.75 0 000 1.5h8.5a.75.75 0 000-1.5h-8.5zm0 5a.75.75 0 000 1.5h8.5a.75.75 0 000-1.5h-8.5zm0 5a.75.75 0 000 1.5h8.5a.75.75 0 000-1.5h-8.5zM3 8a1 1 0 11-2 0 1 1 0 012 0zm-1 6a1 1 0 100-2 1 1 0 000 2z"></path></svg>`

// starlistBarWidth mirrors upstream `420 * (1 + large)` with large=false.
const starlistBarWidth = 420

// pluralRepository returns "y" / "ies" suffix for the "repository" word.
func pluralRepository(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// pluralS mirrors upstream's `s()` helper.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Partial renders the classic SVG fragment for the starlists plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/starlists.ejs.
//
// Output structure (top-level):
//
//	<section data-section="starlists">
//	  <h2 class="field"><svg/>N Star list(s)</h2>
//	  <div class="row"><section>
//	    [for each starlist]:
//	      <div class="starlist">
//	        <h2 class="field"><svg/>${name}</h2>
//	        <div class="count">${count} repositor(y|ies)</div>
//	        <div class="description">${description}</div>
//	        [if languages]:
//	          <div class="languages">
//	            <div class="row">
//	              <svg class="bar"><mask>...</mask><rect.../></svg>
//	            </div>
//	            <div class="row">
//	              <section>...details column 1 (even idx)...</section>
//	              <section>...details column 2 (odd idx)...</section>
//	            </div>
//	          </div>
//	      </div>
//	  </section></div>
//	</section>
//
// Repos rendering (the per-repo card block in upstream lines 56-85) is
// omitted because the M4 data model only carries repo names (Starlist.Repos)
// — no stargazers/forks/language details. Restoring that needs a follow-up
// in starlists.go (per-repo enrichment via repository GraphQL fragment).
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
	fmt.Fprintf(
		&b,
		`<h2 class="field">%s%d Star list%s</h2>`,
		listOcticon, len(r.List), pluralS(len(r.List)),
	)
	b.WriteString(`<div class="row"><section>`)
	for _, s := range r.List {
		b.WriteString(`<div class="starlist">`)
		// Per-list header.
		fmt.Fprintf(
			&b,
			`<h2 class="field">%s%s</h2>`,
			perListOcticon, partials.EscapeXML(s.Name),
		)
		fmt.Fprintf(
			&b,
			`<div class="count">%d repositor%s</div>`,
			s.Count, pluralRepository(s.Count),
		)
		if s.Description != "" {
			fmt.Fprintf(
				&b,
				`<div class="description">%s</div>`,
				partials.EscapeXML(s.Description),
			)
		}
		if len(s.Languages) > 0 {
			writeStarlistLanguages(&b, s.Languages, s.Count)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeStarlistLanguages emits the per-list language bar + 2-column
// details block matching upstream EJS lines 30-54. total is the
// starlist's repository count (passed through so the per-language
// `value/count` percentage is computed against the list size, not the
// total bytes).
func writeStarlistLanguages(b *strings.Builder, langs []plugins.LanguageStat, total int) {
	if total <= 0 {
		total = 1
	}
	b.WriteString(`<div class="languages">`)
	// Bar with mask.
	b.WriteString(`<div class="row">`)
	fmt.Fprintf(
		b,
		`<svg class="bar" xmlns="http://www.w3.org/2000/svg" width="%d" height="8" role="img" aria-label="Starlist language distribution"><title>Starlist language distribution</title>`,
		starlistBarWidth,
	)
	fmt.Fprintf(
		b,
		`<mask id="languages-bar"><rect x="0" y="0" width="%d" height="8" fill="white" rx="5"/></mask>`,
		starlistBarWidth,
	)
	fmt.Fprintf(
		b,
		`<rect mask="url(#languages-bar)" x="0" y="0" width="0" height="8" fill="#d1d5da"/>`,
	)
	offset := 0.0
	for _, l := range langs {
		p := float64(l.Size) / float64(total) // share of list-size
		width := p * float64(starlistBarWidth)
		if width <= 0 {
			continue
		}
		color := l.Color
		if color == "" {
			color = "#959DA5"
		}
		fmt.Fprintf(
			b,
			`<rect mask="url(#languages-bar)" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
			offset, width, partials.EscapeXML(color), partials.EscapeXML(l.Name),
		)
		offset += width
	}
	b.WriteString(`</svg>`)
	b.WriteString(`</div>`)

	// 2-column details — upstream EJS line 39: `rows = [0, 1]` (small).
	b.WriteString(`<div class="row">`)
	for row := 0; row < 2; row++ {
		b.WriteString(`<section>`)
		for i, l := range langs {
			if i%2 != row {
				continue
			}
			color := l.Color
			if color == "" {
				color = "#959DA5"
			}
			pct := float64(l.Size) / float64(total) * 100
			fmt.Fprintf(b, `<div class="field language details" data-language="%s">`, partials.EscapeXML(l.Name))
			fmt.Fprintf(
				b,
				`<div class="field"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>%s</div>`,
				partials.EscapeXML(color), partials.EscapeXML(l.Name),
			)
			fmt.Fprintf(
				b,
				`<small><div>%.1f%%</div><div>%d★</div></small>`,
				pct, l.Size,
			)
			b.WriteString(`</div>`)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
}
