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
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/assets"
	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
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

	mu     sync.Mutex
	styles cachedCSS
}

type cachedCSS struct {
	loaded bool
	fonts  string
	style  string
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

// Run renders the classic SVG envelope. The pipeline follows
// contracts/classic-template.md §1:
//
//  1. Open <svg width="480" height="99999" class="">
//  2. <defs><style><!-- fonts.css --></style></defs>
//  3. <style data-optimizable="true"><!-- style.css --></style>
//  4. <style><!-- extras placeholder --></style>
//  5. Open <foreignObject> + <div class="items-wrapper">
//  6. Concatenate partials in the declared order
//  7. Optional metadata <footer> when base.metadata input is true
//  8. Close the wrapper, <div id="metrics-end" />, foreignObject, svg
func (t *classicTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil {
		return "", fmt.Errorf("classic: nil PartialContext")
	}
	if err := t.loadStyles(); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="99999" class="">`)
	fmt.Fprintf(&b, `<defs><style>%s</style></defs>`, t.styles.fonts)
	fmt.Fprintf(&b, `<style data-optimizable="true">%s</style>`, t.styles.style)
	b.WriteString(`<style></style>`)
	b.WriteString(`<foreignObject x="0" y="0" width="100%" height="100%">`)
	b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink" class="items-wrapper">`)

	for _, name := range t.partials {
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
	// Contract: specs/004-m4-github-plugins/contracts/partial-classic-m4.md §3
	//
	// Walks partials.PluginPartialOrder, applying the truthy gate, the
	// Skipped check, and the Lookup miss tolerance described in the
	// contract. Each non-empty fragment is wrapped in
	//   <div class="plugin-<slug>" data-plugin="<slug>">...</div>
	// so downstream DOM diffing (data-changed mode, M6) can locate each
	// plugin's output unambiguously.
	for _, slug := range partials.PluginPartialOrder {
		if !truthyInput(pc.Inputs, "plugin_"+slug) {
			continue
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

	if footer := metadataFooter(pc); footer != "" {
		b.WriteString(`<div id="metrics-end"></div>`)
		b.WriteString(footer)
	} else {
		b.WriteString(`<div id="metrics-end"></div>`)
	}

	b.WriteString(`</div></foreignObject></svg>`)
	return b.String(), nil
}

// loadStyles lazily reads fonts.css + style.css from the embedded FS
// the first time Run is called. Held under a mutex so concurrent
// Compute calls do not race on the first hit.
func (t *classicTemplate) loadStyles() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.styles.loaded {
		return nil
	}
	fonts, err := fs.ReadFile(t.fsys, "fonts.css")
	if err != nil {
		return fmt.Errorf("classic: read fonts.css: %w", err)
	}
	style, err := fs.ReadFile(t.fsys, "style.css")
	if err != nil {
		return fmt.Errorf("classic: read style.css: %w", err)
	}
	t.styles = cachedCSS{
		loaded: true,
		fonts:  string(fonts),
		style:  string(style),
	}
	return nil
}

// metadataFooter renders the optional metadata <footer> when the
// `base.metadata` input is truthy. Contract: contracts/classic-template.md §4.
func metadataFooter(pc *templates.PartialContext) string {
	if pc == nil || pc.Inputs == nil {
		return ""
	}
	if !truthyInput(pc.Inputs, "base.metadata") {
		return ""
	}
	tz := ""
	if pc.Data != nil {
		tz = pc.Data.Config.Timezone.Name
	}
	var b strings.Builder
	b.WriteString(`<footer>`)
	if pc.Data != nil && pc.Data.Account == plugins.AccountUser {
		b.WriteString(`<span>These metrics include private contributions</span>`)
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if tz != "" && tz != "UTC" {
		fmt.Fprintf(&b, `<span>Last updated %s (timezone %s) with mjun0812/github-metrics@%s</span>`,
			stamp, partials.EscapeXML(tz), partials.EscapeXML(engine.Version()))
	} else {
		fmt.Fprintf(&b, `<span>Last updated %s with mjun0812/github-metrics@%s</span>`,
			stamp, partials.EscapeXML(engine.Version()))
	}
	b.WriteString(`</footer>`)
	return b.String()
}

func truthyInput(in map[string]any, key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "yes" || x == "1"
	default:
		return false
	}
}
