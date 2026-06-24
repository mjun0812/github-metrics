// Package header owns the "header" plugin: avatar / login /
// follower-sponsor / 2-week commit calendar. It extracts the header
// card that was previously hard-wired as the base.header partial into
// a composable plugin so users can enable/disable it independently.
//
// Failure model: errors from Provider are plugin-local. A failed Profile
// or CommitCalendar call records the error on Data.Plugins[Name].Error
// and returns (nil, nil) so the engine continues rendering other plugins.
package header

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "header"

// Plugin is the singleton registered in the global plugin registry.
var Plugin plugins.Plugin = &headerPlugin{}

func init() {
	plugins.Register(Plugin)
}

type headerPlugin struct{}

func (*headerPlugin) Name() string                     { return Name }
func (*headerPlugin) Metadata() *config.PluginMetadata { return nil }

// Requires declares the Provider methods this plugin calls during Run.
func (*headerPlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyCommitCalendar,
	}
}

// Result is the JSON payload published under data.Plugins["header"].
type Result struct {
	Profile        *plugins.Profile              `json:"profile,omitempty"`
	CommitCalendar *plugins.ContributionCalendar `json:"commit_calendar,omitempty"`
	// Error records plugin-local failures so callers can surface them
	// without stopping the render pipeline.
	Error error `json:"-"`
}

// IsSkipped satisfies the classic dispatcher SkippableResult interface.
func (r *Result) IsSkipped() bool { return r == nil }

// Run fetches profile and commit-calendar data. On provider error the
// failure is recorded on the Result and (nil, nil) is returned so the
// engine continues with other plugins.
func (*headerPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Provider == nil {
		return &Result{}, nil
	}

	res := &Result{}

	profile, err := pc.Provider.Profile(ctx)
	if err != nil {
		res.Error = err
		pc.Data.SetPlugin(Name, res)
		return nil, nil //nolint:nilerr // plugin-local failure; do not propagate
	}
	res.Profile = profile

	cal, err := pc.Provider.CommitCalendar(ctx)
	if err != nil {
		res.Error = err
		pc.Data.SetPlugin(Name, res)
		return nil, nil //nolint:nilerr // plugin-local failure; do not propagate
	}
	res.CommitCalendar = cal

	return res, nil
}
