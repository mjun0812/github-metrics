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
