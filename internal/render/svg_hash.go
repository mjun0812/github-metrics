package render

import (
	"crypto/md5" //nolint:gosec // MD5 is a non-security comparison key (upstream metrics.utils.svg.hash compatibility)
	"encoding/hex"
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Hash returns the MD5 hex digest (32 lowercase chars) of the SVG's
// outer HTML after the metadata footer is removed. The hash is the
// comparison key that backs `output_condition=data-changed` (M6 T-114):
// two SVGs whose DOM differs only in the footer (timezone / version /
// generated time) MUST hash identically.
//
// Since #409 Phase C the footer is native SVG wrapped in
// `<g data-section="metadata">` (the old HTML `<footer>` element is
// gone), so the timestamp-bearing group is stripped by that marker. A
// legacy first `<footer>` is still removed for backward compatibility
// with any pre-Phase-C SVG fed to the hasher.
//
// Edge cases:
//
//   - Whitespace-only or empty input → ("", nil)
//   - Document without an <svg> root → ("", *xerrors.InputError)
//   - Multiple metadata groups / footers → only the first is removed
//
// The implementation is pure Go (goquery / crypto/md5); no rasterizer
// dependency lands on the hash path.
func Hash(rendered string) (string, error) {
	if strings.TrimSpace(rendered) == "" {
		return "", nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rendered))
	if err != nil {
		return "", xerrors.NewInputError("rendered", err)
	}

	svgSel := doc.Find("svg").First()
	if svgSel.Length() == 0 {
		return "", xerrors.NewInputError("rendered", errors.New("no <svg> root"))
	}

	// Remove ONLY the first metadata group (native-SVG footer); the
	// legacy `<footer>` element is stripped too so pre-Phase-C SVGs keep
	// hashing footer-invariantly. Subsequent nodes are load-bearing
	// content that affects the hash.
	metaSel := svgSel.Find(`[data-section="metadata"]`).First()
	if metaSel.Length() > 0 {
		metaSel.Remove()
	}
	footerSel := svgSel.Find("footer").First()
	if footerSel.Length() > 0 {
		footerSel.Remove()
	}

	outer, err := goquery.OuterHtml(svgSel)
	if err != nil {
		return "", xerrors.NewInputError("rendered", err)
	}

	sum := md5.Sum([]byte(outer)) //nolint:gosec // hash is a non-security key, see import comment
	return hex.EncodeToString(sum[:]), nil
}
