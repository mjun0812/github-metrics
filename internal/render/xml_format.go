package render

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// FormatXML returns a re-indented copy of the input XML/SVG payload.
// The output uses two-space indentation, a single `\n` line separator,
// and emits empty elements as self-closing `<foo/>` tokens (matching
// the upstream xml-formatter `collapseContent=true` knob).
//
// Whitespace-only text nodes are discarded; non-empty text content
// inside an element ends up on its own line. CDATA, processing
// instructions, comments, and numeric character references survive
// the round trip via encoding/xml's standard tokenizer.
//
// The function is intentionally permissive: malformed input bubbles
// up as an error so the engine dispatch can append it to res.Errors
// (FR-018) and fall back to the prior pipeline stage's output.
func FormatXML(in string) (string, error) {
	if strings.TrimSpace(in) == "" {
		return in, nil
	}

	dec := xml.NewDecoder(strings.NewReader(in))
	// Honor numeric refs like &#xE2; verbatim — they would otherwise
	// be normalized away by the default decoder behavior on Go 1.26.
	dec.Strict = false
	dec.AutoClose = nil

	var buf bytes.Buffer
	depth := 0
	pendingOpen := false // tracks whether the previous token was an
	// open tag we should treat as potentially self-closing.

	// defaultNSStack tracks the inherited default xmlns at each depth
	// so we can re-emit `xmlns="..."` whenever an element introduces a
	// new default namespace. Go's encoding/xml decoder consumes the
	// xmlns attribute on the way in and surfaces it only as
	// StartElement.Name.Space, so writeStart cannot know which
	// elements need the attribute back without this state.
	defaultNSStack := []string{""}

	writeIndent := func(d int) {
		for i := 0; i < d; i++ {
			buf.WriteString("  ")
		}
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return in, fmt.Errorf("render: FormatXML decode: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if pendingOpen {
				// The previous start element had no children; emit
				// `>` to close it normally.
				buf.WriteString(">\n")
			}
			writeIndent(depth)
			parentDefaultNS := defaultNSStack[len(defaultNSStack)-1]
			writeStart(&buf, t, parentDefaultNS)
			newDefaultNS := parentDefaultNS
			if isURINamespace(t.Name.Space) {
				newDefaultNS = t.Name.Space
			}
			defaultNSStack = append(defaultNSStack, newDefaultNS)
			pendingOpen = true
			depth++

		case xml.EndElement:
			depth--
			if len(defaultNSStack) > 1 {
				defaultNSStack = defaultNSStack[:len(defaultNSStack)-1]
			}
			if pendingOpen {
				// Empty element: collapse to self-closing.
				buf.WriteString("/>\n")
				pendingOpen = false
				continue
			}
			writeIndent(depth)
			buf.WriteString("</")
			if prefix := localPrefix(t.Name.Space); prefix != "" {
				buf.WriteString(prefix)
				buf.WriteString(":")
			}
			buf.WriteString(t.Name.Local)
			buf.WriteString(">\n")

		case xml.CharData:
			text := string(t)
			trimmed := strings.TrimSpace(text)
			if trimmed == "" {
				continue
			}
			if pendingOpen {
				buf.WriteString(">\n")
				pendingOpen = false
			}
			writeIndent(depth)
			if err := xml.EscapeText(&buf, []byte(trimmed)); err != nil {
				return in, fmt.Errorf("render: FormatXML escape: %w", err)
			}
			buf.WriteString("\n")

		case xml.Comment:
			if pendingOpen {
				buf.WriteString(">\n")
				pendingOpen = false
			}
			writeIndent(depth)
			buf.WriteString("<!--")
			buf.Write(t)
			buf.WriteString("-->\n")

		case xml.ProcInst:
			if pendingOpen {
				buf.WriteString(">\n")
				pendingOpen = false
			}
			writeIndent(depth)
			buf.WriteString("<?")
			buf.WriteString(t.Target)
			if len(t.Inst) > 0 {
				buf.WriteString(" ")
				buf.Write(t.Inst)
			}
			buf.WriteString("?>\n")

		case xml.Directive:
			if pendingOpen {
				buf.WriteString(">\n")
				pendingOpen = false
			}
			writeIndent(depth)
			buf.WriteString("<!")
			buf.Write(t)
			buf.WriteString(">\n")
		}
	}

	if pendingOpen {
		// Trailing open tag with no closer — should not happen on
		// well-formed input but guard anyway.
		buf.WriteString("/>\n")
	}
	return buf.String(), nil
}

// writeStart serializes an xml.StartElement opening tag (without the
// closing '>' so callers can decide between '>' and '/>').
//
// Go's encoding/xml decoder consumes `xmlns="..."` attributes on the
// way in and surfaces the resolved URI only via Name.Space. To keep
// the output well-formed, we re-emit `xmlns="<uri>"` whenever an
// element introduces a default namespace that differs from the
// parent's (parentDefaultNS == "" for the root element). The
// localPrefix helper keeps short prefixes (e.g. xml, xlink) so
// attributes like `xml:lang` round-trip correctly; URI-shaped
// namespaces map to the empty prefix because they belong to the
// default xmlns scope.
func writeStart(buf *bytes.Buffer, t xml.StartElement, parentDefaultNS string) {
	buf.WriteString("<")
	if prefix := localPrefix(t.Name.Space); prefix != "" {
		buf.WriteString(prefix)
		buf.WriteString(":")
	}
	buf.WriteString(t.Name.Local)

	// Re-emit `xmlns="..."` when this element starts a new default
	// namespace scope. Without this, FormatXML drops the SVG xmlns
	// attribute and produces a document that cannot be reopened as
	// SVG.
	if isURINamespace(t.Name.Space) && t.Name.Space != parentDefaultNS {
		buf.WriteString(` xmlns="`)
		_ = xml.EscapeText(buf, []byte(t.Name.Space))
		buf.WriteString(`"`)
	}

	for _, a := range t.Attr {
		buf.WriteString(" ")
		if prefix := localPrefix(a.Name.Space); prefix != "" {
			buf.WriteString(prefix)
			buf.WriteString(":")
		}
		buf.WriteString(a.Name.Local)
		buf.WriteString(`="`)
		_ = xml.EscapeText(buf, []byte(a.Value))
		buf.WriteString(`"`)
	}
}

// isURINamespace reports whether `ns` is a URI-shaped namespace
// (e.g. "http://www.w3.org/2000/svg") rather than a short prefix
// (e.g. "xml", "xlink"). Used by both localPrefix (decides whether to
// drop the value) and writeStart (decides whether to emit xmlns).
func isURINamespace(ns string) bool {
	return strings.Contains(ns, "://")
}

// localPrefix returns the prefix to emit before a local name. URI-
// shaped namespaces map to "" because they are already carried by
// the default xmlns scope (writeStart re-emits the xmlns attribute
// at the appropriate boundary). Short prefixes (e.g. "xlink", "xml")
// are kept so attributes like xlink:href round trip correctly.
func localPrefix(ns string) string {
	if ns == "" {
		return ""
	}
	if isURINamespace(ns) {
		return ""
	}
	return ns
}
