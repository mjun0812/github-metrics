package starlists

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
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

// repoOcticon is the per-repository card octicon (EJS line 61).
const repoOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4 5.75C4 4.784 4.784 4 5.75 4h4.5c.966 0 1.75.784 1.75 1.75v4.5A1.75 1.75 0 0110.25 12h-4.5A1.75 1.75 0 014 10.25v-4.5zm1.75-.25a.25.25 0 00-.25.25v4.5c0 .138.112.25.25.25h4.5a.25.25 0 00.25-.25v-4.5a.25.25 0 00-.25-.25h-4.5z"></path></svg>`

// starlistBarWidth mirrors upstream `420 * (1 + large)` with large=false.
const starlistBarWidth = 420

// pluralRepository returns "y" / "ies" suffix for the "repository" word.
func pluralRepository(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// starlistIndent mirrors `.starlist { padding-left: 28px }`: each list's
// content is laid out in a local coordinate system translated right by
// this amount.
const starlistIndent = 28.0

// Partial renders the classic SVG fragment for the starlists plugin as
// native SVG (#409 Phase B2): a `<section data-section="starlists">`
// anchor wrapping a nested <svg> with the count header and one
// `<g class="starlist">` block per list — per-list header, repo count,
// description, an optional language bar + 2-column details, and an
// optional repositories card block. Each list is translated right by
// starlistIndent and laid out top-down; the partial reports its total
// consumed height.
//
// The repositories block carries only name + description — the upstream
// `infos` sub-block (language / stargazers / forks) is not rendered
// because the user.lists GraphQL items are not enriched with those
// per-repo details (see Starlist.Repositories / issue #675).
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", 0, nil
	}
	// Note: an empty List is NOT a skip condition. Upstream renders the
	// section header ("0 Star lists") even when the account has no star
	// lists; the for-loop below simply does not run, so only the header
	// is emitted. See issue #474.

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(listOcticon,
		fmt.Sprintf("%d Star list%s", len(r.List), pluginutil.Plural(len(r.List))))
	body.WriteString(header)

	for i, s := range r.List {
		m, h := renderStarlist(s, i)
		fmt.Fprintf(&body, `<g class="starlist" transform="translate(%d,%d)">%s</g>`,
			int(starlistIndent), int(y), m)
		y += h
	}

	height := int(y)
	return chrome.WrapSection("starlists", height, body.String()), height, nil
}

// renderStarlist lays one star list out in a local coordinate system
// (x=0 is the list's indented left edge). Returns the markup and the
// height it consumes.
func renderStarlist(s Starlist, index int) (string, float64) {
	const (
		countLeft = 32.0 // .starlist > .count { margin-left: 32px }
		descLeft  = 32.0 // .starlist > .description { margin-left: 32px }
		descWidth = 460.0 - descLeft
	)
	var b strings.Builder

	hdr, y := chrome.SVGSectionHeader(perListOcticon, s.Name)
	b.WriteString(hdr)

	// Repo count (12px grey).
	countText := fmt.Sprintf("%d repositor%s", s.Count, pluralRepository(s.Count))
	fmt.Fprintf(&b, `<g class="count">%s</g>`,
		chrome.SVGText(countLeft, y+14, countText, chrome.SVGTextOpts{Size: 12, Fill: "#666666"}))
	y += 18

	if s.Description != "" {
		m, dh := chrome.SVGParagraph(descLeft, y, descWidth, 14, "#777777", s.Description)
		b.WriteString(m)
		y += dh
	}

	if len(s.Languages) > 0 {
		// Per-list mask id so sibling starlists (and the languages plugin)
		// don't collide on the same SVG id.
		maskID := fmt.Sprintf("starlists-bar-%d", index)
		m, lh := renderStarlistLanguages(s.Languages, s.Count, maskID, y)
		b.WriteString(m)
		y += lh
	}

	if len(s.Repositories) > 0 {
		m, rh := renderStarlistRepositories(s.Repositories, y)
		b.WriteString(m)
		y += rh
	}

	return b.String(), y + 6
}

