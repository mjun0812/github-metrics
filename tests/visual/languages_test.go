package visual

import "testing"

// TestLanguages_Visual exercises the languages plugin's rendered SVG
// against R-002's DOM assertion menu, per the per-plugin parity
// checklist at specs/011-plugin-rendering-parity/plugins/languages.md.
//
// Input is the existing M4 byte golden at
// tests/golden/classic/m4/languages.svg, wrapped in a minimal SVG
// envelope by loadGoldenSVG (visual_test.go). The golden contains
// the post-T006 partial output (count header, <svg>-wrapped progress
// bar, per-language color-dot list, etc.) so these assertions verify
// the rendered DOM, not just the partial source.
//
// Per spec Q4 clarification, languages is the documented exception
// to default-config-only sub-mode coverage: the golden fixture
// includes the most-used section by default. Recently-used + indepth
// sub-modes are exercised by the partial-level unit tests (not here).
func TestLanguages_Visual(t *testing.T) {
	svg := loadGoldenSVG(t, "languages")

	t.Run("count_header_exists", func(t *testing.T) {
		// A1: <h2 class="field"> count header — present after T006.
		assertElementExists(t, svg, "h2.field", 1)
	})

	t.Run("count_header_has_languages_text", func(t *testing.T) {
		// A4: count header text "Language" (with or without trailing 's').
		assertTextContent(t, svg, "Language")
	})

	t.Run("section_subheader_exists", func(t *testing.T) {
		// A1: <h3 class="field"> section sub-header — present after T006.
		assertElementExists(t, svg, "h3.field", 1)
	})

	t.Run("most_used_section_text", func(t *testing.T) {
		// A4: section sub-header text.
		assertTextContent(t, svg, "Most used languages")
	})

	t.Run("language_bar_renders", func(t *testing.T) {
		// A2: rect.language-bar must have non-zero rendered width —
		// the KEY assertion that catches the v1.0.0 bare-<g> bug. The
		// post-T006 partial wraps the <g class="languages-progress">
		// in <svg class="bar"> so the rects layout correctly inside
		// foreignObject.
		assertBoundingBoxNonZero(t, svg, "rect.language-bar")
	})
}
