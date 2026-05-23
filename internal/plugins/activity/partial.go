package activity

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

// pulseOcticon is the upstream `<%- octicon "pulse" %>`-style 16x16 path
// used in the activity section header (EJS line 4).
const pulseOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M0 8a8 8 0 1116 0v5.25a.75.75 0 01-1.5 0V8a6.5 6.5 0 10-13 0v5.25a.75.75 0 01-1.5 0V8zm5.5 4.25a.75.75 0 01.75-.75h3.5a.75.75 0 010 1.5h-3.5a.75.75 0 01-.75-.75zM3 6.75C3 5.784 3.784 5 4.75 5h6.5c.966 0 1.75.784 1.75 1.75v1.5A1.75 1.75 0 0111.25 10h-6.5A1.75 1.75 0 013 8.25v-1.5zm1.47-.53a.75.75 0 011.06 0l.97.97.97-.97a.75.75 0 011.06 0l.97.97.97-.97a.75.75 0 111.06 1.06l-1.5 1.5a.75.75 0 01-1.06 0L8 7.81l-.97.97a.75.75 0 01-1.06 0l-1.5-1.5a.75.75 0 010-1.06z"></path></svg>`

// eventOcticons maps GitHub Events API event types to the matching
// primer/octicon name and inline 16x16 SVG path. Each entry mirrors the
// EJS conditional block in classic/partials/activity.ejs (lines 24-130).
//
// Returns (octicon-svg, human-readable verb). For unknown event types
// returns the broadcast octicon + the raw type.
func eventOcticonAndVerb(eventType string) (string, string) {
	// Each entry: full inline 16x16 octicon SVG + verb.
	switch eventType {
	case "PushEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`, "Pushed"
	case "PullRequestEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`, "Opened PR"
	case "IssuesEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`, "Issue activity"
	case "ReleaseEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8.878.392a1.75 1.75 0 00-1.756 0l-5.25 3.045A1.75 1.75 0 001 4.951v6.098c0 .624.332 1.2.872 1.514l5.25 3.045a1.75 1.75 0 001.756 0l5.25-3.045c.54-.313.872-.89.872-1.514V4.951c0-.624-.332-1.2-.872-1.514L8.878.392zM7.875 1.69a.25.25 0 01.25 0l4.63 2.685L8 7.133 3.245 4.375l4.63-2.685zM2.5 5.677v5.372c0 .09.047.171.125.216l4.625 2.683V8.432L2.5 5.677zm6.25 8.271l4.625-2.683a.25.25 0 00.125-.216V5.677L8.75 8.432v5.516z"></path></svg>`, "Released"
	case "WatchEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25z"></path></svg>`, "Starred"
	case "CreateEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2.5 7.775V2.75a.25.25 0 01.25-.25h5.025a.25.25 0 01.177.073l6.25 6.25a.25.25 0 010 .354l-5.025 5.025a.25.25 0 01-.354 0l-6.25-6.25a.25.25 0 01-.073-.177zm-1.5 0V2.75C1 1.784 1.784 1 2.75 1h5.025c.464 0 .91.184 1.238.513l6.25 6.25a1.75 1.75 0 010 2.474l-5.026 5.026a1.75 1.75 0 01-2.474 0l-6.25-6.25A1.75 1.75 0 011 7.775zM6 5a1 1 0 100 2 1 1 0 000-2z"></path></svg>`, "Created"
	case "ForkEvent":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`, "Forked"
	}
	// Fallback — broadcast octicon for unknown event types.
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M3.05 3.05a7 7 0 000 9.9.75.75 0 11-1.06 1.06c-3.32-3.32-3.32-8.7 0-12.02a.75.75 0 011.06 1.06zm9.9-1.06a.75.75 0 011.06 0c3.32 3.32 3.32 8.7 0 12.02a.75.75 0 11-1.06-1.06 7 7 0 000-9.9.75.75 0 010-1.06zM5.879 5.879a3 3 0 000 4.243.75.75 0 11-1.061 1.06 4.5 4.5 0 010-6.364.75.75 0 011.06 1.06zm5.303-1.061a.75.75 0 011.06 0 4.5 4.5 0 010 6.364.75.75 0 11-1.06-1.06 3 3 0 000-4.243.75.75 0 010-1.061zM8 9a1 1 0 100-2 1 1 0 000 2z"></path></svg>`, eventType
}

// Partial renders the classic SVG fragment for the activity plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/activity.ejs.
//
// Output structure:
//
//	<section data-section="activity">
//	  <h2 class="field"><svg pulse/>Recent activity</h2>
//	  <div class="row"><section>
//	    [for each event]:
//	      <div class="row fill-width">
//	        <section class="activity">
//	          <div class="field"><svg/>${verb} in <span class="repo">${repo}</span></div>
//	          <div class="timestamp">${date}</div>
//	        </section>
//	      </div>
//	  </section></div>
//	</section>
//
// Each per-event row is HTML (not bare <text> / <svg> primitives) so it
// renders inside the classic template's foreignObject (fixes the
// v1.0.0 bare-element invisibility bug).
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

	var b strings.Builder
	b.WriteString(`<section data-section="activity">`)
	fmt.Fprintf(&b, `<h2 class="field">%sRecent activity</h2>`, pulseOcticon)
	b.WriteString(`<div class="row"><section>`)
	if len(r.Events) == 0 {
		b.WriteString(`<div class="field">No recent activity</div>`)
	}
	for _, e := range r.Events {
		octicon, verb := eventOcticonAndVerb(e.Type)
		dateStr := ""
		if !e.Date.IsZero() {
			dateStr = e.Date.UTC().Format("2006-01-02")
		}
		b.WriteString(`<div class="row fill-width">`)
		fmt.Fprintf(
			&b,
			`<section class="activity" data-type="%s" data-repo="%s" data-date="%s">`,
			partials.EscapeXML(e.Type),
			partials.EscapeXML(e.Repo),
			partials.EscapeXML(dateStr),
		)
		fmt.Fprintf(
			&b,
			`<div class="field">%s<div class="content">%s in <span class="repo">%s</span></div></div>`,
			octicon,
			partials.EscapeXML(verb),
			partials.EscapeXML(e.Repo),
		)
		if dateStr != "" {
			fmt.Fprintf(&b, `<div class="timestamp">%s</div>`, partials.EscapeXML(dateStr))
		}
		b.WriteString(`</section></div>`)
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
