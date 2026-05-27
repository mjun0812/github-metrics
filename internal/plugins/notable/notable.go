// Package notable owns the M4 "notable" plugin. The Go port uses the
// typed ViewerNotable GraphQL query for the viewer's top owned
// repositories and, when indepth is requested, enriches each entry with
// commit / issue / pull request counts available from that query.
package notable

import (
	"context"
	"sort"
	"strings"

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

// NotableContrib mirrors one entry from the upstream notable
// contributions list. Each entry renders an avatar + chip label and,
// in indepth mode only, additional gauge visualizations for the
// per-repository commit / star / issue / pull counters. See
// source/templates/classic/partials/notable.ejs and the issue #422
// chip layout fix.
type NotableContrib struct {
	// Name is the rendered chip label (without the leading "@"). In
	// basic mode it is the bare repository name (issue #422: the
	// ViewerNotable query is scoped to the page user, so labelling
	// every chip by owner would collapse them into identical chips).
	// In indepth mode it is the "owner/repo" handle, mirroring the
	// upstream notable.ejs indepth grouping.
	Name string `json:"name"`
	// AvatarURL is the owner avatar shown on the chip.
	AvatarURL string `json:"avatar,omitempty"`
	// Organization marks the chip as an organization (upstream toggles
	// the `organization` avatar CSS class accordingly).
	Organization bool `json:"organization,omitempty"`

	Login   string `json:"login"`
	Repo    string `json:"repo"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Indepth bool   `json:"indepth"`

	Description    string `json:"description,omitempty"`
	StargazerCount int    `json:"stargazerCount,omitempty"`
	ForkCount      int    `json:"forkCount,omitempty"`

	// Indepth-only per-repository statistics. Upstream uses these both
	// for the optional gauge visualizations and for sort ordering
	// (maintainers first, then by contribution percentage).
	Commits    int     `json:"commits,omitempty"`
	Issues     int     `json:"issues,omitempty"`
	Pulls      int     `json:"pulls,omitempty"`
	Maintainer bool    `json:"maintainer,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

// Run wires viewer.repositories(orderBy:STARGAZERS DESC, owner) for the
// notable plugin. Indepth mode reuses the same typed payload and exposes
// the extra per-repository counters on each result entry.
func (p *notablePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	indepth := truthy(pc.Inputs["plugin_notable_indepth"])
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
	base.List = collectNotable(resp, indepth)
	return base, nil
}

func collectNotable(resp *githubapi.ViewerNotableResponse, indepth bool) []NotableContrib {
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
		owner := notableOwnerLogin(n)
		// Basic mode labels each chip with the repository name only
		// (chip name "@repo") because the ViewerNotable query is scoped
		// to the page user, so "@owner" would collapse every chip into
		// an identical label (issue #422). Indepth mode keeps the
		// fully-qualified "@org/repo" handle since it can mix
		// repositories from multiple owners and pairs each chip with
		// gauge visualizations.
		name := notableRepoName(n.NameWithOwner)
		if indepth {
			name = n.NameWithOwner
		}
		contrib := NotableContrib{
			Name:           name,
			AvatarURL:      notableOwnerAvatar(n),
			Organization:   notableOwnerIsOrganization(n),
			Login:          owner,
			Repo:           n.NameWithOwner,
			Title:          n.NameWithOwner,
			Type:           "owner",
			Indepth:        indepth,
			Description:    desc,
			StargazerCount: n.StargazerCount,
			ForkCount:      n.ForkCount,
		}
		if indepth {
			contrib.Commits = notableCommitCount(n.DefaultBranchRef)
			if n.Issues != nil {
				contrib.Issues = n.Issues.TotalCount
			}
			if n.PullRequests != nil {
				contrib.Pulls = n.PullRequests.TotalCount
			}
			// The ViewerNotable query only resolves repositories owned by
			// the viewer, so the viewer is the maintainer at full
			// contribution share (upstream sorts maintainers first).
			contrib.Maintainer = true
			contrib.Percentage = 1
		}
		out = append(out, contrib)
	}
	if indepth {
		sortIndepth(out)
	}
	return out
}

// sortIndepth mirrors upstream's indepth ordering: maintainers first,
// then by descending contribution percentage. A stable sort keeps the
// underlying STARGAZERS-DESC order for ties.
func sortIndepth(list []NotableContrib) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Maintainer != list[j].Maintainer {
			return list[i].Maintainer
		}
		return list[i].Percentage > list[j].Percentage
	})
}

// notableRepoName returns the repository name portion of a
// "owner/repo" handle. It is the basic-mode chip label.
func notableRepoName(nameWithOwner string) string {
	if _, repo, ok := strings.Cut(nameWithOwner, "/"); ok {
		return repo
	}
	return nameWithOwner
}

func notableOwnerLogin(n *githubapi.ViewerNotableViewerUserRepositoriesRepositoryConnectionNodesRepository) string {
	if n == nil {
		return ""
	}
	if n.Owner != nil {
		if login := n.Owner.GetLogin(); login != "" {
			return login
		}
	}
	owner, _, ok := strings.Cut(n.NameWithOwner, "/")
	if ok {
		return owner
	}
	return ""
}

// notableOwnerAvatar returns the owner avatar URL projected by the
// ViewerNotable query (avatarUrl(size: 64)).
func notableOwnerAvatar(n *githubapi.ViewerNotableViewerUserRepositoriesRepositoryConnectionNodesRepository) string {
	if n == nil || n.Owner == nil {
		return ""
	}
	return n.Owner.GetAvatarUrl()
}

// notableOwnerIsOrganization reports whether the repository owner is an
// organization, toggling upstream's `organization` avatar CSS class.
func notableOwnerIsOrganization(n *githubapi.ViewerNotableViewerUserRepositoriesRepositoryConnectionNodesRepository) bool {
	if n == nil || n.Owner == nil {
		return false
	}
	tn := n.Owner.GetTypename()
	return tn != nil && *tn == "Organization"
}

func notableCommitCount(ref *githubapi.ViewerNotableViewerUserRepositoriesRepositoryConnectionNodesRepositoryDefaultBranchRef) int {
	if ref == nil || ref.Target == nil || *ref.Target == nil {
		return 0
	}
	commit, ok := (*ref.Target).(*githubapi.ViewerNotableViewerUserRepositoriesRepositoryConnectionNodesRepositoryDefaultBranchRefTargetCommit)
	if !ok || commit.History == nil {
		return 0
	}
	return commit.History.TotalCount
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
