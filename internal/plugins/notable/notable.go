// Package notable owns the M4 "notable" plugin. The Go port mirrors
// upstream source/plugins/notable: it lists the repositories the page
// user contributed to on *other* users'/organizations' accounts
// (user.repositoriesContributedTo), defaulting to organization-owned
// repositories only and excluding the user's own repositories. See
// issue #447.
package notable

import (
	"context"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// Name is the canonical plugin slug.
const Name = "notable"

// defaultLimit caps the number of aggregated contributions rendered.
const defaultLimit = 5

// notablePageSize is the per-page count requested from
// repositoriesContributedTo (upstream defaults to 100 via
// repositories.batch). The connection is already ordered by stargazers
// descending so the most notable contributions surface first.
const notablePageSize = 100

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &notablePlugin{}

func init() {
	plugins.Register(Plugin)
}

type notablePlugin struct{}

func (p *notablePlugin) Name() string                     { return Name }
func (p *notablePlugin) Metadata() *config.PluginMetadata { return nil }

func (p *notablePlugin) Requires() []plugins.DataKey {
	// notable reads from pc.Data fields populated by base; it does not
	// call Provider directly.
	return []plugins.DataKey{}
}

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
// in indepth mode, additional gauge visualizations for the
// per-repository commit / star / issue / pull counters. See
// source/plugins/notable/index.mjs and source/templates/classic/
// partials/notable.ejs.
type NotableContrib struct {
	// Name is the rendered chip label (without the leading "@").
	// Upstream uses the owner segment (handle.split("/").shift(), e.g.
	// "@huggingface") by default, or the full "owner/repo" handle when
	// plugin_notable_repositories is enabled.
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

	// Indepth-only per-repository statistics. The Go port populates
	// these from the GraphQL connection where available; user-specific
	// commit attribution (upstream's REST-derived percentage /
	// maintainer flag) is not computed, so gauges only render when the
	// counters are non-zero.
	Commits    int     `json:"commits,omitempty"`
	Issues     int     `json:"issues,omitempty"`
	Pulls      int     `json:"pulls,omitempty"`
	Maintainer bool    `json:"maintainer,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

// notableRepoNode is the generated GraphQL node type for one
// repositoriesContributedTo entry.
type notableRepoNode = githubapi.UserNotableUserRepositoriesContributedToRepositoryConnectionNodesRepository

// Run wires user.repositoriesContributedTo(orderBy:STARGAZERS DESC) for
// the notable plugin. It honors the upstream owner-type / self /
// contribution-type / repository-label inputs (issue #447).
func (p *notablePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, nil
	}
	indepth := pluginutil.Truthy(pc.Inputs["plugin_notable_indepth"])
	base := &Result{List: []NotableContrib{}}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		base.Skipped = true
		base.SkippedReason = reason
		return base, nil
	}
	if pc.GraphQL == nil || !pluginutil.Truthy(pc.Inputs["plugin_notable"]) {
		base.Skipped = true
		base.SkippedReason = "GraphQL client unavailable"
		return base, nil
	}
	login := pluginutil.LoginFromInputs(pc.Inputs)
	if login == "" {
		base.Skipped = true
		base.SkippedReason = "user login unavailable"
		return base, nil
	}

	from := ownerTypeFilter(pc.Inputs)    // default "organization"
	self := includeSelf(pc.Inputs)        // default false (exclude own repos)
	types := contributionTypes(pc.Inputs) // default [COMMIT]
	useHandle := pluginutil.Truthy(pc.Inputs["plugin_notable_repositories"])
	skipped := skippedSet(pc.Inputs)
	limit := notableLimit(pc.Inputs)
	// repositories_skip_private (#656) is a cross-plugin filter;
	// repositoriesContributedTo is fetched here (not via the provider
	// repo list), so the node-level isPrivate check happens locally.
	skipPrivate := pluginutil.Truthy(pc.Inputs["repositories_skip_private"])

	resp, err := pc.GraphQL.UserNotable(ctx, login, notablePageSize, nil, types, &self)
	if err != nil {
		base.Skipped = true
		base.SkippedReason = "GraphQL fetch failed"
		pc.Data.AppendError(xerrors.NewRetryableError(err))
		return base, nil
	}
	base.List = collectNotable(resp, indepth, from, useHandle, skipped, limit, skipPrivate)
	return base, nil
}

func collectNotable(
	resp *githubapi.UserNotableResponse,
	indepth bool,
	from string,
	useHandle bool,
	skipped map[string]struct{},
	limit int,
	skipPrivate bool,
) []NotableContrib {
	if resp == nil || resp.User == nil || resp.User.RepositoriesContributedTo == nil {
		return []NotableContrib{}
	}
	nodes := resp.User.RepositoriesContributedTo.Nodes

	// Aggregate by chip key (owner segment by default, full handle when
	// plugin_notable_repositories is enabled), mirroring upstream's
	// Map-based dedup so multiple repositories under one owner collapse
	// into a single owner chip.
	order := make([]string, 0, len(nodes))
	byKey := make(map[string]*NotableContrib, len(nodes))

	for _, n := range nodes {
		if n == nil {
			continue
		}
		// repositories_skip_private: drop private contributions entirely.
		if skipPrivate && n.IsPrivate {
			continue
		}
		// from filter: all / organization / user via isInOrganization.
		if !ownerTypeMatches(from, n.IsInOrganization) {
			continue
		}
		// skipped filter: drop repositories by name or owner/repo handle.
		if isSkipped(n.NameWithOwner, skipped) {
			continue
		}

		owner := notableOwnerLogin(n)
		key := owner
		if useHandle {
			key = n.NameWithOwner
		}
		if existing, ok := byKey[key]; ok {
			if indepth {
				existing.Commits += notableCommitCount(n.DefaultBranchRef)
				existing.Issues += issueCount(n)
				existing.Pulls += pullCount(n)
			}
			continue
		}

		desc := ""
		if n.Description != nil {
			desc = *n.Description
		}
		contrib := NotableContrib{
			Name:           key,
			AvatarURL:      notableOwnerAvatar(n),
			Organization:   n.IsInOrganization,
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
			contrib.Issues = issueCount(n)
			contrib.Pulls = pullCount(n)
		}
		byKey[key] = &contrib
		order = append(order, key)
	}

	out := make([]NotableContrib, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	if indepth {
		sortIndepth(out)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
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

// ownerTypeFilter reads plugin_notable_from (default "organization").
func ownerTypeFilter(in map[string]any) string {
	v := strings.ToLower(strings.TrimSpace(stringInput(in, "plugin_notable_from")))
	switch v {
	case "all", "organization", "user":
		return v
	default:
		return "organization"
	}
}

// ownerTypeMatches applies the from filter to a repository's
// isInOrganization flag.
func ownerTypeMatches(from string, isOrg bool) bool {
	switch from {
	case "all":
		return true
	case "user":
		return !isOrg
	default: // organization
		return isOrg
	}
}

// includeSelf reads plugin_notable_self (default no = exclude own repos).
func includeSelf(in map[string]any) bool {
	return pluginutil.Truthy(in["plugin_notable_self"])
}

// contributionTypes reads plugin_notable_types (default "commit"),
// mapping the comma-separated slugs to the GraphQL enum.
func contributionTypes(in map[string]any) []githubapi.RepositoryContributionType {
	raw := stringInput(in, "plugin_notable_types")
	if v, ok := in["plugin_notable_types"]; ok {
		if list, ok := v.([]string); ok {
			raw = strings.Join(list, ",")
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "commit"
	}
	out := make([]githubapi.RepositoryContributionType, 0, 3)
	seen := make(map[githubapi.RepositoryContributionType]struct{}, 3)
	for _, tok := range strings.Split(raw, ",") {
		var t githubapi.RepositoryContributionType
		switch strings.ToLower(strings.TrimSpace(tok)) {
		case "commit":
			t = githubapi.RepositoryContributionTypeCommit
		case "pull_request":
			t = githubapi.RepositoryContributionTypePullRequest
		case "issue":
			t = githubapi.RepositoryContributionTypeIssue
		default:
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		out = append(out, githubapi.RepositoryContributionTypeCommit)
	}
	return out
}

// skippedSet reads plugin_notable_skipped (newline- or comma-separated
// repository names / handles).
func skippedSet(in map[string]any) map[string]struct{} {
	set := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			set[strings.ToLower(s)] = struct{}{}
		}
	}
	switch v := in["plugin_notable_skipped"].(type) {
	case string:
		for _, line := range strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r'
		}) {
			add(line)
		}
	case []string:
		for _, s := range v {
			add(s)
		}
	}
	return set
}

// isSkipped reports whether a repository handle matches the skipped set,
// either by bare repository name or by full "owner/repo" handle.
func isSkipped(nameWithOwner string, skipped map[string]struct{}) bool {
	if len(skipped) == 0 {
		return false
	}
	full := strings.ToLower(nameWithOwner)
	if _, ok := skipped[full]; ok {
		return true
	}
	repo := strings.ToLower(notableRepoName(nameWithOwner))
	_, ok := skipped[repo]
	return ok
}

// notableLimit reads plugin_notable_limit (default 5).
func notableLimit(in map[string]any) int {
	if v, ok := in["plugin_notable_limit"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return defaultLimit
}

// stringInput returns the string value for key, or "".
func stringInput(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	if v, ok := in[key].(string); ok {
		return v
	}
	return ""
}

// notableRepoName returns the repository name portion of a
// "owner/repo" handle.
func notableRepoName(nameWithOwner string) string {
	if _, repo, ok := strings.Cut(nameWithOwner, "/"); ok {
		return repo
	}
	return nameWithOwner
}

func notableOwnerLogin(n *notableRepoNode) string {
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
// UserNotable query (avatarUrl(size: 64)).
func notableOwnerAvatar(n *notableRepoNode) string {
	if n == nil || n.Owner == nil {
		return ""
	}
	return n.Owner.GetAvatarUrl()
}

func notableCommitCount(ref *githubapi.UserNotableUserRepositoriesContributedToRepositoryConnectionNodesRepositoryDefaultBranchRef) int {
	if ref == nil || ref.Target == nil || *ref.Target == nil {
		return 0
	}
	commit, ok := (*ref.Target).(*githubapi.UserNotableUserRepositoriesContributedToRepositoryConnectionNodesRepositoryDefaultBranchRefTargetCommit)
	if !ok || commit.History == nil {
		return 0
	}
	return commit.History.TotalCount
}

func issueCount(n *notableRepoNode) int {
	if n == nil || n.Issues == nil {
		return 0
	}
	return n.Issues.TotalCount
}

func pullCount(n *notableRepoNode) int {
	if n == nil || n.PullRequests == nil {
		return 0
	}
	return n.PullRequests.TotalCount
}
