package base

import (
	"context"
	"fmt"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// runOrganization fetches the organization base profile, drives the
// member-paging loop, and then folds the paged repository totals via
// populateRepositories.
//
// The accumulator semantics are deliberately defensive: a transient
// member-paging failure records a *RetryableError on Data.Errors and
// stops the loop but never aborts the request, so downstream plugins
// keep seeing the partial Organization payload.
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
	pc.Data.Organization = &plugins.Organization{
		Login:       o.Login,
		Name:        derefString(o.Name),
		Description: derefString(o.Description),
		AvatarURL:   o.AvatarUrl,
	}

	membersBatch := repositoriesLimit(pc.Settings) // upstream shares plugin_repositories_batch
	if membersBatch > 0 {
		if err := fetchMembers(ctx, pc, login, membersBatch); err != nil {
			return nil, err
		}
	}

	if reposLimit := repositoriesLimit(pc.Settings); reposLimit > 0 {
		if err := populateRepositories(ctx, pc, login, reposLimit, false); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// fetchMembers pages through membersWithRole, writing the accumulator
// into pc.Data.Organization. Transient failures stop the loop with a
// *RetryableError on Data.Errors so downstream plugins still see the
// partial member list.
func fetchMembers(ctx context.Context, pc *plugins.PluginContext, login string, batch int) error {
	if pc.Data.Organization == nil {
		// Defensive: runOrganization always primes this before calling
		// us, but the helper is exported package-internally for tests.
		pc.Data.Organization = &plugins.Organization{Login: login}
	}
	var cursor *string
	for {
		resp, err := pc.GraphQL.OrganizationMembers(ctx, login, batch, cursor)
		if err != nil {
			if isTransient(err) {
				pc.Data.AppendError(xerrors.NewRetryableError(
					fmt.Errorf("base: organization members paging: %w", err),
				))
				return nil
			}
			return fmt.Errorf("base: organizationMembers(%q): %w", login, err)
		}
		if resp == nil || resp.Organization == nil || resp.Organization.MembersWithRole == nil {
			return nil
		}
		conn := resp.Organization.MembersWithRole
		if pc.Data.Organization.MembersCount == 0 {
			pc.Data.Organization.MembersCount = conn.TotalCount
		}
		for _, node := range conn.Nodes {
			if node == nil {
				continue
			}
			pc.Data.Organization.Members = append(pc.Data.Organization.Members, memberFromNode(node))
		}
		pi := conn.PageInfo
		if !pi.HasNextPage || pi.EndCursor == nil || *pi.EndCursor == "" {
			return nil
		}
		next := *pi.EndCursor
		cursor = &next
	}
}

func memberFromNode(n *githubapi.OrganizationMembersOrganizationMembersWithRoleOrganizationMemberConnectionNodesUser) plugins.OrgMember {
	return plugins.OrgMember{
		Login:     n.Login,
		Name:      derefString(n.Name),
		AvatarURL: n.AvatarUrl,
	}
}
