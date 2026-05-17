// Package base owns the special "base" plugin: it runs before every
// other plugin and populates the shared Data.User / Data.Organization /
// Data.Computed fields that downstream plugins depend on.
//
// The M4 implementation covers:
//   - user-account branch (runUser) and organization-account branch
//     (runOrganization)
//   - repository paging with batch-halving on transient 5xx / timeout
//     (repositories.go), writing both totals into Computed.Repositories
//     and the per-node accumulator into Computed.RepositoryList
//   - indepth GraphQL query (indepth.go) that augments Computed with
//     contribution-calendar / commits / issues / PR totals when at
//     least one indepth-dependent plugin is enabled
//
// The repository-account branch (templates that target a single
// repository) is M7 territory and currently returns (nil, nil).
// Contracts: specs/004-m4-github-plugins/contracts/plugin-base-extension.md.
package base

import (
	"context"
	"fmt"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "base"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &basePlugin{}

func init() {
	plugins.Register(Plugin)
}

type basePlugin struct{}

func (p *basePlugin) Name() string                     { return Name }
func (p *basePlugin) Metadata() *config.PluginMetadata { return nil }

// Run dispatches by account kind. Each branch populates data.User /
// data.Organization and data.Computed.{Repositories, RepositoryList,
// ContributionCalendar, TotalCommits / Issues / PullRequests} from the
// GraphQL client. The full upstream-compatible behavior (batch-halving
// paging, organization branch, indepth augmentation) lands with M4 per
// specs/004-m4-github-plugins/contracts/plugin-base-extension.md.
func (p *basePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Data == nil {
		return nil, fmt.Errorf("base: nil PluginContext or Data")
	}
	if pc.GraphQL == nil {
		return nil, fmt.Errorf("base: nil GraphQL client")
	}
	login := loginFromInputs(pc.Inputs)
	if login == "" {
		return nil, fmt.Errorf("base: input %q is required", "user")
	}

	switch pc.Data.Account {
	case "", plugins.AccountUser:
		return p.runUser(ctx, pc, login)
	case plugins.AccountOrganization:
		return p.runOrganization(ctx, pc, login)
	case plugins.AccountRepository:
		return p.runRepository(ctx, pc, login)
	default:
		return nil, fmt.Errorf("base: unknown account kind %q", pc.Data.Account)
	}
}

// runRepository is the M7 base-plugin entry point for the
// repository template. It runs the same user fetch as runUser
// (downstream plugins still need data.User populated for things like
// avatar / sponsorshipsAsMaintainer) and then layers the single-repo
// fetch on top.
func (p *basePlugin) runRepository(ctx context.Context, pc *plugins.PluginContext, login string) (any, error) {
	// Fetch user first — downstream plugins + the repository template
	// header still need data.User.AvatarURL, etc.
	if _, err := p.runUser(ctx, pc, login); err != nil {
		return nil, err
	}
	pc.Data.Account = plugins.AccountRepository

	repo := repoFromInputs(pc.Inputs)
	if repo == "" {
		return nil, fmt.Errorf("base: repository template requires `repo` input")
	}

	r, err := FetchRepo(ctx, login, repo, pc.REST, pc.GraphQL)
	if err != nil {
		return nil, err
	}
	// Mirror upstream template.mjs:21 — copy maintainer sponsorships
	// from the user payload so the sponsors partial can render. The
	// sponsors plugin (M4) is the source of truth for the count when
	// it runs; until then we leave the field zero.
	pc.Data.SetRepo(r)
	return nil, nil
}

// repoFromInputs returns the `repo` input value, or "" when unset.
func repoFromInputs(inputs map[string]any) string {
	if inputs == nil {
		return ""
	}
	v, ok := inputs["repo"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (p *basePlugin) runUser(ctx context.Context, pc *plugins.PluginContext, login string) (any, error) {
	resp, err := pc.GraphQL.User(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("base: user(%q): %w", login, err)
	}
	if resp == nil || resp.User == nil {
		return nil, fmt.Errorf("base: user(%q): not found", login)
	}
	u := resp.User
	pc.Data.Account = plugins.AccountUser
	pc.Data.User = &plugins.User{
		Login:     u.Login,
		Name:      derefString(u.Name),
		AvatarURL: u.AvatarUrl,
	}

	// M4: walk the entire repository connection with batch-halving on
	// transient 5xx / timeout. Downstream plugins consume both the
	// totals (Count, Stargazers, ...) and the per-node accumulator
	// (RepositoryList).
	if reposLimit := repositoriesLimit(pc.Settings); reposLimit > 0 {
		if err := populateRepositories(ctx, pc, login, reposLimit, true); err != nil {
			return nil, err
		}
	}

	// Trigger the indepth GraphQL query when at least one indepth-
	// dependent plugin is enabled. Per the contract this stays best-
	// effort: indepth failures land on Data.Errors and the standard
	// fields still surface.
	if indepthTriggered(pc.Inputs) {
		if err := runIndepth(ctx, pc, login); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// loginFromInputs returns the configured GitHub login.
func loginFromInputs(inputs map[string]any) string {
	if inputs == nil {
		return ""
	}
	if v, ok := inputs["user"].(string); ok && v != "" {
		return v
	}
	if v, ok := inputs["login"].(string); ok {
		return v
	}
	return ""
}

// repositoriesLimit reads Settings.Repositories with a default of 100
// to match upstream behavior.
func repositoriesLimit(s *config.Settings) int {
	if s == nil {
		return 100
	}
	if s.Repositories > 0 {
		return s.Repositories
	}
	return 100
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
