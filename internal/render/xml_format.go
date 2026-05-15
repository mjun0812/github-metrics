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

	writeIndent := func(d int) {
		for i := 0; i < d; i++ {
			buf.WriteString("  ")
		}
	}

	flushOpen := func() {
		// no-op holder so we can revisit the structure if needed.
	}
	_ = flushOpen

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
			writeStart(&buf, t)
			pendingOpen = true
			depth++

		case xml.EndElement:
			depth--
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
// Go's encoding/xml decoder reports Name.Space as the namespace URI
// (e.g. "http://www.w3.org/2000/svg") even for elements declared via
// the default xmlns attribute. We do not want that URI re-emitted as
// a prefix; the URI is preserved as the xmlns="..." attribute. So
// localPrefix returns "" for URI-shaped namespaces and the namespace
// verbatim for short prefixes like "xlink".
func writeStart(buf *bytes.Buffer, t xml.StartElement) {
	buf.WriteString("<")
	if prefix := localPrefix(t.Name.Space); prefix != "" {
		buf.WriteString(prefix)
		buf.WriteString(":")
	}
	buf.WriteString(t.Name.Local)
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

// localPrefix returns the prefix to emit before a local name. URI-
// shaped namespaces (those containing "://") map to "" because they
// are already carried by the xmlns="..." attribute. Short prefixes
// (e.g. "xlink", "xml") are kept so attributes like xlink:href round
// trip correctly.
func localPrefix(ns string) string {
	if ns == "" {
		return ""
	}
	if strings.Contains(ns, "://") {
		return ""
	}
	return ns
}
