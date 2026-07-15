// Command gen-plugin-docs emits per-plugin markdown pages under
// docs/plugins/ and refreshes the plugins-gallery AUTOGEN block
// inside README.md. Each page mixes auto-generated zones
// (regenerated on every run) and human-authored zones (preserved
// byte-identical when the markers are intact).
//
// Two locales are supported: English (canonical, always emitted) and
// Japanese (emitted only when a sibling
// `assets/plugins/<slug>/metadata_ja.yml` translation file exists;
// see `translationLocales` and `loadJATranslation`).
//
// Usage:
//
//	go run ./internal/tools/gen-plugin-docs
//
// The repository root is detected by walking up from the cwd until a
// go.mod file is found. The tool always operates on the repo's own
// docs/plugins/ and README.md.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// adoptedSlugs is the canonical list of user-facing plugin slugs that
// the docs gallery covers. It mirrors `tests/compliance/compliance_test.go::adoptedM4Plugins`
// minus the `base` / `core` infrastructure plugins and the
// `languages.recent` / `languages.indepth` sub-modes (which share the
// `languages` page). A unit test in this package asserts the two
// lists stay aligned.
var adoptedSlugs = []string{
	"achievements",
	"activity",
	"calendar",
	"contributors",
	"habits",
	"header",
	"isocalendar",
	"languages",
	"notable",
	"people",
	"reactions",
	"repositories",
	"sponsors",
	"sponsorships",
	"stargazers",
	"starlists",
	"stars",
	"topics",
	"traffic",
}

// foundationalSlugs are the infrastructure plugins that ship alongside
// the adopted user-facing plugins. They ship a doc page so their inputs
// are documented in one place even though they are excluded from the
// README gallery.
//
//   - `core`     — configuration + parallel-runner plugin; no
//     standalone visual output.
//   - `base`     — activity / community / repositories summary panels
//     (#625). Structurally foundational: emits no standalone card on
//     its own; composes the legacy `base` chrome on top of any other
//     plugin set. The panels live behind `plugin_base*` opt-in toggles.
var foundationalSlugs = []string{
	"base",
	"core",
}

// slugsWithoutSample is the set of plugin slugs whose `docs/plugins/<slug>.md`
// page intentionally omits the sample-image section. `core` has no
// standalone visual output (configuration parsing + parallel runner);
// `base` renders a real composed sample under docs/examples/plugin-base.svg
// (see scripts/samples.json) and therefore does NOT belong here.
var slugsWithoutSample = map[string]struct{}{
	"core": {},
}

// sampleOverrides maps a plugin slug to the example SVG basename (without
// the `.svg` extension, relative to docs/examples/) that best represents
// it, when that is NOT the default `plugin-<slug>` user-mode render.
//
// `contributors` is repository-mode-only: in user mode it short-circuits
// to Skipped (internal/plugins/modegate.go::RequireRepoMode) and renders
// an empty card. The representative sample is therefore the repo-mode
// `plugin-contributors-repo-contributions` render, which shows populated
// per-contributor rows (issue #448).
var sampleOverrides = map[string]string{
	"contributors": "plugin-contributors-repo-contributions",
}

// sampleImageBase returns the example SVG basename (without `.svg`) used
// for the given plugin slug in docs pages and the README gallery.
func sampleImageBase(slug string) string {
	if override, ok := sampleOverrides[slug]; ok {
		return override
	}
	return "plugin-" + slug
}

func main() {
	root, err := repoRoot()
	if err != nil {
		fail("repo root: %v", err)
	}
	if err := run(root); err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-plugin-docs: "+format+"\n", args...)
	os.Exit(1)
}

func run(root string) error {
	allSlugs := append(append([]string(nil), adoptedSlugs...), foundationalSlugs...)
	for _, slug := range allSlugs {
		if err := generatePluginPage(root, slug, enStrings); err != nil {
			return fmt.Errorf("plugin %s (en): %w", slug, err)
		}
		// Japanese page is emitted only when a hand-authored
		// translation exists. See design decision #2 in issue #761:
		// half-translated pages are worse than none, so a missing
		// metadata_ja.yml means the JA page is intentionally skipped.
		if err := generatePluginPage(root, slug, jaStrings); err != nil {
			return fmt.Errorf("plugin %s (ja): %w", slug, err)
		}
	}
	return updateReadme(root)
}

