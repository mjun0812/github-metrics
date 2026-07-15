package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdoptedSlugsMatchCompliance asserts the doc generator's plugin
// list (`adoptedSlugs`) stays aligned with
// `tests/compliance/compliance_test.go::adoptedM4Plugins`, less the
// foundational `base` / `core` slugs (which live in `foundationalSlugs`
// — they ship a doc page but are not in the README gallery) and the
// `languages.recent` / `languages.indepth` sub-modes (which share the
// `languages` page).
func TestAdoptedSlugsMatchCompliance(t *testing.T) {
	t.Parallel()
	root := repoRootForTest(t)
	src, err := os.ReadFile(filepath.Join(root, "tests", "compliance", "compliance_test.go"))
	if err != nil {
		t.Fatalf("read compliance_test.go: %v", err)
	}
	want := compliancePluginsFromSource(string(src))
	if len(want) == 0 {
		t.Fatalf("could not extract adoptedM4Plugins from compliance_test.go")
	}

	got := map[string]struct{}{}
	for _, s := range adoptedSlugs {
		got[s] = struct{}{}
	}
	for s := range want {
		if _, ok := got[s]; !ok {
			t.Errorf("missing slug in adoptedSlugs: %q", s)
		}
	}
	for s := range got {
		if _, ok := want[s]; !ok {
			t.Errorf("extra slug in adoptedSlugs: %q", s)
		}
	}
}

func compliancePluginsFromSource(src string) map[string]struct{} {
	out := map[string]struct{}{}
	start := strings.Index(src, "var adoptedM4Plugins = []string{")
	if start < 0 {
		return out
	}
	end := strings.Index(src[start:], "}")
	if end < 0 {
		return out
	}
	body := src[start : start+end]
	for _, tok := range strings.Split(body, `"`) {
		tok = strings.TrimSpace(tok)
		if tok == "" || strings.HasPrefix(tok, ",") || strings.HasPrefix(tok, "var") {
			continue
		}
		if strings.Contains(tok, ".") {
			// languages.recent / languages.indepth share the languages page.
			continue
		}
		if tok == "core" || tok == "base" {
			// foundational plugins ship a doc page but are not in the
			// adopted-19 gallery list.
			continue
		}
		// Only accept tokens that look like plain slugs.
		if isPlainSlug(tok) {
			out[tok] = struct{}{}
		}
	}
	return out
}

func isPlainSlug(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// TestFoundationalSlugs_IsBaseAndCore — the foundational set carries
// the two infrastructure plugins (`base`, `core`) that ship a doc page
// but are excluded from the README gallery. #605 removed `base`; #625
// re-added it as a foundational plugin (no standalone card, composes
// chrome via plugin_base*). Any further addition needs a constitution
// amendment per docs/scope.md.
func TestFoundationalSlugs_IsBaseAndCore(t *testing.T) {
	t.Parallel()
	want := map[string]struct{}{"base": {}, "core": {}}
	got := map[string]struct{}{}
	for _, s := range foundationalSlugs {
		got[s] = struct{}{}
	}
	for s := range want {
		if _, ok := got[s]; !ok {
			t.Errorf("foundationalSlugs missing %q", s)
		}
	}
	for s := range got {
		if _, ok := want[s]; !ok {
			t.Errorf("foundationalSlugs has unexpected %q", s)
		}
	}
}

// TestRenderPluginPage_CoreOmitsSampleImage — `core` has no standalone
// visual output, so its rendered page MUST NOT reference a non-existent
// plugin-core.svg image and SHOULD include the canonical no-output notice.
func TestRenderPluginPage_CoreOmitsSampleImage(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Name: "core", Description: "Global configuration and options"}
	got := renderPluginPage("core", meta, nil, nil)
	if strings.Contains(got, "plugin-core.svg") {
		t.Errorf("core page must not reference plugin-core.svg sample image:\n%s", got)
	}
	if !strings.Contains(got, "This plugin emits no standalone SVG") {
		t.Errorf("core page should carry the no-standalone-SVG notice:\n%s", got)
	}
	if !strings.Contains(got, "## Requirements") {
		t.Errorf("core page should emit Requirements section on first gen:\n%s", got)
	}
	if !strings.Contains(got, "Core has no standalone visual output") {
		t.Errorf("core Requirements should explain why no image is rendered:\n%s", got)
	}
}

