// Package repository implements the M7 "repository" template — the
// upstream-equivalent SVG layout focused on a single repository
// instead of a user profile.
//
// Like the M2 classic template, this package owns the template
// scaffolding (root SVG, fonts/style/extras blocks, partial dispatch)
// and delegates per-section rendering to partial functions registered
// in the shared partials registry. Plugin partials (`languages`,
// `activity`, ...) are reused as-is — the per-plugin packages register
// them once at process start, and both templates look them up from
// the same registry.
package repository

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
	classicpart "github.com/mjun0812/github-metrics/internal/templates/classic/partials"
	repopart "github.com/mjun0812/github-metrics/internal/templates/repository/partials"

	"gopkg.in/yaml.v3"
)

// Name is the canonical template name.
const Name = "repository"

// Template is the singleton exposed for direct callers + tests. The
// init() below also registers it through templates.Register so
// engine.Compute can resolve it by name.
var Template templates.Template = &repositoryTemplate{}

func init() {
	t, err := newRepositoryTemplate()
	if err != nil {
		panic(fmt.Sprintf("repository: init: %v", err))
	}
	Template = t
	templates.Register(Template)
}

type repositoryTemplate struct {
	fsys     fs.FS
	meta     *config.TemplateMetadata
	partials []string

	styles chrome.Styles
}

func newRepositoryTemplate() (*repositoryTemplate, error) {
	sub, err := fs.Sub(assets.FS(), "templates/repository")
	if err != nil {
		return nil, fmt.Errorf("repository: sub: %w", err)
	}
	rawMeta, err := fs.ReadFile(sub, "metadata.yml")
	if err != nil {
		return nil, fmt.Errorf("repository: read metadata.yml: %w", err)
	}
	var meta config.TemplateMetadata
	if uErr := yaml.Unmarshal(rawMeta, &meta); uErr != nil {
		return nil, fmt.Errorf("repository: parse metadata.yml: %w", uErr)
	}
	rawPartials, err := fs.ReadFile(sub, "partials/_.json")
	if err != nil {
		return nil, fmt.Errorf("repository: read partials/_.json: %w", err)
	}
	var partialNames []string
	if uErr := json.Unmarshal(rawPartials, &partialNames); uErr != nil {
		return nil, fmt.Errorf("repository: parse partials/_.json: %w", uErr)
	}
	return &repositoryTemplate{
		fsys:     sub,
		meta:     &meta,
		partials: partialNames,
	}, nil
}

func (t *repositoryTemplate) Name() string                       { return Name }
func (t *repositoryTemplate) Metadata() *config.TemplateMetadata { return t.meta }
func (t *repositoryTemplate) FS() fs.FS                          { return t.fsys }

// Check validates the requested account/format combination against
// the metadata-declared supports/formats lists, plus the M7-specific
// requirement that the `repo` input be set.
func (t *repositoryTemplate) Check(inputs map[string]any, account, format string) error {
	if err := templates.CheckAccount(t.meta, account); err != nil {
		return err
	}
	if err := templates.CheckFormat(t.meta, format); err != nil {
		return err
	}
	if inputs == nil {
		return fmt.Errorf("repository: requires non-empty `repo` input (set INPUT_REPO or --repo)")
	}
	repo, _ := inputs["repo"].(string)
	if strings.TrimSpace(repo) == "" {
		return fmt.Errorf("repository: requires non-empty `repo` input (set INPUT_REPO or --repo)")
	}
	return nil
}

