package isocalendar

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Upstream octicons used in the stats panel (mirrors EJS lines 4, 19, 24,
// 29, 33, 37, 41).
const (
	calendarOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4.75 0a.75.75 0 01.75.75V2h5V.75a.75.75 0 011.5 0V2h1.25c.966 0 1.75.784 1.75 1.75v10.5A1.75 1.75 0 0113.25 16H2.75A1.75 1.75 0 011 14.25V3.75C1 2.784 1.784 2 2.75 2H4V.75A.75.75 0 014.75 0zm0 3.5h8.5a.25.25 0 01.25.25V6h-11V3.75a.25.25 0 01.25-.25h2zm-2.25 4v6.75c0 .138.112.25.25.25h10.5a.25.25 0 00.25-.25V7.5h-11z"></path></svg>`

	streaksOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M7.75 14A1.75 1.75 0 016 12.25v-8.5C6 2.784 6.784 2 7.75 2h6.5c.966 0 1.75.784 1.75 1.75v8.5A1.75 1.75 0 0114.25 14h-6.5zm-.25-1.75c0 .138.112.25.25.25h6.5a.25.25 0 00.25-.25v-8.5a.25.25 0 00-.25-.25h-6.5a.25.25 0 00-.25.25v8.5zM4.9 3.508a.75.75 0 01-.274 1.025.25.25 0 00-.126.217v6.5a.25.25 0 00.126.217.75.75 0 01-.752 1.298A1.75 1.75 0 013 11.25v-6.5c0-.649.353-1.214.874-1.516a.75.75 0 011.025.274zM1.625 5.533a.75.75 0 10-.752-1.299A1.75 1.75 0 000 5.75v4.5c0 .649.353 1.214.874 1.515a.75.75 0 10.752-1.298.25.25 0 01-.126-.217v-4.5a.25.25 0 01.126-.217z"></path></svg>`

	flameOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M7.998 14.5c2.832 0 5-1.98 5-4.5 0-1.463-.68-2.19-1.879-3.383l-.036-.037c-1.013-1.008-2.3-2.29-2.834-4.434-.322.256-.63.579-.864.953-.432.696-.621 1.58-.046 2.73.473.947.67 2.284-.278 3.232-.61.61-1.545.84-2.403.633a2.788 2.788 0 01-1.436-.874A3.21 3.21 0 003 10c0 2.53 2.164 4.5 4.998 4.5zM9.533.753C9.496.34 9.16.009 8.77.146 7.035.75 4.34 3.187 5.997 6.5c.344.689.285 1.218.003 1.5-.419.419-1.54.487-2.04-.832-.173-.454-.659-.762-1.035-.454C2.036 7.44 1.5 8.702 1.5 10c0 3.512 2.998 6 6.498 6s6.5-2.5 6.5-6c0-2.137-1.128-3.26-2.312-4.438-1.19-1.184-2.436-2.425-2.653-4.81z"></path></svg>`

	plusXOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M8.5.75a.75.75 0 00-1.5 0v5.19L4.391 3.33a.75.75 0 10-1.06 1.061L5.939 7H.75a.75.75 0 000 1.5h5.19l-2.61 2.609a.75.75 0 101.061 1.06L7 9.561v5.189a.75.75 0 001.5 0V9.56l2.609 2.61a.75.75 0 101.06-1.061L9.561 8.5h5.189a.75.75 0 000-1.5H9.56l2.61-2.609a.75.75 0 00-1.061-1.06L8.5 5.939V.75z"></path></svg>`

	commitsPerDayOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`

	upArrowOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M7.823 1.677L4.927 4.573A.25.25 0 005.104 5H7.25v3.236a.75.75 0 101.5 0V5h2.146a.25.25 0 00.177-.427L8.177 1.677a.25.25 0 00-.354 0zM13.75 11a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5zm-3.75.75a.75.75 0 01.75-.75h.5a.75.75 0 010 1.5h-.5a.75.75 0 01-.75-.75zM7.75 11a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5zM4 11.75a.75.75 0 01.75-.75h.5a.75.75 0 010 1.5h-.5a.75.75 0 01-.75-.75zM1.75 11a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5z"></path></svg>`

	arrowsOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M10.896 2H8.75V.75a.75.75 0 00-1.5 0V2H5.104a.25.25 0 00-.177.427l2.896 2.896a.25.25 0 00.354 0l2.896-2.896A.25.25 0 0010.896 2zM8.75 15.25a.75.75 0 01-1.5 0V14H5.104a.25.25 0 01-.177-.427l2.896-2.896a.25.25 0 01.354 0l2.896 2.896a.25.25 0 01-.177.427H8.75v1.25zm-6.5-6.5a.75.75 0 000-1.5h-.5a.75.75 0 000 1.5h.5zM6 8a.75.75 0 01-.75.75h-.5a.75.75 0 010-1.5h.5A.75.75 0 016 8zm2.25.75a.75.75 0 000-1.5h-.5a.75.75 0 000 1.5h.5zM12 8a.75.75 0 01-.75.75h-.5a.75.75 0 010-1.5h.5A.75.75 0 0112 8zm2.25.75a.75.75 0 000-1.5h-.5a.75.75 0 000 1.5h.5z"></path></svg>`
)

// Partial renders the classic SVG fragment for the isocalendar plugin.
// Output structure mirrors upstream
// org_repo/source/templates/classic/partials/isocalendar.ejs:
//
//	<section data-section="isocalendar" data-duration="...">
//	  <section>
//	    <h2 class="field"><svg cal/>Contributions calendar</h2>
//	    <div class="row">
//	      <section/>                                          <!-- error path (empty) -->
//	      <section>                                           <!-- stats panel -->
//	        <h3 class="field"><svg/>Commits streaks</h3>
//	        <div class="field"><svg flame/>Current streak N day</div>
//	        <div class="field"><svg/>Best streak N day</div>
//	        <h3 class="field"><svg/>Commits per day</h3>
//	        <div class="field"><svg/>Highest in a day at N</div>
//	        <div class="field"><svg/>Average per day at ~A</div>
//	      </section>
//	    </div>
//	    <svg class="isocalendar-grid" ...>                    <!-- 3D isometric heatmap -->
//	      <g transform="scale(4) translate(12, 0)">
//	        <g transform="translate(i*1.7, i)">               <!-- per week -->
//	          <g transform="translate(j*-1.7, j+(1-ratio)*6)">  <!-- per day -->
//	            <path d="M1.7,2 0,1 1.7,0 3.4,1 z"/>           <!-- top diamond -->
//	            <path filter="url(#brightness1)" d="..."/>     <!-- left face -->
//	            <path filter="url(#brightness2)" d="..."/>     <!-- right face -->
//	          </g>
//	        </g>
//	      </g>
//	    </svg>
//	  </section>
//	</section>
//
// Settings: mjun0812 uses plugin_isocalendar: yes, plugin_isocalendar_duration: full-year.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Weeks) == 0 {
		return "", 0, nil
	}

	header, hh := chrome.SVGSectionHeader(calendarOcticon, "Contributions calendar")

	var body strings.Builder
	body.WriteString(header)

	// 3D isometric heatmap SVG (upstream EJS line 47-49 — pre-computed
	// in upstream's index.mjs lines 38-69; ported faithfully here).
	// Upstream pulls it up with `margin-top: -130px` so it overlaps the
	// stats panel; here it is positioned directly under the header (full
	// card width) and the stats panel is laid out in the right column so
	// the two coexist (3D calendar on the left, stats on the right).
	grid, gridH := buildIsometricSVG(r, hh)
	body.WriteString(grid)

	// Stats panel — the right-hand flex column (`<div class="row"><section/>
	// <section>…`). Rendered as native SVG in the right half of the card.
	y := hh
	y += writeStatHeader(&body, y, streaksOcticon, "Commits streaks")
	if r.Streak.Current > 0 {
		y += writeStatField(&body, y, flameOcticon,
			fmt.Sprintf("Current streak %d day%s", r.Streak.Current, pluginutil.Plural(r.Streak.Current)))
	}
	y += writeStatField(&body, y, plusXOcticon,
		fmt.Sprintf("Best streak %d day%s", r.Streak.Max, pluginutil.Plural(r.Streak.Max)))
	y += writeStatHeader(&body, y, commitsPerDayOcticon, "Commits per day")
	y += writeStatField(&body, y, upArrowOcticon,
		fmt.Sprintf("Highest in a day at %d", r.Max))
	y += writeStatField(&body, y, arrowsOcticon,
		fmt.Sprintf("Average per day at ~%.2f", r.Average))

	// The block height spans whichever of the grid / stats column reaches
	// lower (the grid is the taller element for both durations).
	height := int(hh) + gridH
	if statsBottom := int(y); statsBottom > height {
		height = statsBottom
	}

	// Keep the data-duration hook the upstream DOM carried.
	section := chrome.WrapSection("isocalendar", height, body.String())
	section = strings.Replace(section,
		`<section data-section="isocalendar">`,
		fmt.Sprintf(`<section data-section="isocalendar" data-duration="%s">`, partials.EscapeXML(r.Duration)),
		1)
	return section, height, nil
}

