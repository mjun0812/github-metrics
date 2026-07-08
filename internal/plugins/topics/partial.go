package topics

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// pinOcticon is the upstream `<%- octicon "pin" %>` 16x16 path used in
// the topics section header. Mirrors EJS line 4.
const pinOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M14.184 1.143a1.75 1.75 0 00-2.502-.57L.912 7.916a1.75 1.75 0 00-.53 2.32l.447.775a1.75 1.75 0 002.275.702l11.745-5.656a1.75 1.75 0 00.757-2.451l-1.422-2.464zm-1.657.669a.25.25 0 01.358.081l1.422 2.464a.25.25 0 01-.108.35l-2.016.97-1.505-2.605 1.85-1.26zM9.436 3.92l1.391 2.41-5.42 2.61-.942-1.63 4.97-3.39zM3.222 8.157l-1.466 1a.25.25 0 00-.075.33l.447.775a.25.25 0 00.325.1l1.598-.769-.83-1.436zm6.253 2.306a.75.75 0 00-.944-.252l-1.809.87a.75.75 0 00-.293.253L4.38 14.326a.75.75 0 101.238.848l1.881-2.75v2.826a.75.75 0 001.5 0v-2.826l1.881 2.75a.75.75 0 001.238-.848l-2.644-3.863z"></path></svg>`

// headerLabel mirrors upstream EJS line 5:
//
//	{starred:"Starred topics", labels:"Starred topics",
//	 icons:"Starred topics", mastered:"Mastered technologies and topics"}[mode]
func headerLabel(mode string) string {
	switch mode {
	case "mastered":
		return "Mastered technologies and topics"
	default:
		return "Starred topics"
	}
}

// renderType derives the EJS `plugins.topics.type` switch value using
// the metadata.yml aliases: `starred` is an alias for labels and
// `mastered` for icons (#672). "icons"/"mastered" emit images;
// everything else emits text labels.
func renderType(mode string) string {
	switch mode {
	case "icons", "mastered":
		return "icons"
	default:
		return "labels"
	}
}

// topics icon-mode geometry: 24px images on a 4px margin, 5px corner
// radius (`.topics img { border-radius: 5px; margin: 4px }`).
const (
	topicInset     = 5.0
	topicIconSize  = 24.0
	topicIconGap   = 4.0
	topicIconRadiu = 5.0
	topicIconPitch = topicIconSize + 2*topicIconGap
)

// Partial renders the classic SVG fragment for the topics plugin.
// Returns "" when the result is missing or skipped.
//
// Output (native SVG): a `<g data-section="topics">` anchor
// wrapping a nested `<svg>` with a section header and a wrapping flow of
// either `.label` pills (labels mode) or 24px icon images (icons mode),
// reporting the pixel height it consumes (#409 Phase B2).
//
// Settings: mjun0812 uses plugin_topics: yes, plugin_topics_limit: 15,
// plugin_topics_mode: starred (default) and a variant with mode: icons.
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

	mode := r.Mode
	if mode == "" {
		mode = "starred"
	}
	typ := renderType(mode)

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(pinOcticon, headerLabel(mode))
	body.WriteString(header)

	maxRight := float64(chrome.CardWidth) - topicInset
	if len(r.List) > 0 {
		switch typ {
		case "icons":
			m, h := iconFlow(r.List, topicInset, y, maxRight)
			body.WriteString(m)
			y += h
		default: // "labels"
			texts := make([]string, 0, len(r.List))
			for _, t := range r.List {
				texts = append(texts, strings.ToLower(t.Name))
			}
			m, h := chrome.SVGChipFlow(topicInset, y, maxRight, texts)
			body.WriteString(m)
			y += h
		}
	}

	height := int(y)
	return chrome.WrapSection("topics", height, body.String()), height, nil
}

// iconFlow lays the topic icon images out left-to-right, wrapping to a
// new row when the next 24px image would exceed maxRight. Each image is
// a rounded-corner `<image>` the render pipeline's image-inline stage
// folds into a data URI. Returns the markup and consumed height.
func iconFlow(list []Topic, x, top, maxRight float64) (string, float64) {
	var b strings.Builder
	cx, rowTop, rows := x, top, 1
	idx := 0
	for _, t := range list {
		if t.Icon == "" {
			continue
		}
		if cx+topicIconSize > maxRight && cx > x {
			cx, rowTop = x, rowTop+topicIconPitch
			rows++
		}
		ix, iy := int(cx+topicIconGap), int(rowTop+topicIconGap)
		clip := fmt.Sprintf("topic-icon-%d", idx)
		fmt.Fprintf(
			&b,
			`<defs><clipPath id=%q><rect x="%d" y="%d" width="%d" height="%d" rx="%d" ry="%d"/></clipPath></defs>`+
				`<image href="%s" x="%d" y="%d" width="%d" height="%d" clip-path="url(#%s)"/>`,
			clip, ix, iy, int(topicIconSize), int(topicIconSize), int(topicIconRadiu), int(topicIconRadiu),
			partials.EscapeXML(t.Icon), ix, iy, int(topicIconSize), int(topicIconSize), clip,
		)
		cx += topicIconPitch
		idx++
	}
	return b.String(), float64(rows) * topicIconPitch
}
