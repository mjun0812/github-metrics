package sponsorships

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/render/fontmetrics"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// heartOcticon is the upstream `<%- octicon "heart" %>`-style 16x16
// path used in the sponsorships section header (EJS line 4).
const heartOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M10.586 1C12.268 1 13.5 2.37 13.5 4.25c0 1.745-.996 3.359-2.622 4.831-.166.15-.336.297-.509.438l1.116 5.584a.75.75 0 0 1-.991.852l-2.409-.876a.25.25 0 0 0-.17 0l-2.409.876a.75.75 0 0 1-.991-.852L5.63 9.519a13.78 13.78 0 0 1-.51-.438C3.497 7.609 2.5 5.995 2.5 4.25 2.5 2.37 3.732 1 5.414 1c.963 0 1.843.403 2.474 1.073L8 2.198l.112-.125a3.385 3.385 0 0 1 2.283-1.068L10.586 1Zm-3.621 9.495-.718 3.594 1.155-.42a1.75 1.75 0 0 1 1.028-.051l.168.051 1.154.42-.718-3.592c-.199.13-.37.235-.505.314l-.169.097a.75.75 0 0 1-.72 0 9.54 9.54 0 0 1-.515-.308l-.16-.105ZM10.586 2.5c-.863 0-1.611.58-1.866 1.459-.209.721-1.231.721-1.44 0C7.025 3.08 6.277 2.5 5.414 2.5 4.598 2.5 4 3.165 4 4.25c0 1.23.786 2.504 2.128 3.719.49.443 1.018.846 1.546 1.198l.325.21.076-.047.251-.163a13.341 13.341 0 0 0 1.546-1.198C11.214 6.754 12 5.479 12 4.25c0-1.085-.598-1.75-1.414-1.75Z"></path></svg>`

// Partial renders the classic SVG fragment for the sponsorships plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/sponsorships.ejs:
// it iterates the configured sections (default "amount, sponsorships")
// and renders each branch. Unlike the prior implementation it never
// short-circuits on an empty list — the "amount" section ("$0.00 to open
// source software") and the "0 users" goal text render even at zero
// sponsorships, matching the upstream reference card (#449).
//
// Output (native SVG): a `<section data-section="sponsorships">` anchor
// wrapping a nested <svg> with a heart header and, per configured
// section, an "amount" box (heart image + bold total-spend sentence) or
// a "sponsorships" box (goal-text count line + sponsor avatar grid). The
// partial reports its consumed pixel height (#409 Phase B2).
func Partial(ctx context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", 0, nil
	}
	sections := r.Sections
	if len(sections) == 0 {
		sections = []string{sectionAmount, sectionSponsorships}
	}

	user := userLogin(ctx, pc)

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(heartOcticon, "Sponsorships")
	body.WriteString(header)

	for _, section := range sections {
		switch section {
		case sectionAmount:
			m, h := renderAmountSection(r, user, y)
			body.WriteString(m)
			y += h
		case sectionSponsorships:
			m, h := renderSponsorshipsSection(r, user, y)
			body.WriteString(m)
			y += h
		}
	}

	height := int(y)
	return chrome.WrapSection("sponsorships", height, body.String()), height, nil
}

// Native-SVG sponsorships geometry, mirroring the `.sponsors.goal` grey
// rounded box (inset 13px, pad 8/6px).
const (
	shipInset = 13.0
	shipBoxW  = chrome.CardWidth - 2*shipInset
	shipPadX  = 8.0
	shipPadY  = 6.0
	shipContX = shipInset + shipPadX
	shipContW = shipBoxW - 2*shipPadX
	shipBG    = "#777777"
	shipText  = "#777777"
	shipFont  = 14.0
	shipLineH = shipFont * 1.35
)