// pluginMetadata is the subset of assets/plugins/<slug>/metadata.yml
// the doc generator consumes.
type pluginMetadata struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Supports    []string               `yaml:"supports"`
	Scopes      []string               `yaml:"scopes"`
	Inputs      map[string]pluginInput `yaml:"inputs"`
}

type pluginInput struct {
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Default     any    `yaml:"default"`
	Required    bool   `yaml:"required"`
}

// loadMetadata parses assets/plugins/<slug>/metadata.yml. Inputs are
// returned as an ordered slice (yaml.v3 Node walking) so the table
// rows match the upstream metadata declaration order.
func loadMetadata(root, slug string) (pluginMetadata, []string, error) {
	path := filepath.Join(root, "assets", "plugins", slug, "metadata.yml")
	raw, err := os.ReadFile(path) //nolint:gosec // operator-controlled paths inside the project tree
	if err != nil {
		return pluginMetadata{}, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m pluginMetadata
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return pluginMetadata{}, nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}

	// Re-walk the YAML tree to recover the inputs key order, because
	// Go maps lose ordering.
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return pluginMetadata{}, nil, fmt.Errorf("yaml ast %s: %w", path, err)
	}
	keys := extractInputKeys(&doc)
	return m, keys, nil
}

// loadJATranslation reads the optional Japanese translation overlay for
// a plugin. Returns (empty, false, nil) when the file is absent — that
// is the documented signal to skip the JA page entirely (design
// decision #2 in issue #761). Any other error (unreadable, malformed
// YAML) is propagated.
func loadJATranslation(root, slug string) (pluginMetadata, bool, error) {
	path := filepath.Join(root, "assets", "plugins", slug, "metadata_ja.yml")
	raw, err := os.ReadFile(path) //nolint:gosec // operator-controlled paths inside the project tree
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pluginMetadata{}, false, nil
		}
		return pluginMetadata{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var m pluginMetadata
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return pluginMetadata{}, false, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return m, true, nil
}

// applyTranslation merges a translation overlay onto the base metadata.
// Only human-facing prose fields are overlayed: `description` (page
// intro) and the per-input `description` (config-table rows). Machine
// fields (`type`, `default`, `required`, `supports`, `scopes`) always
// come from the base to stay upstream-compatible. Empty fields in the
// overlay fall through to the base (partial translations are allowed).
func applyTranslation(base, overlay pluginMetadata) pluginMetadata {
	out := base
	if s := strings.TrimSpace(overlay.Description); s != "" {
		out.Description = overlay.Description
	}
	if len(overlay.Inputs) > 0 {
		merged := make(map[string]pluginInput, len(base.Inputs))
		for k, v := range base.Inputs {
			merged[k] = v
		}
		for k, ov := range overlay.Inputs {
			bv, ok := merged[k]
			if !ok {
				// Overlay may include entries for inputs that no
				// longer exist in the base — ignore to keep the
				// generator upstream-compatible.
				continue
			}
			if s := strings.TrimSpace(ov.Description); s != "" {
				bv.Description = ov.Description
			}
			merged[k] = bv
		}
		out.Inputs = merged
	}
	return out
}

// extractInputKeys returns the input map keys in declaration order.
func extractInputKeys(doc *yaml.Node) []string {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		k := root.Content[i]
		v := root.Content[i+1]
		if k.Value == "inputs" && v.Kind == yaml.MappingNode {
			out := make([]string, 0, len(v.Content)/2)
			for j := 0; j < len(v.Content)-1; j += 2 {
				out = append(out, v.Content[j].Value)
			}
			return out
		}
	}
	return nil
}

// pluginPagePath returns the docs/plugins/<slug>[<suffix>].md path for
// the requested locale. English uses no suffix (canonical); Japanese
// uses the `_ja` suffix.
func pluginPagePath(root, slug string, ls localeStrings) string {
	return filepath.Join(root, "docs", "plugins", slug+ls.FileSuffix+".md")
}

// generatePluginPage writes docs/plugins/<slug>[_ja].md for the given
// locale. Existing human-authored zones (outside AUTOGEN markers) are
// preserved. For non-English locales, if no translation overlay exists
// the page is skipped entirely (see loadJATranslation for the rule).
func generatePluginPage(root, slug string, ls localeStrings) error {
	meta, inputKeys, err := loadMetadata(root, slug)
	if err != nil {
		return err
	}

	if ls.Code == "ja" {
		overlay, ok, err := loadJATranslation(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			// Design decision #2: skip when no translation exists.
			return nil
		}
		meta = applyTranslation(meta, overlay)
	}

	out := pluginPagePath(root, slug, ls)
	existing, _ := os.ReadFile(out) //nolint:gosec // missing file is fine — first-gen path

	page := renderPluginPageLocale(slug, meta, inputKeys, existing, ls)
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(page), 0o600)
}

