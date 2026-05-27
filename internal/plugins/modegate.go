package plugins

import (
	"fmt"
	"log/slog"
)

// ModeGate centralizes the "plugin X only supports account-mode Y"
// guard that every per-plugin Run() needs at the top of its body.
//
// Each adopted plugin is meaningful in either user mode, repository
// mode, or both. The engine itself does not refuse to dispatch a
// plugin against the "wrong" mode — historically the plugins silently
// emitted empty / chrome-only output when invoked under a mode they
// did not cover. That produced byte-identical repository-mode docs
// samples for 14 of the 19 adopted plugins (e.g. an
// `--template repository --plugin plugin_achievements=yes` render that
// was byte-identical to the plain `--template repository` chrome).
// The mode gate surfaces this state to operators by emitting a single
// WARN line and to downstream renderers by returning a descriptive
// Skipped reason the partial can display verbatim.
//
// The two helpers (RequireUserMode / RequireRepoMode) do not know
// the concrete Result struct — every plugin has its own — so they
// return (reason, skip). The plugin maps the reason into its own
// Result.SkippedReason field and returns from Run().

// RequireUserMode returns a descriptive Skipped reason and skip=true
// when the engine populated Data.Repo (i.e. we are rendering the
// repository template), since plugins that aggregate per-user signals
// have nothing meaningful to compute against a single repository.
// The helper logs one WARN line via pc.Logger (or slog.Default when
// Logger is nil) so operators see the mode mismatch in CI logs.
//
// slug must be the canonical plugin name (e.g. "achievements"). The
// caller passes its own Name constant.
//
// Usage:
//
//	if reason, skip := plugins.RequireUserMode(pc, Name); skip {
//	    return &Result{Skipped: true, SkippedReason: reason}, nil
//	}
func RequireUserMode(pc *PluginContext, slug string) (string, bool) {
	if pc == nil || pc.Data == nil {
		return "", false
	}
	if pc.Data.RepoRef() == nil {
		return "", false
	}
	reason := fmt.Sprintf(
		"plugin %s is only supported in user mode (current mode: repository)",
		slug,
	)
	loggerFor(pc).Warn(reason, "plugin", slug, "mode", "repository", "supported", "user")
	return reason, true
}

// RequireRepoMode is the mirror of RequireUserMode: it returns a
// descriptive Skipped reason and skip=true when the engine did NOT
// populate Data.Repo (i.e. we are rendering the classic / user
// template) for a plugin that only has meaning against a single
// repository (today: contributors).
func RequireRepoMode(pc *PluginContext, slug string) (string, bool) {
	if pc == nil || pc.Data == nil {
		return "", false
	}
	if pc.Data.RepoRef() != nil {
		return "", false
	}
	reason := fmt.Sprintf(
		"plugin %s is only supported in repository mode (current mode: user)",
		slug,
	)
	loggerFor(pc).Warn(reason, "plugin", slug, "mode", "user", "supported", "repository")
	return reason, true
}

// loggerFor returns pc.Logger when non-nil, falling back to
// slog.Default(). Plugins do not always carry a logger in tests, so
// the mode gate must not nil-panic.
func loggerFor(pc *PluginContext) *slog.Logger {
	if pc != nil && pc.Logger != nil {
		return pc.Logger
	}
	return slog.Default()
}
