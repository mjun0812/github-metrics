package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/tdewolff/minify/v2"
	mcss "github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

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
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(in))
	if err != nil {
		return in, fmt.Errorf("render: OptimizeCSS parse: %w", err)
	}

	var firstErr error
	doc.Find(`style[data-optimizable="true"]`).Each(func(_ int, sel *goquery.Selection) {
		original := sel.Text()
		optimized, optErr := purgeAndMinify(original, doc)
		if optErr != nil {
			if firstErr == nil {
				firstErr = optErr
			}
			return
		}
		sel.SetText(optimized)
	})
	if firstErr != nil {
		return in, firstErr
	}

	// goquery's Html() always wraps fragments in <html><body>; the
	// upstream caller wants the original SVG envelope preserved. We
	// rebuild the output by reading the document selection's HTML
	// content rather than the auto-wrapped page.
	if doc.Find("html").Length() > 0 {
		return goquery.OuterHtml(doc.Find("html").First())
	}
	return goquery.OuterHtml(doc.Selection)
}

// purgeAndMinify tokenizes `source` (the inner text of a single
// <style> element), drops unused selectors, and minifies the rest.
// Selectors are evaluated against the supplied document — anything
// that matches at least one node, or that contains an ID selector,
// survives.
func purgeAndMinify(source string, doc *goquery.Document) (string, error) {
	p := css.NewParser(parse.NewInput(strings.NewReader(source)), false)

	var rebuilt bytes.Buffer
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
			if keepSelector(selectorText, doc) {
				rebuilt.WriteString(selectorText)
				rebuilt.WriteString("{")
				// Copy declarations until we see EndRulesetGrammar.
				copyRulesetBody(p, &rebuilt)
				rebuilt.WriteString("}")
			} else {
				skipRulesetBody(p)
			}

		case css.AtRuleGrammar, css.BeginAtRuleGrammar, css.EndAtRuleGrammar:
			// At-rules (`@media`, `@keyframes`) keep their entire
			// body — purging them would require recursive analysis
			// that the upstream optimizer also skips.
			rebuilt.Write(data)
			rebuilt.Write(valuesBytes(p.Values()))
			if gt == css.AtRuleGrammar {
				rebuilt.WriteString(";")
			}

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
