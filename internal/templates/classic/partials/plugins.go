// M4 plugin partial conventions
// =============================
//
// classic.go's M4 dispatcher iterates PluginPartialOrder in order and,
// for each slug whose `plugin_<slug>` input is truthy and whose
// pc.Data.Plugins[slug] result is non-Skipped, calls Lookup("plugin." +
// slug) to obtain the partial function. Each plugin partial returns an
// HTML fragment that classic.go wraps in:
//
//	<div class="plugin-<slug>" data-plugin="<slug>">...</div>
//
// Conventions for adding a new plugin partial (US1/US2/US3 tasks):
//
//  1. Add the implementation function to internal/templates/classic/partials/.
//     Name it after the slug in PascalCase (Languages, Activity, ...).
//  2. Register it via partials.Register("plugin."+slug, ...) from the
//     owning plugin package's init() function.
//  3. Inside the function, return ("", 0, nil) when pc or pc.Data is
//     nil, when pc.Data.Plugins[slug] is missing or asserted to *T but
//     .Skipped == true, or when the resulting fragment would be empty.
//     Empty returns are the canonical Skipped signal at this layer.
//  4. Escape dynamic strings with EscapeXML; format integer counts via
//     FormatCount; reuse the M2 helpers in text.go.
//
// Height reporting (Phase B0, #409): the middle return value is the
// vertical px this partial consumes. Today every partial returns 0 =
// "not self-reported" — the fragment is HTML laid out by the outer
// foreignObject. When a partial is converted to a self-laying-out native
// SVG (Phase B1-B7), return the exact consumed height there instead of 0
// so the template can eventually sum heights and size the root <svg>
// (Phase C). See templates.PartialFunc for the full semantics.

package partials

// PluginPartialOrder defines the M4 plugin partial render order. It
// mirrors the upstream lowlighter/metrics classic ordering: P1 MVP
// 5 plugins first, then P2 GraphQL/REST 12 plugins, then P3
// heavy 4 plugins. classic.go iterates this slice; slugs whose partial
// is not yet registered are silently skipped during incremental M4
// landing so the build stays green between US1/US2/US3 PRs.
var PluginPartialOrder = []string{
	// Identity block — always at the top so the profile header
	// stays above per-plugin cards.
	"header",
	// P1 MVP — US1
	"languages",
	"activity",
	"achievements",
	"repositories",
	"isocalendar",
	// P2 GraphQL/REST — US2
	"calendar",
	"habits",
	"stars",
	"people",
	"notable",
	"contributors",
	"reactions",
	"sponsors",
	"sponsorships",
	"stargazers",
	"traffic",
	// P3 heavy — US3
	"topics",
	"starlists",
	// Note: languages.recent / languages.indepth are sub-modes of the
	// `languages` slug, not separate dispatch entries.
}
