// Package header owns the M4 "header" plugin extracted from `base`
// in #602. It produces the top-of-card identity block — avatar / display
// name, joined-GitHub age, follower/following counters, the trailing
// two-week commit calendar, and a "contributed to N repositories" row —
// historically rendered by upstream `assets/templates/classic/partials/
// base.header.ejs` and the in-tree `BaseHeader` partial.
//
// Unlike the legacy partial, this plugin pulls its inputs through
// pc.Provider (Profile + CommitCalendar) instead of reading eagerly-
// populated pc.Data.User fields. That makes the header card composable
// with the per-plugin SVG embedding workflow (mjun0812/mjun0812 README
// style): users can embed `plugin-header.svg` standalone without paying
// for the every-plugin base fetch.
//
// The render.go file in this package registers the `plugin.header`
// partial that the classic template dispatches.
package header

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// Name is the canonical plugin slug.
const Name = "header"

// Plugin is the singleton registered with the global plugin registry.
var Plugin plugins.Plugin = &headerPlugin{}

func init() {
	plugins.Register(Plugin)
}

type headerPlugin struct{}

func (*headerPlugin) Name() string                     { return Name }
func (*headerPlugin) Metadata() *config.PluginMetadata { return nil }

// Requires reports the Provider data sources Run reads. The header card
// is account-kind agnostic (user + organization both render); Profile
// returns the discriminated union and CommitCalendar feeds the trailing
// two-week mini calendar.
func (*headerPlugin) Requires() []plugins.DataKey {
	return []plugins.DataKey{
		plugins.KeyProfile,
		plugins.KeyCommitCalendar,
	}
}

// Result is the JSON payload published under data.Plugins["header"].
// The render partial reads it via PartialContext.Data.GetPlugin.
//
// Skipped surfaces the dispatcher carve-out for organization-only
// fixtures that lack the user-flavoured Profile shape, mirroring the
// other plugins' Skipped contract; render.go returns "" when set.
type Result struct {
	Skipped       bool   `json:"skipped,omitempty"`
	SkippedReason string `json:"-"`

	// Profile is the discriminated union from Provider.Profile. The
	// render path branches on Profile.Kind to pick the user / org
	// rendering, mirroring upstream `base.header.ejs`.
	Profile *plugins.Profile `json:"profile,omitempty"`

	// Calendar carries the trailing-N-days slice of the user's
	// contribution calendar. nil for organizations (no calendar data)
	// and when GraphQL returned no data.
	Calendar []plugins.ContributionDay `json:"calendar,omitempty"`
}

// IsSkipped lets the classic dispatcher detect the skipped path.
func (r *Result) IsSkipped() bool { return r != nil && r.Skipped }

// headerCalendarDays mirrors upstream `core/index.mjs`'s `slice(-14)`
// over the flattened day list (see `header.ejs`).
const headerCalendarDays = 14

// Run fetches the Profile + CommitCalendar via Provider and packages
// them into a Result. Plugin-local failures are surfaced as an error
// return so the engine can decide whether to short-circuit the run;
// successful runs always return a non-nil Result so the dispatcher can
// distinguish "ran but skipped" from "did not run".
//
// Header is a user/organization-only plugin: the repository template
// owns its own `base.header` chrome (internal/templates/repository/
// partials/partials.go) and does not consume this plugin's Result. We
// short-circuit in repository mode so the JSON output for repository
// renders does not gain a stray `plugins.header` entry.
func (*headerPlugin) Run(ctx context.Context, pc *plugins.PluginContext) (any, error) {
	if pc == nil || pc.Provider == nil {
		// Legacy harness without a Provider: hand back a skipped
		// Result so the dispatcher renders nothing.
		return &Result{Skipped: true, SkippedReason: "no provider"}, nil
	}
	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
		return &Result{Skipped: true, SkippedReason: reason}, nil
	}
	profile, err := pc.Provider.Profile(ctx)
	if err != nil {
		return nil, err
	}
	cal, err := pc.Provider.CommitCalendar(ctx)
	if err != nil {
		// CommitCalendar failures are degraded behaviour, not fatal —
		// the header still renders identity rows from the Profile. We
		// intentionally do NOT AppendError here: the indepth GraphQL
		// query is best-effort and any plugin that *requires* the
		// calendar (e.g. calendar / habits / isocalendar) surfaces the
		// error from its own Run. Silently nil the calendar so the
		// render path hides the calendar row.
		cal = nil
	}
	return &Result{
		Profile:  profile,
		Calendar: trailingContributionDays(cal, headerCalendarDays),
	}, nil
}

// trailingContributionDays flattens
// `ContributionCalendar.Weeks` into a chronological day list and
// returns the trailing `n` days, mirroring upstream
// `core/index.mjs`'s `slice(-14)`. Returns nil when cal is nil so the
// partial hides the calendar row instead of rendering empty cells.
func trailingContributionDays(cal *plugins.ContributionCalendar, n int) []plugins.ContributionDay {
	if cal == nil || len(cal.Weeks) == 0 {
		return nil
	}
	days := make([]plugins.ContributionDay, 0, len(cal.Weeks)*7)
	for _, w := range cal.Weeks {
		days = append(days, w.Days...)
	}
	if len(days) == 0 {
		return nil
	}
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	return days
}
