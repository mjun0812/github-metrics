package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
}

// TestRenderPluginPage_HasRequiredSections enforces that the rendered
// plugin page contains the 3 AUTOGEN sections, the sample image and the
// usage snippet.
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
		"![languages sample](../examples/plugin-languages.svg)",
		"plugin_languages: yes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered page missing %q in:\n%s", want, got)
		}
	}
}

// TestRenderPluginPageLocale_PreservesHumanZones verifies the
// re-generation path: existing prose under the locale's section headings
// is pulled forward into the new render. Covers all three human-authored
// zones (When to use, Requirements, Notes) in English and Japanese, and
// the case where a previous page has Requirements but no Notes.
func TestRenderPluginPageLocale_PreservesHumanZones(t *testing.T) {
	t.Parallel()
	// page lays out a previously generated page: the title block, the
	// optional "when to use" prose, the generated config / usage blocks,
	// then the trailing human zones.
	page := func(title, when, tail string) string {
		return "<!-- AUTOGEN_START: title-and-description -->\n# " + title + "\n\nold description\n" +
			"<!-- AUTOGEN_END: title-and-description -->\n\n" + when +
			"<!-- AUTOGEN_START: config-table -->\nold config\n<!-- AUTOGEN_END: config-table -->\n\n" +
			"<!-- AUTOGEN_START: usage-snippet -->\nold usage\n<!-- AUTOGEN_END: usage-snippet -->\n\n" + tail
	}

	cases := []struct {
		name     string
		strs     localeStrings
		existing string
		want     []string
		notWant  []string
	}{
		{
			name: "en all zones",
			strs: enStrings,
			existing: page("Plugin: languages",
				"## When to use\n\nHand-authored prose for the when-to-use section.\nSpanning multiple lines.\n\n",
				"## Requirements\n\n**Public repositories with detectable source code.** Hand-authored Requirements paragraph.\n\n"+
					"## Notes\n\nHand-authored notes preserved across regeneration.\n\n"+
					"## References\n\n- ...\n"),
			want: []string{
				"Hand-authored prose for the when-to-use section.",
				"Public repositories with detectable source code",
				"Hand-authored notes preserved across regeneration.",
			},
		},
		{
			name: "en requirements without notes",
			strs: enStrings,
			existing: page("Plugin: languages", "",
				"## Requirements\n\nHand-authored Requirements without a Notes section.\n\n"+
					"## References\n\n- ...\n"),
			want:    []string{"Hand-authored Requirements without a Notes section."},
			notWant: []string{"## Notes"},
		},
		{
			name: "ja all zones",
			strs: jaStrings,
			existing: page("プラグイン: languages",
				"## 利用シーン\n\n手書きの利用シーン説明を保存します。\n\n",
				"## 前提条件\n\n手書きの前提条件。\n\n"+
					"## 備考\n\n手書きの備考。\n\n"+
					"## 参考\n\n- ...\n"),
			want: []string{
				"手書きの利用シーン説明を保存します。",
				"手書きの前提条件。",
				"手書きの備考。",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta := pluginMetadata{Name: "languages", Description: "desc"}
			got := renderPluginPageLocale("languages", meta, nil, []byte(tc.existing), tc.strs)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("human zone lost (%q):\n%s", want, got)
				}
			}
			for _, bad := range tc.notWant {
				if strings.Contains(got, bad) {
					t.Errorf("unexpected %q in:\n%s", bad, got)
				}
			}
		})
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

// TestMergeReadme_Idempotent — the first merge injects the gallery block
// at the documented anchor and a second invocation produces zero diff.
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
	if !strings.Contains(once, galleryMarkerStart) || !strings.Contains(once, galleryMarkerEnd) {
		t.Errorf("gallery markers missing after merge:\n%s", once)
	}
	twice, err := mergeReadme(once, renderGallery())
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if once != twice {
		t.Errorf("second merge produced diff (re-runs must be idempotent)")
	}
}

// TestApplyTranslation_MergesDescriptionAndInputs verifies the JA
// overlay merge: description and per-input description are pulled from
// the overlay when present; unmodified fields fall through to the
// base. Overlay entries for non-existent inputs are now rejected as
// translator typos — see TestApplyTranslation_UnknownInputIsError.
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
			"a": {Description: "日本語 A"},
		},
	}
	got, err := applyTranslation(base, overlay)
	if err != nil {
		t.Fatalf("applyTranslation: %v", err)
	}
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
}

