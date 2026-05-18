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

// codeOcticon is the upstream `<%- octicon "code" %>` 16x16 path — emitted
// as the leading glyph in the count header per upstream classic EJS line 4.
const codeOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 2.75a.25.25 0 01.25-.25h12.5a.25.25 0 01.25.25v8.5a.25.25 0 01-.25.25h-6.5a.75.75 0 00-.53.22L4.5 14.44v-2.19a.75.75 0 00-.75-.75h-2a.25.25 0 01-.25-.25v-8.5zM1.75 1A1.75 1.75 0 000 2.75v8.5C0 12.216.784 13 1.75 13H3v1.543a1.457 1.457 0 002.487 1.03L8.061 13h6.189A1.75 1.75 0 0016 11.25v-8.5A1.75 1.75 0 0014.25 1H1.75zm5.03 3.47a.75.75 0 010 1.06L5.31 7l1.47 1.47a.75.75 0 01-1.06 1.06l-2-2a.75.75 0 010-1.06l2-2a.75.75 0 011.06 0zm2.44 0a.75.75 0 000 1.06L10.69 7 9.22 8.47a.75.75 0 001.06 1.06l2-2a.75.75 0 000-1.06l-2-2a.75.75 0 00-1.06 0z"></path></svg>`

// colorDotOcticon returns a 16x16 colored-dot SVG used in per-language
// list entries — mirrors upstream EJS line 76 (`<%- octicon "primitive-dot" %>`-style)
// where the dot's fill is the language's color.
func colorDotOcticon(color string) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>`,
		partials.EscapeXML(colorOrDefault(color)),
	)
}

// pluralS mirrors upstream's `s()` helper: returns "s" when n != 1, else "".
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Partial renders the classic SVG fragment for the languages plugin.
// Returns "" when the result is missing or skipped — classic.go's
// dispatcher then suppresses the wrapper entirely (contract §6).
//
// Output structure (matches upstream org_repo/source/templates/classic/partials/languages.ejs):
//
//	<section data-section="languages">
//	  <h2 class="field"><svg/>N Language(s)</h2>          ← count header (a11y title via aria-hidden octicon)
//	  <section class="column">
//	    <h3 class="field">Most used languages</h3>        ← section sub-header
//	    [<small>estimation from N kb...</small>]          ← indepth summary (when indepth result present)
//	    <svg class="bar" xmlns=... width=460 height=8>    ← progress bar wrapper (FIXES bare-<g> bug)
//	      <title>Languages distribution</title>           ← a11y title (Q1 verbatim preservation)
//	      <g class="languages-progress">
//	        <rect class="language-bar" .../>...
//	      </g>
//	    </svg>
//	    <div class="field center horizontal-wrap fill-width">  ← per-language dot+name list
//	      <div class="field center no-wrap language">
//	        <svg/>Go
//	      </div>...
//	    </div>
//	    [<ul class="languages-list">...]                  ← compat shim retained per parity checklist deviation
//	  </section>
//	  [<section class="column"><h3>Recently used languages</h3>
//	    <small>estimation from...</small>
//	    <svg class="bar"><g class="languages-recent">...</g></svg>
//	  </section>]
//	  [<g class="languages-indepth">...]                  ← wrapped in <svg> for visibility
//	</section>
//
// Spec: specs/011-plugin-rendering-parity/spec.md FR-001..FR-003
// Parity checklist: specs/011-plugin-rendering-parity/plugins/languages.md
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

	// Sections list: upstream defaults to ["most-used"]; honor any
	// caller-set sections list.
	sections := r.Sections
	if len(sections) == 0 {
		sections = []string{"most-used"}
	}

	// Distinct-language count for the header. Counts the favorites
	// (+1 for Other when present). NOTE: upstream's
	// `plugins.languages.unique` may be a larger number derived from
	// the full unique-language set across all analyzed repos; ours is
	// the truncated favorites-row count. Parity is approximate here
	// (deviation documented in plugins/languages.md "Notable
	// deviations" section).
	uniqueCount := len(bars)

	var b strings.Builder
	b.WriteString(`<section data-section="languages">`)

	// ── Count header (upstream EJS lines 2-7) ────────────────────
	fmt.Fprintf(
		&b,
		`<section><h2 class="field">%s%d Language%s</h2></section>`,
		codeOcticon, uniqueCount, pluralS(uniqueCount),
	)

	// ── Per-section blocks (upstream EJS lines 8-100) ────────────
	for _, section := range sections {
		switch section {
		case "most-used":
			writeMostUsedSection(&b, pc, bars)
		case "recently-used":
			writeRecentlyUsedSection(&b, pc)
		}
	}

	// ── Indepth section (wrapped in <svg> to fix bare-<g> bug) ───
	writeIndepthSection(&b, pc)

	b.WriteString(`</section>`)
	return b.String(), nil
}

// writeMostUsedSection emits the "Most used languages" column block
// per upstream EJS lines 9-99 (most-used branch). Includes section
// header, optional indepth summary, the progress bar wrapped in
// <svg class="bar"> (fixes bare-<g> bug), per-language color-dot
// list, and the legacy <ul class="languages-list"> compat shim.
func writeMostUsedSection(b *strings.Builder, pc *templates.PartialContext, bars []plugins.LanguageStat) {
	b.WriteString(`<section class="column">`)
	b.WriteString(`<h3 class="field">Most used languages</h3>`)

	// Indepth summary (upstream EJS lines 32-39 — most-used + indepth).
	if hasIndepth(pc) {
		summary, ok := buildIndepthSummary(pc)
		if ok {
			fmt.Fprintf(b, `<small>%s</small>`, summary)
		}
	}

	if len(bars) > 0 {
		// Progress bar wrapped in <svg class="bar"> per upstream EJS
		// lines 42-50 — fixes the v1.0.0 bare-<g> invisible-render bug.
		fmt.Fprintf(
			b,
			`<svg class="bar" xmlns="http://www.w3.org/2000/svg" width="%d" height="8" role="img" aria-label="Languages distribution"><title>Languages distribution</title><g class="languages-progress">`,
			partialBarWidth,
		)
		offset := 0.0
		for _, lang := range bars {
			width := lang.Value * partialBarWidth
			if width <= 0 {
				continue
			}
			fmt.Fprintf(
				b,
				`<rect class="language-bar" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
				offset, width, partials.EscapeXML(colorOrDefault(lang.Color)), partials.EscapeXML(lang.Name),
			)
			offset += width
		}
		b.WriteString(`</g></svg>`)

		// Per-language color-dot + name list (upstream EJS lines 73-80
		// — default-details branch).
		b.WriteString(`<div class="field center horizontal-wrap fill-width">`)
		for _, lang := range bars {
			fmt.Fprintf(
				b,
				`<div class="field center no-wrap language" data-language="%s">%s%s</div>`,
				partials.EscapeXML(lang.Name),
				colorDotOcticon(lang.Color),
				partials.EscapeXML(lang.Name),
			)
		}
		b.WriteString(`</div>`)

		// Legacy <ul class="languages-list"> retained as a compat shim
		// per parity-checklist deviation — keeps downstream CSS hooks
		// stable while the new per-language div grid carries the
		// upstream-parity rendering.
		b.WriteString(`<ul class="languages-list">`)
		for _, lang := range bars {
			fmt.Fprintf(
				b,
				`<li class="language-entry" data-language="%s"><span class="language-name">%s</span> <span class="language-value">%.1f%%</span></li>`,
				partials.EscapeXML(lang.Name),
				partials.EscapeXML(lang.Name),
				lang.Value*100,
			)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</section>`)
}

// writeRecentlyUsedSection emits the "Recently used languages" column
// block per upstream EJS lines 21-31 (recently-used branch). Includes
// section header, the "estimation from N kb..." summary or
// "No recent push activity found" empty-state, and the progress bar
// wrapped in <svg class="bar"> for the recent data.
func writeRecentlyUsedSection(b *strings.Builder, pc *templates.PartialContext) {
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

	b.WriteString(`<section class="column">`)
	b.WriteString(`<h3 class="field">Recently used languages</h3>`)

	if len(bars) == 0 {
		// Upstream EJS lines 26-29: "No recent push activity found"
		// with an optional "over last D day(s)" suffix.
		if r.Days > 0 {
			fmt.Fprintf(
				b,
				`<small>No recent push activity found over last %d day%s</small>`,
				r.Days, pluralS(r.Days),
			)
		} else {
			b.WriteString(`<small>No recent push activity found</small>`)
		}
		b.WriteString(`</section>`)
		return
	}

	// Recent activity summary (upstream EJS lines 23-25). Without
	// upstream's `stats.recent.total / files / commits` fields wired
	// through, we emit a simplified "active over last N days" line.
	if r.Days > 0 {
		fmt.Fprintf(
			b,
			`<small>activity from %d repositor%s analysed over last %d day%s</small>`,
			r.Load, pluralRepository(r.Load), r.Days, pluralS(r.Days),
		)
	}

	// Progress bar wrapped in <svg class="bar"> per upstream EJS
	// lines 42-50 — fixes the v1.0.0 bare-<g> invisible-render bug.
	fmt.Fprintf(
		b,
		`<svg class="bar" xmlns="http://www.w3.org/2000/svg" width="%d" height="8" role="img" aria-label="Recently used languages distribution"><title>Recently used languages distribution</title><g class="languages-recent" data-days="%d">`,
		partialBarWidth, r.Days,
	)
	offset := 0.0
	for _, lang := range bars {
		width := lang.Value * partialBarWidth
		if width <= 0 {
			continue
		}
		fmt.Fprintf(
			b,
			`<rect class="language-bar-recent" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
			offset, width, partials.EscapeXML(colorOrDefault(lang.Color)), partials.EscapeXML(lang.Name),
		)
		offset += width
	}
	b.WriteString(`</g></svg>`)

	// Per-language color-dot + name list for the recent section.
	b.WriteString(`<div class="field center horizontal-wrap fill-width">`)
	for _, lang := range bars {
		fmt.Fprintf(
			b,
			`<div class="field center no-wrap language" data-language="%s">%s%s</div>`,
			partials.EscapeXML(lang.Name),
			colorDotOcticon(lang.Color),
			partials.EscapeXML(lang.Name),
		)
	}
	b.WriteString(`</div>`)

	b.WriteString(`</section>`)
}