// renderAmountSection renders the "amount" branch: a heart image and the
// bold total-spend sentence flowed to its right, inside a grey box.
func renderAmountSection(r *Result, user string, top float64) (string, float64) {
	image := r.Image
	if image == "" {
		image = amountImageURL
	}
	const imgSize = 40.0
	boxTop := top + 4
	textX := shipContX + imgSize + 8
	textW := shipInset + shipBoxW - shipPadX - textX

	date := ""
	if r.Started != nil {
		date = r.Started.Format("January 2, 2006")
	}
	words := amountWords(user, formatUSD(r.Amount), date)
	flow, flowH := flowRich(words, textX, boxTop+shipPadY, textW)

	innerH := imgSize
	if flowH > innerH {
		innerH = flowH
	}
	boxH := innerH + 2*shipPadY

	imgY := boxTop + (boxH-imgSize)/2
	var b strings.Builder
	fmt.Fprintf(&b, `<g class="sponsorships sponsors goal"><rect x="%d" y="%d" width="%d" height="%d" rx="5" ry="5" fill=%q fill-opacity="0.12"/>`,
		int(shipInset), int(boxTop), int(shipBoxW), int(boxH), shipBG)
	fmt.Fprintf(&b, `<image href=%q x="%d" y="%d" width="%d" height="%d"/>`,
		partials.EscapeXML(image), int(shipContX), int(imgY), int(imgSize), int(imgSize))
	b.WriteString(flow)
	b.WriteString(`</g>`)
	return b.String(), boxH + 4
}

// renderSponsorshipsSection renders the "sponsorships" branch: the
// goal-text count line and the sponsor avatar grid (active 64px, past
// 51px), inside a grey box. Renders the "0 users" zero-state when both
// lists are empty.
func renderSponsorshipsSection(r *Result, user string, top float64) (string, float64) {
	const (
		size     = 64.0 // upstream `plugins.sponsorships.size` default
		pastSize = 51.0 // 0.8 * size
		avGap    = 2.0
	)
	boxTop := top + 4
	y := boxTop + shipPadY

	var inner strings.Builder
	totalFunded := len(r.Active) + len(r.Past)
	inner.WriteString(chrome.SVGText(shipContX, y+12,
		fmt.Sprintf("%s helped funding the work of %d user%s and organizations.",
			user, totalFunded, plural(totalFunded)),
		chrome.SVGTextOpts{Size: 12, Fill: shipText}))
	y += 20

	specs := make([]chrome.SVGAvatarSpec, 0, len(r.Active)+len(r.Past))
	for _, s := range r.Active {
		specs = append(specs, chrome.SVGAvatarSpec{URL: avatarURL(s.Login), IsOrg: s.Type == "organization"})
	}
	m, ah := chrome.SVGAvatarGrid(shipContX, y, shipContW, size, avGap, "ship-av", specs)
	inner.WriteString(m)
	y += ah
	if len(r.Past) > 0 {
		pspecs := make([]chrome.SVGAvatarSpec, 0, len(r.Past))
		for _, s := range r.Past {
			pspecs = append(pspecs, chrome.SVGAvatarSpec{URL: avatarURL(s.Login), IsOrg: s.Type == "organization"})
		}
		if len(r.Active) > 0 {
			y += avGap
		}
		m2, ah2 := chrome.SVGAvatarGrid(shipContX, y, shipContW, pastSize, avGap, "ship-pav", pspecs)
		inner.WriteString(m2)
		y += ah2
	}

	y += shipPadY
	boxH := y - boxTop
	out := fmt.Sprintf(`<g class="sponsors goal"><rect x="%d" y="%d" width="%d" height="%d" rx="5" ry="5" fill=%q fill-opacity="0.12"/>%s</g>`,
		int(shipInset), int(boxTop), int(shipBoxW), int(boxH), shipBG, inner.String())
	return out, boxH + 4
}

// rword is one word of the amount sentence, tagged bold for the
// `<span class="bold">` runs (user / amount / date).
type rword struct {
	text string
	bold bool
}

