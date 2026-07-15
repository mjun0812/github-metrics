// Package stargazers owns the "stargazers" plugin.
//
// It surfaces two variants: the base cumulative / daily-increment
// chart (classic / graph), and the "worldmap" origins map that plots
// each stargazer's declared location on a Natural Earth base map. The
// worldmap uses the offline geocoder in internal/geo — no external API
// is contacted at render time.
package stargazers

import (
	"context"
	"strings"
	"sync"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/geo"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "stargazers"

const (
	chartsTypeClassic  = "classic"
	chartsTypeGraph    = "graph"
	chartsTypeChartist = "chartist"
)

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &stargazersPlugin{}

func init() {
	plugins.Register(Plugin)
}

type stargazersPlugin struct{}

func (p *stargazersPlugin) Name() string { return Name }

func (p *stargazersPlugin) Requires() []plugins.DataKey {
	// stargazers reads from pc.Data fields populated by base; it does not
	// call Provider directly.
	return []plugins.DataKey{}
}

// Result is the JSON payload published under data.Plugins["stargazers"].
// Worldmap is populated when plugin_stargazers_worldmap is truthy and
// nil otherwise.
type Result struct {
	Skipped       bool                `json:"skipped,omitempty"`
	SkippedReason string              `json:"-"`
	Mode          string              `json:"mode,omitempty"`
	List          []Stargazer         `json:"list"`
	Charts        StargazersCharts    `json:"charts"`
	Worldmap      *StargazersWorldmap `json:"worldmap,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Stargazer represents one starrer.
type Stargazer struct {
	Login     string    `json:"login"`
	StarredAt time.Time `json:"starredAt"`
	Location  string    `json:"location"`
}

// StargazersCharts is the chart data slice.
type StargazersCharts struct {
	Type   string       `json:"type"`
	Series []ChartPoint `json:"series"`
}

// ChartPoint is one data point on the stargazers chart.
type ChartPoint struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
	New   int       `json:"new,omitempty"`
}

const stargazersWindowDays = 14

var (
	nowMu   sync.RWMutex
	nowFunc = func() time.Time { return time.Now().UTC() }
)

// SetNowForTest overrides the stargazers clock and returns a restore function.
func SetNowForTest(fn func() time.Time) func() {
	nowMu.Lock()
	old := nowFunc
	nowFunc = fn
	nowMu.Unlock()
	return func() {
		nowMu.Lock()
		nowFunc = old
		nowMu.Unlock()
	}
}

func currentNow() time.Time {
	nowMu.RLock()
	fn := nowFunc
	nowMu.RUnlock()
	return fn()
}

// StargazersWorldmap carries the geocoded stargazer origins used by
// the worldmap variant. Points are deduplicated by (lat,lng) — each
// entry represents a unique location and its Count is the number of
// stargazers whose profile resolved there.
type StargazersWorldmap struct {
	Points []WorldmapPoint `json:"points"`
}

// WorldmapPoint is one aggregated marker on the worldmap.
type WorldmapPoint struct {
	// Location is the resolved place label (city or country name)
	// preserved for tooltip / debugging use. Not necessarily unique.
	Location string `json:"location"`
	// Lat / Lng are the geocoded coordinates. Two stargazers that
	// resolved to the same coordinates share one WorldmapPoint.
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	// Count is the number of stargazers mapped to this point.
	Count int `json:"count"`
}

// Run wires the viewer-mode stargazers chart (spec 013) and, when
// requested, the worldmap variant. Repo-mode returns the existing M7
// stub. User-mode aggregates each owned repo's stargazers (latest 100)
// into daily buckets for the chart, and geocodes each stargazer's
// declared location for the worldmap.
//
// Two legacy inputs are silently accepted for backward compatibility
// with upstream configs: `plugin_stargazers_worldmap_token` (a Google
// Maps API key upstream — irrelevant here since the geocoder is
// offline) and `plugin_stargazers_worldmap_sample` (a subsampling knob
// upstream that only mattered for API cost — no-op here because
// geocoding is free and deterministic).
func (p *stargazersPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	chartsType := selectedChartsType(pc.Inputs)
	worldmapEnabled := isTruthy(pc.Inputs["plugin_stargazers_worldmap"])
	if r := pc.Data.RepoRef(); r != nil {
		// M7 repo-mode: surface the totals already populated by
		// base.FetchRepo. Per-stargazer time-series (Charts.Series)
		// requires a follow-up REST scrape that's deferred — the M7
		// MVP surfaces totals only.
		return &Result{
			Mode:   plugins.ModeRepo,
			List:   []Stargazer{},
			Charts: StargazersCharts{Type: chartsType, Series: []ChartPoint{}},
		}, nil
	}
	// Spec 013: user-mode chart from viewer.repositories(owner).stargazers
	base := &Result{
		Mode:   plugins.ModeUser,
		List:   []Stargazer{},
		Charts: StargazersCharts{Type: chartsType, Series: []ChartPoint{}},
	}
	if pc.GraphQL == nil || !isTruthy(pc.Inputs["plugin_stargazers"]) {
		base.Skipped = true
		base.SkippedReason = "GraphQL client unavailable"
		return base, nil
	}
	resp, err := pc.GraphQL.ViewerStargazersRepos(ctx, 10, 100)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	series := buildSeries(resp)
	base.Charts.Series = series
	if worldmapEnabled {
		base.Worldmap = buildWorldmap(resp, pc)
	}
	return base, nil
}

// LookupLocationForSample is an internal helper exposed only so the
// gen-worldmap-sample tool can resolve fixture locations through the
// exact same offline geocoder the plugin uses at runtime. It should
// not be called by other plugins.
func LookupLocationForSample(location string) (geo.Location, bool) {
	return geo.Default().Lookup(location)
}

// buildWorldmap turns the raw stargazer response into a deduplicated
// list of WorldmapPoints. Each stargazer's location is normalized and
// resolved through the offline geocoder; unresolvable entries are
// dropped silently (they are the majority — most GitHub users leave
// location empty).
func buildWorldmap(resp *githubapi.ViewerStargazersReposResponse, pc *plugins.PluginContext) *StargazersWorldmap {
	if resp == nil || resp.Viewer == nil || resp.Viewer.Repositories == nil {
		return &StargazersWorldmap{Points: []WorldmapPoint{}}
	}
	g := geo.Default()
	// Dedupe by rounded lat/lng so nearby markers merge — the map is
	// 480×240 px and sub-degree precision is invisible at that scale.
	type key struct{ lat, lng float64 }
	agg := map[key]*WorldmapPoint{}
	// Dedupe by unique stargazer login within a single run so a user
	// starring several of the viewer's repos still contributes exactly
	// one marker weight — otherwise a mildly popular repo owner would
	// see spurious clustering.
	seenLogin := map[string]struct{}{}
	for _, repo := range resp.Viewer.Repositories.Nodes {
		if repo == nil || repo.Stargazers == nil {
			continue
		}
		for _, edge := range repo.Stargazers.Edges {
			if edge == nil || edge.Node == nil {
				continue
			}
			login := edge.Node.Login
			if login != "" {
				if _, ok := seenLogin[login]; ok {
					continue
				}
				seenLogin[login] = struct{}{}
			}
			if edge.Node.Location == nil {
				continue
			}
			loc := strings.TrimSpace(*edge.Node.Location)
			if loc == "" {
				continue
			}
			resolved, ok := g.Lookup(loc)
			if !ok {
				if pc != nil && pc.Logger != nil {
					pc.Logger.Debug("stargazers.worldmap: geocode miss", "location", loc)
				}
				continue
			}
			k := key{
				lat: roundCoord(resolved.Lat),
				lng: roundCoord(resolved.Lng),
			}
			if p, exists := agg[k]; exists {
				p.Count++
				continue
			}
			agg[k] = &WorldmapPoint{
				Location: loc,
				Lat:      k.lat,
				Lng:      k.lng,
				Count:    1,
			}
		}
	}
	out := make([]WorldmapPoint, 0, len(agg))
	for _, p := range agg {
		out = append(out, *p)
	}
	sortWorldmapPoints(out)
	return &StargazersWorldmap{Points: out}
}

// roundCoord rounds a coordinate to two decimals so slightly different
// city entries collapse into one marker.
func roundCoord(v float64) float64 {
	return float64(int(v*100+sign(v)*0.5)) / 100
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// sortWorldmapPoints orders points by descending Count, then by lat and
// lng, so a stable JSON serialization is produced.
func sortWorldmapPoints(pts []WorldmapPoint) {
	// Small N, insertion sort keeps output deterministic without
	// pulling in sort.SliceStable's allocations.
	for i := 1; i < len(pts); i++ {
		for j := i; j > 0 && worldmapLess(pts[j], pts[j-1]); j-- {
			pts[j-1], pts[j] = pts[j], pts[j-1]
		}
	}
}

func worldmapLess(a, b WorldmapPoint) bool {
	if a.Count != b.Count {
		return a.Count > b.Count
	}
	if a.Lat != b.Lat {
		return a.Lat < b.Lat
	}
	return a.Lng < b.Lng
}

// buildSeries flattens stargazers across all owned repos into the
// upstream last-14-days daily window and accumulates into a cumulative
// count series.
func buildSeries(resp *githubapi.ViewerStargazersReposResponse) []ChartPoint {
	if resp == nil || resp.Viewer == nil || resp.Viewer.Repositories == nil {
		return []ChartPoint{}
	}
	return buildSeriesAt(resp, currentNow().UTC())
}

func buildSeriesAt(resp *githubapi.ViewerStargazersReposResponse, now time.Time) []ChartPoint {
	if resp == nil || resp.Viewer == nil || resp.Viewer.Repositories == nil {
		return []ChartPoint{}
	}
	end := dayStart(now.UTC())
	start := end.AddDate(0, 0, -(stargazersWindowDays - 1))
	daily := map[string]int{}
	total := 0
	windowStars := 0
	for _, repo := range resp.Viewer.Repositories.Nodes {
		if repo == nil || repo.Stargazers == nil {
			continue
		}
		total += repo.StargazerCount
		for _, edge := range repo.Stargazers.Edges {
			if edge == nil {
				continue
			}
			starred := dayStart(edge.StarredAt.UTC())
			if starred.Before(start) || starred.After(end) {
				continue
			}
			daily[dayKey(starred)]++
			windowStars++
		}
	}
	out := make([]ChartPoint, 0, stargazersWindowDays)
	cum := total - windowStars
	for i := 0; i < stargazersWindowDays; i++ {
		day := start.AddDate(0, 0, i)
		newStars := daily[dayKey(day)]
		cum += newStars
		out = append(out, ChartPoint{Date: day, Count: cum, New: newStars})
	}
	return out
}

func dayStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dayKey(t time.Time) string {
	return dayStart(t).Format("2006-01-02")
}

func isTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	}
	return false
}

// selectedChartsType resolves the user-supplied
// `plugin_stargazers_charts_type` input into a canonical chart type.
//
// The resolution is a switch keyed on the lower-cased, trimmed input so
// new aliases can be added with a single extra case. Unknown or empty
// values fall back to the upstream default `classic`.
//
// `chartist` is deprecated upstream and treated as an alias of `graph`
// (see metadata.yml). Existing configs that still pass `chartist` will
// continue to work without any change.
func selectedChartsType(inputs map[string]any) string {
	raw, ok := inputs["plugin_stargazers_charts_type"]
	if !ok {
		return chartsTypeClassic
	}
	s, ok := raw.(string)
	if !ok {
		return chartsTypeClassic
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case chartsTypeGraph, chartsTypeChartist:
		return chartsTypeGraph
	default:
		return chartsTypeClassic
	}
}
