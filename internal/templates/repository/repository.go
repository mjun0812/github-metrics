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
	"sync"

	"github.com/mjun0812/github-metrics/assets"
	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/templates"
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

	mu          sync.Mutex
	stylesReady bool
	fonts       string
	style       string
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
// 7 M4 plugins that overlap with the repository template (languages,
// projects, stargazers, people, activity, contributors, sponsors)
// register their plugin partials globally at import time, so this
// template inherits them automatically once their packages are imported
// via `cmd/metrics-action/plugins.go`.
func (t *repositoryTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil {
		return "", fmt.Errorf("repository: nil PartialContext")
	}
	if err := t.loadStyles(); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="99999" class="">`)
	fmt.Fprintf(&b, `<defs><style>%s</style></defs>`, t.fonts)
	fmt.Fprintf(&b, `<style data-optimizable="true">%s</style>`, t.style)
	b.WriteString(`<style></style>`)
	b.WriteString(`<foreignObject x="0" y="0" width="100%" height="100%">`)
	b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink" class="items-wrapper">`)

	// Resolve which base.* sections are enabled. Mirrors the classic
	// template: `base` is a CSV of section names. Absent → all on;
	// present-but-empty → none (used by per-plugin / `base=` renders to
	// strip the base.header chrome). #464.
	baseSections := repopart.ResolveBaseSections(pc.Inputs)

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
		if !repopart.PartialEnabledByBase(name, baseSections) {
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

	b.WriteString(`<div id="metrics-end"></div>`)
	b.WriteString(`</div></foreignObject></svg>`)
	return b.String(), nil
}

// pluginPartialName reports whether a `_.json` entry is a plugin
// partial (gated by `plugin_<slug>`) rather than a repository-owned
// chrome partial (`base.header` / `introduction`). The repository
// template owns only `base.header`; `introduction` is an unadopted
// plugin slug, so it is gated by `plugin_introduction` like any other
// plugin entry.
func pluginPartialName(name string) bool {
	switch name {
	case "base.header":
		return false
	}
	return true
}

// pluginEnabled reports whether the plugin partial named `slug` should
// render: its `plugin_<slug>` input must be truthy AND its plugin
// result (when present) must not be Skipped. Mirrors the classic
// dispatcher's gate so a plain `base` run renders only the chrome.
func pluginEnabled(pc *templates.PartialContext, slug string) bool {
	if pc == nil || pc.Inputs == nil {
		return false
	}
	if !repopart.TruthyInput(pc.Inputs, "plugin_"+slug) {
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

// loadStyles lazily reads fonts.css + style.css from the classic
// template's embedded assets (they're template-agnostic — both
// templates render with the same upstream-equivalent CSS).
func (t *repositoryTemplate) loadStyles() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stylesReady {
		return nil
	}
	classicFS, err := fs.Sub(assets.FS(), "templates/classic")
	if err != nil {
		return fmt.Errorf("repository: locate classic fs: %w", err)
	}
	fonts, err := fs.ReadFile(classicFS, "fonts.css")
	if err != nil {
		return fmt.Errorf("repository: read fonts.css: %w", err)
	}
	style, err := fs.ReadFile(classicFS, "style.css")
	if err != nil {
		return fmt.Errorf("repository: read style.css: %w", err)
	}
	t.fonts = string(fonts)
	t.style = string(style)
	t.stylesReady = true
	return nil
}
