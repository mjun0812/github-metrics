package languages

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

const partialBarWidth = 460

func init() {
	partials.Register("plugin."+Name, Partial)
}

// codeOcticon is the upstream `<%- octicon "code" %>` 16x16 path — emitted
// as the leading glyph in the count header per upstream classic EJS line 4.
const codeOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 2.75a.25.25 0 01.25-.25h12.5a.25.25 0 01.25.25v8.5a.25.25 0 01-.25.25h-6.5a.75.75 0 00-.53.22L4.5 14.44v-2.19a.75.75 0 00-.75-.75h-2a.25.25 0 01-.25-.25v-8.5zM1.75 1A1.75 1.75 0 000 2.75v8.5C0 12.216.784 13 1.75 13H3v1.543a1.457 1.457 0 002.487 1.03L8.061 13h6.189A1.75 1.75 0 0016 11.25v-8.5A1.75 1.75 0 0014.25 1H1.75zm5.03 3.47a.75.75 0 010 1.06L5.31 7l1.47 1.47a.75.75 0 01-1.06 1.06l-2-2a.75.75 0 010-1.06l2-2a.75.75 0 011.06 0zm2.44 0a.75.75 0 000 1.06L10.69 7 9.22 8.47a.75.75 0 001.06 1.06l2-2a.75.75 0 000-1.06l-2-2a.75.75 0 00-1.06 0z"></path></svg>`

// Native-SVG languages geometry (#409 Phase B7). The centered progress
// bar mirrors upstream's 460px `svg.bar` (the column is `align-items:
// center`, so it sits 10px in from each 480px card edge); list / detail
// rows mirror the `.field` box metrics.
const (
	langBarX      = (chrome.CardWidth - partialBarWidth) / 2 // 10
	langBarHeight = 8.0
	langBarTopGap = 6.0
	langBarBotGap = 8.0

	langSmallFont  = 11.0 // <small> summary / empty-state line
	langSmallFill  = "#666666"
	langSmallPitch = langSmallFont*1.35 + 4

	langListFont    = 14.0 // per-language name (svg body)
	langListFill    = "#777777"
	langIconSize    = 16.0
	langIconMargin  = 8.0 // .field svg { margin: 0 8px }
	langListPitch   = chrome.FieldPitch
	langListItemGap = 16.0 // .field.language { margin: 0 8px }
	langBaseRatio   = 0.32

	langDetailFont  = 11.0 // .field.language.details small
	langDetailFill  = "#666666"
	langDetailInset = 8.0
)

