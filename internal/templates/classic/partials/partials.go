// Package partials hosts the partial functions assembled by the classic
// template. Each partial is a templates.PartialFunc that takes a
// PartialContext and returns the SVG fragment it owns.
//
// Partials follow three rules:
//
//  1. Empty when nil-safe: missing data (e.g. Data.User == nil) must
//     yield "" without panicking.
//  2. XML safety: dynamic strings flow through classic.EscapeXML.
//  3. Magnitude shortening: integer counts flow through
//     classic.FormatCount.
//
// The classic template owns the call order via partials/_.json.
package partials

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/templates"
)

// Introduction is a stub: the introduction plugin lands in M4. Until
// then the partial returns "" so the partial dispatch order stays
// stable.
func Introduction(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	if _, ok := pc.Data.GetPlugin("introduction"); !ok {
		return "", 0, nil
	}
	// Plugin will populate this in M4; for now we keep the structure
	// addressable but empty so the DOM does not gain a stray section.
	return "", 0, nil
}

// emptyPartial is the no-op fallback installed for the base.* static
// partial names by partials.init() below. It exists so the classic
// template's `partial X listed in _.json but not implemented` guard
// stays satisfied even in test binaries that compile classic without
// pulling in the internal/plugins/base package as a side-effect import.
//
// The owning plugin package (internal/plugins/base) overrides these
// entries from its own init() via Register (which overwrites by
// design), so at runtime the production binary always renders the
// real partial body when plugin_base* inputs are truthy.
func emptyPartial(_ context.Context, _ *templates.PartialContext) (string, int, error) {
	return "", 0, nil
}

// registry maps partial names (e.g. "plugin.languages") to their
// PartialFunc implementations. Populated by init() in this package for
// the M2-era `introduction` stub, and by per-plugin packages
// (internal/plugins/<name>/) via the Register entry point for M4
// plugin partials.
//
// #605: the three base.* identity-chrome partials are gone — the
// per-plugin `plugin.header` (extracted by #602) replaces base.header,
// and base.activity+community / base.repositories were superseded by
// the standalone activity / repositories plugin panels.
var registry = map[string]templates.PartialFunc{}

func init() {
	Register("introduction", Introduction)
	// Seed the base.* slots with no-op partials so the static
	// dispatcher does not error out when a test binary compiles the
	// classic template without the owning plugin packages as
	// side-effect imports. The plugin packages' init()s override these
	// entries with the real implementations whenever the binary links
	// them in (internal/plugins/base for the activity+community and
	// repositories partials; internal/plugins/header for base.header).
	Register("base.header", emptyPartial)
	Register("base.activity+community", emptyPartial)
	Register("base.repositories", emptyPartial)
}

// Register adds a PartialFunc under the given name. Subsequent calls
// with the same name overwrite the previous registration — this is the
// expected behavior for both the M2 init-based base partial setup and
// the M4 plugin partial registration path where each plugin package
// registers itself at process start. Not goroutine-safe; intended to
// run from init() only.
func Register(name string, fn templates.PartialFunc) {
	registry[name] = fn
}

// Lookup returns the registered partial by canonical name. M4 plugin
// partials register themselves via Register from their owning plugin
// package's init(). Returns (nil, false) for unknown names; the classic
// template treats that as a contract failure for _.json entries and as
// a silent skip for M4 plugin partials still in flight (US1/US2/US3
// land incrementally).
func Lookup(name string) (templates.PartialFunc, bool) {
	fn, ok := registry[name]
	return fn, ok
}