// TestRenderPluginPage_HasRequiredSections enforces that the rendered
// plugin page contains the 3 AUTOGEN sections + a `## Sample` heading.
func TestRenderPluginPage_HasRequiredSections(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{
		Name:        "languages",
		Description: "Display language usage across repositories.",
		Inputs: map[string]pluginInput{
			"plugin_languages": {Description: "Enable languages plugin", Type: "boolean", Default: false},
		},
	}
	got := renderPluginPage("languages", meta, []string{"plugin_languages"}, nil)
	for _, want := range []string{
		"<!-- AUTOGEN_START: title-and-description -->",
		"<!-- AUTOGEN_END: title-and-description -->",
		"<!-- AUTOGEN_START: config-table -->",
		"<!-- AUTOGEN_END: config-table -->",
		"<!-- AUTOGEN_START: usage-snippet -->",
		"<!-- AUTOGEN_END: usage-snippet -->",
		"## Sample",
		"## Configuration (inputs)",
		"## Usage",
		"## References",
		"![languages sample](../examples/plugin-languages.svg)",
		"plugin_languages: yes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered page missing %q in:\n%s", want, got)
		}
	}
}

// TestRenderPluginPage_PreservesHumanZones verifies the re-generation
// path: existing prose between AUTOGEN markers and headings is pulled
// forward into the new render. Covers all three human-authored zones
// (When to use, Requirements, Notes) under English headings.
func TestRenderPluginPage_PreservesHumanZones(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{
		Name:        "languages",
		Description: "languages desc",
	}
	existing := `<!-- AUTOGEN_START: title-and-description -->
# Plugin: languages

old description
<!-- AUTOGEN_END: title-and-description -->

## Sample

![languages sample](../examples/plugin-languages.svg)

## When to use

Hand-authored prose for the when-to-use section.
Spanning multiple lines.

<!-- AUTOGEN_START: config-table -->
old config
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
old usage
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Public repositories with detectable source code.** Hand-authored Requirements paragraph.

## Notes

Hand-authored notes preserved across regeneration.

## References

- ...
`
	got := renderPluginPage("languages", meta, nil, []byte(existing))
	if !strings.Contains(got, "Hand-authored prose for the when-to-use section.") {
		t.Errorf("when-section human zone lost:\n%s", got)
	}
	if !strings.Contains(got, "Public repositories with detectable source code") {
		t.Errorf("Requirements human zone lost:\n%s", got)
	}
	if !strings.Contains(got, "Hand-authored notes preserved across regeneration.") {
		t.Errorf("notes human zone lost:\n%s", got)
	}
}

// TestRenderPluginPage_RequirementsRegexHandlesMissingNotes pins the
// regex behaviour when a previously-generated page has Requirements
// but no Notes section: the Requirements prose must still be pulled
// forward into the new render.
func TestRenderPluginPage_RequirementsRegexHandlesMissingNotes(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Name: "languages", Description: "languages desc"}
	existing := "<!-- AUTOGEN_START: title-and-description -->\n" +
		"# Plugin: languages\n\nold description\n" +
		"<!-- AUTOGEN_END: title-and-description -->\n\n" +
		"<!-- AUTOGEN_START: config-table -->\nold config\n<!-- AUTOGEN_END: config-table -->\n\n" +
		"<!-- AUTOGEN_START: usage-snippet -->\nold usage\n<!-- AUTOGEN_END: usage-snippet -->\n\n" +
		"## Requirements\n\nHand-authored Requirements without a Notes section.\n\n" +
		"## References\n\n- ...\n"
	got := renderPluginPage("languages", meta, nil, []byte(existing))
	if !strings.Contains(got, "Hand-authored Requirements without a Notes section.") {
		t.Errorf("Requirements prose lost when Notes is absent:\n%s", got)
	}
	if strings.Contains(got, "## Notes") {
		t.Errorf("Notes section should not be emitted when there is no prose:\n%s", got)
	}
}