// itoa formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func itoa(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// colorDotOcticon returns a 16x16 colored-dot SVG used in per-language
// list entries — mirrors upstream EJS line 76 (`<%- octicon "primitive-dot" %>`-style)
// where the dot's fill is the language's color.
func colorDotOcticon(color string) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>`,
		partials.EscapeXML(colorOrDefault(color)),
	)
}

// Partial renders the classic SVG fragment for the languages plugin as
// native SVG (#409 Phase B7). Returns "" when the result is missing or
// skipped — classic.go's dispatcher then suppresses the wrapper entirely.
//
// Output: a `<g data-section="languages">` anchor wrapping a
// nested `<svg>` with the code-octicon count header, then per configured
// section (most-used / recently-used) a centered `<h3>` sub-header, an
// optional indepth `<small>` summary, the 460px `<svg class="bar">`
// progress bar (kept verbatim — already native SVG), and either the
// centered color-dot name flow or the 1-2 column details rows. A hidden
// `<g class="languages-indepth">` carries the per-language byte totals
// for downstream JSON contract spot-checks. Reports its consumed height.
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
	bars := append([]plugins.LanguageStat(nil), r.Favorites...)
	if r.Other.Size > 0 {
		bars = append(bars, r.Other)
	}
	if len(bars) == 0 && !hasRecentSection(pc) {
		return "", 0, nil
	}

	// Sections list: upstream defaults to ["most-used"]; honor any
	// caller-set sections list.
	sections := r.Sections
	if len(sections) == 0 {
		sections = []string{"most-used"}
	}

	// Distinct-language count for the header — mirrors upstream's
	// `plugins.languages.unique` (count of all distinct languages
	// across analyzed repositories, before favorites truncation).
	// Falls back to len(bars) when Run hasn't populated Unique.
	uniqueCount := r.Unique
	if uniqueCount == 0 {
		uniqueCount = len(bars)
	}

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(codeOcticon,
		fmt.Sprintf("%d Language%s", uniqueCount, pluginutil.Plural(uniqueCount)))
	body.WriteString(header)

	for _, section := range sections {
		switch section {
		case "most-used":
			y = writeMostUsedSection(&body, pc, bars, y)
		case "recently-used":
			y = writeRecentlyUsedSection(&body, pc, y)
		}
	}

	// Indepth byte-totals breakdown (hidden metadata for JSON spot-checks).
	writeIndepthSection(&body, pc)

	height := int(y)
	return chrome.WrapSection("languages", height, body.String()), height, nil
}

// writeMostUsedSection emits the "Most used languages" column: centered
// sub-header, optional indepth summary, the progress bar, and the
// per-language color-dot flow (or details rows). Returns the y cursor
// after the block.
func writeMostUsedSection(b *strings.Builder, pc *templates.PartialContext, bars []plugins.LanguageStat, top float64) float64 {
	m, hh := chrome.SVGSubHeader(float64(chrome.CardWidth)/2, top, float64(chrome.CardWidth), "Most used languages")
	b.WriteString(m)
	y := top + hh

	if hasIndepth(pc) {
		if summary, ok := buildIndepthSummary(pc); ok {
			b.WriteString(chrome.SVGText(float64(chrome.CardWidth)/2, y+langSmallFont, summary,
				chrome.SVGTextOpts{Size: langSmallFont, Fill: langSmallFill, Anchor: "middle", MaxWidth: float64(chrome.CardWidth) - 20}))
			y += langSmallPitch
		}
	}

	if len(bars) > 0 {
		y += langBarTopGap
		writeBar(b, bars, y, "languages-bar-most", "languages-progress", "Languages distribution", "language-bar")
		y += langBarHeight + langBarBotGap

		r := mustResult(pc)
		if len(r.Details) > 0 {
			y = writeDetailsRows(b, bars, r.Details, pc, y)
		} else {
			y = writeSimpleList(b, bars, y)
		}
	}
	return y
}

// writeBar positions the upstream `<svg class="bar">` block (kept
// verbatim — already native SVG) 10px in from the card edge at y=top.
func writeBar(b *strings.Builder, bars []plugins.LanguageStat, top float64, maskID, gClass, titleText, rectClass string) {
	fmt.Fprintf(b, `<g transform="translate(%d,%s)">`, langBarX, itoa(top))
	writeLanguageBar(b, bars, maskID, gClass, titleText, rectClass)
	b.WriteString(`</g>`)
}

// writeLanguageBar emits the upstream EJS lines 42-50 `<svg class="bar">`
// block: a <mask id="..."> for rounded corners, a 0-width `#d1d5da`
// placeholder, then per-language rects with `mask="url(...)"`. maskID
// must be unique within the rendered SVG document (most-used vs
// recently-used use distinct ids); gClass is the <g> class (e.g.,
// "languages-progress", "languages-recent") and rectClass is the
// per-language rect class. titleText is the a11y title.
func writeLanguageBar(b *strings.Builder, bars []plugins.LanguageStat, maskID, gClass, titleText, rectClass string) {
	maskRef := partials.EscapeXML(maskID)
	fmt.Fprintf(
		b,
		`<svg class="bar" xmlns="http://www.w3.org/2000/svg" width="%d" height="8" role="img" aria-label="%s"><title>%s</title>`,
		partialBarWidth, partials.EscapeXML(titleText), partials.EscapeXML(titleText),
	)
	fmt.Fprintf(
		b,
		`<mask id="%s"><rect x="0" y="0" width="%d" height="8" fill="white" rx="5"/></mask>`,
		maskRef, partialBarWidth,
	)
	fmt.Fprintf(
		b,
		`<rect mask="url(#%s)" x="0" y="0" width="0" height="8" fill="#d1d5da"/>`,
		maskRef,
	)
	fmt.Fprintf(b, `<g class="%s">`, partials.EscapeXML(gClass))
	offset := 0.0
	for _, lang := range bars {
		width := lang.Value * partialBarWidth
		if width <= 0 {
			continue
		}
		fmt.Fprintf(
			b,
			`<rect class="%s" mask="url(#%s)" x="%.2f" y="0" width="%.2f" height="8" fill="%s" data-language="%s"></rect>`,
			partials.EscapeXML(rectClass),
			maskRef,
			offset, width, partials.EscapeXML(colorOrDefault(lang.Color)), partials.EscapeXML(lang.Name),
		)
		offset += width
	}
	b.WriteString(`</g></svg>`)
}

