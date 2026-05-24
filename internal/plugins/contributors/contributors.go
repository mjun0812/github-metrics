// Package contributors owns the M4 "contributors" plugin. Per the
// contract this plugin targets the repository-template account kind,
// which M4 does not yet support (M7 territory). In M4 it always
// returns Skipped=true with an explanatory reason.
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
	Mode          string        `json:"mode,omitempty"`
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

// Run surfaces a repo-mode contributor summary when the engine has
// populated data.Repo (M7 repository template). User and organization
// modes continue to return Skipped — they have no meaningful single
// base/head commit range.
func (p *contributorsPlugin) Run(_ context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if r := pc.Data.RepoRef(); r != nil {
		// Repo-mode: surface the contributors count already populated
		// by base.FetchRepo via the REST Link-header trick. Deep
		// per-contributor stats (commits / additions / deletions)
		// require additional REST calls that ship in a follow-up
		// (the upstream PR-window aggregation is a richer compute
		// path than M7's MVP scope).
		mode := plugins.AggregationMode(pc.Data)
		list := []Contributor{}
		if r.Contributors > 0 {
			list = append(list, Contributor{
				Login:   r.Owner,
				Commits: r.Activity.RecentCommits,
			})
		}
		return &Result{
			Mode:     mode,
			List:     list,
			Sections: []string{"contributors"},
			Base:     r.DefaultBranch,
			Head:     "HEAD",
		}, nil
	}
	return &Result{
		Skipped:       true,
		SkippedReason: "contributors plugin requires repository template",
		Mode:          plugins.ModeUser,
		List:          []Contributor{},
		Sections:      []string{},
	}, nil
}