// h3 sub-header / field geometry, mirroring the `.field` CSS the HTML
// rows used (`section > .field { margin-left: 5px }`, `.field svg {
// margin: 0 8px }`, `.octicon { 16px }`) so the native rows line up with
// the h2 header rendered by chrome.SVGSectionHeader.
const (
	statInset    = 5.0
	statIconGap  = 8.0
	statIconSize = 16.0
	// h3 { font-size: 14px; color: #0366d6; margin: 8px 0 2px }.
	statH3Font   = 14.0
	statH3Fill   = "#0366d6"
	statH3Top    = 8.0
	statH3Band   = 18.0
	statIconFill = "#959da5"
	baselineFrac = 0.32
)

// writeStatHeader renders an `<h3 class="field">` stats sub-header (grey
// 16px octicon + 14px blue label) in the right-hand stats column (fixed
// at half the card width). Returns the height consumed (margin-top 8 +
// band 18 + margin-bottom 2).
func writeStatHeader(b *strings.Builder, top float64, icon, label string) float64 {
	iconX := float64(chrome.CardWidth)/2 + statInset + statIconGap
	iconY := top + statH3Top + (statH3Band-statIconSize)/2
	textX := iconX + statIconSize + statIconGap
	baseline := top + statH3Top + statH3Band/2 + statH3Font*baselineFrac
	b.WriteString(chrome.SVGIcon(iconX, iconY, statIconFill, icon))
	b.WriteString(chrome.SVGText(textX, baseline, label, chrome.SVGTextOpts{Size: statH3Font, Fill: statH3Fill}))
	return statH3Top + statH3Band + 2
}