// pluralRepository returns "y" / "ies" suffix for the "repository" word.
func pluralRepository(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// hasIndepth reports whether the languages.indepth plugin returned a
// non-skipped result with at least one totalled language.
func hasIndepth(pc *templates.PartialContext) bool {
	if pc == nil || pc.Data == nil {
		return false
	}
	raw, ok := pc.Data.GetPlugin(IndepthName)
	if !ok || raw == nil {
		return false
	}
	r, ok := raw.(*IndepthResult)
	return ok && r != nil && !r.Skipped && len(r.Total.Bytes) > 0
}

// buildIndepthSummary returns the "estimation from N kb of code in M
// edited files across K commits" string when indepth data is present.
// Returns ("", false) when no indepth data. Without upstream's
// `plugins.languages.files / commits` fields, we emit a simplified
// "estimation from N kb of code in M analyzed repositor(y/ies)" line.
func buildIndepthSummary(pc *templates.PartialContext) (string, bool) {
	if pc == nil || pc.Data == nil {
		return "", false
	}
	raw, ok := pc.Data.GetPlugin(IndepthName)
	if !ok || raw == nil {
		return "", false
	}
	r, ok := raw.(*IndepthResult)
	if !ok || r == nil || r.Skipped {
		return "", false
	}
	var totalBytes int64
	for _, n := range r.Total.Bytes {
		totalBytes += n
	}
	if totalBytes <= 0 {
		return "", false
	}
	analyzed := len(r.Analyzed)
	return fmt.Sprintf(
		"estimation from %s of code in %d analyzed repositor%s",
		formatBytes(totalBytes), analyzed, pluralRepository(analyzed),
	), true
}

// formatBytes returns a human-friendly byte size: "1.2 MB", "345 kB", etc.
// Mirrors upstream's `f.bytes(n)` helper at a basic level.
func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f kB", float64(n)/float64(kb))
	}
	return fmt.Sprintf("%d B", n)
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