// TestApplyTranslation_UnknownInputIsError guards SHOULD FIX #1 (map-key
// leg): an overlay entry for an input slug that does not exist in the
// base is treated as a translator typo. Silently dropping it would ship
// JA pages missing the intended translation without any warning — the
// exact fail-silent trap the review flagged.
func TestApplyTranslation_UnknownInputIsError(t *testing.T) {
	t.Parallel()
	base := pluginMetadata{
		Description: "English",
		Inputs: map[string]pluginInput{
			"plugin_languages": {Description: "Enable", Type: "boolean"},
		},
	}
	overlay := pluginMetadata{
		Inputs: map[string]pluginInput{
			// Typo — intended `plugin_languages`.
			"pluin_languages": {Description: "有効化"},
		},
	}
	_, err := applyTranslation(base, overlay)
	if err == nil {
		t.Fatalf("expected error for unknown overlay input, got nil")
	}
	if !strings.Contains(err.Error(), "pluin_languages") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

// TestRenderPluginPageLocale_JAUsesTranslatedHeadings verifies that
// the JA locale swaps the section headings to their Japanese labels,
// preserves the (locale-invariant) AUTOGEN markers and does not leak
// English headings.
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
		"## 使い方",
		"## 参考",
		"日本語の説明",
		"<!-- AUTOGEN_START: title-and-description -->",
		"<!-- AUTOGEN_END: usage-snippet -->",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JA page missing %q in:\n%s", want, got)
		}
	}
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

// TestGeneratePluginPage_JAOverlay drives generatePluginPage for the JA
// locale against a temporary repo layout. A page is written only when a
// content-bearing overlay exists; an absent or content-empty overlay is
// skipped without error ("half-translated is worse than none"), and a
// mis-spelled key or malformed YAML fails loudly naming the source file.
func TestGeneratePluginPage_JAOverlay(t *testing.T) {
	t.Parallel()
	const baseYAML = "name: test\ndescription: |\n  English description.\ninputs:\n  plugin_x:\n    description: |\n      Enable\n    type: boolean\n    default: no\n"

	cases := []struct {
		name     string
		overlay  *string // nil = no metadata_ja.yml
		wantErr  []string
		wantPage bool
		wantBody []string
	}{
		{name: "absent overlay is skipped"},
		{name: "comment-only overlay is skipped", overlay: ptr("# TODO: translate\n\n")},
		{
			name:    "typoed top-level key errors",
			overlay: ptr("descripton: |\n  日本語の説明。\n"),
			// The error must name the offending key and the source file.
			wantErr: []string{"descripton", "metadata_ja.yml"},
		},
		{
			name:    "malformed YAML errors",
			overlay: ptr("description: { unterminated\n"),
			wantErr: []string{"metadata_ja.yml"},
		},
		{
			name:     "translated overlay emits page",
			overlay:  ptr("description: |\n  日本語の説明。\ninputs:\n  plugin_x:\n    description: |\n      有効化\n"),
			wantPage: true,
			wantBody: []string{"日本語の説明", "有効化", "## サンプル"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			assetsDir := filepath.Join(root, "assets", "plugins", "x")
			if err := os.MkdirAll(assetsDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(assetsDir, "metadata.yml"), []byte(baseYAML), 0o600); err != nil {
				t.Fatalf("write metadata.yml: %v", err)
			}
			if tc.overlay != nil {
				if err := os.WriteFile(filepath.Join(assetsDir, "metadata_ja.yml"), []byte(*tc.overlay), 0o600); err != nil {
					t.Fatalf("write metadata_ja.yml: %v", err)
				}
			}
			if err := os.MkdirAll(filepath.Join(root, "docs", "plugins"), 0o755); err != nil {
				t.Fatalf("mkdir docs: %v", err)
			}

			err := generatePluginPage(root, "x", jaStrings)
			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				for _, w := range tc.wantErr {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error should mention %q, got: %v", w, err)
					}
				}
			} else if err != nil {
				t.Fatalf("generatePluginPage(ja): %v", err)
			}

			body, readErr := os.ReadFile(filepath.Join(root, "docs", "plugins", "x_ja.md"))
			if !tc.wantPage {
				if !os.IsNotExist(readErr) {
					t.Errorf("expected no _ja.md file, read err: %v", readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatalf("read _ja.md: %v", readErr)
			}
			for _, w := range tc.wantBody {
				if !strings.Contains(string(body), w) {
					t.Errorf("generated page missing %q:\n%s", w, body)
				}
			}
			// English description must not leak through when JA overrides it.
			if strings.Contains(string(body), "English description.") {
				t.Errorf("EN description leaked into JA page:\n%s", body)
			}
		})
	}
}

func ptr(s string) *string { return &s }
