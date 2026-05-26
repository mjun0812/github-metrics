package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdoptedSlugsMatchCompliance asserts the doc generator's plugin
// list stays aligned with `tests/compliance/compliance_test.go::adoptedM4Plugins`
// (minus base / core / languages.recent / languages.indepth).
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
		if tok == "base" || tok == "core" {
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

// TestFoundationalSlugs_AreBaseAndCore — the foundational set is the
// exact pair {base, core}. Adding a third foundational plugin would
// require a constitution amendment per docs/design/15-selection-answer.md.
func TestFoundationalSlugs_AreBaseAndCore(t *testing.T) {
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
	if !strings.Contains(got, "No standalone visual output") {
		t.Errorf("core page should carry the No-standalone-visual-output notice:\n%s", got)
	}
	if !strings.Contains(got, "## Requirements") {
		t.Errorf("core page should emit Requirements section on first gen:\n%s", got)
	}
	if !strings.Contains(got, "Core has no standalone visual output") {
		t.Errorf("core Requirements should explain why no image is rendered:\n%s", got)
	}
}

// TestRenderPluginPage_BaseEmitsCustomUsageSnippet — `base` is always
// active and does not respond to a `plugin_base: yes` toggle. The usage
// snippet must show the canonical `base: header, ...` sections input
// instead of the generic `plugin_<slug>: yes` shape used by the 19
// adopted plugins.
func TestRenderPluginPage_BaseEmitsCustomUsageSnippet(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Name: "base", Description: ""}
	got := renderPluginPage("base", meta, nil, nil)
	if strings.Contains(got, "plugin_base: yes") {
		t.Errorf("base usage snippet must not use the generic plugin_<slug>=yes shape:\n%s", got)
	}
	if !strings.Contains(got, "base: header, activity, community, repositories, metadata") {
		t.Errorf("base usage snippet should show the canonical sections list:\n%s", got)
	}
	if !strings.Contains(got, "plugin-base.svg") {
		t.Errorf("base page should reference its sample SVG (base has visual output):\n%s", got)
	}
	if !strings.Contains(got, "## Requirements") {
		t.Errorf("base page should emit Requirements section on first gen:\n%s", got)
	}
}

// TestRenderPluginPage_HasRequiredSections enforces that the rendered
// plugin page contains the 3 AUTOGEN sections + a `## サンプル出力`
// heading.
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
		"## サンプル出力",
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
// (when-to-use, Requirements, 既知の制約).
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

## サンプル出力

![languages sample](../examples/plugin-languages.svg)

## このプラグインを使うべきケース

ハンドメイドの説明文があります。
複数行にわたります。

<!-- AUTOGEN_START: config-table -->
old config
<!-- AUTOGEN_END: config-table -->

<!-- AUTOGEN_START: usage-snippet -->
old usage
<!-- AUTOGEN_END: usage-snippet -->

## Requirements

**Public repositories with detectable source code.** This zone was added by PR #410.

## 既知の制約 / 注意点

注意点もハンドメイドで書かれた内容です。

## 参照

- ...
`
	got := renderPluginPage("languages", meta, nil, []byte(existing))
	if !strings.Contains(got, "ハンドメイドの説明文があります") {
		t.Errorf("when-section human zone lost:\n%s", got)
	}
	if !strings.Contains(got, "Public repositories with detectable source code") {
		t.Errorf("Requirements human zone lost:\n%s", got)
	}
	if !strings.Contains(got, "注意点もハンドメイドで書かれた内容です") {
		t.Errorf("pitfalls human zone lost:\n%s", got)
	}
	if strings.Contains(got, "<!-- TODO:") {
		t.Errorf("TODO placeholder should NOT appear when human zones are present:\n%s", got)
	}
}

// TestRenderPluginPage_EmitsTODOOnFirstGen verifies that first
// generation (no existing file) inserts the maintainer TODO placeholders.
func TestRenderPluginPage_EmitsTODOOnFirstGen(t *testing.T) {
	t.Parallel()
	meta := pluginMetadata{Name: "habits", Description: "habits desc"}
	got := renderPluginPage("habits", meta, nil, nil)
	occurrences := strings.Count(got, "<!-- TODO:")
	if occurrences != 2 {
		t.Errorf("expected 2 TODO placeholders on first-gen, got %d:\n%s", occurrences, got)
	}
}

// TestRenderGallery_AllSlugsLinkedAlphabetically — the gallery table
// references every adopted slug exactly once, in alphabetical order.
func TestRenderGallery_AllSlugsLinkedAlphabetically(t *testing.T) {
	t.Parallel()
	got := renderGallery()
	for _, s := range adoptedSlugs {
		if !strings.Contains(got, "plugin-"+s+".svg") {
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

func TestCountTODOs_HandlesMissingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	n, err := CountTODOs(dir) // no docs/plugins/ subdir
	if err != nil {
		t.Fatalf("CountTODOs: %v", err)
	}
	if n != 0 {
		t.Errorf("CountTODOs on empty repo = %d, want 0", n)
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