// renderPluginPage is the English-locale entry point used by tests and
// callers that do not need locale awareness. It preserves the historic
// 4-arg signature (`slug`, `meta`, `inputKeys`, `existing`) so existing
// unit tests continue to compile unchanged.
func renderPluginPage(slug string, m pluginMetadata, inputKeys []string, existing []byte) string {
	return renderPluginPageLocale(slug, m, inputKeys, existing, enStrings)
}

// renderPluginPageLocale emits the full markdown for one plugin in the
// requested locale. When `existing` is non-empty, the human-authored
// zones (the three prose sections between AUTOGEN blocks: "When to
// use", "Requirements", "Notes") are pulled forward. Section headings
// are drawn from `ls`, so the same preserve-around-AUTOGEN flow works
// for both English and Japanese pages.
func renderPluginPageLocale(slug string, m pluginMetadata, inputKeys []string, existing []byte, ls localeStrings) string {
	whenSection, requirementsSection, pitfallsSection := extractHumanZonesLocale(string(existing), ls)

	var b strings.Builder
	// AUTOGEN: title + description
	b.WriteString("<!-- AUTOGEN_START: title-and-description -->\n")
	fmt.Fprintf(&b, "# %s: %s\n\n", ls.PluginHeading, slug)
	desc := firstParagraph(m.Description)
	if desc == "" {
		desc = defaultDescription(slug, ls)
	}
	b.WriteString(desc)
	b.WriteString("\n<!-- AUTOGEN_END: title-and-description -->\n\n")

	// Sample image (skipped for slugs that produce no standalone visual
	// output, e.g. `core` — it implements configuration parsing and the
	// parallel plugin runner, with no card of its own).
	fmt.Fprintf(&b, "## %s\n\n", ls.SampleHeading)
	if _, skip := slugsWithoutSample[slug]; skip {
		b.WriteString(ls.NoStandaloneSampleNotice)
		b.WriteString("\n\n")
	} else {
		fmt.Fprintf(&b, "![%s sample](../examples/%s.svg)\n\n", slug, sampleImageBase(slug))
		fmt.Fprintf(&b, "> %s\n\n", ls.SampleCaption)
	}

	// Human-authored: when-to-use. Skip the header entirely when no
	// prose exists so the page does not show an empty heading.
	if whenSection != "" {
		fmt.Fprintf(&b, "## %s\n\n", ls.WhenToUseHeading)
		b.WriteString(whenSection)
		b.WriteString("\n\n")
	}

	// AUTOGEN: config table
	b.WriteString("<!-- AUTOGEN_START: config-table -->\n")
	fmt.Fprintf(&b, "## %s\n\n", ls.InputsHeading)
	if len(inputKeys) == 0 {
		b.WriteString(ls.NoInputsNotice)
		b.WriteString("\n")
	} else {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			ls.InputColName, ls.InputColDesc, ls.InputColDefault, ls.InputColRequired, ls.InputColType)
		b.WriteString("| ----- | ----------- | ------- | -------- | ---- |\n")
		for _, k := range inputKeys {
			in := m.Inputs[k]
			b.WriteString(formatInputRow(k, in, ls))
		}
	}
	b.WriteString("<!-- AUTOGEN_END: config-table -->\n\n")

	// AUTOGEN: usage snippet
	b.WriteString("<!-- AUTOGEN_START: usage-snippet -->\n")
	fmt.Fprintf(&b, "## %s\n\n", ls.UsageHeading)
	switch slug {
	case "core":
		// `core` is the configuration / parallel-runner plugin. It is
		// never toggled on/off; users interact with it by supplying the
		// global inputs documented in the table above (`template`,
		// `config_*`, `optimize`, etc.).
		fmt.Fprintf(&b, "### %s\n\n", ls.GHActionSubheading)
		b.WriteString("```yaml\n")
		b.WriteString("- uses: mjun0812/github-metrics@v1\n")
		b.WriteString("  with:\n")
		b.WriteString("    user: <your-login>\n")
		b.WriteString("    token: ${{ secrets.METRICS_TOKEN }}\n")
		b.WriteString("    template: classic\n")
		b.WriteString("    config_timezone: Asia/Tokyo\n")
		b.WriteString("    config_output: svg\n")
		b.WriteString("```\n\n")
		fmt.Fprintf(&b, "### %s\n\n", ls.CLISubheading)
		b.WriteString("```sh\n")
		b.WriteString("# export GITHUB_TOKEN=$(gh auth token)\n")
		b.WriteString("metrics-cli --user <your-login> \\\n")
		b.WriteString("  --template classic \\\n")
		b.WriteString("  --output svg --filename github-metrics.svg \\\n")
		b.WriteString("  --plugin config_timezone=Asia/Tokyo\n")
		b.WriteString("```\n")
	default:
		fmt.Fprintf(&b, "### %s\n\n", ls.GHActionSubheading)
		b.WriteString("```yaml\n")
		b.WriteString("- uses: mjun0812/github-metrics@v1\n")
		b.WriteString("  with:\n")
		b.WriteString("    user: <your-login>\n")
		b.WriteString("    token: ${{ secrets.METRICS_TOKEN }}\n")
		fmt.Fprintf(&b, "    plugin_%s: yes\n", slug)
		b.WriteString("```\n\n")
		fmt.Fprintf(&b, "### %s\n\n", ls.CLISubheading)
		b.WriteString("```sh\n")
		b.WriteString("# export GITHUB_TOKEN=$(gh auth token)\n")
		b.WriteString("metrics-cli --user <your-login> \\\n")
		b.WriteString("  --output svg --filename - \\\n")
		fmt.Fprintf(&b, "  --plugin plugin_%s=yes\n", slug)
		b.WriteString("```\n")
	}
	b.WriteString("<!-- AUTOGEN_END: usage-snippet -->\n\n")

	// Human-authored: Requirements. Lives between the usage-snippet
	// AUTOGEN block and the "Notes" heading and is preserved verbatim
	// when `existing` already contains one. For the foundational `base`
	// and `core` plugins we inject a canonical first-gen paragraph so
	// the page is self-explanatory without manual editing.
	if requirementsSection != "" {
		fmt.Fprintf(&b, "## %s\n\n", ls.RequirementsHeading)
		b.WriteString(requirementsSection)
		b.WriteString("\n\n")
	} else if req := defaultRequirements(slug, ls); req != "" {
		fmt.Fprintf(&b, "## %s\n\n", ls.RequirementsHeading)
		b.WriteString(req)
		b.WriteString("\n\n")
	}

	// Human-authored: notes. Skip the header entirely when empty.
	if pitfallsSection != "" {
		fmt.Fprintf(&b, "## %s\n\n", ls.NotesHeading)
		b.WriteString(pitfallsSection)
		b.WriteString("\n\n")
	}

	// References
	fmt.Fprintf(&b, "## %s\n\n", ls.ReferencesHeading)
	fmt.Fprintf(&b, "- [`action.yml`](../../action.yml) — %s\n", ls.RefActionYml)
	fmt.Fprintf(&b, "- [`assets/plugins/%s/metadata.yml`](../../assets/plugins/%s/metadata.yml) — %s\n", slug, slug, ls.RefMetadataYml)
	if len(m.Supports) > 0 {
		fmt.Fprintf(&b, "- %s: %s\n", ls.RefSupports, strings.Join(m.Supports, ", "))
	}
	if len(m.Scopes) > 0 {
		fmt.Fprintf(&b, "- %s: %s\n", ls.RefScopes, strings.Join(m.Scopes, ", "))
	}

	return b.String()
}