// TestRenderPluginPage_SkipsEmptySections verifies that first-gen
// pages (no existing human prose) omit the "When to use" and "Notes"
// section headers entirely instead of leaving them empty or filled
// with a TODO placeholder.
func TestRenderPluginPage_SkipsEmptySections(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Name: "habits", Description: "habits desc"}
	got := renderPluginPage("habits", meta, nil, nil)
	if strings.Contains(got, "<!-- TODO:") {
		t.Errorf("TODO placeholder should not be emitted:\n%s", got)
	}
	if strings.Contains(got, "## When to use") {
		t.Errorf("empty When-to-use section should be omitted:\n%s", got)
	}
	if strings.Contains(got, "## Notes") {
		t.Errorf("empty Notes section should be omitted:\n%s", got)
	}
}

// TestRenderGallery_AllSlugsLinkedAlphabetically — the gallery table
// references every adopted slug exactly once, in alphabetical order.
func TestRenderGallery_AllSlugsLinkedAlphabetically(t *testing.T) {
	t.Parallel()
	got := renderGallery()
	for _, s := range adoptedSlugs {
		if !strings.Contains(got, sampleImageBase(s)+".svg") {
			t.Errorf("gallery missing image for slug %q", s)
		}
		if !strings.Contains(got, "docs/plugins/"+s+".md") {
			t.Errorf("gallery missing link for slug %q", s)
		}
	}
	// Spot check alphabetical order: achievements before activity.
	if strings.Index(got, "plugin-achievements") > strings.Index(got, "plugin-activity") {
		t.Errorf("gallery rows not alphabetical (achievements should precede activity)")
	}
}

// TestMergeReadme_InsertsGalleryFromScratch — first-time generation
// injects the plugins-gallery block at the documented anchor.
func TestMergeReadme_InsertsGalleryFromScratch(t *testing.T) {
	t.Parallel()
	readme := `# github-metrics

Some intro paragraph.

---

## Highlights

- bullet 1

## Plugins

(existing plugin table here)

## Output formats
`
	got, err := mergeReadme(readme, renderGallery())
	if err != nil {
		t.Fatalf("mergeReadme: %v", err)
	}
	if !strings.Contains(got, galleryMarkerStart) || !strings.Contains(got, galleryMarkerEnd) {
		t.Errorf("gallery markers missing after merge:\n%s", got)
	}
}

// TestMergeReadme_Idempotent — second invocation produces zero diff.
func TestMergeReadme_Idempotent(t *testing.T) {
	t.Parallel()
	readme := `# github-metrics

Intro.

---

## Highlights

stuff

## Plugins

table

## Output formats
`
	once, err := mergeReadme(readme, renderGallery())
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	twice, err := mergeReadme(once, renderGallery())
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if once != twice {
		t.Errorf("second merge produced diff (re-runs must be idempotent)")
	}
}