// amountWords builds the total-spend sentence as bold/plain words:
// "<user> has given a total of <amount> to open source software[ since
// <date>]." with user, amount and date bold.
func amountWords(user, amount, date string) []rword {
	var ws []rword
	add := func(s string, bold bool) {
		for _, w := range strings.Fields(s) {
			ws = append(ws, rword{text: w, bold: bold})
		}
	}
	add(user, true)
	add("has given a total of", false)
	add(amount, true)
	add("to open source software", false)
	if date != "" {
		add("since", false)
		add(date, true)
	}
	if len(ws) > 0 {
		ws[len(ws)-1].text += "."
	}
	return ws
}

// flowRich word-wraps the bold/plain words to maxWidth px and renders
// each line as a single `<text>` whose bold words are `<tspan
// font-weight="bold">`. Emitting whole lines (with `<tspan>` for weight
// changes) lets the rasterizer handle intra-line spacing, so words never
// collide when the render font is wider than the metrics font. Returns
// the markup and the consumed height.
func flowRich(words []rword, x, top, maxWidth float64) (string, float64) {
	spaceW := fontmetrics.Width(" ", shipFont)
	wordW := func(w rword) float64 {
		return fontmetrics.WidthWeight(w.text, shipFont, weightOf(w.bold))
	}
	// Greedy wrap into rows.
	var rows [][]rword
	cur := make([]rword, 0, len(words))
	curW := 0.0
	for _, w := range words {
		add := wordW(w)
		if len(cur) > 0 {
			add += spaceW
		}
		if len(cur) > 0 && curW+add > maxWidth {
			rows = append(rows, cur)
			cur, curW = nil, 0
			add = wordW(w)
		}
		cur = append(cur, w)
		curW += add
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}

	var b strings.Builder
	y := top
	for _, row := range rows {
		baseline := y + shipFont
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="%d" fill=%q>`,
			int(x), int(baseline), int(shipFont), shipText)
		for i, w := range row {
			if i > 0 {
				b.WriteString(" ")
			}
			if w.bold {
				fmt.Fprintf(&b, `<tspan font-weight="bold">%s</tspan>`, partials.EscapeXML(w.text))
			} else {
				b.WriteString(partials.EscapeXML(w.text))
			}
		}
		b.WriteString(`</text>`)
		y += shipLineH
	}
	return b.String(), y - top
}

func weightOf(bold bool) fontmetrics.Weight {
	if bold {
		return fontmetrics.Bold
	}
	return fontmetrics.Regular
}

// formatUSD mirrors Intl.NumberFormat("en", {style:"currency",
// currency:"USD"}): a "$" prefix, comma thousands separators, and two
// decimal places (e.g. 0 -> "$0.00", 1234.5 -> "$1,234.50"). Negative
// amounts render as "-$X.XX".
func formatUSD(amount float64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	cents := int64(amount*100 + 0.5)
	dollars := cents / 100
	frac := cents % 100

	whole := strconv.FormatInt(dollars, 10)
	var grouped strings.Builder
	n := len(whole)
	for i, c := range whole {
		if i > 0 && (n-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(c)
	}

	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s$%s.%02d", sign, grouped.String(), frac)
}

// plural returns "s" when n != 1, else "".
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// avatarURL produces the canonical github.com avatar URL for a login.
// Our Sponsored struct does not yet carry the avatar field upstream
// exposes via `viewer.sponsorshipsAsSponsor` — fall back to the
// stable `/login.png?size=N` redirect.
func avatarURL(login string) string {
	if login == "" {
		return ""
	}
	return "https://github.com/" + login + ".png?size=64"
}

// userLogin returns the rendered user's login, preferring the shared
// dataprovider (#603) over the legacy pc.Data.User fallback (kept for
// unit tests that build PartialContext by hand without wiring a
// Provider). Falls back to "this user" when neither source carries a
// non-empty Login.
func userLogin(ctx context.Context, pc *templates.PartialContext) string {
	if pc == nil {
		return "this user"
	}
	if pc.Provider != nil {
		if u, err := pc.Provider.User(ctx); err == nil && u != nil && u.Login != "" {
			return u.Login
		}
	}
	if pc.Data != nil && pc.Data.User != nil && pc.Data.User.Login != "" {
		return pc.Data.User.Login
	}
	return "this user"
}
