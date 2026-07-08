package render

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/tdewolff/minify/v2"
	mcss "github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// optimizableStyleRe matches an embedded `<style data-optimizable="true">`
// block and captures its inner CSS. Non-greedy so adjacent blocks are
// matched individually. Mirrors upstream's regex
// (org_repo/source/app/metrics/utils.mjs optimize.css).
var optimizableStyleRe = regexp.MustCompile(`(?s)<style data-optimizable="true">(.*?)</style>`)

// OptimizeCSS walks the embedded `<style data-optimizable="true">`
// nodes, drops selectors that no element in the document matches,
// and minifies the result. Selectors that contain an ID component
// (`#metrics-end`, `#header`, etc.) are kept unconditionally so the
// svg.Resize anchor / template scaffold stays addressable even if no
// CSS targets it in the body (FR-015 + SC-006).
//
// CSS embedded in `<style>` tags without the `data-optimizable="true"`
// attribute is left as-is — that's the upstream contract for opting
// out per template.
func OptimizeCSS(in string) (string, error) {
	if strings.TrimSpace(in) == "" {
		return in, nil
	}

	matches := optimizableStyleRe.FindAllStringSubmatch(in, -1)
	if len(matches) == 0 {
		return in, nil
	}

	// Parse a READ-ONLY copy of the document for the selector-purge
	// analysis only (keepSelector needs to know which selectors match a
	// node in the body). The actual replacement happens via regex on the
	// original string below so the SVG body is preserved byte-for-byte,
	// matching upstream's optimize.css, which regex-extracts the style
	// blocks and never DOM-round-trips the rendered SVG (utils.mjs). A
	// goquery round-trip here would re-serialize the document as HTML —
	// injecting an <html>/<body> wrapper and dropping the SVG `xmlns` —
	// which the downstream FormatXML pass then mis-parses.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(in))
	if err != nil {
		return in, fmt.Errorf("render: OptimizeCSS parse: %w", err)
	}

	// Concatenate every optimizable block's (unescaped) CSS and optimize
	// them together, matching upstream which joins the extracted styles
	// before a single purge+minify pass.
	var combined strings.Builder
	for i, m := range matches {
		if i > 0 {
			combined.WriteByte('\n')
		}
		combined.WriteString(html.UnescapeString(m[1]))
	}
	optimized, optErr := purgeAndMinify(combined.String(), doc)
	if optErr != nil {
		return in, optErr
	}

	// Replace the FIRST optimizable block with the combined, optimized
	// <style>; drop any remaining optimizable blocks (their CSS now
	// lives in the first one). The optimized CSS is emitted raw — the
	// downstream FormatXML pass re-escapes text content, matching
	// upstream's optimize.css → optimize.xml ordering.
	replaced := false
	out := optimizableStyleRe.ReplaceAllStringFunc(in, func(string) string {
		if replaced {
			return ""
		}
		replaced = true
		return "<style>" + optimized + "</style>"
	})
	return out, nil
}

// purgeAndMinify tokenizes `source` (the inner text of a single
// <style> element), drops unused selectors, and minifies the rest.
// Selectors are evaluated against the supplied document — anything
// that matches at least one node, or that contains an ID selector,
// survives.
func purgeAndMinify(source string, doc *goquery.Document) (string, error) {
	p := css.NewParser(parse.NewInput(strings.NewReader(source)), false)

	var rebuilt bytes.Buffer
	// atRuleStack tracks nested at-rules so we know how to treat the
	// rulesets inside them. A `true` entry means the enclosing at-rule
	// is `@keyframes` (or a vendor-prefixed variant): its inner blocks
	// (`0%`, `from`, `to`, …) are keyframe selectors, NOT document
	// selectors, so they must be copied verbatim rather than purged.
	// `@media`/`@supports` push `false` — their inner rulesets are real
	// selectors and still get the unused-selector purge (matching
	// upstream purgecss, which drops empty `@media` blocks afterward).
	var atRuleStack []bool
	insideKeyframes := func() bool {
		for _, isKeyframes := range atRuleStack {
			if isKeyframes {
				return true
			}
		}
		return false
	}
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			if errMsg := p.Err(); errMsg != nil && errMsg.Error() != "EOF" {
				return "", fmt.Errorf("render: CSS parse: %w", errMsg)
			}
			break
		}

		switch gt {
		case css.BeginRulesetGrammar:
			values := p.Values()
			selectorText := joinValues(values)
			// Keyframe blocks are kept unconditionally — their `0%` /
			// `from` / `to` "selectors" are not addressable in the
			// document and must never be purged.
			if insideKeyframes() || keepSelector(selectorText, doc) {
				rebuilt.WriteString(selectorText)
				rebuilt.WriteString("{")
				// Copy declarations until we see EndRulesetGrammar.
				copyRulesetBody(p, &rebuilt)
				rebuilt.WriteString("}")
			} else {
				skipRulesetBody(p)
			}

		case css.BeginAtRuleGrammar:
			// `@media (...)` / `@keyframes name` / `@supports (...)`:
			// emit the prelude AND the opening brace that the parser
			// leaves implicit. Forgetting the brace collapses the
			// nested body into the prelude and produces invalid CSS
			// (e.g. `@keyframes name0%,100%{…}`), which the minifier
			// then mangles further.
			rebuilt.Write(data)
			rebuilt.Write(valuesBytes(p.Values()))
			rebuilt.WriteString("{")
			atRuleStack = append(atRuleStack, isKeyframesAtRule(data))

		case css.EndAtRuleGrammar:
			rebuilt.WriteString("}")
			if len(atRuleStack) > 0 {
				atRuleStack = atRuleStack[:len(atRuleStack)-1]
			}

		case css.AtRuleGrammar:
			// Statement at-rule (`@import …;`, `@charset …;`): prelude
			// followed by a semicolon, no body.
			rebuilt.Write(data)
			rebuilt.Write(valuesBytes(p.Values()))
			rebuilt.WriteString(";")

		case css.QualifiedRuleGrammar, css.DeclarationGrammar, css.CustomPropertyGrammar:
			rebuilt.Write(data)
			rebuilt.Write(valuesBytes(p.Values()))
			rebuilt.WriteString(";")

		case css.CommentGrammar, css.TokenGrammar:
			rebuilt.Write(data)

		case css.EndRulesetGrammar:
			// Already handled by copyRulesetBody.
		}
	}

	// Minify the rebuilt CSS.
	m := minify.New()
	m.AddFunc("text/css", mcss.Minify)
	out := &bytes.Buffer{}
	if err := mcss.Minify(m, out, bytes.NewReader(rebuilt.Bytes()), nil); err != nil {
		// On minifier failure, surface the unminified-but-purged CSS.
		return rebuilt.String(), nil //nolint:nilerr // graceful fallback
	}
	return out.String(), nil
}