// writeSimpleList lays the per-language color-dot + name entries out as a
// centered, wrapping flow (mirroring `.field.center.horizontal-wrap`),
// starting at y=top. Each entry keeps a `data-language` hook. Returns the
// y cursor after the block.
func writeSimpleList(b *strings.Builder, bars []plugins.LanguageStat, top float64) float64 {
	// itemW = [8px][dot 16][8px][name].
	widths := make([]float64, len(bars))
	for i, l := range bars {
		widths[i] = langIconMargin + langIconSize + langIconMargin + fontmetrics.Width(l.Name, langListFont)
	}

	// Greedy-wrap indices into rows no wider than the card.
	maxRowW := float64(chrome.CardWidth)
	var rows [][]int
	cur := make([]int, 0, len(bars))
	curW := 0.0
	for i := range bars {
		add := widths[i]
		if len(cur) > 0 {
			add += langListItemGap
		}
		if len(cur) > 0 && curW+add > maxRowW {
			rows = append(rows, cur)
			cur, curW = nil, 0
			add = widths[i]
		}
		cur = append(cur, i)
		curW += add
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}

	b.WriteString(`<g class="languages-names">`)
	y := top
	for _, row := range rows {
		rowW := 0.0
		for j, idx := range row {
			rowW += widths[idx]
			if j > 0 {
				rowW += langListItemGap
			}
		}
		x := (float64(chrome.CardWidth) - rowW) / 2
		dotY := y + (langListPitch-langIconSize)/2
		baseline := y + langListPitch/2 + langListFont*langBaseRatio
		for _, idx := range row {
			l := bars[idx]
			fmt.Fprintf(b, `<g data-language="%s">`, partials.EscapeXML(l.Name))
			b.WriteString(chrome.SVGIcon(x+langIconMargin, dotY, "", colorDotOcticon(l.Color)))
			nameX := x + langIconMargin + langIconSize + langIconMargin
			b.WriteString(chrome.SVGText(nameX, baseline, l.Name, chrome.SVGTextOpts{Size: langListFont, Fill: langListFill}))
			b.WriteString(`</g>`)
			x += widths[idx] + langListItemGap
		}
		y += langListPitch
	}
	b.WriteString(`</g>`)
	return y
}

