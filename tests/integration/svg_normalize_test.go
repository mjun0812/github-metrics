// Package integration_test exposes shared test helpers for engine
// integration tests. NormalizeSVG returns a canonical representation
// of an SVG document so golden-file comparisons stay deterministic
// across runs.
package integration_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// NormalizeSVG produces a byte-stable representation of an SVG
// document by:
//
//  1. Replacing the dynamic footer fragments ("Last updated <time>"
//     and "<repo>/github-metrics@<version>") with __MASKED__ so test
//     runs don't fluctuate with timestamps or version strings.
//  2. Re-emitting elements with their attributes sorted
//     alphabetically.
//  3. Collapsing whitespace inside text nodes to a single space.
//  4. Dropping XML comments.
//
// The output is suitable for MD5-comparison golden files.
func NormalizeSVG(raw []byte) ([]byte, error) {
	masked := maskDynamic(raw)
	dec := xml.NewDecoder(bytes.NewReader(masked))
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("svg normalize: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			t.Attr = sortAttrs(t.Attr)
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.CharData:
			collapsed := collapseSpace(string(t))
			if collapsed == "" {
				continue
			}
			if err := enc.EncodeToken(xml.CharData(collapsed)); err != nil {
				return nil, err
			}
		case xml.Comment:
			// drop
		default:
			if err := enc.EncodeToken(tok); err != nil {
				return nil, err
			}
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	lastUpdatedRE = regexp.MustCompile(`Last updated [^<]+`)
	versionRE     = regexp.MustCompile(`github-metrics@[^<\s]+`)
)

func maskDynamic(raw []byte) []byte {
	out := lastUpdatedRE.ReplaceAll(raw, []byte("Last updated __MASKED__"))
	out = versionRE.ReplaceAll(out, []byte("github-metrics@__MASKED__"))
	return out
}

func sortAttrs(attrs []xml.Attr) []xml.Attr {
	out := append([]xml.Attr(nil), attrs...)
	sort.SliceStable(out, func(i, j int) bool {
		ki := out[i].Name.Space + ":" + out[i].Name.Local
		kj := out[j].Name.Space + ":" + out[j].Name.Local
		return ki < kj
	})
	return out
}

var multiSpace = regexp.MustCompile(`\s+`)

func collapseSpace(s string) string {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return ""
	}
	return multiSpace.ReplaceAllString(trim, " ")
}