// writeIndepthSection emits the indepth byte-totals breakdown wrapped
// in an <svg> so the inner <g class="languages-indepth"> renders inside
// foreignObject (fixes the v1.0.0 bare-<g> invisible-render bug). The
// inner content is the same per-language <text> set as v1.0.0 to
// preserve downstream JSON contract spot-checks.
func writeIndepthSection(b *strings.Builder, pc *templates.PartialContext) {
	if !hasIndepth(pc) {
		return
	}
	raw, _ := pc.Data.GetPlugin(IndepthName)
	r := raw.(*IndepthResult)

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

	// Wrap in <svg> per FR-002 — <g> alone in foreignObject silently
	// drops. The svg has zero rendered dimensions because the indepth
	// section is metadata-only (no visual bar) but the wrap is still
	// required to keep the inner <text> + <g> nodes valid SVG.
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="0" class="languages-indepth-wrapper" aria-hidden="true"><g class="languages-indepth">`)
	for _, e := range entries {
		fmt.Fprintf(
			b,
			`<text class="indepth-language" data-language="%s" data-bytes="%d">%s</text>`,
			partials.EscapeXML(e.name), e.bytes, partials.EscapeXML(e.name),
		)
	}
	b.WriteString(`</g></svg>`)
}

func colorOrDefault(c string) string {
	if c == "" {
		return "#cccccc"
	}
	return c
}