// writeRecentlyUsedSection emits the "Recently used languages" column per
// upstream EJS lines 21-31: centered sub-header, the "activity from N
// repositories" summary or "No recent push activity found" empty state,
// the recent progress bar, and the per-language list. Returns the y
// cursor after the block.
func writeRecentlyUsedSection(b *strings.Builder, pc *templates.PartialContext, top float64) float64 {
	if pc == nil || pc.Data == nil {
		return top
	}
	raw, ok := pc.Data.GetPlugin(RecentName)
	if !ok || raw == nil {
		return top
	}
	r, ok := raw.(*RecentResult)
	if !ok || r == nil || r.Skipped {
		return top
	}
	bars := append([]plugins.LanguageStat(nil), r.Favorites...)
	if r.Other.Size > 0 {
		bars = append(bars, r.Other)
	}

	m, hh := chrome.SVGSubHeader(float64(chrome.CardWidth)/2, top, float64(chrome.CardWidth), "Recently used languages")
	b.WriteString(m)
	y := top + hh

	center := float64(chrome.CardWidth) / 2
	if len(bars) == 0 {
		// Upstream EJS lines 26-29: "No recent push activity found"
		// with an optional "over last D day(s)" suffix.
		text := "No recent push activity found"
		if r.Days > 0 {
			text = fmt.Sprintf("No recent push activity found over last %d day%s", r.Days, pluginutil.Plural(r.Days))
		}
		b.WriteString(chrome.SVGText(center, y+langSmallFont, text,
			chrome.SVGTextOpts{Size: langSmallFont, Fill: langSmallFill, Anchor: "middle", MaxWidth: float64(chrome.CardWidth) - 20}))
		return y + langSmallPitch
	}

	// Recent activity summary (upstream EJS lines 23-25).
	if r.Days > 0 {
		summary := fmt.Sprintf("activity from %d repositor%s analysed over last %d day%s",
			r.Load, pluralRepository(r.Load), r.Days, pluginutil.Plural(r.Days))
		b.WriteString(chrome.SVGText(center, y+langSmallFont, summary,
			chrome.SVGTextOpts{Size: langSmallFont, Fill: langSmallFill, Anchor: "middle", MaxWidth: float64(chrome.CardWidth) - 20}))
		y += langSmallPitch
	}

	y += langBarTopGap
	writeBar(b, bars, y, "languages-bar-recent", "languages-recent", "Recently used languages distribution", "language-bar-recent")
	y += langBarHeight + langBarBotGap

	parent := mustResult(pc)
	if len(parent.Details) > 0 {
		y = writeDetailsRows(b, bars, parent.Details, pc, y)
	} else {
		y = writeSimpleList(b, bars, y)
	}
	return y
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
// analyzed repositor(y/ies)" string when indepth data is present.
// Returns ("", false) when no indepth data.
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

// writeIndepthSection emits the indepth byte-totals breakdown as a hidden
// `<g class="languages-indepth">` (upstream renders it metadata-only —
// no visual bar — so it stays invisible while preserving the per-language
// `<text>` nodes downstream JSON contract spot-checks read). Now that the
// partial is a nested `<svg>` rather than foreignObject HTML, the bare
// `<g>` renders fine, so the v1.0.0 `<svg width="0" height="0">` wrapper
// hack is gone — visibility:hidden keeps the nodes in the DOM but off the
// canvas.
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

	b.WriteString(`<g visibility="hidden" aria-hidden="true"><g class="languages-indepth">`)
	for _, e := range entries {
		fmt.Fprintf(
			b,
			`<text class="indepth-language" data-language="%s" data-bytes="%d">%s</text>`,
			partials.EscapeXML(e.name), e.bytes, partials.EscapeXML(e.name),
		)
	}
	b.WriteString(`</g></g>`)
}

func colorOrDefault(c string) string {
	if c == "" {
		return "#cccccc"
	}
	return c
}

// mustResult re-fetches the languages Result from PartialContext for
// helper functions that need access to the typed Result (e.g.,
// Details mode rendering). Returns an empty Result if missing — the
// caller's bars slice is already valid.
func mustResult(pc *templates.PartialContext) *Result {
	if pc == nil || pc.Data == nil {
		return &Result{}
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return &Result{}
	}
	r, ok := raw.(*Result)
	if !ok || r == nil {
		return &Result{}
	}
	return r
}

// detailIncludes reports whether `details` contains the named column.
func detailIncludes(details []string, name string) bool {
	for _, d := range details {
		if d == name {
			return true
		}
	}
	return false
}

// formatPercent mirrors upstream's `f.percentage(value)` — value is in
// 0..1 range; output is "12.3%".
func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

