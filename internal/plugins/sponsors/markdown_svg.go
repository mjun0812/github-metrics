package sponsors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// Native-SVG markdown flow used by the sponsors "about" bio and goal
// description (#409 Phase B2). It handles the small subset the sponsors
// bio uses (paragraphs, inline links, images, emphasis) — the same
// subset renderMarkdown targets for the HTML path — flowing text into
// wrapped `<text>` lines with links wrapped in `<a>`. Images keep only
// their alt text (a raster target cannot size an unknown remote badge).
const (
	mdFont      = 14.0
	mdLineH     = mdFont * 1.35
	mdParaGap   = 8.0       // .markdown p { margin: 8px 0 }
	mdTextFill  = "#777777" // inherited svg body color
	mdLinkFill  = "#58a6ff" // .markdown a { color: #58a6ff }
	mdParaSplit = `\n[ \t]*\n+`
)

var (
	mdSVGImage   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	mdSVGLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdSVGParaRe  = regexp.MustCompile(mdParaSplit)
	mdSVGStashRe = regexp.MustCompile("\x00(\\d+)\x00")
	mdSVGEmph    = strings.NewReplacer("**", "", "`", "")
)

// mdWord is one whitespace-delimited token. When href is non-empty the
// word renders as link-colored text wrapped in an `<a>`.
type mdWord struct {
	text string
	href string
}

// mdParaWords tokenizes one paragraph into flowable words, extracting
// inline links (words carry the link href) and images (reduced to their
// alt text) and stripping emphasis markers from plain runs.
func mdParaWords(para string) []mdWord {
	type stash struct {
		label, href string
		link        bool
	}
	var items []stash
	repl := func(link bool, re *regexp.Regexp) func(string) string {
		return func(m string) string {
			g := re.FindStringSubmatch(m)
			items = append(items, stash{label: g[1], href: g[2], link: link})
			return fmt.Sprintf("\x00%d\x00", len(items)-1)
		}
	}
	para = mdSVGImage.ReplaceAllStringFunc(para, repl(false, mdSVGImage))
	para = mdSVGLink.ReplaceAllStringFunc(para, repl(true, mdSVGLink))

	var words []mdWord
	appendText := func(s string) {
		s = mdSVGEmph.Replace(s)
		s = strings.ReplaceAll(s, "*", "")
		for _, w := range strings.Fields(s) {
			words = append(words, mdWord{text: w})
		}
	}

	last := 0
	for _, loc := range mdSVGStashRe.FindAllStringSubmatchIndex(para, -1) {
		if loc[0] > last {
			appendText(para[last:loc[0]])
		}
		idx, _ := strconv.Atoi(para[loc[2]:loc[3]])
		it := items[idx]
		href := ""
		if it.link {
			href = it.href
		}
		for _, w := range strings.Fields(it.label) {
			words = append(words, mdWord{text: w, href: href})
		}
		last = loc[1]
	}
	if last < len(para) {
		appendText(para[last:])
	}
	return words
}

// mdWrap greedily word-wraps words to maxWidth px at mdFont.
func mdWrap(words []mdWord, maxWidth float64) [][]mdWord {
	spaceW := fontmetrics.Width(" ", mdFont)
	var out [][]mdWord
	cur := make([]mdWord, 0, len(words))
	curW := 0.0
	for _, w := range words {
		ww := fontmetrics.Width(w.text, mdFont)
		add := ww
		if len(cur) > 0 {
			add += spaceW
		}
		if len(cur) > 0 && curW+add > maxWidth {
			out = append(out, cur)
			cur, curW = nil, 0
			add = ww
		}
		cur = append(cur, w)
		curW += add
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// mdRenderLine emits one wrapped line, coalescing consecutive words that
// share a href (or none) into a single `<text>`; linked runs are wrapped
// in `<a>`.
func mdRenderLine(b *strings.Builder, line []mdWord, x, baseline float64) {
	spaceW := fontmetrics.Width(" ", mdFont)
	cx := x
	for i := 0; i < len(line); {
		href := line[i].href
		j := i
		var parts []string
		for j < len(line) && line[j].href == href {
			parts = append(parts, line[j].text)
			j++
		}
		text := strings.Join(parts, " ")
		fill := mdTextFill
		if href != "" {
			fill = mdLinkFill
		}
		txt := chrome.SVGText(cx, baseline, text, chrome.SVGTextOpts{Size: mdFont, Fill: fill})
		if href != "" {
			fmt.Fprintf(b, `<a href=%q>%s</a>`, partials.EscapeXML(href), txt)
		} else {
			b.WriteString(txt)
		}
		cx += fontmetrics.Width(text, mdFont)
		if j < len(line) {
			cx += spaceW
		}
		i = j
	}
}

// renderMarkdownSVG lays the markdown source out as wrapped native-SVG
// text starting at (x, top), constrained to maxWidth px. Returns the
// markup and the height consumed.
func renderMarkdownSVG(src string, x, top, maxWidth float64) (string, float64) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	paras := mdSVGParaRe.Split(strings.Trim(src, "\n"), -1)

	var b strings.Builder
	y := top
	first := true
	for _, para := range paras {
		if strings.TrimSpace(para) == "" {
			continue
		}
		if !first {
			y += mdParaGap
		}
		first = false
		for _, line := range mdWrap(mdParaWords(para), maxWidth) {
			mdRenderLine(&b, line, x, y+mdFont)
			y += mdLineH
		}
	}
	return b.String(), y - top
}