// formatInputRow renders a single row in the input config table.
func formatInputRow(key string, in pluginInput, ls localeStrings) string {
	desc := firstParagraph(in.Description)
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "|", `\|`)
	if desc == "" {
		desc = ls.NoDescriptionPlaceholder
	}

	def := formatDefault(in.Default)
	// Collapse newlines that appear in multi-line JSON / YAML defaults
	// (e.g. `plugin_contributors_categories`) so the table row stays on
	// a single line; trim surrounding whitespace introduced by YAML
	// folding.
	def = strings.Join(strings.Fields(def), " ")
	def = strings.ReplaceAll(def, "|", `\|`)
	req := ls.NoLabel
	if in.Required {
		req = ls.YesLabel
	}
	typ := in.Type
	if typ == "" {
		typ = "string"
	}
	return fmt.Sprintf("| `%s` | %s | `%s` | %s | %s |\n", key, desc, def, req, typ)
}

func formatDefault(d any) string {
	if d == nil {
		return ""
	}
	switch x := d.(type) {
	case string:
		return x
	case bool:
		if x {
			return "yes"
		}
		return "no"
	case int:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%g", x)
	}
	return fmt.Sprintf("%v", d)
}

// defaultRequirements returns the first-gen Requirements paragraph for
// the foundational `core` / `base` plugins. Returns "" for every other
// slug and for locales other than English — the 19 adopted plugins
// have Requirements text that landed hand-written in PR #410 and is
// pulled forward from the existing file via extractHumanZones rather
// than emitted here. Japanese translations of these paragraphs will be
// added when a metadata_ja.yml exists for base / core; until then the
// Requirements section stays blank on JA pages (see #761).
func defaultRequirements(slug string, ls localeStrings) string {
	if ls.Code != "en" {
		return ""
	}
	switch slug {
	case "core":
		return "Core has no standalone visual output; this page documents its inputs only. The plugin implements global configuration parsing (template selection, timezone, animations, output format, etc.) and the parallel plugin runner that drives every other plugin. There are no API scopes or render prerequisites of its own — every other plugin in this repository depends on `core` having populated `data.Config` before it runs."
	case "base":
		return "Base reads `Provider.Profile(ctx)` and `Provider.RepositorySummary(ctx)` from the shared `internal/dataprovider`. Both are populated lazily by the standard GraphQL user/organization + repositories paging queries — no extra API scopes beyond `public_access` are required. The plugin emits no standalone card on its own; its two panels (gated by `chrome_activity` / `chrome_community` and `chrome_repositories` from `assets/plugins/chrome/metadata.yml`, #640) compose with any other plugin selection to restore the legacy `base` chrome look."
	}
	return ""
}

