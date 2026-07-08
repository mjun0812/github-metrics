package notable

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

// notableOcticon is the upstream `<%- octicon "rocket" %>` 16x16 path
// used in the notable section header (notable.ejs line 4).
const notableOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1 2.5A2.5 2.5 0 013.5 0h8.75a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0V1.5h-8a1 1 0 00-1 1v6.708A2.492 2.492 0 013.5 9h3.25a.75.75 0 010 1.5H3.5a1 1 0 100 2h5.75a.75.75 0 010 1.5H3.5A2.5 2.5 0 011 11.5v-9zm13.23 7.79a.75.75 0 001.06-1.06l-2.505-2.505a.75.75 0 00-1.06 0L9.22 9.229a.75.75 0 001.06 1.061l1.225-1.224v6.184a.75.75 0 001.5 0V9.066l1.224 1.224z"></path></svg>`

// Gauge icon octicons rendered next to each indepth gauge, mirroring the
// upstream notable.ejs commit / star / issue / pull markup.
const (
	commitsIcon = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`
	starsIcon   = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`
	issuesIcon  = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`
	pullsIcon   = `<svg class="icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`
)

// contributionLevel mirrors the upstream class suffix on each chip
// (notable.ejs): maintainer="s", then percentage tiers a/b/c, else "".
func contributionLevel(c NotableContrib) string {
	switch {
	case c.Maintainer:
		return "s"
	case c.Percentage > 0.2:
		return "a"
	case c.Percentage > 0.1:
		return "b"
	case c.Percentage > 0.05:
		return "c"
	default:
		return ""
	}
}

// gauge renders a single upstream `.gauge` arc + value + icon. The arc
// fills `fraction` of the 329-unit circumference.
func gauge(b *strings.Builder, fraction float64, value, icon string) {
	fmt.Fprintf(
		b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30" width="16" height="16" class="gauge"><circle class="gauge-base" r="12.5" cx="15" cy="15"></circle><circle class="gauge-arc" transform="rotate(-90 15 15)" r="12.5" cx="15" cy="15" stroke-dasharray="%s 329"></circle><text x="15" y="15" dominant-baseline="central">%s</text></svg>`,
		formatFraction(fraction*329), partials.EscapeXML(value),
	)
	b.WriteString(icon)
}

// formatFraction trims a float to a compact, deterministic form so the
// stroke-dasharray matches upstream's numeric output without trailing
// zeros.
func formatFraction(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

// maxBasicChipNameLen caps the basic-mode chip label so the 480 px card
// width still wraps deterministically. The chip font-size is 12 px,
// so a 36-character ceiling keeps even the widest chip narrower than
// the full card width while still fitting the common
// "kebab-case-multi-word-repo-name" style verbatim (the chip flex row
// wraps, so a single overflowing chip can take a row of its own).
// Names longer than this are truncated with an ellipsis (issue #422).
const maxBasicChipNameLen = 36

// truncateName ellipsizes a chip label once it exceeds the per-chip
// budget defined by maxBasicChipNameLen. The trim happens on rune
// boundaries so multi-byte names (e.g. CJK characters) are not split
// in the middle.
func truncateName(name string, limit int) string {
	if limit <= 0 {
		return name
	}
	runes := []rune(name)
	if len(runes) <= limit {
		return name
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

// Partial renders the classic SVG fragment for the notable plugin,
// mirroring upstream source/templates/classic/partials/notable.ejs
// (issue #447). The chip label is the aggregation key chosen by Run:
// the owner segment ("@huggingface") by default, or the full
// "@owner/repo" handle when plugin_notable_repositories is enabled.
//
// Basic mode renders only the avatar + owner chip — no star count is
// drawn (the previous "★ N" badge from #422 was a regression).
// Indepth mode additionally renders the upstream gauge stack
// (commits / stars / issues / pulls), each gauge guarded on a non-zero
// counter.
//
// Returns "" when there is no data (Skipped or empty List). When data
// arrives it emits the rocket-octicon header followed by a flex row of
// contribution chips.
//
// Output structure (basic mode):
//
//	<section data-section="notable">
//	  <h2 class="field"><svg/>Notable contributions</h2>
//	  <div class="row organization contributions">
//	    [for each contrib]:
//	      <div class="organization contribution {s|a|b|c} ">
//	        <img class="[organization] avatar" src="..." width="16" height="16" />
//	        <span class="name">@${owner}</span>
//	      </div>
//	  </div>
//	</section>
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.List) == 0 {
		return "", 0, nil
	}

	// total.stars normalizes the star gauge (upstream `total.stars`).
	totalStars := 0
	for _, c := range r.List {
		totalStars += c.StargazerCount
	}

	var b strings.Builder
	b.WriteString(`<section data-section="notable">`)
	fmt.Fprintf(
		&b,
		`<h2 class="field">%sNotable contributions</h2>`,
		notableOcticon,
	)
	b.WriteString(`<div class="row organization contributions">`)
	for _, c := range r.List {
		// Upstream `partials/notable.ejs` emits the same chip class for
		// basic and indepth modes — only the gauge SVGs below differ.
		// Adding an `indepth` marker here let our CSS force a 204 px
		// fixed-width box (#545) that diverged from upstream's natural
		// flex sizing and made the gauge stack collide (#557). Mirror
		// upstream verbatim so the chip width is content-driven.
		fmt.Fprintf(&b, `<div class="organization contribution %s ">`, contributionLevel(c))
		avatarClass := "avatar"
		if c.Organization {
			avatarClass = "organization avatar"
		}
		fmt.Fprintf(
			&b,
			`<img class="%s" src="%s" width="16" height="16" />`,
			avatarClass, partials.EscapeXML(c.AvatarURL),
		)
		// The chip label is the aggregation key produced by Run: the
		// owner segment ("@huggingface") by default, or the full
		// "@owner/repo" handle when plugin_notable_repositories is
		// enabled (issue #447). It is overflow-truncated so a single
		// long handle cannot blow out the 480 px card width.
		fmt.Fprintf(&b, `<span class="name">@%s</span>`, partials.EscapeXML(truncateName(c.Name, maxBasicChipNameLen)))
		if c.Indepth {
			// Indepth mode adds the upstream gauge stack (commits /
			// stars / issues / pulls). Each gauge only renders when its
			// counter is populated, mirroring upstream's
			// `<% if (commits) %>` guards.
			if c.Commits > 0 {
				gauge(&b, c.Percentage, strconv.Itoa(c.Commits), commitsIcon)
			}
			if c.StargazerCount > 0 {
				gauge(&b, float64(c.StargazerCount)/float64(max(totalStars, 1)), partials.FormatCount(int64(c.StargazerCount)), starsIcon)
			}
			if c.Issues > 0 {
				gauge(&b, float64(c.Issues)/float64(max(totalIssues(r.List), 1)), strconv.Itoa(c.Issues), issuesIcon)
			}
			if c.Pulls > 0 {
				gauge(&b, float64(c.Pulls)/float64(max(totalPulls(r.List), 1)), strconv.Itoa(c.Pulls), pullsIcon)
			}
		}
		// Basic mode renders the avatar + owner chip only. Upstream
		// notable.ejs does not draw star counts in basic mode (issue
		// #447 — the stray "★ N" badge was a #422 regression).
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	return b.String(), 0, nil
}

func totalIssues(list []NotableContrib) int {
	total := 0
	for _, c := range list {
		total += c.Issues
	}
	return total
}

func totalPulls(list []NotableContrib) int {
	total := 0
	for _, c := range list {
		total += c.Pulls
	}
	return total
}
