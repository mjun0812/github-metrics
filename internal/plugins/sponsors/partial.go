package sponsors

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// heartOcticon is the upstream `<%- octicon "heart" %>` 16x16 path used
// in the sponsors section header. Mirrors EJS line 4.
const heartOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M4.25 2.5c-1.336 0-2.75 1.164-2.75 3 0 2.15 1.58 4.144 3.365 5.682A20.565 20.565 0 008 13.393a20.561 20.561 0 003.135-2.211C12.92 9.644 14.5 7.65 14.5 5.5c0-1.836-1.414-3-2.75-3-1.373 0-2.609.986-3.029 2.456a.75.75 0 01-1.442 0C6.859 3.486 5.623 2.5 4.25 2.5zM8 14.25l-.345.666-.002-.001-.006-.003-.018-.01a7.643 7.643 0 01-.31-.17 22.075 22.075 0 01-3.434-2.414C2.045 10.731 0 8.35 0 5.5 0 2.836 2.086 1 4.25 1 5.797 1 7.153 1.802 8 3.02 8.847 1.802 10.203 1 11.75 1 13.914 1 16 2.836 16 5.5c0 2.85-2.045 5.231-3.885 6.818a22.08 22.08 0 01-3.744 2.584l-.018.01-.006.003h-.002L8 14.25zm0 0l.345.666a.752.752 0 01-.69 0L8 14.25z"></path></svg>`

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func sponsorsAreFundingVerb(n int) string {
	if n == 1 {
		return " is"
	}
	return "s are"
}

// containsString reports whether s is in slice.
func containsString(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}

