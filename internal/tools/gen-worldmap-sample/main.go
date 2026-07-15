// Command gen-worldmap-sample writes docs/examples/plugin-stargazers-worldmap.svg
// using an offline fixture stargazer set.
//
// The real docs/examples/ pipeline (scripts/gen-doc-samples.sh) drives
// the docker image against live GitHub data — but the worldmap variant
// is brand new and no live sample exists in the reference set yet, so
// this generator bootstraps a first sample from a hand-picked list of
// well-known GitHub users spread across continents. After merge the
// regen-doc-samples workflow will replace this file with a real-data
// render.
//
// Invoke: go run ./internal/tools/gen-worldmap-sample
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
)

// fixturePoints lists (location, weight) pairs. Locations must resolve
// against the embedded geocoder — the list was tuned so every entry
// hits either a cities15000 row or a country-centroid fallback.
var fixturePoints = []struct {
	Location string
	Count    int
}{
	{"Tokyo, Japan", 5},
	{"Kyoto, Japan", 2},
	{"Osaka, Japan", 2},
	{"Seoul, South Korea", 3},
	{"Beijing, China", 4},
	{"Shanghai, China", 3},
	{"Hong Kong", 2},
	{"Singapore", 3},
	{"Bangalore, India", 3},
	{"Mumbai, India", 2},
	{"Sydney, Australia", 2},
	{"Melbourne, Australia", 1},
	{"San Francisco", 6},
	{"New York", 5},
	{"Seattle", 3},
	{"Los Angeles", 2},
	{"Austin", 2},
	{"Boston", 2},
	{"Toronto, Canada", 3},
	{"Vancouver, Canada", 2},
	{"Mexico City, Mexico", 2},
	{"Sao Paulo, Brazil", 3},
	{"Buenos Aires, Argentina", 2},
	{"London", 5},
	{"Berlin, Germany", 4},
	{"Paris, France", 3},
	{"Amsterdam, Netherlands", 2},
	{"Madrid, Spain", 2},
	{"Rome, Italy", 2},
	{"Stockholm, Sweden", 2},
	{"Warsaw, Poland", 1},
	{"Moscow, Russia", 3},
	{"Istanbul, Turkey", 2},
	{"Tel Aviv, Israel", 2},
	{"Dubai, UAE", 2},
	{"Cairo, Egypt", 1},
	{"Lagos, Nigeria", 1},
	{"Nairobi, Kenya", 1},
	{"Cape Town, South Africa", 2},
	{"Auckland, New Zealand", 1},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gen-worldmap-sample <output-path>")
		os.Exit(2)
	}
	outPath := os.Args[1]

	pts := buildPoints()
	// Fabricate a Result carrying just the worldmap so the partial only
	// renders the worldmap section.
	data := plugins.NewData()
	data.SetPlugin("stargazers", &stargazers.Result{
		Mode:     plugins.ModeUser,
		List:     []stargazers.Stargazer{},
		Charts:   stargazers.StargazersCharts{Type: "classic", Series: []stargazers.ChartPoint{}},
		Worldmap: &stargazers.StargazersWorldmap{Points: pts},
	})

	markup, height, err := stargazers.Partial(context.Background(), &templates.PartialContext{Data: data})
	if err != nil {
		fmt.Fprintln(os.Stderr, "partial:", err)
		os.Exit(1)
	}
	svg := wrapStandalone(markup, height)
	outPath = filepath.Clean(outPath)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil { //nolint:gosec // dev-only tool, caller supplies path
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(svg), 0o600); err != nil { //nolint:gosec // dev-only tool, caller supplies path
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes, height=%d, points=%d)\n", outPath, len(svg), height, len(pts))
}

// buildPoints resolves each fixture location through the offline
// geocoder used at runtime and returns the deduplicated marker list.
func buildPoints() []stargazers.WorldmapPoint {
	// Simulate GraphQL edges → buildWorldmap indirectly by using the
	// public geocoder path. We construct WorldmapPoints directly here
	// because we don't have a live GraphQL server — but we go through
	// the same geo.Default() lookup so the sample stays consistent with
	// the runtime pipeline.
	pts := make([]stargazers.WorldmapPoint, 0, len(fixturePoints))
	for _, f := range fixturePoints {
		loc, ok := stargazers.LookupLocationForSample(f.Location)
		if !ok {
			fmt.Fprintf(os.Stderr, "skip: %q did not resolve\n", f.Location)
			continue
		}
		pts = append(pts, stargazers.WorldmapPoint{
			Location: f.Location,
			Lat:      loc.Lat,
			Lng:      loc.Lng,
			Count:    f.Count,
		})
	}
	return pts
}

// wrapStandalone builds a fully-formed top-level SVG document around a
// single partial's markup, using the classic chrome envelope so the
// docs/examples/*.svg files stay visually consistent with the ones
// scripts/gen-doc-samples.sh produces.
func wrapStandalone(markup string, height int) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<style>svg{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif;font-size:14px;color:#777}</style>`+
			`<g transform="translate(0,0)" class="plugin-stargazers" data-plugin="stargazers">%s</g>`+
			`</svg>`,
		chrome.CardWidth, height, chrome.CardWidth, height, markup,
	)
}