// Run renders the repository SVG envelope. Mirrors the classic
// template's scaffolding (root SVG + fonts/style + partial dispatch)
// but uses the repository-specific partial set declared by `_.json`.
//
// Plugin partials are looked up in the shared partials registry; the
// 6 M4 plugins that overlap with the repository template (languages,
// stargazers, people, activity, contributors, sponsors)
// register their plugin partials globally at import time, so this
// template inherits them automatically once their packages are imported
// via `cmd/metrics-action/plugins.go`.
func (t *repositoryTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil {
		return "", fmt.Errorf("repository: nil PartialContext")
	}
	classicFS, err := fs.Sub(assets.FS(), "templates/classic")
	if err != nil {
		return "", fmt.Errorf("repository: locate classic fs: %w", err)
	}
	if err := t.styles.Load(classicFS); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="99999" class="">`)
	fmt.Fprintf(&b, `<defs><style>%s</style></defs>`, t.styles.Fonts)
	fmt.Fprintf(&b, `<style data-optimizable="true">%s</style>`, t.styles.Style)
	b.WriteString(`<style></style>`)
	b.WriteString(`<foreignObject x="0" y="0" width="100%" height="100%">`)
	b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink" class="items-wrapper">`)

	// Resolve which base.* sections are enabled. Mirrors the classic
	// template: each section opts in via its `chrome_<section>`
	// boolean (#640); v3.0 (#649) dropped the legacy `base`=CSV /
	// default-all fallbacks.
	baseSections := chrome.ResolveBaseSections(pc.Inputs)

	// Dispatch partials in `_.json` order. Unknown / not-yet-adopted
	// partial names are silently skipped (constitution III: zero
	// unadopted code; the manifest stays full upstream parity).
	//
	// Gating (#464) brings the repository template in line with the
	// classic dispatcher:
	//   - `base.header` is gated by the resolved `base` sections.
	//   - every other `_.json` entry is a plugin partial gated by its
	//     `plugin_<slug>` toggle PLUS a non-Skipped plugin result, so a
	//     plain `base` run renders only the repo chrome (matching
	//     upstream `metrics.repository.svg`).
	for _, name := range t.partials {
		if !partialEnabledByBase(name, baseSections) {
			continue
		}
		if pluginPartialName(name) && !pluginEnabled(pc, name) {
			continue
		}
		fn, ok := lookupPartial(name)
		if !ok {
			continue
		}
		frag, err := fn(ctx, pc)
		if err != nil {
			return "", fmt.Errorf("repository: partial %q: %w", name, err)
		}
		b.WriteString(frag)
	}

	if footer := chrome.MetadataFooter(pc, baseSections, chrome.FooterOpts{}); footer != "" {
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

// partialEnabledByBase reports whether the named repository-owned
// partial should render given the resolved base sections. Only
// `base.header` is gated here (mapped to the "header" section); every
// other `_.json` entry is a plugin partial whose `plugin_<slug>`
// toggle is applied separately by pluginEnabled.
func partialEnabledByBase(name string, sections map[string]struct{}) bool {
	if name == "base.header" {
		_, ok := sections["header"]
		return ok
	}
	return true
}

// pluginPartialName reports whether a `_.json` entry is a plugin
// partial (gated by `plugin_<slug>`) rather than a repository-owned
// chrome partial (`base.header` / `introduction`). The repository
// template owns only `base.header`; `introduction` is an unadopted
// plugin slug, so it is gated by `plugin_introduction` like any other
// plugin entry.
func pluginPartialName(name string) bool {
	return name != "base.header"
}

// pluginEnabled reports whether the plugin partial named `slug` should
// render: its `plugin_<slug>` input must be truthy AND its plugin
// result (when present) must not be Skipped. Mirrors the classic
// dispatcher's gate so a plain `base` run renders only the chrome.
func pluginEnabled(pc *templates.PartialContext, slug string) bool {
	if pc == nil || pc.Inputs == nil {
		return false
	}
	if slug == "languages" && pc.Data != nil && pc.Data.RepoRef() != nil {
		raw, present := pc.Data.GetPlugin(slug)
		if !present || raw == nil {
			return false
		}
		if sr, ok := raw.(interface{ IsSkipped() bool }); ok && sr.IsSkipped() {
			return false
		}
		return true
	}
	if !chrome.TruthyInput(pc.Inputs, "plugin_"+slug) {
		return false
	}
	if pc.Data == nil {
		return false
	}
	raw, present := pc.Data.GetPlugin(slug)
	if !present || raw == nil {
		// Toggle is on but the plugin produced no result (e.g. unadopted
		// slug). The Lookup miss below will drop it anyway; allow it
		// through so adopted plugins whose result lands lazily still
		// render.
		return true
	}
	if sr, ok := raw.(interface{ IsSkipped() bool }); ok && sr.IsSkipped() {
		return false
	}
	return true
}

// lookupPartial wraps the package-level registry lookup. Falls back
// to the "plugin.<slug>" registration convention used by per-plugin
// partial packages.
func lookupPartial(name string) (templates.PartialFunc, bool) {
	// Repository-specific partials (base.header, introduction,
	// base.community, base.activity) live in the repository/partials
	// package and override any classic-package registration.
	if fn, ok := repopart.Lookup(name); ok {
		return fn, true
	}
	// Plugin partials are registered by per-plugin packages into the
	// global classic partials registry under their bare slug or
	// "plugin.<slug>" — both forms are checked here.
	if fn, ok := classicpart.Lookup(name); ok {
		return fn, true
	}
	if fn, ok := classicpart.Lookup("plugin." + name); ok {
		return fn, true
	}
	return nil, false
}