// keepSelector reports whether `selectorText` should survive the
// purge. The decision rules:
//
//  1. Any token starting with `#` (ID selector) is kept regardless of
//     whether the body references it — the spec mandates the
//     svg.Resize anchor stay addressable (SC-006).
//  2. Otherwise, the selector is compiled via cascadia and we check
//     whether the document has at least one matching node.
//  3. Compile failures (unsupported selectors) keep the selector to
//     avoid silently dropping valid CSS.
func keepSelector(selectorText string, doc *goquery.Document) bool {
	trimmed := strings.TrimSpace(selectorText)
	if trimmed == "" {
		return false
	}
	// ID selector survives unconditionally — split on combinator
	// boundaries to catch `#x .y`, `#x:hover`, etc.
	for _, part := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '>' || r == '+' || r == '~'
	}) {
		if strings.HasPrefix(part, "#") {
			return true
		}
	}

	// Split comma-separated groups; keep the whole rule if ANY group
	// matches.
	for _, group := range strings.Split(trimmed, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		sel, err := cascadia.Compile(group)
		if err != nil {
			// Selector cascadia cannot understand — be conservative
			// and keep it.
			return true
		}
		if doc.Find(group).FilterFunction(func(_ int, _ *goquery.Selection) bool {
			return true
		}).Length() > 0 {
			return true
		}
		_ = sel
	}
	return false
}

// isKeyframesAtRule reports whether the at-rule keyword in `data`
// (e.g. `@keyframes`, `@-webkit-keyframes`) introduces a keyframes
// block. The tdewolff parser lowercases the keyword, so a simple
// suffix check covers the standard rule and every vendor prefix.
func isKeyframesAtRule(data []byte) bool {
	name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(data))), "@")
	return strings.HasSuffix(name, "keyframes")
}

// joinValues concatenates the token values produced by
// css.Parser.Values() into a single string for the selector header.
func joinValues(values []css.Token) string {
	var b strings.Builder
	for _, v := range values {
		b.Write(v.Data)
	}
	return b.String()
}

// valuesBytes is the bytes.Buffer-friendly equivalent of joinValues.
func valuesBytes(values []css.Token) []byte {
	var b bytes.Buffer
	for _, v := range values {
		b.Write(v.Data)
	}
	return b.Bytes()
}

// copyRulesetBody copies every token between BeginRulesetGrammar and
// the matching EndRulesetGrammar into `dst`, preserving declarations.
// Used when the selector survives the purge.
func copyRulesetBody(p *css.Parser, dst *bytes.Buffer) {
	for {
		gt, _, data := p.Next()
		switch gt {
		case css.EndRulesetGrammar:
			return
		case css.ErrorGrammar:
			return
		case css.DeclarationGrammar, css.CustomPropertyGrammar:
			dst.Write(data)
			dst.WriteString(":")
			dst.Write(valuesBytes(p.Values()))
			dst.WriteString(";")
		default:
			dst.Write(data)
			dst.Write(valuesBytes(p.Values()))
		}
	}
}

// skipRulesetBody consumes tokens until the matching
// EndRulesetGrammar. Used when the selector loses the purge.
func skipRulesetBody(p *css.Parser) {
	for {
		gt, _, _ := p.Next()
		if gt == css.EndRulesetGrammar || gt == css.ErrorGrammar {
			return
		}
	}
}
