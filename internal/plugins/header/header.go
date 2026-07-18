// Package header owns the "header" plugin: avatar / login /
// follower / following / 2-week commit calendar. It extracts the
// header card that was previously hard-wired as the base.header
// partial into a composable plugin so users can enable/disable it
// independently.
//
// Failure model: errors from Provider are plugin-local. A failed Profile
// or CommitCalendar call records the error on both Result.Error (for
// inspection) and the shared Data accumulator via recordError, then
// returns the Result with a nil error so the engine keeps rendering other
// plugins. Threading it onto Data is what lets engine.collectPluginErrors
// log the failure and honour plugins_errors_fatal.
package header

import (
	"context"
	"fmt"

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

func (*headerPlugin) Name() string { return Name }

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
// failure is recorded on the Result and the populated Result is
// returned (with nil error) so core.RunPlugins stores it under
// data.Plugins["header"] and the render pipeline keeps going.
//
// The runner (internal/plugins/core.RunPlugins) unconditionally writes
// pc.Data.SetPlugin(Name, result) using whatever this function returns,
// so we must NOT call SetPlugin from inside Run — a nil return would
// overwrite the error-state Result and silently drop the failure.
func (*headerPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Provider == nil {
		return &Result{}, nil
	}
	// Auto-enable (#640): only fetch when the user enabled the header
	// plugin directly OR opted into the chrome_header section. The
	// legacy `plugin_base=yes` master switch is honoured as v2 compat
	// while no chrome_* input is declared.
	if !runEnabledForInputs(pc.Inputs) {
		return &Result{}, nil
	}

	res := &Result{}

	profile, err := pc.Provider.Profile(ctx)
	if err != nil {
		res.Error = err
		recordError(pc, err)
		return res, nil //nolint:nilerr // plugin-local failure: recordError threads it onto Data.Errors; returning nil lets the partial degrade without aborting the render.
	}
	res.Profile = profile

	cal, err := pc.Provider.CommitCalendar(ctx)
	if err != nil {
		res.Error = err
		recordError(pc, err)
		return res, nil //nolint:nilerr // same plugin-local failure contract; do not propagate so other plugins still render.
	}
	res.CommitCalendar = cal

	return res, nil
}

// recordError threads a plugin-local Provider failure onto the shared
// Data error accumulator so engine.collectPluginErrors surfaces it in the
// run log and honours plugins_errors_fatal. Without this the failure
// would live only on Result.Error (json:"-"), invisible to logs, the
// error list, and the exit code.
func recordError(pc *plugins.PluginContext, err error) {
	if pc == nil || pc.Data == nil || err == nil {
		return
	}
	pc.Data.AppendError(fmt.Errorf("%s: %w", Name, err))
}
