// Package svgcontent extracts the *semantic* (visible / structural)
// content of a rendered metrics SVG so tests can assert on what a
// viewer actually sees — not on byte equality or pixel height.
//
// It exists to close the verification gap behind issues #463–#472:
// the byte-golden and height/file-size suites are circular (the
// goldens were generated from the same implementation under test) or
// too coarse to notice "License is missing", "files changed is
// absent", or "the card rendered empty". This package parses the SVG
// (including the xhtml inside <foreignObject>) with goquery, strips
// non-rendered nodes (<style>/<defs>/<script>), and exposes the
// remaining visible text and DOM structure for content-level
// assertions.
package svgcontent

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Doc parses raw SVG/HTML bytes into a goquery document with the
// non-rendered nodes (<style>, <defs>, <script>) removed, so every
// subsequent query and Text() call sees only content a viewer would
// actually render.
func Doc(raw []byte) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	// goquery/net-html lowercases element names, so a single
	// lowercase selector covers SVG and xhtml alike.
	doc.Find("style, defs, script").Remove()
	return doc, nil
}

var multiSpace = regexp.MustCompile(`\s+`)

// VisibleText returns the document's rendered text content collapsed
// to single spaces. Style/defs/script have already been stripped by
// Doc, so CSS rules and `<defs>` payloads do not leak in.
func VisibleText(raw []byte) (string, error) {
	doc, err := Doc(raw)
	if err != nil {
		return "", err
	}
	return Normalize(doc.Text()), nil
}

// Normalize trims and collapses internal whitespace to single
// spaces. Exported so callers can normalize an expected marker the
// same way before a Contains check.
func Normalize(s string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(s, " "))
}

// Contains reports whether the visible text contains sub, comparing
// case-insensitively after whitespace normalization. Returns an
// error only if the SVG fails to parse.
func Contains(raw []byte, sub string) (bool, error) {
	text, err := VisibleText(raw)
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(Normalize(sub))), nil
}

// Tokens returns the sorted, de-duplicated, lowercased whitespace
// tokens of the visible text. Useful for set-subset comparisons
// between an upstream reference and our output.
func Tokens(raw []byte) ([]string, error) {
	text, err := VisibleText(raw)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, tok := range strings.Fields(strings.ToLower(text)) {
		seen[tok] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out, nil
}

// MatchCount returns how many times re matches across the visible
// text (not the raw markup). Use for "does a '<n> files changed'
// label appear" style structural assertions.
func MatchCount(raw []byte, re *regexp.Regexp) (int, error) {
	text, err := VisibleText(raw)
	if err != nil {
		return 0, err
	}
	return len(re.FindAllString(text, -1)), nil
}

// SelectorCount returns how many nodes match the goquery selector
// (after style/defs/script removal). Use for "the card body is not
// empty" structural assertions where text alone is insufficient.
func SelectorCount(raw []byte, selector string) (int, error) {
	doc, err := Doc(raw)
	if err != nil {
		return 0, err
	}
	return doc.Find(selector).Length(), nil
}
