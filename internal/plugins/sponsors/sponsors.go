// Package sponsors owns the M4 "sponsors" plugin. The plugin is
// gated by the `read:user` AND `read:org` OAuth scopes; the MVP wires
// the scope-gate path and returns an empty (but non-Skipped) Result
// when both scopes are present. The full GraphQL fetch lands as a
// follow-up.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §9
// Data model: specs/004-m4-github-plugins/data-model.md E-030
package sponsors

import (
	"context"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "sponsors"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &sponsorsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type sponsorsPlugin struct{}

func (p *sponsorsPlugin) Name() string                     { return Name }
func (p *sponsorsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["sponsors"].
type Result struct {
	Skipped       bool      `json:"skipped,omitempty"`
	SkippedReason string    `json:"-"`
	Sections      []string  `json:"sections"`
	Sponsors      []Sponsor `json:"sponsors"`
	Past          []Sponsor `json:"past,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Sponsor mirrors one entry from the upstream sponsor list.
type Sponsor struct {
	Login string    `json:"login"`
	Tier  string    `json:"tier"`
	Since time.Time `json:"since"`
}

// Run requires `read:user` OR `read:org` scope (per upstream
// behavior, either is sufficient because GitHub uses the broader of
// the two). With neither, Skipped=true. With at least one the MVP
// returns an empty (non-Skipped) Result.
func (p *sponsorsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if pc.REST == nil {
		return &Result{
			Skipped:       true,
			SkippedReason: "REST client unavailable",
			Sponsors:      []Sponsor{},
		}, nil
	}
	scopes, err := pc.REST.Scopes(ctx)
	if err != nil {
		//nolint:nilerr // intentional: Scopes failure maps to Skipped
		return &Result{
			Skipped:       true,
			SkippedReason: "could not determine token scopes",
			Sponsors:      []Sponsor{},
		}, nil
	}
	if !hasScope(scopes, "read:user") && !hasScope(scopes, "read:org") {
		return &Result{
			Skipped:       true,
			SkippedReason: "missing read:user / read:org scope",
			Sponsors:      []Sponsor{},
		}, nil
	}
	return &Result{
		Sections: []string{"sponsors"},
		Sponsors: []Sponsor{},
	}, nil
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
