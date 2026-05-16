// Package stargazers owns the M4 "stargazers" plugin. Per the contract
// this plugin targets the repository-template account kind, which M4
// does not support yet. Worldmap (`_worldmap=true`) is explicitly
// deferred per research note R-012 — the M4 implementation always
// returns Skipped=true with an explanatory reason.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §11
// Data model: specs/004-m4-github-plugins/data-model.md E-032
package stargazers

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
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

// Run always returns Skipped in M4. Worldmap input is observed and a
// WARN log is emitted to make the deferred state explicit.
func (p *stargazersPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
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
	return &Result{
		Skipped:       true,
		SkippedReason: "stargazers requires repository account kind (M7 territory)",
		List:          []Stargazer{},
		Charts:        StargazersCharts{Type: "classic", Series: []ChartPoint{}},
	}, nil
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
