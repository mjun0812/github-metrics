// Package base owns the special "base" plugin: it runs before every
// other plugin and populates the shared Data.User / Data.Computed
// fields that downstream plugins depend on.
//
// In M1 the implementation covers the user-account and organization-
// account branches plus a single-page repository fetch. The full
// upstream behavior (bulk-then-fallback query, multi-page cursor
// traversal with batch-halving) is documented in
// specs/001-project-foundation/contracts/plugin-interface.md §5 and
// lands incrementally with the M4 plugin work that actually exercises
// every Computed field.
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

// Run dispatches by account kind. Each branch populates
// data.User and (when relevant) data.Computed.Repositories from the
// GraphQL client. M1 uses simple single-shot queries; the field-level
// fallback path described in the spec is documented but not yet
// exercised because the bulk UserX query lands with M4.
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
		// M7 territory; base does no work for repository templates.
		return nil, nil
	default:
		return nil, fmt.Errorf("base: unknown account kind %q", pc.Data.Account)
	}
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

	// Fetch the first page of repositories so downstream plugins can
	// observe Computed.Repositories.Count. The cursor-based loop with
	// batch-halving on timeout is a documented M4 follow-up.
	if reposLimit := repositoriesLimit(pc.Settings); reposLimit > 0 {
		if err := populateRepositories(ctx, pc, login, reposLimit, true); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (p *basePlugin) runOrganization(ctx context.Context, pc *plugins.PluginContext, login string) (any, error) {
	resp, err := pc.GraphQL.Organization(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("base: organization(%q): %w", login, err)
	}
	if resp == nil || resp.Organization == nil {
		return nil, fmt.Errorf("base: organization(%q): not found", login)
	}
	o := resp.Organization
	pc.Data.Account = plugins.AccountOrganization
	pc.Data.User = &plugins.User{
		Login:     o.Login,
		Name:      derefString(o.Name),
		AvatarURL: o.AvatarUrl,
	}

	if reposLimit := repositoriesLimit(pc.Settings); reposLimit > 0 {
		if err := populateRepositories(ctx, pc, login, reposLimit, false); err != nil {
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
