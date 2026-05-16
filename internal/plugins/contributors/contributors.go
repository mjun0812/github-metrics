// Package contributors owns the M4 "contributors" plugin. Per the
// contract this plugin targets the repository-template account kind,
// which M4 does not yet support (M7 territory). In M4 it always
// returns Skipped=true with an explanatory reason.
//
// Contracts: specs/004-m4-github-plugins/contracts/plugin-p2-graphql.md §6
// Data model: specs/004-m4-github-plugins/data-model.md E-027
package contributors

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "contributors"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &contributorsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type contributorsPlugin struct{}

func (p *contributorsPlugin) Name() string                     { return Name }
func (p *contributorsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["contributors"].
type Result struct {
	Skipped       bool          `json:"skipped,omitempty"`
	SkippedReason string        `json:"-"`
	List          []Contributor `json:"list"`
	Sections      []string      `json:"sections"`
	Base          string        `json:"base"`
	Head          string        `json:"head"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// Contributor mirrors one entry from the upstream contributors list.
type Contributor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
	Commits   int    `json:"commits"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Run always returns Skipped=true in M4 because the repository
// template that this plugin targets is M7 territory. User and
// organization accounts do not have a meaningful "base/head" commit
// range.
func (p *contributorsPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	return &Result{
		Skipped:       true,
		SkippedReason: "repository template not yet available",
		List:          []Contributor{},
		Sections:      []string{},
	}, nil
}