func TestExtractInputKeys_PreservesYAMLOrder(t *testing.T) {
	t.Parallel()
	root := repoRootForTest(t)
	_, keys, err := loadMetadata(root, "languages")
	if err != nil {
		t.Fatalf("loadMetadata languages: %v", err)
	}
	if len(keys) == 0 {
		t.Fatalf("expected non-empty input keys for languages")
	}
	if keys[0] != "plugin_languages" {
		t.Errorf("expected first key to be plugin_languages, got %q", keys[0])
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	return root
}

// TestPluginPagePath_LocaleSuffix pins the output path derivation for
// each supported locale: English lives at `docs/plugins/<slug>.md`
// (canonical, no suffix); Japanese lives at
// `docs/plugins/<slug>_ja.md`.
func TestPluginPagePath_LocaleSuffix(t *testing.T) {
	t.Parallel()
	got := pluginPagePath("/repo", "languages", enStrings)
	if !strings.HasSuffix(got, "docs/plugins/languages.md") {
		t.Errorf("en path suffix wrong: %s", got)
	}
	got = pluginPagePath("/repo", "languages", jaStrings)
	if !strings.HasSuffix(got, "docs/plugins/languages_ja.md") {
		t.Errorf("ja path suffix wrong: %s", got)
	}
}

// TestApplyTranslation_MergesDescriptionAndInputs verifies the JA
// overlay merge: description and per-input description are pulled from
// the overlay when present; unmodified fields fall through to the
// base; overlay entries for non-existent inputs are ignored.
func TestApplyTranslation_MergesDescriptionAndInputs(t *testing.T) {
	t.Parallel()
	base := pluginMetadata{
		Description: "English description",
		Inputs: map[string]pluginInput{
			"a": {Description: "English A", Type: "boolean", Default: false},
			"b": {Description: "English B", Type: "number", Default: 5},
		},
	}
	overlay := pluginMetadata{
		Description: "日本語の説明",
		Inputs: map[string]pluginInput{
			"a":       {Description: "日本語 A"},
			"missing": {Description: "ignored"},
		},
	}
	got := applyTranslation(base, overlay)
	if got.Description != "日本語の説明" {
		t.Errorf("description not overridden: %q", got.Description)
	}
	if got.Inputs["a"].Description != "日本語 A" {
		t.Errorf("input a description not overridden: %q", got.Inputs["a"].Description)
	}
	// Machine field must survive the overlay.
	if got.Inputs["a"].Type != "boolean" {
		t.Errorf("input a type dropped: %q", got.Inputs["a"].Type)
	}
	// Input b was not in overlay — must keep English text.
	if got.Inputs["b"].Description != "English B" {
		t.Errorf("input b description clobbered: %q", got.Inputs["b"].Description)
	}
	// Overlay entry for an input that does not exist in base is ignored.
	if _, ok := got.Inputs["missing"]; ok {
		t.Errorf("overlay-only input leaked into merged output")
	}
}

// TestRenderPluginPageLocale_JAUsesTranslatedHeadings verifies that
// the JA locale swaps every section heading and column header to its
// Japanese label and preserves the (locale-invariant) AUTOGEN markers.
func TestRenderPluginPageLocale_JAUsesTranslatedHeadings(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{
		Name:        "languages",
		Description: "日本語の説明",
		Inputs: map[string]pluginInput{
			"plugin_languages": {Description: "有効化", Type: "boolean", Default: false},
		},
	}
	got := renderPluginPageLocale("languages", meta, []string{"plugin_languages"}, nil, jaStrings)
	for _, want := range []string{
		"# プラグイン: languages",
		"## サンプル",
		"## 設定 (inputs)",
		"| 入力 | 説明 | 既定値 | 必須 | 型 |",
		"## 使い方",
		"### GitHub Action",
		"### CLI",
		"## 参考",
		"日本語の説明",
		"<!-- AUTOGEN_START: title-and-description -->",
		"<!-- AUTOGEN_END: title-and-description -->",
		"<!-- AUTOGEN_START: config-table -->",
		"<!-- AUTOGEN_END: config-table -->",
		"<!-- AUTOGEN_START: usage-snippet -->",
		"<!-- AUTOGEN_END: usage-snippet -->",
		"入力スキーマの正本",
		"upstream の metadata",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JA page missing %q in:\n%s", want, got)
		}
	}
	// English headings must NOT leak into the JA page.
	for _, forbidden := range []string{
		"# Plugin: ",
		"## Sample",
		"## Configuration (inputs)",
		"## Usage",
		"## References",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("JA page contains English heading %q:\n%s", forbidden, got)
		}
	}
}

