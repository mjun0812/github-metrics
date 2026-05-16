// Package notable owns the M4 "notable" plugin. The upstream behavior
// traverses `repositoryOwner` for organization / company affiliation
// signals; that traversal requires a dedicated GraphQL query that
// lands in a follow-up. The M4 MVP wires the Result type so consumers
// see the slot but returns Skipped=true with a deferred-reason note.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §5
// Data model: specs/004-m4-github-plugins/data-model.md E-026
package notable

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "notable"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &notablePlugin{}

func init() {
	plugins.Register(Plugin)
}

type notablePlugin struct{}

func (p *notablePlugin) Name() string                     { return Name }
func (p *notablePlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["notable"].
type Result struct {
	Skipped       bool             `json:"skipped,omitempty"`
	SkippedReason string           `json:"-"`
	List          []NotableContrib `json:"list"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// NotableContrib mirrors one entry from the upstream notable list.
type NotableContrib struct {
	Login   string `json:"login"`
	Repo    string `json:"repo"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Indepth bool   `json:"indepth"`
}

// Run returns Skipped in M4. The full repositoryOwner traversal lands
// alongside the dedicated GraphQL fragment in a follow-up.
func (p *notablePlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	return &Result{
		Skipped:       true,
		SkippedReason: "notable repositoryOwner traversal is wired in a follow-up to US2",
		List:          []NotableContrib{},
	}, nil
}
