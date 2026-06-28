// Package classic implements the M2 "classic" template — the default
// upstream GitHub-visual-identity SVG layout.
//
// classic.go is the templates.Template implementation. Partial
// functions live in classic/partials/, where text rendering helpers
// (EscapeXML, FormatCount) are also colocated so this package and the
// partials package compose without an import cycle.
package classic

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/mjun0812/github-metrics/assets"
	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
	"gopkg.in/yaml.v3"
)

// Name is the canonical template name used by callers (CLI --template,
// engine.Request.Template, registry key).
const Name = "classic"

// Template is the singleton exposed for tests / direct callers. The
// init() below also registers it through the templates.Register
// registry so engine.Compute can resolve it by name.
var Template templates.Template = &classicTemplate{}

func init() {
	t, err := newClassicTemplate()
	if err != nil {
		panic(fmt.Sprintf("classic: init: %v", err))
	}
	Template = t
	templates.Register(Template)
}

// classicTemplate is the templates.Template implementation. The
// embedded files are pulled from the package-wide assets.FS() so we
// stay consistent with the metadata/plugin loader.
type classicTemplate struct {
	fsys     fs.FS
	meta     *config.TemplateMetadata
	partials []string

	styles chrome.Styles
}

func newClassicTemplate() (*classicTemplate, error) {
	sub, err := fs.Sub(assets.FS(), "templates/classic")
	if err != nil {
		return nil, fmt.Errorf("classic: sub: %w", err)
	}
	rawMeta, err := fs.ReadFile(sub, "metadata.yml")
	if err != nil {
		return nil, fmt.Errorf("classic: read metadata.yml: %w", err)
	}
	var meta config.TemplateMetadata
	if uErr := yaml.Unmarshal(rawMeta, &meta); uErr != nil {
		return nil, fmt.Errorf("classic: parse metadata.yml: %w", uErr)
	}

	rawPartials, err := fs.ReadFile(sub, "partials/_.json")
	if err != nil {
		return nil, fmt.Errorf("classic: read partials/_.json: %w", err)
	}
	var partialNames []string
	if uErr := json.Unmarshal(rawPartials, &partialNames); uErr != nil {
		return nil, fmt.Errorf("classic: parse partials/_.json: %w", uErr)
	}
	return &classicTemplate{
		fsys:     sub,
		meta:     &meta,
		partials: partialNames,
	}, nil
}

func (t *classicTemplate) Name() string                       { return Name }
func (t *classicTemplate) Metadata() *config.TemplateMetadata { return t.meta }
func (t *classicTemplate) FS() fs.FS                          { return t.fsys }

// Check validates the requested account/format combination against the
// metadata-declared supports/formats lists.
func (t *classicTemplate) Check(_ map[string]any, account, format string) error {
	if err := templates.CheckAccount(t.meta, account); err != nil {
		return err
	}
	return templates.CheckFormat(t.meta, format)
}