// TestRenderPluginPageLocale_JAPreservesHumanZones pins the
// preserve-around-AUTOGEN behavior on the JA path: hand-authored prose
// under the Japanese section headings must be pulled forward on
// re-generation, mirroring the EN behavior.
func TestRenderPluginPageLocale_JAPreservesHumanZones(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Description: "説明"}
	existing := `<!-- AUTOGEN_START: title-and-description -->
# プラグイン: languages

古い説明
<!-- AUTOGEN_END: title-and-description -->

## サンプル

![languages sample](../examples/plugin-languages.svg)

## 利用シーン

手書きの利用シーン説明を保存します。

<!-- AUTOGEN_START: config-table -->
old config
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
old usage
<!-- AUTOGEN_END: usage-snippet -->

## 前提条件

手書きの前提条件。

## 備考

手書きの備考。

## 参考

- ...
`
	got := renderPluginPageLocale("languages", meta, nil, []byte(existing), jaStrings)
	for _, want := range []string{
		"手書きの利用シーン説明を保存します。",
		"手書きの前提条件。",
		"手書きの備考。",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JA human zone lost (%q):\n%s", want, got)
		}
	}
}

// TestGeneratePluginPage_JASkipsWhenTranslationAbsent asserts that
// running the generator against a slug without a metadata_ja.yml
// overlay produces no JA file and does not error — the design
// decision behind #761 is that a half-translated page is worse than
// none.
func TestGeneratePluginPage_JASkipsWhenTranslationAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Minimal base metadata.
	assetsDir := filepath.Join(root, "assets", "plugins", "untranslated")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	baseYAML := "name: test\ndescription: |\n  English only.\ninputs:\n  plugin_untranslated:\n    description: |\n      Enable\n    type: boolean\n    default: no\n"
	if err := os.WriteFile(filepath.Join(assetsDir, "metadata.yml"), []byte(baseYAML), 0o600); err != nil {
		t.Fatalf("write metadata.yml: %v", err)
	}
	// NOTE: intentionally no metadata_ja.yml.

	if err := os.MkdirAll(filepath.Join(root, "docs", "plugins"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	if err := generatePluginPage(root, "untranslated", jaStrings); err != nil {
		t.Fatalf("generatePluginPage(ja): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plugins", "untranslated_ja.md")); !os.IsNotExist(err) {
		t.Errorf("expected no _ja.md file when metadata_ja.yml is absent, stat err: %v", err)
	}
}

// TestGeneratePluginPage_JAEmitsPageWhenTranslationPresent covers the
// happy path end-to-end: with a metadata_ja.yml overlay in place, the
// generator writes docs/plugins/<slug>_ja.md and the JA description
// makes it into the rendered page.
func TestGeneratePluginPage_JAEmitsPageWhenTranslationPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	assetsDir := filepath.Join(root, "assets", "plugins", "translated")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	baseYAML := "name: test\ndescription: |\n  English description.\ninputs:\n  plugin_translated:\n    description: |\n      Enable\n    type: boolean\n    default: no\n"
	if err := os.WriteFile(filepath.Join(assetsDir, "metadata.yml"), []byte(baseYAML), 0o600); err != nil {
		t.Fatalf("write metadata.yml: %v", err)
	}
	jaYAML := "description: |\n  日本語の説明。\ninputs:\n  plugin_translated:\n    description: |\n      有効化\n"
	if err := os.WriteFile(filepath.Join(assetsDir, "metadata_ja.yml"), []byte(jaYAML), 0o600); err != nil {
		t.Fatalf("write metadata_ja.yml: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "docs", "plugins"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := generatePluginPage(root, "translated", jaStrings); err != nil {
		t.Fatalf("generatePluginPage(ja): %v", err)
	}
	out, err := os.ReadFile(filepath.Join(root, "docs", "plugins", "translated_ja.md"))
	if err != nil {
		t.Fatalf("read _ja.md: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "日本語の説明") {
		t.Errorf("JA description missing from generated page:\n%s", body)
	}
	if !strings.Contains(body, "有効化") {
		t.Errorf("JA input description missing from generated page:\n%s", body)
	}
	if !strings.Contains(body, "## サンプル") {
		t.Errorf("JA section heading missing from generated page:\n%s", body)
	}
	// English description must not leak through when JA overrides it.
	if strings.Contains(body, "English description.") {
		t.Errorf("EN description leaked into JA page:\n%s", body)
	}
}