// defaultDescription returns the AUTOGEN title-and-description fallback
// text for plugins whose `assets/plugins/<slug>/metadata.yml` does not
// supply a `description:` field. The `core` foundational plugin uses
// this path because its upstream metadata leaves `description` empty;
// the generic fallback ("`<slug>` plugin output for GitHub metrics.")
// would be misleading for it, so we provide a purpose-written summary
// instead. For non-English locales, only a minimal generic fallback is
// emitted — locale-specific hand-written descriptions live in
// `assets/plugins/<slug>/metadata_ja.yml` when available.
func defaultDescription(slug string, ls localeStrings) string {
	if ls.Code != "en" {
		return fmt.Sprintf("`%s` %s", slug, ls.GenericPluginBlurb)
	}
	switch slug {
	case "core":
		return "`core` is the configuration plugin: it parses the global `config_*` / `template` / `optimize` inputs into `data.Config` and drives the parallel runner that fans every other plugin out across workers. It has no card of its own — every visible output comes from another plugin running on top of the state `core` produces."
	case "base":
		return "`base` restores the activity / community / repositories summary panels that originally lived in the upstream `base` chrome (deleted by the #623 refactor). The panels read aggregated counters from the shared `internal/dataprovider`, so enabling them adds no GitHub API calls beyond what other plugins already trigger."
	}
	return fmt.Sprintf("`%s` plugin output for GitHub metrics.", slug)
}

// firstParagraph returns the leading non-empty paragraph (text up to
// the first blank line) trimmed of surrounding whitespace.
func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// humanZoneRegex bundles the three compiled regexes used to extract
// the human-authored zones from a previously-generated page. Each
// locale gets its own set because the heading text is localized (e.g.
// `## Requirements` vs `## 前提条件`).
type humanZoneRegex struct {
	when         *regexp.Regexp
	requirements *regexp.Regexp
	pitfalls     *regexp.Regexp
}

// newHumanZoneRegex compiles the three human-zone regexes for the
// given locale. The AUTOGEN marker text is locale-invariant; only the
// heading names vary.
//
// Three human-authored zones live in a previously-generated page:
//  1. When-to-use   — between title-and-description and the
//     config-table AUTOGEN block.
//  2. Requirements  — between the usage-snippet AUTOGEN block and
//     the next heading (Notes or References when Notes is absent).
//     Optional.
//  3. Notes         — between Requirements (or the usage-snippet
//     block when Requirements is absent) and References.
func newHumanZoneRegex(ls localeStrings) humanZoneRegex {
	when := regexp.MustCompile(`(?s)## ` +
		regexp.QuoteMeta(ls.WhenToUseHeading) +
		`\s*\n+(.*?)\n+<!-- AUTOGEN_START: config-table -->`)
	requirements := regexp.MustCompile(`(?s)## ` +
		regexp.QuoteMeta(ls.RequirementsHeading) +
		`\s*\n+(.*?)\n+## (?:` +
		regexp.QuoteMeta(ls.NotesHeading) + `|` +
		regexp.QuoteMeta(ls.ReferencesHeading) + `)`)
	pitfalls := regexp.MustCompile(`(?s)## ` +
		regexp.QuoteMeta(ls.NotesHeading) +
		`\s*\n+(.*?)\n+## ` +
		regexp.QuoteMeta(ls.ReferencesHeading))
	return humanZoneRegex{when: when, requirements: requirements, pitfalls: pitfalls}
}