// Run renders the classic SVG envelope. The pipeline is:
//
//  1. Open <svg width="480" height="99999" class="">
//  2. <defs><style><!-- fonts.css --></style></defs>
//  3. <style data-optimizable="true"><!-- style.css --></style>
//  4. <style><!-- extras placeholder --></style>
//  5. Open <foreignObject> + <div class="items-wrapper">
//  6. Concatenate partials in the declared order
//  7. Optional metadata <footer> when the metadata base section is enabled
//  8. Close the wrapper, <div id="metrics-end" />, foreignObject, svg
func (t *classicTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil {
		return "", fmt.Errorf("classic: nil PartialContext")
	}
	if err := t.styles.Load(t.fsys); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="99999" class="">`)
	fmt.Fprintf(&b, `<defs><style>%s</style></defs>`, t.styles.Fonts)
	fmt.Fprintf(&b, `<style data-optimizable="true">%s</style>`, t.styles.Style)
	b.WriteString(`<style></style>`)
	b.WriteString(`<foreignObject x="0" y="0" width="100%" height="100%">`)
	b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink" class="items-wrapper">`)

	// Resolve which base.* partials are enabled. The canonical input
	// surface is the six `chrome_<section>` booleans (#640). The
	// legacy `base` CSV / "input-absent → all sections" paths are
	// preserved as deprecated fallbacks inside ResolveBaseSections so
	// direct engine callers keep working until v3.
	baseSections := chrome.ResolveBaseSections(pc.Inputs)

	for _, name := range t.partials {
		if !partialEnabledByBase(name, baseSections) {
			continue
		}
		fn, ok := partials.Lookup(name)
		if !ok {
			// Partial declared in _.json but no implementation registered:
			// constitution principle III forbids carrying unadopted code,
			// so the manifest and the lookup must stay in sync. Surface
			// the mismatch loudly.
			return "", fmt.Errorf("classic: partial %q listed in _.json but not implemented", name)
		}
		frag, err := fn(ctx, pc)
		if err != nil {
			return "", fmt.Errorf("classic: partial %q: %w", name, err)
		}
		b.WriteString(frag)
	}

	// M4 plugin partial dispatcher
	//
	// Walks partials.PluginPartialOrder, applying the truthy gate, the
	// Skipped check, and the Lookup miss tolerance. Each non-empty
	// fragment is wrapped in
	//   <div class="plugin-<slug>" data-plugin="<slug>">...</div>
	// so downstream DOM diffing (data-changed mode, M6) can locate each
	// plugin's output unambiguously.
	for _, slug := range partials.PluginPartialOrder {
		if !chrome.TruthyInput(pc.Inputs, "plugin_"+slug) {
			continue
		}
		// Dedup: when the user enables both `chrome_header=yes` (or
		// the legacy `base=header`) and `plugin_header=yes`, the static
		// base.header partial above has already emitted the identity
		// card. Skipping the plugin entry here keeps the SVG to a
		// single <section data-section="header"> instead of
		// duplicating it.
		if slug == "header" {
			if _, headerEnabledByBase := baseSections["header"]; headerEnabledByBase {
				continue
			}
		}
		// Plugin result must exist and not be Skipped. We use the
		// minimal SkippableResult interface so the dispatcher does not
		// need a hard dependency on every plugin's Result type.
		if pc.Data == nil {
			continue
		}
		raw, present := pc.Data.GetPlugin(slug)
		if !present || raw == nil {
			continue
		}
		if sr, ok := raw.(interface{ IsSkipped() bool }); ok && sr.IsSkipped() {
			continue
		}
		fn, ok := partials.Lookup("plugin." + slug)
		if !ok {
			// Plugin partial not yet implemented (US1/US2/US3 land
			// these incrementally). Silently skip; the per-plugin task
			// will register Lookup before its integration test asserts
			// the wrapper.
			continue
		}
		frag, err := fn(ctx, pc)
		if err != nil {
			return "", fmt.Errorf("classic: plugin partial %q: %w", slug, err)
		}
		if frag == "" {
			// Double guard: a registered partial that chose to render
			// nothing (e.g. empty result list) emits no wrapper.
			continue
		}
		fmt.Fprintf(&b, `<div class="plugin-%s" data-plugin="%s">`,
			partials.EscapeXML(slug), partials.EscapeXML(slug))
		b.WriteString(frag)
		b.WriteString(`</div>`)
	}

	if footer := chrome.MetadataFooter(pc, baseSections, chrome.FooterOpts{IncludePrivateNotice: true}); footer != "" {
		b.WriteString(footer)
	}
	// #metrics-end sits at the very end of the foreignObject so the
	// chromedp trim measures the height *including* the footer. The
	// earlier placement (before the footer) clipped the metadata span
	// because the trim stopped above the section.
	b.WriteString(`<div id="metrics-end"></div>`)

	b.WriteString(`</div></foreignObject></svg>`)
	return b.String(), nil
}

// partialEnabledByBase reports whether the named partial should be
// rendered given the resolved section set. Only partials whose name
// starts with "base." (plus "introduction") are gated by this check;
// other partials (plugin.*) pass through unaffected.
//
// Mapping (the section names themselves now come from `chrome_<X>`
// booleans — see chrome.ResolveBaseSections):
//
//	introduction              → "introduction"
//	base.header               → "header"
//	base.activity+community   → "activity" OR "community"
//	base.repositories         → "repositories"
//
// base.header is rendered ahead of the plugin partials so the
// identity card always sits at the top of the SVG. The standalone
// `header` plugin still exists; the M4 dispatcher deduplicates the
// slug when sections["header"] is also enabled.
//
// `metadata` is gated separately by chrome.MetadataFooter.
//
// The base.activity+community / base.repositories partials no longer
// need their own `plugin_base*` sub-gates — the chrome-driven section
// set is the single gating surface (#640).
func partialEnabledByBase(name string, sections map[string]struct{}) bool {
	switch name {
	case "introduction":
		_, ok := sections["introduction"]
		return ok
	case "base.header":
		_, ok := sections["header"]
		return ok
	case "base.activity+community":
		_, a := sections["activity"]
		_, c := sections["community"]
		return a || c
	case "base.repositories":
		_, ok := sections["repositories"]
		return ok
	}
	// Non-base partials (anything else listed in _.json) render
	// unconditionally — they don't have a `base` gate semantic.
	return true
}