// writeStatField renders one `<div class="field">` stats row (grey 16px
// octicon + 14px grey label) via chrome.SVGField in the right-hand
// stats column (fixed at half the card width).
func writeStatField(b *strings.Builder, top float64, icon, label string) float64 {
	half := float64(chrome.CardWidth) / 2
	m, h := chrome.SVGField(half, top, half, icon, label)
	b.WriteString(m)
	return h
}

// buildIsometricSVG renders the 3D isometric contribution heatmap as
// an inline <svg>. Mirrors upstream index.mjs lines 38-69 exactly —
// each day cell becomes 3 <path> elements forming an extruded prism
// (top diamond + left face + right face), scaled 4× via the outer
// transform.
//
// Cell color uses ContributionDay.Color (GitHub palette) and the
// extruded height is proportional to count/max — high-contribution
// days are tall prisms.
func buildIsometricSVG(r *Result, top float64) (string, int) {
	const size = 6
	height := 170
	if r.Duration == "full-year" {
		height = 270
	}

	maxDay := r.Max
	if maxDay <= 0 {
		maxDay = 1 // avoid /0
	}

	var b strings.Builder
	// Positioned via x/y/width/height (upstream's `margin-top: -130px` CSS
	// is dropped — resvg lays out no HTML box model). Rendered at the 480px
	// card width, so viewBox scale is 1 and the drawn height equals the
	// viewBox height.
	fmt.Fprintf(
		&b,
		`<svg version="1.1" xmlns="http://www.w3.org/2000/svg" class="isocalendar-grid" x="0" y="%d" width="%d" height="%d" viewBox="0,0 480,%d">`,
		int(top), chrome.CardWidth, height, height,
	)

	// Brightness filters (upstream uses linear slope 0.6 and 0.2).
	for _, k := range []int{1, 2} {
		slope := 1.0 - float64(k)*0.4
		fmt.Fprintf(&b, `<filter id="brightness%d"><feComponentTransfer>`, k)
		for _, ch := range []string{"R", "G", "B"} {
			fmt.Fprintf(&b, `<feFunc%s type="linear" slope="%.2f"/>`, ch, slope)
		}
		b.WriteString(`</feComponentTransfer></filter>`)
	}

	// Outer scale + translate (matches upstream EJS line 49).
	b.WriteString(`<g transform="scale(4) translate(12, 0)">`)

	for i, week := range r.Weeks {
		// Per-week group: translate(i*1.7, i).
		fmt.Fprintf(&b, `<g transform="translate(%.2f, %d)">`, float64(i)*1.7, i)
		// Per-day stack.
		for j := 0; j < 7; j++ {
			count := week.Days[j]
			color := week.DayColors[j]
			if color == "" {
				color = "#ebedf0"
			}
			ratio := float64(count) / float64(maxDay)
			if count == 0 {
				ratio = 0
			}
			// Per-day group: translate(j*-1.7, j+(1-ratio)*size).
			yOffset := float64(j) + (1.0-ratio)*float64(size)
			fmt.Fprintf(&b, `<g transform="translate(%.2f, %.2f)">`, float64(j)*-1.7, yOffset)
			// 3 paths per day (top diamond + left face + right face).
			extruded := ratio * float64(size)
			fmt.Fprintf(
				&b,
				`<path fill="%s" d="M1.7,2 0,1 1.7,0 3.4,1 z"/>`,
				partials.EscapeXML(color),
			)
			fmt.Fprintf(
				&b,
				`<path fill="%s" filter="url(#brightness1)" d="M0,1 1.7,2 1.7,%.2f 0,%.2f z"/>`,
				partials.EscapeXML(color),
				2+extruded, 1+extruded,
			)
			fmt.Fprintf(
				&b,
				`<path fill="%s" filter="url(#brightness2)" d="M1.7,2 3.4,1 3.4,%.2f 1.7,%.2f z"/>`,
				partials.EscapeXML(color),
				1+extruded, 2+extruded,
			)
			b.WriteString(`</g>`)
		}
		b.WriteString(`</g>`)
	}

	b.WriteString(`</g>`)
	b.WriteString(`</svg>`)
	return b.String(), height
}