// extractHumanZonesLocale returns the prose that lives in the three
// human-authored sections of a previously-generated page, using
// locale-aware heading regexes.
func extractHumanZonesLocale(existing string, ls localeStrings) (when, requirements, pitfalls string) {
	if existing == "" {
		return "", "", ""
	}
	rx := newHumanZoneRegex(ls)
	if m := rx.when.FindStringSubmatch(existing); len(m) == 2 {
		when = strings.TrimSpace(m[1])
	}
	if m := rx.requirements.FindStringSubmatch(existing); len(m) == 2 {
		requirements = strings.TrimSpace(m[1])
	}
	if m := rx.pitfalls.FindStringSubmatch(existing); len(m) == 2 {
		pitfalls = strings.TrimSpace(m[1])
	}
	return when, requirements, pitfalls
}

// ---------- README plugins-gallery ----------

var (
	galleryMarkerStart = "<!-- AUTOGEN_START: plugins-gallery -->"
	galleryMarkerEnd   = "<!-- AUTOGEN_END: plugins-gallery -->"
	galleryBlockRe     = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(galleryMarkerStart) + `.*?` + regexp.QuoteMeta(galleryMarkerEnd))
)

// updateReadme rewrites the plugins-gallery AUTOGEN block in the repo's
// README.md. If the block is missing, it is inserted at the canonical
// anchor.
func updateReadme(root string) error {
	path := filepath.Join(root, "README.md")
	raw, err := os.ReadFile(path) //nolint:gosec // operator-controlled paths inside the project tree
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}

	gallery := renderGallery()

	updated, err := mergeReadme(string(raw), gallery)
	if err != nil {
		return err
	}
	if updated == string(raw) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o600) //nolint:gosec // operator-controlled README path
}

func mergeReadme(content, gallery string) (string, error) {
	// Gallery block.
	if galleryBlockRe.MatchString(content) {
		content = galleryBlockRe.ReplaceAllStringFunc(content, func(_ string) string { return gallery })
	} else {
		// Insert directly under the "## Plugins" heading, before the
		// existing Data source table.
		anchor := "## Plugins\n"
		idx := strings.Index(content, anchor)
		if idx < 0 {
			return "", fmt.Errorf("README gallery anchor (%q) not found", "## Plugins")
		}
		insertAt := idx + len(anchor)
		content = content[:insertAt] + "\n" + gallery + "\n" + content[insertAt:]
	}

	return content, nil
}

func renderGallery() string {
	slugs := append([]string(nil), adoptedSlugs...)
	sort.Strings(slugs)
	var b strings.Builder
	b.WriteString(galleryMarkerStart)
	b.WriteString("\n")
	b.WriteString("| | | |\n")
	b.WriteString("|:---:|:---:|:---:|\n")

	for i := 0; i < len(slugs); i += 3 {
		// Thumbnail row.
		b.WriteString("|")
		for c := 0; c < 3; c++ {
			if i+c >= len(slugs) {
				b.WriteString(" |")
				continue
			}
			s := slugs[i+c]
			fmt.Fprintf(&b, " [![%s](docs/examples/%s.svg)](docs/plugins/%s.md) |", s, sampleImageBase(s), s)
		}
		b.WriteString("\n")
		// Caption row.
		b.WriteString("|")
		for c := 0; c < 3; c++ {
			if i+c >= len(slugs) {
				b.WriteString(" |")
				continue
			}
			s := slugs[i+c]
			fmt.Fprintf(&b, " [`%s`](docs/plugins/%s.md) |", s, s)
		}
		b.WriteString("\n")
	}
	b.WriteString(galleryMarkerEnd)
	return b.String()
}

// ---------- repoRoot ----------

func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; dir != "" && dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("could not find go.mod above %s", cwd)
}
