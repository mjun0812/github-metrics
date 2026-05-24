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
	"bytes"
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
// `existing` is non-empty, the human-authored zones (the two prose
// sections between AUTOGEN blocks) are pulled forward.
func renderPluginPage(slug string, m pluginMetadata, inputKeys []string, existing []byte) string {
	whenSection, pitfallsSection := extractHumanZones(string(existing))

	var b strings.Builder
	// AUTOGEN: title + description
	b.WriteString("<!-- AUTOGEN_START: title-and-description -->\n")
	fmt.Fprintf(&b, "# Plugin: %s\n\n", slug)
	desc := firstParagraph(m.Description)
	if desc == "" {
		desc = fmt.Sprintf("`%s` plugin output for GitHub metrics.", slug)
	}
	b.WriteString(desc)
	b.WriteString("\n<!-- AUTOGEN_END: title-and-description -->\n\n")

	// Sample image
	b.WriteString("## サンプル出力\n\n")
	fmt.Fprintf(&b, "![%s sample](../examples/plugin-%s.svg)\n\n", slug, slug)
	b.WriteString("> サンプルは `--user mjun0812` のデータで本プラグインのみを有効化してレンダリングした例です。再生成は `make docs-examples`。\n\n")

	// Human-authored: when-to-use
	b.WriteString("## このプラグインを使うべきケース\n\n")
	if whenSection != "" {
		b.WriteString(whenSection)
	} else {
		b.WriteString("<!-- TODO: 1-2段落で記述。このプラグインがどんなユーザー / リポジトリで価値を持つか、どんな入力データに依存するか、を書いてください。 -->\n")
	}
	b.WriteString("\n")

	// AUTOGEN: config table
	b.WriteString("<!-- AUTOGEN_START: config-table -->\n")
	b.WriteString("## 設定 (inputs)\n\n")
	if len(inputKeys) == 0 {
		b.WriteString("(このプラグインには専用 input がありません。)\n")
	} else {
		b.WriteString("| Input | 説明 | デフォルト | 必須 | 型 |\n")
		b.WriteString("|-------|------|------------|------|----|\n")
		for _, k := range inputKeys {
			in := m.Inputs[k]
			b.WriteString(formatInputRow(k, in))
		}
	}
	b.WriteString("<!-- AUTOGEN_END: config-table -->\n\n")

	// AUTOGEN: usage snippet
	b.WriteString("<!-- AUTOGEN_START: usage-snippet -->\n")
	b.WriteString("## 使い方\n\n")
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
	b.WriteString("<!-- AUTOGEN_END: usage-snippet -->\n\n")

	// Human-authored: known constraints
	b.WriteString("## 既知の制約 / 注意点\n\n")
	if pitfallsSection != "" {
		b.WriteString(pitfallsSection)
	} else {
		b.WriteString("<!-- TODO: token scope の要件、empty-state の挙動、関連プラグインとの相互作用などを書いてください。 -->\n")
	}
	b.WriteString("\n")

	// References
	b.WriteString("## 参照\n\n")
	b.WriteString("- [`action.yml`](../../action.yml) — canonical input schema\n")
	fmt.Fprintf(&b, "- [`assets/plugins/%s/metadata.yml`](../../assets/plugins/%s/metadata.yml) — upstream metadata\n", slug, slug)
	if len(m.Supports) > 0 {
		fmt.Fprintf(&b, "- 対応アカウント種別: %s\n", strings.Join(m.Supports, ", "))
	}
	if len(m.Scopes) > 0 {
		fmt.Fprintf(&b, "- 必要スコープ: %s\n", strings.Join(m.Scopes, ", "))
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
var (
	whenSectionRe     = regexp.MustCompile(`(?s)## このプラグインを使うべきケース\s*\n+(.*?)\n+<!-- AUTOGEN_START: config-table -->`)
	pitfallsSectionRe = regexp.MustCompile(`(?s)## 既知の制約 / 注意点\s*\n+(.*?)\n+## 参照`)
)

// extractHumanZones returns the prose that lives in the two
// human-authored sections of a previously-generated page.
func extractHumanZones(existing string) (when, pitfalls string) {
	if existing == "" {
		return "", ""
	}
	if m := whenSectionRe.FindStringSubmatch(existing); len(m) == 2 {
		when = strings.TrimSpace(m[1])
	}
	if m := pitfallsSectionRe.FindStringSubmatch(existing); len(m) == 2 {
		pitfalls = strings.TrimSpace(m[1])
	}
	// TODO placeholders are not pulled forward — first-gen file paths
	// will re-emit them next run.
	if isTODOPlaceholder(when) {
		when = ""
	}
	if isTODOPlaceholder(pitfalls) {
		pitfalls = ""
	}
	return when, pitfalls
}

func isTODOPlaceholder(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<!-- TODO:") && strings.HasSuffix(s, "-->")
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
			fmt.Fprintf(&b, " [![%s](docs/examples/plugin-%s.svg)](docs/plugins/%s.md) |", s, s, s)
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

// ---------- linting helper: count TODO blocks under docs/plugins/ ----------

// CountTODOs walks docs/plugins/ and counts files containing the
// `<!-- TODO:` placeholder. Returned separately so a future Makefile
// `docs-lint` target (and the package's own tests) can share the
// behavior without forking a bash one-liner. Currently unused by main
// but kept as a stable API for the lint target.
func CountTODOs(root string) (int, error) {
	dir := filepath.Join(root, "docs", "plugins")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	needle := []byte("<!-- TODO:")
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // operator-controlled docs/plugins path
		if err != nil {
			return 0, err
		}
		if bytes.Contains(raw, needle) {
			n++
		}
	}
	return n, nil
}
