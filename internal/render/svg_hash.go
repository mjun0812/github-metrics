package render

import (
	"crypto/md5" //nolint:gosec // MD5 is a non-security comparison key (upstream metrics.utils.svg.hash compatibility); see contracts/svg-hash.md
	"encoding/hex"
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Hash returns the MD5 hex digest (32 lowercase chars) of the
// SVG's outer HTML after the first <footer> element is removed. The
// hash is the comparison key that backs `output_condition=data-changed`
// (M6 T-114): two SVGs whose DOM differs only in the footer
// (timezone / version / generated time) MUST hash identically.
//
// Edge cases:
//
//   - Whitespace-only or empty input → ("", nil)
//   - Document without an <svg> root → ("", *xerrors.InputError)
//   - Multiple <footer> nodes → only the first one is removed
//
// The implementation is pure Go (goquery / crypto/md5); no chromedp
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

	// Remove ONLY the first <footer>; subsequent footers (if any) are
	// load-bearing content that affects the hash.
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