// Partial renders the classic SVG fragment for the sponsors plugin
// matching upstream org_repo/source/templates/classic/partials/sponsors.ejs.
//
// Output structure:
//
//	<section data-section="sponsors">
//	  <section>
//	    <h2 class="field"><svg heart/>${title}</h2>
//	    [for each section in sections — skip "list" when "goal" also present]
//	      [section === "goal" || "list"]:
//	        <div class="fill-width">
//	          <section class="sponsors goal">
//	            [if section === "goal" && goal exists]:
//	              <div class="markdown">${goal.description}</div>
//	              <svg class="bar"><mask>...<rect fill="#ec6cb9"/></svg>
//	            <div class="goal-text">
//	              <span>${count.active.total} sponsor(s) are funding ${user}'s work</span>
//	              [if section === "goal" && goal]:
//	                <span>${goal.title}</span>
//	            </div>
//	            [if section === "list" || sections.includes("list")]:
//	              <div class="row"><img class="avatar" ...></div>
//	              [if past && count.past.total]:
//	                <div class="past-text">${count.past.total} sponsor(s) helped ${user} in the past</div>
//	                <div class="row"><img class="avatar past" ...></div>
//	          </section>
//	        </div>
//	      [section === "about"]:
//	        <div class="row fill-width">
//	          <section class="sponsors"><div class="markdown">${about}</div></section>
//	        </div>
//	  </section>
//	</section>
//
// Settings: mjun0812 uses plugin_sponsors_sections: goal, about, list +
// plugin_sponsors_past: yes.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", nil
	}

	title := r.Title
	if title == "" {
		title = "Sponsor Me!"
	}
	user := r.User
	if user == "" {
		user = "this user"
	}
	size := r.Size
	if size == 0 {
		size = 24
	}

	var b strings.Builder
	b.WriteString(`<section data-section="sponsors">`)
	b.WriteString(`<section>`)
	fmt.Fprintf(&b, `<h2 class="field">%s%s</h2>`, heartOcticon, partials.EscapeXML(title))

	// Upstream loops over sections; if both "goal" and "list" are
	// present, the "list" iteration is skipped because "goal" emits the
	// list inline (EJS line 17).
	sections := r.Sections
	if len(sections) == 0 {
		sections = []string{"goal", "list", "about"}
	}
	hasGoal := containsString(sections, "goal")
	hasList := containsString(sections, "list")

	for _, section := range sections {
		if hasGoal && hasList && section == "list" {
			continue
		}
		switch section {
		case "goal", "list":
			b.WriteString(`<div class="fill-width">`)
			b.WriteString(`<section class="sponsors goal">`)
			// Goal description + progress bar (when configured).
			if section == "goal" && r.Goal != nil {
				if r.Goal.Description != "" {
					fmt.Fprintf(&b, `<div class="markdown">%s</div>`, partials.EscapeXML(r.Goal.Description))
				}
				const width = 440
				progress := r.Goal.Progress
				if progress < 0 {
					progress = 0
				}
				if progress > 100 {
					progress = 100
				}
				filled := float64(progress) / 100.0 * float64(width)
				empty := float64(100-progress) / 100.0 * float64(width)
				fmt.Fprintf(&b, `<div class="center horizontal-wrap "><svg class="bar" xmlns="http://www.w3.org/2000/svg" width="%d" height="8" role="img" aria-label="Sponsorship goal progress"><title>Sponsorship goal progress</title><mask id="project-bar"><rect x="0" y="0" width="%d" height="8" fill="white" rx="5"/></mask><rect mask="url(#project-bar)" x="0" y="0" width="%.2f" height="8" fill="#ec6cb9"/><rect mask="url(#project-bar)" x="%.2f" y="0" width="%.2f" height="8" fill="#d1d5da"/></svg></div>`,
					width, width, filled, filled, empty)
			}
			// "N sponsors are funding ${user}'s work" line + goal title.
			// Upstream (sponsors.ejs) only emits the text when
			// count.active.total is truthy; with zero active sponsors the
			// span is left empty (matching docs/reference_examples).
			b.WriteString(`<div class="goal-text">`)
			b.WriteString(`<span>`)
			if r.Count.Active.Total > 0 {
				fmt.Fprintf(
					&b,
					`%d sponsor%s funding %s's work`,
					r.Count.Active.Total,
					sponsorsAreFundingVerb(r.Count.Active.Total),
					partials.EscapeXML(user),
				)
			}
			b.WriteString(`</span>`)
			if section == "goal" && r.Goal != nil && r.Goal.Title != "" {
				fmt.Fprintf(&b, `<span>%s</span>`, partials.EscapeXML(r.Goal.Title))
			}
			b.WriteString(`</div>`)
			// List section content: avatar grid + past block.
			if section == "list" || hasList {
				b.WriteString(`<div class="row">`)
				for _, s := range r.Sponsors {
					orgClass := ""
					if s.Type == "organization" {
						orgClass = "organization"
					}
					fmt.Fprintf(
						&b,
						`<img class="avatar %s" src="%s" width="%d" height="%d" alt=""/>`,
						partials.EscapeXML(orgClass),
						partials.EscapeXML(s.Avatar),
						size, size,
					)
				}
				b.WriteString(`</div>`)
				if r.PastIncluded && r.Count.Past.Total > 0 {
					fmt.Fprintf(
						&b,
						`<div class="past-text"><span>%d sponsor%s helped %s in the past</span></div>`,
						r.Count.Past.Total, pluralS(r.Count.Past.Total),
						partials.EscapeXML(user),
					)
					b.WriteString(`<div class="row">`)
					for _, s := range r.Past {
						orgClass := ""
						if s.Type == "organization" {
							orgClass = "organization"
						}
						pastSize := int(float64(size) * 0.8)
						fmt.Fprintf(
							&b,
							`<img class="avatar past %s" src="%s" width="%d" height="%d" alt=""/>`,
							partials.EscapeXML(orgClass),
							partials.EscapeXML(s.Avatar),
							pastSize, pastSize,
						)
					}
					b.WriteString(`</div>`)
				}
			}
			b.WriteString(`</section>`)
			b.WriteString(`</div>`)
		case "about":
			b.WriteString(`<div class="row fill-width">`)
			b.WriteString(`<section class="sponsors">`)
			b.WriteString(`<h2 class="field">About Me</h2>`)
			if r.About != "" {
				// Upstream emits the bio unescaped (`<%- %>` in
				// sponsors.ejs) after running it through `imports.markdown`.
				// renderMarkdown reproduces that markup (paragraphs, links,
				// images, emphasis) while escaping the text nodes.
				fmt.Fprintf(&b, `<div class="markdown">%s</div>`, renderMarkdown(r.About))
			} else {
				b.WriteString(`<div class="markdown"></div>`)
			}
			b.WriteString(`</section>`)
			b.WriteString(`</div>`)
		}
	}

	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
