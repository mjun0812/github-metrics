// Package sponsorships owns the M4 "sponsorships" plugin. It returns
// the sponsorships the viewer is paying for. In M4 the plugin is
// scaffolded; the underlying `viewer.sponsorshipsAsSponsor` GraphQL
// call lands as a follow-up once the schema covers it. The MVP wires
// the type surface so downstream consumers see the slot.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §10
// Data model: specs/004-m4-github-plugins/data-model.md E-031
package sponsorships

import (
	"context"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "sponsorships"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &sponsorshipsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type sponsorshipsPlugin struct{}

func (p *sponsorshipsPlugin) Name() string                     { return Name }
func (p *sponsorshipsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["sponsorships"].
type Result struct {
	Skipped       bool        `json:"skipped,omitempty"`
	SkippedReason string      `json:"-"`
	Active        []Sponsored `json:"active"`
	Past          []Sponsored `json:"past,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Sponsored carries one entry from the upstream sponsorships list.
type Sponsored struct {
	Login  string     `json:"login"`
	Tier   string     `json:"tier"`
	Since  time.Time  `json:"since"`
	Until  *time.Time `json:"until,omitempty"`
	Avatar string     `json:"avatar,omitempty"`
	Type   string     `json:"type,omitempty"`
}

// Run wires viewer.sponsorshipsAsSponsor (spec 013). With a nil GraphQL
// client the M4 baseline (empty, non-Skipped) is preserved.
func (p *sponsorshipsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	base := &Result{Active: []Sponsored{}}
	if pc.GraphQL == nil || !truthy(pc.Inputs["plugin_sponsorships"]) {
		return base, nil
	}
	resp, err := pc.GraphQL.ViewerSponsorships(ctx, 12)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	base.Active = collectViewerSponsorships(resp)
	return base, nil
}

func collectViewerSponsorships(resp *githubapi.ViewerSponsorshipsResponse) []Sponsored {
	if resp == nil || resp.Viewer == nil || resp.Viewer.SponsorshipsAsSponsor == nil {
		return []Sponsored{}
	}
	nodes := resp.Viewer.SponsorshipsAsSponsor.Nodes
	out := make([]Sponsored, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || n.Sponsorable == nil {
			continue
		}
		switch x := (*n.Sponsorable).(type) {
		case *githubapi.ViewerSponsorshipsViewerUserSponsorshipsAsSponsorSponsorshipConnectionNodesSponsorshipSponsorableUser:
			out = append(out, Sponsored{Login: x.Login, Avatar: x.AvatarUrl, Type: "user", Since: n.CreatedAt})
		case *githubapi.ViewerSponsorshipsViewerUserSponsorshipsAsSponsorSponsorshipConnectionNodesSponsorshipSponsorableOrganization:
			out = append(out, Sponsored{Login: x.Login, Avatar: x.AvatarUrl, Type: "organization", Since: n.CreatedAt})
		}
	}
	return out
}

// truthy mirrors the shared helper across plugins; spec 013 wiring uses
// it to gate the GraphQL fetch on the `plugin_sponsorships` input.
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := x
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}
