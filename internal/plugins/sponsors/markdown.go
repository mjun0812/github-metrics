package sponsors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// renderMarkdown converts the small subset of Markdown that GitHub
// Sponsors `shortDescription` fields use into the HTML the classic
// template emits inside `<div class="markdown">`. Upstream renders the
// bio with `imports.markdown` and emits it unescaped (`<%- %>` in
// sponsors.ejs); we mirror the resulting markup for the common cases
// (paragraphs, links, images, bold/italic/code) while escaping all text
// nodes so the output stays valid, injection-safe XML.
//
// This is intentionally a focused renderer rather than a full CommonMark
// implementation: the sponsors bio only ever contains paragraphs,
// inline links and images.
func renderMarkdown(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	// Split into paragraphs on one-or-more blank lines.
	blocks := regexp.MustCompile(`\n[ \t]*\n+`).Split(strings.Trim(src, "\n"), -1)

	var b strings.Builder
	for _, block := range blocks {
		block = strings.Trim(block, "\n")
		if strings.TrimSpace(block) == "" {
			continue
		}
		fmt.Fprintf(&b, "<p>%s</p>", renderInline(block))
	}
	return b.String()
}

var (
	mdImage = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	mdLink  = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdBold  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItal  = regexp.MustCompile(`\*([^*]+)\*`)
	mdCode  = regexp.MustCompile("`([^`]+)`")

	// inlinePlaceholderRe matches the stash markers emitted by
	// renderInline (NUL-wrapped indices), used to restore rendered HTML
	// fragments after the surrounding text has been escaped.
	inlinePlaceholderRe = regexp.MustCompile("\x00(\\d+)\x00")
)

// inlinePlaceholder marks already-rendered HTML fragments so subsequent
// passes (and the final text escape) leave them untouched.
type inlineToken struct {
	html string
}

// renderInline applies the supported inline transforms (images, links,
// emphasis, code) and escapes the remaining text. Rendered fragments are
// protected from re-escaping via a placeholder/substitution pass.
func renderInline(s string) string {
	tokens := []inlineToken{}
	stash := func(html string) string {
		tokens = append(tokens, inlineToken{html: html})
		return fmt.Sprintf("\x00%d\x00", len(tokens)-1)
	}

	// Images must be handled before links (they share `[...](...)`).
	s = mdImage.ReplaceAllStringFunc(s, func(m string) string {
		g := mdImage.FindStringSubmatch(m)
		return stash(fmt.Sprintf(
			`<img src="%s" alt="%s" />`,
			partials.EscapeXML(g[2]), partials.EscapeXML(g[1]),
		))
	})
	s = mdLink.ReplaceAllStringFunc(s, func(m string) string {
		g := mdLink.FindStringSubmatch(m)
		return stash(fmt.Sprintf(
			`<a href="%s">%s</a>`,
			partials.EscapeXML(g[2]), partials.EscapeXML(g[1]),
		))
	})
	s = mdBold.ReplaceAllStringFunc(s, func(m string) string {
		g := mdBold.FindStringSubmatch(m)
		return stash(fmt.Sprintf(`<strong>%s</strong>`, partials.EscapeXML(g[1])))
	})
	s = mdItal.ReplaceAllStringFunc(s, func(m string) string {
		g := mdItal.FindStringSubmatch(m)
		return stash(fmt.Sprintf(`<em>%s</em>`, partials.EscapeXML(g[1])))
	})
	s = mdCode.ReplaceAllStringFunc(s, func(m string) string {
		g := mdCode.FindStringSubmatch(m)
		return stash(fmt.Sprintf(`<code>%s</code>`, partials.EscapeXML(g[1])))
	})

	// Escape the remaining literal text, then restore the rendered HTML.
	s = partials.EscapeXML(s)
	s = inlinePlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
		g := inlinePlaceholderRe.FindStringSubmatch(m)
		idx, err := strconv.Atoi(g[1])
		if err != nil || idx < 0 || idx >= len(tokens) {
			return ""
		}
		return tokens[idx].html
	})
	return s
}
