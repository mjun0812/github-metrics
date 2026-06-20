package pluginutil

import (
	"context"
	"fmt"

	"github.com/mjun0812/github-metrics/internal/plugins"
)

// ResolveProfile fetches the shared profile via pc.Provider, returning
// (nil, false) when the dataprovider reports an error. Callers can
// branch on the second return value to skip plugin work and let
// core.RunPlugins record the error on the plugin's Data.Plugins slot:
//
//	prof, ok := pluginutil.ResolveProfile(ctx, pc, "achievements")
//	if !ok {
//	    return nil, nil // error already attached via Provider failure
//	}
//
// When the Provider is nil (legacy code paths, unit tests that build
// PluginContext by hand) the helper returns (nil, false) without
// recording anything so the plugin behaves as before.
func ResolveProfile(ctx context.Context, pc *plugins.PluginContext, name string) (*plugins.Profile, bool) {
	if pc == nil || pc.Provider == nil {
		return nil, false
	}
	prof, err := pc.Provider.Profile(ctx)
	if err != nil {
		recordProviderError(pc, name, "profile", err)
		return nil, false
	}
	return prof, true
}

// ResolveUser fetches the user profile via pc.Provider. Returns
// (nil, false) on error (typed kind mismatch or fetch failure) so the
// caller can `return nil, nil` and let the orchestrator record the
// error onto the plugin's Data.Plugins slot.
func ResolveUser(ctx context.Context, pc *plugins.PluginContext, name string) (*plugins.User, bool) {
	if pc == nil || pc.Provider == nil {
		return nil, false
	}
	u, err := pc.Provider.User(ctx)
	if err != nil {
		recordProviderError(pc, name, "user", err)
		return nil, false
	}
	return u, true
}

// ResolveOrganization mirrors ResolveUser for the organization branch.
func ResolveOrganization(ctx context.Context, pc *plugins.PluginContext, name string) (*plugins.Organization, bool) {
	if pc == nil || pc.Provider == nil {
		return nil, false
	}
	o, err := pc.Provider.Organization(ctx)
	if err != nil {
		recordProviderError(pc, name, "organization", err)
		return nil, false
	}
	return o, true
}

// ResolveRepositories fetches the paged repository accumulator via
// pc.Provider. Returns (nil, false) on fetch error.
func ResolveRepositories(ctx context.Context, pc *plugins.PluginContext, name string) ([]plugins.Repository, bool) {
	if pc == nil || pc.Provider == nil {
		return nil, false
	}
	repos, err := pc.Provider.Repositories(ctx)
	if err != nil {
		recordProviderError(pc, name, "repositories", err)
		return nil, false
	}
	return repos, true
}

// ResolveCommitCalendar fetches the aggregated contribution-calendar
// payload via pc.Provider. Returns (nil, false) on fetch error and
// (nil, true) when the upstream simply has no calendar (organization
// account, fresh user) so callers can distinguish absence from failure.
func ResolveCommitCalendar(ctx context.Context, pc *plugins.PluginContext, name string) (*plugins.ContributionCalendar, bool) {
	if pc == nil || pc.Provider == nil {
		return nil, false
	}
	cal, err := pc.Provider.CommitCalendar(ctx)
	if err != nil {
		recordProviderError(pc, name, "commit-calendar", err)
		return nil, false
	}
	return cal, true
}

// recordProviderError stores the wrapped error on the plugin's slot
// inside pc.Data.Plugins, mirroring the convention core.RunPlugins
// applies when a plugin Run returns a non-nil error. It also appends
// the error to Data.Errors so engine.Compute's collectPluginErrors
// merges it into the final result.
func recordProviderError(pc *plugins.PluginContext, name, resource string, err error) {
	wrapped := fmt.Errorf("%s: %s: %w", name, resource, err)
	if pc.Data != nil {
		pc.Data.SetPlugin(name, wrapped)
		pc.Data.AppendError(wrapped)
	}
}