// writeDetailsRows emits the upstream per-language detail blocks (EJS
// lines 52-71) as native SVG: each language is a `<g class="language
// details" data-language>` with the color-dot + name on the left and the
// requested detail columns (lines / bytes-size / percentage) right-aligned
// grey. Languages split into 1-2 columns, mirroring upstream's `rows`:
//
//	const rows = large ? [0, 1, 2, 3]
//	  : (plugins.languages.details.length > 2) ? [0]
//	  : [0, 1]
//
// We render `large=false`, so numCols = 1 when details has > 2 entries,
// else 2. The `:not(:last-child)` inter-column spacing the HTML relied on
// is replaced by explicit writer positioning. Returns the y cursor after
// the block.
func writeDetailsRows(b *strings.Builder, bars []plugins.LanguageStat, details []string, pc *templates.PartialContext, top float64) float64 {
	numCols := 2
	if len(details) > 2 {
		numCols = 1
	}
	colW := float64(chrome.CardWidth) / float64(numCols)
	showLines := detailIncludes(details, "lines")
	showBytes := detailIncludes(details, "bytes-size")
	showPct := detailIncludes(details, "percentage")

	// Lookup table from language name → indepth bytes (best estimate of
	// "size" upstream uses). Falls back to bars[i].Size when indepth
	// isn't wired.
	indepthBytes := indepthBytesByLanguage(pc)
	indepthLines := indepthLinesByLanguage(pc)

	maxY := top
	for col := 0; col < numCols; col++ {
		x := float64(col) * colW
		y := top
		for i, lang := range bars {
			if i%numCols != col {
				continue
			}
			size := int64(lang.Size)
			if v, ok := indepthBytes[lang.Name]; ok && v > 0 {
				size = v
			}

			dotY := y + (langListPitch-langIconSize)/2
			nameBaseline := y + langListPitch/2 + langListFont*langBaseRatio
			valBaseline := y + langListPitch/2 + langDetailFont*langBaseRatio

			fmt.Fprintf(b, `<g class="language details" data-language="%s">`, partials.EscapeXML(lang.Name))
			b.WriteString(chrome.SVGIcon(x+langIconMargin, dotY, "", colorDotOcticon(lang.Color)))
			nameX := x + langIconMargin + langIconSize + langIconMargin
			b.WriteString(chrome.SVGText(nameX, nameBaseline, lang.Name, chrome.SVGTextOpts{Size: langListFont, Fill: langListFill}))

			parts := make([]string, 0, 3)
			if showLines {
				parts = append(parts, fmt.Sprintf("%s lines", partials.FormatCount(indepthLines[lang.Name])))
			}
			if showBytes {
				parts = append(parts, formatBytes(size))
			}
			if showPct {
				parts = append(parts, formatPercent(lang.Value))
			}
			b.WriteString(chrome.SVGText(x+colW-langDetailInset, valBaseline, strings.Join(parts, "  "),
				chrome.SVGTextOpts{Size: langDetailFont, Fill: langDetailFill, Anchor: "end"}))
			b.WriteString(`</g>`)

			y += langListPitch
		}
		if y > maxY {
			maxY = y
		}
	}
	return maxY
}

// indepthBytesByLanguage extracts the per-language total bytes from
// the indepth result if present. Used to populate the "bytes-size"
// column in details mode.
func indepthBytesByLanguage(pc *templates.PartialContext) map[string]int64 {
	out := map[string]int64{}
	if pc == nil || pc.Data == nil {
		return out
	}
	raw, ok := pc.Data.GetPlugin(IndepthName)
	if !ok || raw == nil {
		return out
	}
	r, ok := raw.(*IndepthResult)
	if !ok || r == nil || r.Skipped {
		return out
	}
	for name, n := range r.Total.Bytes {
		out[name] = n
	}
	return out
}

func indepthLinesByLanguage(pc *templates.PartialContext) map[string]int64 {
	out := map[string]int64{}
	if pc == nil || pc.Data == nil {
		return out
	}
	raw, ok := pc.Data.GetPlugin(IndepthName)
	if !ok || raw == nil {
		return out
	}
	r, ok := raw.(*IndepthResult)
	if !ok || r == nil || r.Skipped {
		return out
	}
	for name, n := range r.Total.Lines {
		out[name] = n
	}
	return out
}

// pluginSizeStat exists to silence go-vet if we ever need to coerce a
// LanguageStat.Size field; currently unused but reserved.
var _ = plugins.LanguageStat{}
