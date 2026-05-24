// Package stargazers owns the M4 "stargazers" plugin. Per the contract
// this plugin targets the repository-template account kind, which M4
// does not support yet. Worldmap (`_worldmap=true`) is explicitly
// deferred per research note R-012 — the M4 implementation always
// returns Skipped=true with an explanatory reason.
package stargazers

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "stargazers"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &stargazersPlugin{}

func init() {
	plugins.Register(Plugin)
}

type stargazersPlugin struct{}

func (p *stargazersPlugin) Name() string                     { return Name }
func (p *stargazersPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["stargazers"].
// Worldmap is always nil in M4 (R-012).
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
}

// StargazersWorldmap is a placeholder that always serializes to null
// in M4 per R-012.
type StargazersWorldmap struct{}

// Run wires viewer-mode stargazers chart (spec 013). Worldmap input is
// observed and a WARN log is emitted to make the deferred state explicit.
// Repo-mode returns the existing M7 stub. User-mode aggregates each
// owned repo's stargazers (latest 100) into month buckets.
func (p *stargazersPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if v, ok := pc.Inputs["plugin_stargazers_worldmap"]; ok && isTruthy(v) {
		if pc.Logger != nil {
			pc.Logger.Warn("stargazers: worldmap is not yet implemented in M4 (planned as N-task)")
		} else {
			slog.Default().Warn("stargazers: worldmap is not yet implemented in M4 (planned as N-task)")
		}
	}
	if r := pc.Data.RepoRef(); r != nil {
		// M7 repo-mode: surface the totals already populated by
		// base.FetchRepo. Per-stargazer time-series (Charts.Series)
		// requires a follow-up REST scrape that's deferred — the M7
		// MVP surfaces totals only.
		return &Result{
			Mode:   plugins.ModeRepo,
			List:   []Stargazer{},
			Charts: StargazersCharts{Type: "classic", Series: []ChartPoint{}},
		}, nil
	}
	// Spec 013: user-mode chart from viewer.repositories(owner).stargazers
	base := &Result{
		Mode:   plugins.ModeUser,
		List:   []Stargazer{},
		Charts: StargazersCharts{Type: "classic", Series: []ChartPoint{}},
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
	series, latestHint := buildSeries(resp)
	base.Charts.Series = series
	if latestHint {
		base.Charts.Type = "classic-latest100"
	}
	return base, nil
}

// buildSeries flattens stargazers across all owned repos into month
// buckets and accumulates into a cumulative count series. Returns the
// chart series and a "latestHint" flag set when any repo exceeded the
// 100-edge fetch cap (used downstream to add a "Latest 100" suffix).
func buildSeries(resp *githubapi.ViewerStargazersReposResponse) ([]ChartPoint, bool) {
	if resp == nil || resp.Viewer == nil || resp.Viewer.Repositories == nil {
		return []ChartPoint{}, false
	}
	monthly := map[string]int{}
	latestHint := false
	for _, repo := range resp.Viewer.Repositories.Nodes {
		if repo == nil || repo.Stargazers == nil {
			continue
		}
		if repo.Stargazers.TotalCount > 100 {
			latestHint = true
		}
		for _, edge := range repo.Stargazers.Edges {
			if edge == nil {
				continue
			}
			key := monthKey(edge.StarredAt)
			monthly[key]++
		}
	}
	keys := make([]string, 0, len(monthly))
	for k := range monthly {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ChartPoint, 0, len(keys))
	cum := 0
	for _, k := range keys {
		cum += monthly[k]
		t, _ := time.Parse("2006-01", k)
		out = append(out, ChartPoint{Date: t, Count: cum})
	}
	return out, latestHint
}

func monthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
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
