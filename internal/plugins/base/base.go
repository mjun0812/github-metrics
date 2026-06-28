// Package base owns the "base" plugin — the activity / community and
// repositories summary panels that originally lived inside the upstream
// `base` chrome. The pre-refactor base plugin was deleted by #623 along
// with its EJS partials; this plugin restores the missing visual
// surface as an opt-in plugin that reads exclusively from the shared
// dataprovider.Provider so the refactor wins (no fetching duplication)
// stay intact.
//
// Two partials are registered statically alongside `introduction`:
//
//   - base.activity+community — activity counters (commits / PRs / issues
//     / comments) + community stats (orgs / sponsoring / starred /
//     watching / following).
//   - base.repositories — the repositories summary (license preference /
//     releases / packages / disk usage / sponsors / stargazers / forks /
//     watchers).
//
// Failure model: errors from Provider are plugin-local. A failed Profile
// or RepositorySummary call records the error on Result.Error and the
// populated Result is returned (with nil error) so the engine continues
// rendering other plugins while the failure stays inspectable.
package base

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "base"

// Plugin is the singleton registered in the global plugin registry.
var Plugin plugins.Plugin = &basePlugin{}

func init() {
	plugins.Register(Plugin)
}

type basePlugin struct{}

func (*basePlugin) Name() string                     { return Name }
func (*basePlugin) Metadata() *config.PluginMetadata { return nil }

// Requires declares the Provider methods this plugin calls during Run.
func (*basePlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyRepositorySummary,
	}
}

// Result is the JSON payload published under data.Plugins["base"]. It
// snapshots the aggregated counters the partials render so they can
// emit their fragments without re-querying Provider.
type Result struct {
	Profile           *plugins.Profile              `json:"profile,omitempty"`
	RepositorySummary *plugins.ComputedRepositories `json:"repository_summary,omitempty"`
	// Error records plugin-local failures so callers can surface them
	// without stopping the render pipeline.
	Error error `json:"-"`
}

// IsSkipped satisfies the classic dispatcher SkippableResult interface.
// The base plugin uses static partials (not the plugin partial
// dispatcher), so this exists purely for the SetPlugin contract — the
// partials guard themselves on nil Result / nil Profile / nil Summary.
func (r *Result) IsSkipped() bool { return r == nil }

// Run fetches Profile and RepositorySummary from Provider and packs
// them into a Result. On Provider error the failure is recorded on the
// Result and the populated Result is returned (with nil error) so
// core.RunPlugins stores it under data.Plugins["base"] and the render
// pipeline keeps going.
//
// The runner (internal/plugins/core.RunPlugins) unconditionally writes
// pc.Data.SetPlugin(Name, result) using whatever this function returns,
// so we must NOT call SetPlugin from inside Run — a nil return would
// overwrite the error-state Result and silently drop the failure.
func (*basePlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Provider == nil {
		return &Result{}, nil
	}

	res := &Result{}

	profile, err := pc.Provider.Profile(ctx)
	if err != nil {
		res.Error = err
		return res, nil //nolint:nilerr // plugin-local failure: surface via res.Error so RunPlugins records it and the partial can degrade gracefully without aborting the render.
	}
	res.Profile = profile

	summary, err := pc.Provider.RepositorySummary(ctx)
	if err != nil {
		res.Error = err
		return res, nil //nolint:nilerr // same plugin-local failure contract; do not propagate so other plugins still render.
	}
	res.RepositorySummary = summary

	return res, nil
}
