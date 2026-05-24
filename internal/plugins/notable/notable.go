// Package notable owns the M4 "notable" plugin. The upstream behavior
// traverses `repositoryOwner` for organization / company affiliation
// signals; that traversal requires a dedicated GraphQL query that
// lands in a follow-up. The M4 MVP wires the Result type so consumers
// see the slot but returns Skipped=true with a deferred-reason note.
package notable

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
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
	Login          string `json:"login"`
	Repo           string `json:"repo"`
	Title          string `json:"title"`
	Type           string `json:"type"`
	Indepth        bool   `json:"indepth"`
	Description    string `json:"description,omitempty"`
	StargazerCount int    `json:"stargazerCount,omitempty"`
	ForkCount      int    `json:"forkCount,omitempty"`
}

// Run wires viewer.repositories(orderBy:STARGAZERS DESC, owner) for the
// notable plugin in basic mode (spec 013). The repositoryOwner-traversal
// indepth variant is deferred to a future spec (014+).
func (p *notablePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	base := &Result{List: []NotableContrib{}}
	if pc.GraphQL == nil || !truthy(pc.Inputs["plugin_notable"]) {
		base.Skipped = true
		base.SkippedReason = "GraphQL client unavailable"
		return base, nil
	}
	limit := 5
	if v, ok := pc.Inputs["plugin_notable_limit"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			limit = n
		}
	}
	resp, err := pc.GraphQL.ViewerNotable(ctx, limit)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	base.List = collectNotable(resp)
	return base, nil
}

func collectNotable(resp *githubapi.ViewerNotableResponse) []NotableContrib {
	if resp == nil || resp.Viewer == nil || resp.Viewer.Repositories == nil {
		return []NotableContrib{}
	}
	nodes := resp.Viewer.Repositories.Nodes
	out := make([]NotableContrib, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		desc := ""
		if n.Description != nil {
			desc = *n.Description
		}
		out = append(out, NotableContrib{
			Repo:           n.NameWithOwner,
			Title:          n.NameWithOwner,
			Type:           "owner",
			Description:    desc,
			StargazerCount: n.StargazerCount,
			ForkCount:      n.ForkCount,
		})
	}
	return out
}

// truthy mirrors the shared helper across plugins; spec 013 uses it to
// gate the GraphQL fetch on the `plugin_notable` input.
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1" || x == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}