// renderStarlistLanguages emits the per-list language bar plus a
// 2-column details block, starting at y=top. total is the starlist's
// repository count (the per-language percentage is computed against the
// list size). maskID must be unique within the document.
func renderStarlistLanguages(langs []plugins.LanguageStat, total int, maskID string, top float64) (string, float64) {
	if total <= 0 {
		total = 1
	}
	const (
		barX    = 31.0 // padding-left 13 + svg.bar margin-left 18
		barH    = 8.0
		detailW = 217.0
		rowH    = 18.0
	)
	maskRef := partials.EscapeXML(maskID)
	barY := top + 4 // svg.bar { margin: 4px 0 }

	var b strings.Builder
	b.WriteString(`<g class="languages">`)
	// Positioned nested bar svg with mask.
	fmt.Fprintf(&b,
		`<svg class="bar" x="%d" y="%d" width="%d" height="%d" role="img" aria-label="Starlist language distribution"><title>Starlist language distribution</title>`,
		int(barX), int(barY), starlistBarWidth, int(barH))
	fmt.Fprintf(&b, `<mask id="%s"><rect x="0" y="0" width="%d" height="%d" fill="white" rx="5"/></mask>`,
		maskRef, starlistBarWidth, int(barH))
	fmt.Fprintf(&b, `<rect mask="url(#%s)" x="0" y="0" width="0" height="%d" fill="#d1d5da"/>`, maskRef, int(barH))
	offset := 0.0
	for _, l := range langs {
		width := float64(l.Size) / float64(total) * float64(starlistBarWidth)
		if width <= 0 {
			continue
		}
		color := l.Color
		if color == "" {
			color = "#959DA5"
		}
		fmt.Fprintf(&b,
			`<rect mask="url(#%s)" x="%.2f" y="0" width="%.2f" height="%d" fill="%s" data-language="%s"></rect>`,
			maskRef, offset, width, int(barH), partials.EscapeXML(color), partials.EscapeXML(l.Name))
		offset += width
	}
	b.WriteString(`</svg>`)

	// 2-column details: even indices in column 0, odd in column 1.
	detailsTop := barY + barH + 6
	maxRows := 0.0
	for col := 0; col < 2; col++ {
		colX := barX + float64(col)*(detailW+6)
		ry := detailsTop
		for i, l := range langs {
			if i%2 != col {
				continue
			}
			color := l.Color
			if color == "" {
				color = "#959DA5"
			}
			pct := float64(l.Size) / float64(total) * 100
			dot := fmt.Sprintf(
				`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>`,
				partials.EscapeXML(color))
			baseline := ry + rowH/2 + 12*0.32
			fmt.Fprintf(&b, `<g class="language details" data-language="%s">`, partials.EscapeXML(l.Name))
			b.WriteString(chrome.SVGIcon(colX, ry+(rowH-16)/2, "", dot))
			b.WriteString(chrome.SVGText(colX+20, baseline, l.Name,
				chrome.SVGTextOpts{Size: 12, Fill: "#666666", MaxWidth: detailW - 90}))
			b.WriteString(chrome.SVGText(colX+detailW, baseline,
				fmt.Sprintf("%.1f%%  %d★", pct, l.Size),
				chrome.SVGTextOpts{Size: 11, Fill: "#666666", Anchor: "end"}))
			b.WriteString(`</g>`)
			ry += rowH
		}
		if ry-detailsTop > maxRows {
			maxRows = ry - detailsTop
		}
	}
	b.WriteString(`</g>`)
	return b.String(), (detailsTop - top) + maxRows
}

// renderStarlistRepositories emits the per-repo card block starting at
// y=top. Only the octicon + name + description are rendered; the
// upstream `infos` sub-block is skipped because those per-repo details
// aren't fetched. The caller guarantees repos is non-empty.
func renderStarlistRepositories(repos []Repository, top float64) (string, float64) {
	const (
		cardMargin = 6.0
		nameWidth  = 420.0
		descLeft   = 33.0 // card inset(-5) + description margin-left(38)
		descWidth  = 400.0
	)
	var b strings.Builder
	b.WriteString(`<g class="repositories">`)
	y := top
	for _, repo := range repos {
		y += cardMargin
		var card strings.Builder
		nameRow, h := chrome.SVGRepoName(-5, y, nameWidth, 14, repoOcticon, repo.Name, "", "")
		card.WriteString(nameRow)
		y += h
		if repo.Description != "" {
			m, dh := chrome.SVGParagraph(descLeft, y, descWidth, 11, "#666666", repo.Description)
			card.WriteString(m)
			y += dh
		}
		fmt.Fprintf(&b, `<g class="repository">%s</g>`, card.String())
	}
	b.WriteString(`</g>`)
	return b.String(), y - top
}
