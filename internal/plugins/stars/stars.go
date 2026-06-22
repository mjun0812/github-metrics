// Package stars owns the M4 "stars" plugin. It fetches the user's
// most-recently starred repositories via GraphQL
// `user.starredRepositories(orderBy: STARRED_AT)`.
package stars

import (
	"context"
	"fmt"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "stars"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &starsPlugin{}

func init() {
	plugins.Register(Plugin)
}

type starsPlugin struct{}

func (p *starsPlugin) Name() string                     { return Name }
func (p *starsPlugin) Metadata() *config.PluginMetadata { return nil }

// Result is the JSON payload published under data.Plugins["stars"].
type Result struct {
	Skipped       bool          `json:"skipped,omitempty"`
	SkippedReason string        `json:"-"`
	List          []StarredRepo `json:"list"`
	Limit         int           `json:"limit"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// StarredRepo carries one entry from the upstream starred list.
type StarredRepo struct {
	NameWithOwner string    `json:"nameWithOwner"`
	Description   string    `json:"description"`
	URL           string    `json:"url,omitempty"`
	IsFork        bool      `json:"isFork,omitempty"`
	Stars         int       `json:"stars"`
	Forks         int       `json:"forks"`
	Issues        int       `json:"issues"`
	PullRequests  int       `json:"pullRequests"`
	Language      *Language `json:"language,omitempty"`
	License       string    `json:"license,omitempty"`
	StarredAt     time.Time `json:"starredAt"`
}

// Language is the per-repository primary language name + GitHub color.
type Language struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Run issues a single GraphQL call to fetch the user's most recently
// starred repositories (Limit defaults to 4).
func (p *starsPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{Skipped: true, SkippedReason: reason, List: []StarredRepo{}}, nil
	}
	// Per the contract a plugin runs only when plugin_<name> is truthy.
	// Engine glue does not gate today, so each plugin self-gates.
	if !pluginutil.TruthyInput(pc.Inputs, "plugin_"+Name) {
		return &Result{Skipped: true, SkippedReason: "plugin disabled", List: []StarredRepo{}}, nil
	}
	if pc.GraphQL == nil {
		return &Result{Skipped: true, SkippedReason: "GraphQL client unavailable", List: []StarredRepo{}}, nil
	}
	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		return &Result{Skipped: true, SkippedReason: "no login", List: []StarredRepo{}}, nil
	}
	limit := pluginutil.ReadIntDefault(pc.Inputs, "plugin_stars_limit", 4)
	if limit <= 0 {
		limit = 4
	}

	resp, err := pc.GraphQL.UserStarredRepositories(ctx, login, limit)
	if err != nil {
		return nil, xerrors.NewRetryableError(fmt.Errorf("stars: %w", err))
	}
	if resp == nil || resp.User == nil || resp.User.StarredRepositories == nil {
		return &Result{List: []StarredRepo{}, Limit: limit}, nil
	}

	list := make([]StarredRepo, 0, len(resp.User.StarredRepositories.Edges))
	for _, edge := range resp.User.StarredRepositories.Edges {
		if edge == nil || edge.Node == nil {
			continue
		}
		node := edge.Node
		desc := ""
		if node.Description != nil {
			desc = *node.Description
		}
		var lang *Language
		if node.PrimaryLanguage != nil && node.PrimaryLanguage.Name != "" {
			color := ""
			if node.PrimaryLanguage.Color != nil {
				color = *node.PrimaryLanguage.Color
			}
			lang = &Language{Name: node.PrimaryLanguage.Name, Color: color}
		}
		license := ""
		if node.LicenseInfo != nil {
			license = formatLicense(node.LicenseInfo.Name, node.LicenseInfo.SpdxId)
		}
		issues := 0
		if node.Issues != nil {
			issues = node.Issues.TotalCount
		}
		prs := 0
		if node.PullRequests != nil {
			prs = node.PullRequests.TotalCount
		}
		list = append(list, StarredRepo{
			NameWithOwner: node.NameWithOwner,
			Description:   desc,
			URL:           node.Url,
			IsFork:        node.IsFork,
			Stars:         node.StargazerCount,
			Forks:         node.ForkCount,
			Issues:        issues,
			PullRequests:  prs,
			Language:      lang,
			License:       license,
			StarredAt:     edge.StarredAt,
		})
	}
	return &Result{List: list, Limit: limit}, nil
}

// formatLicense mirrors upstream `format.license`
// (org_repo/source/app/metrics/utils.mjs): a "NOASSERTION" spdxId falls
// back to the full name, otherwise the spdxId is preferred over the name.
// Our schema's License type does not expose `nickname`, so the upstream
// `nickname ?? spdxId ?? name` chain collapses to `spdxId ?? name`.
func formatLicense(name string, spdxID *string) string {
	spdx := ""
	if spdxID != nil {
		spdx = *spdxID
	}
	if spdx == "" || spdx == "NOASSERTION" {
		return name
	}
	return spdx
}
