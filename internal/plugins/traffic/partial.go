package traffic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// graphOcticon is the primer/octicon "graph" 16x16 path (the same line-
// chart glyph the base Activity header uses via base.octChart). It is
// duplicated here rather than exported from the base package so this
// partial stays self-contained and does not risk an init-time import
// cycle with internal/plugins/base.
const graphOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 1.75a.75.75 0 00-1.5 0v12.5c0 .414.336.75.75.75h14.5a.75.75 0 000-1.5H1.5V1.75zm14.28 2.53a.75.75 0 00-1.06-1.06L10 7.94 7.53 5.47a.75.75 0 00-1.06 0L3.22 8.72a.75.75 0 001.06 1.06L7 7.06l2.47 2.47a.75.75 0 001.06 0l5.25-5.25z"></path></svg>`

// Partial renders the classic SVG fragment for the traffic plugin.
// Upstream classic does not ship a standalone traffic.ejs (the data is
// merged into base.repositories); we emit a self-contained section with
// header + aggregate counts + per-repo breakdown so downstream consumers
// can see the data the plugin actually collects.
//
// Output (native SVG): a `<g data-section="traffic">` anchor
// wrapping a nested `<svg>` with a section header, the aggregate views
// pill, and one text row per repository (#409 Phase B2).
//
// The aggregate line always renders for a non-Skipped result — even when
// the (possibly filtered) Views map is empty it shows "0 views (0
// unique)". The dispatcher only reaches this partial when the plugin is
// enabled and not Skipped, so an enabled plugin always shows something.
//
// When `plugin_traffic_hide_empty` is true (default), repositories with
// Count == 0 are filtered before sorting/rendering so the per-repo
// breakdown only shows repos that actually received traffic.
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

	// Collect the per-repo rows, dropping zero-view repos when HideEmpty.
	type repoView struct {
		name string
		view TrafficView
	}
	rows := make([]repoView, 0, len(r.Views))
	for name, v := range r.Views {
		if r.HideEmpty && v.Count == 0 {
			continue
		}
		rows = append(rows, repoView{name: name, view: v})
	}
	// Deterministic order: highest view count first, name ascending on
	// ties. Map iteration order must never leak into the SVG output.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].view.Count != rows[j].view.Count {
			return rows[i].view.Count > rows[j].view.Count
		}
		return rows[i].name < rows[j].name
	})

	const (
		inset   = 5.0                   // section > .field { margin-left: 5px }
		chipRow = chrome.FieldPitch + 8 // .label pill row (22px chip + margins)
		textMax = chrome.CardWidth - 2*inset
	)

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(graphOcticon, "Traffic")
	body.WriteString(header)

	// Aggregate views rendered as the `.label` pill.
	chip, _ := chrome.SVGLabelChip(inset, y+2, viewsText(r.Total.Count, r.Total.Uniques))
	body.WriteString(chip)
	y += chipRow

	// Per-repo rows: "${name}: N views (M unique)" as plain body text.
	for _, row := range rows {
		line := fmt.Sprintf("%s: %s", row.name, viewsText(row.view.Count, row.view.Uniques))
		baseline := y + chrome.FieldPitch/2 + 14*0.32
		body.WriteString(chrome.SVGText(inset, baseline, line, chrome.SVGTextOpts{MaxWidth: textMax}))
		y += chrome.FieldPitch
	}

	height := int(y)
	return chrome.WrapSection("traffic", height, body.String()), height, nil
}

// viewsText formats the shared "<N> view[s] (<M> unique)" phrase used by
// both the aggregate line and the per-repo rows. Counts use the same
// k/m/b short form as the base repositories panel (partials.FormatCount);
// "view" pluralises on the view count while "unique" is left invariant to
// match the documented section contract.
func viewsText(count, uniques int) string {
	return fmt.Sprintf(
		"%s view%s (%s unique)",
		partials.FormatCount(int64(count)), pluginutil.Plural(count), partials.FormatCount(int64(uniques)),
	)
}
