// Package partials hosts the partial functions assembled by the classic
// template. Each partial is a templates.PartialFunc that takes a
// PartialContext and returns the SVG fragment it owns.
//
// Partials follow three rules:
//
//  1. Empty when nil-safe: missing data (e.g. the source plugin Result
//     is absent) must yield "" without panicking.
//  2. XML safety: dynamic strings flow through partials.EscapeXML.
//  3. Magnitude shortening: integer counts flow through
//     partials.FormatCount.
//
// After #602 only the `introduction` stub remains as a static partial
// here. The legacy `base.header` / `base.activity+community` /
// `base.repositories` partials were retired: header moved to the
// `header` plugin (`internal/plugins/header/render.go`), and the other
// two were dropped outright because they exist purely as inline
// summaries inside the combined classic card and are dead weight in the
// per-plugin-SVG embedding workflow that github-metrics targets.
package partials

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/templates"
)

// Introduction is a stub: the introduction plugin would land in a
// future milestone. Until then the partial returns "" so the partial
// dispatch order stays stable.
func Introduction(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	if _, ok := pc.Data.GetPlugin("introduction"); !ok {
		return "", nil
	}
	// Plugin would populate this; for now we keep the structure
	// addressable but empty so the DOM does not gain a stray section.
	return "", nil
}

// registry maps partial names (e.g. "introduction" or
// "plugin.languages") to their PartialFunc implementations. Populated
// by init() in this package for the static partials, and by per-plugin
// packages (internal/plugins/<name>/) via the Register entry point.
var registry = map[string]templates.PartialFunc{}

func init() {
	Register("introduction", Introduction)
}

// Register adds a PartialFunc under the given name. Subsequent calls
// with the same name overwrite the previous registration — this is the
// expected behavior for both the static init-based setup and the
// plugin partial registration path where each plugin package registers
// itself at process start. Not goroutine-safe; intended to run from
// init() only.
func Register(name string, fn templates.PartialFunc) {
	registry[name] = fn
}

// Lookup returns the registered partial by canonical name. Static
// partials are registered during package init(); plugin partials
// register themselves via Register from their owning plugin package's
// init(). Returns (nil, false) for unknown names; the classic template
// treats that as a contract failure for _.json entries and as a silent
// skip for plugin partials.
func Lookup(name string) (templates.PartialFunc, bool) {
	fn, ok := registry[name]
	return fn, ok
}
