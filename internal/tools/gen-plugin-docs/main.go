// Command gen-plugin-docs emits per-plugin markdown pages under
// docs/plugins/ and refreshes the plugins-gallery AUTOGEN block
// inside README.md. Each page mixes auto-generated zones
// (regenerated on every run) and human-authored zones (preserved
// byte-identical when the markers are intact).
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
	"isocalendar",
	"languages",
	"notable",
	"people",
	"projects",
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

// foundationalSlugs are the infrastructure plugins (`base`, `core`)
// that ship alongside the 19 adopted user-facing plugins. They live
// under `internal/plugins/` and `assets/plugins/` like the others,
// but they are intentionally excluded from `adoptedSlugs` (the README
// gallery is reserved for plugins that emit user-visible cards).
// We still generate a `docs/plugins/<slug>.md` page for each so the
// inputs they accept are documented in one place.
//
// `base` populates the user/org header card every other plugin sits
// on top of (header / activity / community / repositories / metadata
// sections); `core` is the configuration + parallel-runner plugin and
// has no standalone visual output.
var foundationalSlugs = []string{
	"base",
	"core",
}

// slugsWithoutSample is the set of plugin slugs whose `docs/plugins/<slug>.md`
// page intentionally omits the sample-image section. `core` is the only
// member: it has no standalone visual output (it implements configuration
// parsing and the parallel plugin runner) so an example image would be
// either empty or misleading.
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
	for _, slug := range adoptedSlugs {
		if err := generatePluginPage(root, slug); err != nil {
			return fmt.Errorf("plugin %s: %w", slug, err)
		}
	}
	for _, slug := range foundationalSlugs {
		if err := generatePluginPage(root, slug); err != nil {
			return fmt.Errorf("plugin %s: %w", slug, err)
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

// generatePluginPage writes docs/plugins/<slug>.md. Existing
// human-authored zones (outside AUTOGEN markers) are preserved.
func generatePluginPage(root, slug string) error {
	meta, inputKeys, err := loadMetadata(root, slug)
	if err != nil {
		return err
	}

	out := filepath.Join(root, "docs", "plugins", slug+".md")
	existing, _ := os.ReadFile(out) //nolint:gosec // missing file is fine — first-gen path

	page := renderPluginPage(slug, meta, inputKeys, existing)
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(page), 0o600)
}

// renderPluginPage emits the full markdown for one plugin. When
// `existing` is non-empty, the human-authored zones (the three prose
// sections between AUTOGEN blocks: "When to use", "Requirements",
// "Notes") are pulled forward.
func renderPluginPage(slug string, m pluginMetadata, inputKeys []string, existing []byte) string {
	whenSection, requirementsSection, pitfallsSection := extractHumanZones(string(existing))

	var b strings.Builder
	// AUTOGEN: title + description
	b.WriteString("<!-- AUTOGEN_START: title-and-description -->\n")
	fmt.Fprintf(&b, "# Plugin: %s\n\n", slug)
	desc := firstParagraph(m.Description)
	if desc == "" {
		desc = defaultDescription(slug)
	}
	b.WriteString(desc)
	b.WriteString("\n<!-- AUTOGEN_END: title-and-description -->\n\n")

	// Sample image (skipped for slugs that produce no standalone visual
	// output, e.g. `core` — it implements configuration parsing and the
	// parallel plugin runner, with no card of its own).
	if _, skip := slugsWithoutSample[slug]; skip {
		b.WriteString("## Sample\n\n")
		b.WriteString("This plugin emits no standalone SVG; its inputs are documented below.\n\n")
	} else {
		b.WriteString("## Sample\n\n")
		fmt.Fprintf(&b, "![%s sample](../examples/%s.svg)\n\n", slug, sampleImageBase(slug))
		b.WriteString("> Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.\n\n")
	}

	// Human-authored: when-to-use. Skip the header entirely when no
	// prose exists so the page does not show an empty heading.
	if whenSection != "" {
		b.WriteString("## When to use\n\n")
		b.WriteString(whenSection)
		b.WriteString("\n\n")
	}

	// AUTOGEN: config table
	b.WriteString("<!-- AUTOGEN_START: config-table -->\n")
	b.WriteString("## Configuration (inputs)\n\n")
	if len(inputKeys) == 0 {
		b.WriteString("(This plugin has no dedicated inputs.)\n")
	} else {
		b.WriteString("| Input | Description | Default | Required | Type |\n")
		b.WriteString("| ----- | ----------- | ------- | -------- | ---- |\n")
		for _, k := range inputKeys {
			in := m.Inputs[k]
			b.WriteString(formatInputRow(k, in))
		}
	}
	b.WriteString("<!-- AUTOGEN_END: config-table -->\n\n")

	// AUTOGEN: usage snippet
	b.WriteString("<!-- AUTOGEN_START: usage-snippet -->\n")
	b.WriteString("## Usage\n\n")
	switch slug {
	case "base":
		// `base` is always active (it populates the user/org header
		// every other plugin sits on top of). What the user configures
		// is *which* base sections to render and a few related toggles
		// (indepth / hireable / skip). The Action / CLI snippets reflect
		// the canonical "tweak base sections" usage rather than a
		// non-existent `plugin_base: yes` toggle.
		b.WriteString("### GitHub Action\n\n")
		b.WriteString("```yaml\n")
		b.WriteString("- uses: mjun0812/github-metrics@v1\n")
		b.WriteString("  with:\n")
		b.WriteString("    user: <your-login>\n")
		b.WriteString("    token: ${{ secrets.METRICS_TOKEN }}\n")
		b.WriteString("    base: header, activity, community, repositories, metadata\n")
		b.WriteString("```\n\n")
		b.WriteString("### CLI\n\n")
		b.WriteString("```sh\n")
		b.WriteString("metrics-cli --user <your-login> --token-env GITHUB_TOKEN \\\n")
		b.WriteString("  --output svg --filename - \\\n")
		b.WriteString("  --plugin 'base=header, activity, community, repositories, metadata'\n")
		b.WriteString("```\n")
	case "core":
		// `core` is the configuration / parallel-runner plugin. It is
		// never toggled on/off; users interact with it by supplying the
		// global inputs documented in the table above (`template`,
		// `config_*`, `optimize`, etc.).
		b.WriteString("### GitHub Action\n\n")
		b.WriteString("```yaml\n")
		b.WriteString("- uses: mjun0812/github-metrics@v1\n")
		b.WriteString("  with:\n")
		b.WriteString("    user: <your-login>\n")
		b.WriteString("    token: ${{ secrets.METRICS_TOKEN }}\n")
		b.WriteString("    template: classic\n")
		b.WriteString("    config_timezone: Asia/Tokyo\n")
		b.WriteString("    config_output: svg\n")
		b.WriteString("```\n\n")
		b.WriteString("### CLI\n\n")
		b.WriteString("```sh\n")
		b.WriteString("metrics-cli --user <your-login> --token-env GITHUB_TOKEN \\\n")
		b.WriteString("  --template classic \\\n")
		b.WriteString("  --output svg --filename github-metrics.svg \\\n")
		b.WriteString("  --plugin config_timezone=Asia/Tokyo\n")
		b.WriteString("```\n")
	default:
		b.WriteString("### GitHub Action\n\n")
		b.WriteString("```yaml\n")
		b.WriteString("- uses: mjun0812/github-metrics@v1\n")
		b.WriteString("  with:\n")
		b.WriteString("    user: <your-login>\n")
		b.WriteString("    token: ${{ secrets.METRICS_TOKEN }}\n")
		fmt.Fprintf(&b, "    plugin_%s: yes\n", slug)
		b.WriteString("```\n\n")
		b.WriteString("### CLI\n\n")
		b.WriteString("```sh\n")
		b.WriteString("metrics-cli --user <your-login> --token-env GITHUB_TOKEN \\\n")
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
		b.WriteString("## Requirements\n\n")
		b.WriteString(requirementsSection)
		b.WriteString("\n\n")
	} else if req := defaultRequirements(slug); req != "" {
		b.WriteString("## Requirements\n\n")
		b.WriteString(req)
		b.WriteString("\n\n")
	}

	// Human-authored: notes. Skip the header entirely when empty.
	if pitfallsSection != "" {
		b.WriteString("## Notes\n\n")
		b.WriteString(pitfallsSection)
		b.WriteString("\n\n")
	}

	// References
	b.WriteString("## References\n\n")
	b.WriteString("- [`action.yml`](../../action.yml) — canonical input schema\n")
	fmt.Fprintf(&b, "- [`assets/plugins/%s/metadata.yml`](../../assets/plugins/%s/metadata.yml) — upstream metadata\n", slug, slug)
	if len(m.Supports) > 0 {
		fmt.Fprintf(&b, "- Supported account types: %s\n", strings.Join(m.Supports, ", "))
	}
	if len(m.Scopes) > 0 {
		fmt.Fprintf(&b, "- Required scopes: %s\n", strings.Join(m.Scopes, ", "))
	}

	return b.String()
}

// formatInputRow renders a single row in the input config table.
func formatInputRow(key string, in pluginInput) string {
	desc := firstParagraph(in.Description)
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "|", `\|`)
	if desc == "" {
		desc = "(no description)"
	}

	def := formatDefault(in.Default)
	// Collapse newlines that appear in multi-line JSON / YAML defaults
	// (e.g. `plugin_contributors_categories`) so the table row stays on
	// a single line; trim surrounding whitespace introduced by YAML
	// folding.
	def = strings.Join(strings.Fields(def), " ")
	def = strings.ReplaceAll(def, "|", `\|`)
	req := "no"
	if in.Required {
		req = "yes"
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
// the foundational `base` / `core` plugins. Returns "" for every other
// slug — the 19 adopted plugins have Requirements text that landed
// hand-written in PR #410 and is pulled forward from the existing file
// via extractHumanZones rather than emitted here.
func defaultRequirements(slug string) string {
	switch slug {
	case "base":
		return "**A valid GitHub username (or organization login) and a token with at minimum `read:user` + `public_repo`.** The base plugin queries the GraphQL `user(login:)` / `organization(login:)` endpoint to populate the header card and walks the repositories connection (paged, with batch-halving on transient 5xx) to seed `Computed.RepositoryList` for every downstream plugin. Setting `base: \"\"` disables every base section but **does not** skip the GraphQL fetch — to fully skip base data fetching, use `base_skip: yes` and pair it with a plugin that supports `token: NOT_NEEDED`."
	case "core":
		return "Core has no standalone visual output; this page documents its inputs only. The plugin implements global configuration parsing (template selection, timezone, animations, output format, etc.) and the parallel plugin runner that drives every other plugin. There are no API scopes or render prerequisites of its own — every other plugin in this repository depends on `core` having populated `data.Config` before it runs."
	}
	return ""
}

// defaultDescription returns the AUTOGEN title-and-description fallback
// text for plugins whose `assets/plugins/<slug>/metadata.yml` does not
// supply a `description:` field. The `base` and `core` foundational
// plugins use this path because their upstream metadata leaves
// `description` empty; the generic fallback ("`<slug>` plugin output
// for GitHub metrics.") would be misleading for them, so we provide
// purpose-written summaries instead.
func defaultDescription(slug string) string {
	switch slug {
	case "base":
		return "`base` is the foundational plugin that runs before every other plugin and populates the shared `data.User` / `data.Organization` / `data.Computed` fields downstream plugins depend on. It also owns the user/org header card (avatar, login, follower/sponsor counts, two-week commit calendar) that every other plugin's output sits on top of."
	case "core":
		return "`core` is the configuration plugin: it parses the global `config_*` / `template` / `optimize` inputs into `data.Config` and drives the parallel runner that fans every other plugin out across workers. It has no card of its own — every visible output comes from another plugin running on top of the state `core` and `base` produce."
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

// humanZoneRe matches text between the marker pair that immediately
// precedes/follows the human-authored heading. We use this on the
// existing file to pull forward the maintainer's prose.
//
// Three human-authored zones live in a previously-generated page:
//  1. ## When to use   — between title-and-description and the
//     config-table AUTOGEN block.
//  2. ## Requirements  — between the usage-snippet AUTOGEN block and
//     the next heading (## Notes or ## References when Notes is
//     absent). Optional.
//  3. ## Notes         — between Requirements (or the usage-snippet
//     block when Requirements is absent) and "## References".
var (
	whenSectionRe         = regexp.MustCompile(`(?s)## When to use\s*\n+(.*?)\n+<!-- AUTOGEN_START: config-table -->`)
	requirementsSectionRe = regexp.MustCompile(`(?s)## Requirements\s*\n+(.*?)\n+## (?:Notes|References)`)
	pitfallsSectionRe     = regexp.MustCompile(`(?s)## Notes\s*\n+(.*?)\n+## References`)
)

// extractHumanZones returns the prose that lives in the three
// human-authored sections of a previously-generated page.
func extractHumanZones(existing string) (when, requirements, pitfalls string) {
	if existing == "" {
		return "", "", ""
	}
	if m := whenSectionRe.FindStringSubmatch(existing); len(m) == 2 {
		when = strings.TrimSpace(m[1])
	}
	if m := requirementsSectionRe.FindStringSubmatch(existing); len(m) == 2 {
		requirements = strings.TrimSpace(m[1])
	}
	if m := pitfallsSectionRe.FindStringSubmatch(existing); len(m) == 2 {
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
