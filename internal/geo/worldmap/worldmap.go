// Package worldmap renders a stargazers-origins world map as native SVG.
// The base map is a compact equirectangular projection derived from
// Natural Earth 1:110m Admin 0 Countries (public domain / PDDL,
// attribution recorded in THIRD_PARTY_LICENSES.md) and embedded at
// compile time. Consumers pass a slice of Points (lat/lng plus a count)
// and receive an SVG string plus the pixel height the section consumes,
// matching the "self-report height" contract used elsewhere in
// internal/plugins/**.
package worldmap

import (
	_ "embed"
	"fmt"
	"math"
	"sort"
	"strings"
)

//go:embed worldmap.svg
var baseMapSVG string

// Point is one location on the map.
type Point struct {
	// Lat and Lng are in degrees, positive north / east.
	Lat, Lng float64
	// Count is the number of stargazers mapped to this point after
	// deduplication. It scales the marker radius.
	Count int
	// Label is optional hover / tooltip text (e.g. the resolved city
	// name). It is emitted as a `<title>` child of the circle.
	Label string
}

// Options controls the rendered appearance.
type Options struct {
	// Width in px. Defaults to the classic card width (480) when zero.
	Width int
	// LandFill is the base country fill color (default #ebedf0).
	LandFill string
	// LandStroke is the country border stroke color (default #afafaf).
	LandStroke string
	// MarkerFill is the stargazer marker fill (default #ffab70).
	MarkerFill string
	// MarkerStroke is the marker border color (default #d15704).
	MarkerStroke string
	// MinRadius / MaxRadius bound the marker circle radius in px.
	MinRadius, MaxRadius float64
}

// Constants for the base map viewport. The embedded SVG is projected in
// equirectangular at 480 × 240; consumers can scale by adjusting Width.
const (
	baseViewW = 480.0
	baseViewH = 240.0
)

// applyDefaults fills in zero-value Options fields with sane defaults so
// callers only need to override what they care about.
func (o Options) applyDefaults() Options {
	if o.Width <= 0 {
		o.Width = 480
	}
	if o.LandFill == "" {
		o.LandFill = "#ebedf0"
	}
	if o.LandStroke == "" {
		o.LandStroke = "#afafaf"
	}
	if o.MarkerFill == "" {
		o.MarkerFill = "#ffab70"
	}
	if o.MarkerStroke == "" {
		o.MarkerStroke = "#d15704"
	}
	if o.MinRadius <= 0 {
		o.MinRadius = 2.5
	}
	if o.MaxRadius <= 0 {
		o.MaxRadius = 6.0
	}
	return o
}

// project converts (lat, lng) to (x, y) inside the base viewBox.
func project(lat, lng float64) (float64, float64) {
	// Clamp to the projection domain so out-of-range inputs (bogus
	// geocoder output) stay on the canvas instead of hitting NaN paths.
	if lat > 90 {
		lat = 90
	}
	if lat < -90 {
		lat = -90
	}
	if lng > 180 {
		lng = 180
	}
	if lng < -180 {
		lng = -180
	}
	x := (lng + 180.0) * baseViewW / 360.0
	y := (90.0 - lat) * baseViewH / 180.0
	return x, y
}

// mapHeight returns the rendered map height in px for the requested
// width, preserving the equirectangular 2:1 aspect ratio.
func mapHeight(width int) float64 {
	return float64(width) * baseViewH / baseViewW
}

// Render emits an SVG group containing the embedded base map plus one
// circle per point, and reports the pixel height consumed. Points are
// drawn largest-count first so smaller markers land on top of any
// clustered giant, keeping them visible.
func Render(points []Point, opts Options) (string, int, error) {
	opts = opts.applyDefaults()
	height := int(math.Ceil(mapHeight(opts.Width)))

	// Base map: rewrite the top-level <svg> tag so we can apply the
	// caller's fill/stroke via literal attributes (resvg does not honor
	// CSS custom properties).
	base, err := styleBaseMap(baseMapSVG, opts.LandFill, opts.LandStroke)
	if err != nil {
		return "", 0, err
	}

	// Deduplicated & sorted point list (largest count first).
	sorted := append([]Point(nil), points...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	// Scale factor from base viewBox to output width.
	scale := float64(opts.Width) / baseViewW

	// Compute a radius scale from the count distribution: markers land
	// at MinRadius when count == 1 and reach MaxRadius when count ==
	// max. sqrt keeps big cities visually distinct without blotting out
	// the map.
	maxCount := 1
	for _, p := range sorted {
		if p.Count > maxCount {
			maxCount = p.Count
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<g class="worldmap" transform="scale(%s)">`, floatStr(scale))
	b.WriteString(base)
	if len(sorted) > 0 {
		b.WriteString(`<g class="worldmap-markers" fill="`)
		b.WriteString(opts.MarkerFill)
		b.WriteString(`" stroke="`)
		b.WriteString(opts.MarkerStroke)
		b.WriteString(`" stroke-width="0.5" fill-opacity="0.85">`)
		for _, p := range sorted {
			x, y := project(p.Lat, p.Lng)
			// sqrt scaling for area-proportional visual weight.
			radius := opts.MinRadius
			if maxCount > 1 && p.Count > 1 {
				t := math.Sqrt(float64(p.Count-1) / float64(maxCount-1))
				radius = opts.MinRadius + (opts.MaxRadius-opts.MinRadius)*t
			}
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s">`,
				floatStr(x), floatStr(y), floatStr(radius))
			if p.Label != "" {
				b.WriteString(`<title>`)
				b.WriteString(escapeXML(p.Label))
				b.WriteString(`</title>`)
			}
			b.WriteString(`</circle>`)
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)
	return b.String(), height, nil
}

// styleBaseMap replaces the outer <svg …> wrapper on the embedded map
// with an inline <g> whose paths are styled via literal fill/stroke
// attributes. resvg (the PNG rasterizer) does not evaluate CSS custom
// properties or class selectors, so styling has to live on the elements
// themselves.
func styleBaseMap(svg, fill, stroke string) (string, error) {
	// The embedded file wraps everything in `<svg …><g class="countries">…</g></svg>`.
	// Strip the outer <svg> tags and apply attributes to the group.
	const openTagEnd = "\">\n<g class=\"countries\">"
	i := strings.Index(svg, openTagEnd)
	if i < 0 {
		return "", fmt.Errorf("worldmap: embedded base map has unexpected format")
	}
	inner := svg[i+len(openTagEnd):]
	end := strings.LastIndex(inner, "</g></svg>")
	if end < 0 {
		return "", fmt.Errorf("worldmap: embedded base map missing closing tags")
	}
	paths := inner[:end]
	return fmt.Sprintf(`<g class="worldmap-countries" fill="%s" stroke="%s" stroke-width="0.4">%s</g>`,
		fill, stroke, paths), nil
}

// floatStr formats f with up to 2 decimals, trimming trailing zeros and
// a dangling decimal point so the emitted SVG stays compact.
func floatStr(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// escapeXML applies the minimal set of XML character escapes needed for
// a <title> node's text content.
func escapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

// Height returns the height in px that Render will consume for the
// given options, without doing any actual work. Callers that need the
// height before deciding to render (e.g. reserving space in a card)
// can use this.
func Height(opts Options) int {
	opts = opts.applyDefaults()
	return int(math.Ceil(mapHeight(opts.Width)))
}
