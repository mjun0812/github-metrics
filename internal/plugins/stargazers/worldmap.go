package stargazers

import (
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/geo/worldmap"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
)

// worldmapMarginBottom mirrors the `.margin-bottom { margin-bottom: 16px }`
// spacing the classic template uses to separate stacked sections.
const worldmapMarginBottom = 16.0

// writeWorldmapSection emits the "Stargazers origins" sub-header and
// the rendered world map beneath, returning the new stacking cursor
// (the y-coordinate below the section, including its bottom margin).
//
// The map viewport is a nested `<svg>` so it clips cleanly at the card
// width — an important guarantee per AGENTS.md since resvg does not
// draw content that overflows a nested viewBox.
func writeWorldmapSection(b *strings.Builder, w *StargazersWorldmap, top float64) float64 {
	subMk, subH := chrome.SVGSubHeader(float64(chrome.CardWidth)/2, top, float64(chrome.CardWidth), "Stargazers origins")
	b.WriteString(subMk)

	mapTop := top + subH
	points := make([]worldmap.Point, 0, len(w.Points))
	for _, p := range w.Points {
		points = append(points, worldmap.Point{
			Lat:   p.Lat,
			Lng:   p.Lng,
			Count: p.Count,
			Label: p.Location,
		})
	}
	// Full card-width map. Options.Width in px; the base viewport is
	// 480×240 so at card width the map takes 240 px of vertical space.
	mapMarkup, mapH, err := worldmap.Render(points, worldmap.Options{Width: chrome.CardWidth})
	if err != nil {
		// Should not happen (embedded base map is validated by tests);
		// fall back to just the sub-header rather than crashing the
		// whole card.
		return top + subH + worldmapMarginBottom
	}
	fmt.Fprintf(b, `<svg x="0" y="%d" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg>`,
		int(mapTop), chrome.CardWidth, mapH, chrome.CardWidth, mapH, mapMarkup)
	return mapTop + float64(mapH) + worldmapMarginBottom
}
