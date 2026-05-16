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
	Login string     `json:"login"`
	Tier  string     `json:"tier"`
	Since time.Time  `json:"since"`
	Until *time.Time `json:"until,omitempty"`
}

// Run returns an empty (non-Skipped) Result in M4. The dedicated
// `viewer.sponsorshipsAsSponsor` GraphQL fragment will land in a
// follow-up; until then the plugin surfaces its type but emits no
// rows so the SVG partial stays out.
func (p *sponsorshipsPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	return &Result{Active: []Sponsored{}}, nil
}
