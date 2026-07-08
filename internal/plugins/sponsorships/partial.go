package sponsorships

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
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
// Output structure:
//
//	<section data-section="sponsorships">
//	  <h2 class="field"><svg heart/>Sponsorships</h2>
//	  [amount]:
//	  <div class="row fill-width">
//	    <section class="sponsorships sponsors goal">
//	      <img src="<heart>" alt="" />
//	      <div><span class="bold">${user}</span> has given a total of
//	           <span class="bold">$X.XX</span> to open source software
//	           [since <span class="bold">${date}</span>].</div>
//	    </section>
//	  </div>
//	  [sponsorships]:
//	  <div class="row fill-width">
//	    <section class="sponsors goal">
//	      <div class="goal-text"><span>${user} helped funding the work of N users and organizations.</span></div>
//	      <div class="row"><img class="avatar ..." .../>...</div>
//	    </section>
//	  </div>
//	</section>
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

	var b strings.Builder
	b.WriteString(`<section data-section="sponsorships">`)
	fmt.Fprintf(&b, `<h2 class="field">%sSponsorships</h2>`, heartOcticon)

	for _, section := range sections {
		switch section {
		case sectionAmount:
			writeAmountSection(&b, r, user)
		case sectionSponsorships:
			writeSponsorshipsSection(&b, r, user)
		}
	}

	b.WriteString(`</section>`)
	return b.String(), 0, nil
}

// writeAmountSection renders the upstream "amount" branch: the heart
// image plus the total-spend line. The image URL is inlined as base64 by
// the render pipeline's image-inline stage.
func writeAmountSection(b *strings.Builder, r *Result, user string) {
	image := r.Image
	if image == "" {
		image = amountImageURL
	}
	b.WriteString(`<div class="row fill-width">`)
	b.WriteString(`<section class="sponsorships sponsors goal">`)
	fmt.Fprintf(b, `<img src="%s" alt="" />`, partials.EscapeXML(image))
	fmt.Fprintf(
		b,
		`<div><span class="bold">%s</span> has given a total of <span class="bold">%s</span> to open source software`,
		partials.EscapeXML(user), formatUSD(r.Amount),
	)
	if r.Started != nil {
		fmt.Fprintf(b, ` since <span class="bold">%s</span>`, partials.EscapeXML(r.Started.Format("January 2, 2006")))
	}
	b.WriteString(`.</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
}

// writeSponsorshipsSection renders the upstream "sponsorships" branch:
// the goal-text count line followed by the avatar grid. Renders the
// "0 users" zero-state when both lists are empty.
func writeSponsorshipsSection(b *strings.Builder, r *Result, user string) {
	const (
		size     = 64 // upstream `plugins.sponsorships.size` default
		pastSize = 51 // 0.8 * size, per upstream EJS
	)
	b.WriteString(`<div class="row fill-width">`)
	b.WriteString(`<section class="sponsors goal">`)

	totalFunded := len(r.Active) + len(r.Past)
	fmt.Fprintf(
		b,
		`<div class="goal-text"><span>%s helped funding the work of %d user%s and organizations.</span></div>`,
		partials.EscapeXML(user), totalFunded, plural(totalFunded),
	)

	b.WriteString(`<div class="row">`)
	for _, s := range r.Active {
		fmt.Fprintf(
			b,
			`<img class="avatar" src="%s" width="%d" height="%d" alt=""/>`,
			partials.EscapeXML(avatarURL(s.Login)), size, size,
		)
	}
	for _, s := range r.Past {
		fmt.Fprintf(
			b,
			`<img class="avatar past" src="%s" width="%d" height="%d" alt=""/>`,
			partials.EscapeXML(avatarURL(s.Login)), pastSize, pastSize,
		)
	}
	b.WriteString(`</div>`)

	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
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
