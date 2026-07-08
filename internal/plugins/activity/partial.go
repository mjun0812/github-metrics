package activity

import (
	"context"
	"fmt"
	"math"
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

// pluralSuffix returns "s" unless n == 1, mirroring upstream's `s()`
// helper used for "file"/"files" pluralization in activity.ejs.
func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Native-SVG activity geometry (#409 Phase B7). The event field mirrors
// the CSS `.field` box (section-inset 5px, 8px icon margins, 16px
// octicon); the details / timestamp rows indent 38px to sit under the
// text column (`.activity .details, .timestamp { padding-left: 38px }`),
// and each event carries a 12px bottom gap (`.activity { margin-bottom:
// 12px }`).
const (
	actInset      = 5.0  // section > .field { margin-left: 5px }
	actIconGap    = 8.0  // .field svg { margin: 0 8px }
	actIconSize   = 16.0 // .octicon { width/height: 16px }
	actIconFill   = "#959da5"
	actBodyFont   = 14.0 // svg { font-size: 14px }
	actBodyFill   = "#777777"
	actRepoFill   = "#58a6ff" // .activity .repo { color: #58a6ff }
	actFieldPitch = chrome.FieldPitch
	actBaseRatio  = 0.32

	actDetailIndent = 38.0 // .activity .details/.timestamp { padding-left: 38px }
	actDetailFont   = 13.0 // .activity .details { font-size: 13px }
	actDetailFill   = "#666666"
	actDetailRowH   = 18.0
	actCodeFont     = 10.0 // .activity .code { font-size: 80% of 13px ≈ 10 }
	actCodeFill     = "#777777"
	actCodeBG       = "#777777" // .activity .code background #7777771F (12% opacity)
	actCodePadX     = 5.0       // .activity .code { padding: 1px 5px }

	actTSFont = 10.0 // .activity .timestamp { font-size: 10px }
	actTSTop  = 4.0  // .activity .timestamp { margin-top: 4px }
	actTSRowH = 14.0

	actEventGap = 12.0 // .activity { margin-bottom: 12px }
)

// itoa formats a layout coordinate as a rounded integer for compact,
// stable SVG output.
func itoa(v float64) string { return strconv.Itoa(int(math.Round(v))) }

// Partial renders the classic SVG fragment for the activity plugin as
// native SVG (#409 Phase B7). Mirrors upstream
// org_repo/source/templates/classic/partials/activity.ejs.
//
// Output: a `<section data-section="activity">` anchor wrapping a nested
// `<svg>` with the pulse section header and one `<g class="activity">`
// per event. Each event carries a field line (event-type octicon +
// "${verb} in ${repo}" with the repo in link-blue), an optional PR
// details line ("N file(s) changed ++A --D" with the diff volume in a
// code chip), and an optional timestamp line. Event-type octicons and
// verbs come from eventOcticonAndVerb so every GitHub Events API type is
// covered. Returns the markup and the pixel height it consumes.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
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

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(pulseOcticon, "Recent activity")
	body.WriteString(header)

	if len(r.Events) == 0 {
		// Empty-state field row (upstream "No recent activity").
		baseline := y + actFieldPitch/2 + actBodyFont*actBaseRatio
		body.WriteString(chrome.SVGText(actInset+actIconGap, baseline, "No recent activity",
			chrome.SVGTextOpts{Size: actBodyFont, Fill: actBodyFill}))
		y += actFieldPitch
	}

	for _, e := range r.Events {
		octicon, verb := eventOcticonAndVerb(e.Type)
		dateStr := ""
		if !e.Date.IsZero() {
			dateStr = e.Date.UTC().Format("2006-01-02")
		}

		var g strings.Builder
		gy := y
		writeEventField(&g, gy, octicon, verb, e.Repo)
		gy += actFieldPitch

		// Mirror upstream activity.ejs line 79: PR events show the diff
		// stats ("N file(s) changed ++A --D") in a details block.
		if e.Files != nil && e.Lines != nil {
			writeDetailsLine(&g, gy, e.Files.Changed, e.Lines.Added, e.Lines.Deleted)
			gy += actDetailRowH
		}
		if dateStr != "" {
			baseline := gy + actTSTop + actTSFont
			g.WriteString(chrome.SVGText(actDetailIndent, baseline, dateStr,
				chrome.SVGTextOpts{Size: actTSFont, Fill: actDetailFill}))
			gy += actTSRowH
		}

		// The writeEventField return is the field markup; g already holds
		// the details / timestamp. Reassemble in one <g class="activity">.
		fmt.Fprintf(&body,
			`<g class="activity" data-type="%s" data-repo="%s" data-date="%s">%s</g>`,
			partials.EscapeXML(e.Type), partials.EscapeXML(e.Repo), partials.EscapeXML(dateStr),
			g.String())
		y = gy + actEventGap
	}

	height := int(y)
	return chrome.WrapSection("activity", height, body.String()), height, nil
}

// writeEventField writes the event field line (octicon + "${verb} in
// ${repo}") into b at y=top. The repo name is link-blue and
// ellipsis-truncated to the remaining card width; it keeps a
// `class="repo"` hook.
func writeEventField(b *strings.Builder, top float64, octicon, verb, repo string) {
	iconX := actInset + actIconGap
	iconY := top + (actFieldPitch-actIconSize)/2
	textX := iconX + actIconSize + actIconGap
	baseline := top + actFieldPitch/2 + actBodyFont*actBaseRatio

	b.WriteString(chrome.SVGIcon(iconX, iconY, actIconFill, octicon))

	prefix := verb + " in "
	prefixW := fontmetrics.Width(prefix, actBodyFont)
	repoMax := chrome.CardWidth - textX - actInset - prefixW
	repo = chrome.TruncateToWidth(repo, actBodyFont, fontmetrics.Regular, repoMax)
	fmt.Fprintf(b,
		`<text x="%s" y="%s" font-size="%s" fill="%s">%s<tspan class="repo" fill="%s">%s</tspan></text>`,
		itoa(textX), itoa(baseline), itoa(actBodyFont), actBodyFill,
		partials.EscapeXML(prefix), actRepoFill, partials.EscapeXML(repo))
}

// writeDetailsLine writes the PR diff-stat line ("N file(s) changed"
// followed by a "++A --D" code chip) into b at y=top.
func writeDetailsLine(b *strings.Builder, top float64, changed, added, deleted int) {
	baseline := top + actDetailRowH/2 + actDetailFont*actBaseRatio
	prefix := fmt.Sprintf("%d file%s changed ", changed, pluralSuffix(changed))
	b.WriteString(chrome.SVGText(actDetailIndent, baseline, prefix,
		chrome.SVGTextOpts{Size: actDetailFont, Fill: actDetailFill}))

	code := fmt.Sprintf("++%d --%d", added, deleted)
	codeX := actDetailIndent + fontmetrics.Width(prefix, actDetailFont)
	codeW := fontmetrics.Width(code, actCodeFont) + 2*actCodePadX
	chipH := actCodeFont + 4
	chipY := top + (actDetailRowH-chipH)/2
	codeBaseline := chipY + chipH/2 + actCodeFont*actBaseRatio
	fmt.Fprintf(b,
		`<g class="code"><rect x="%s" y="%s" width="%s" height="%s" rx="3" ry="3" fill="%s" fill-opacity="0.12"/><text x="%s" y="%s" font-size="%s" fill="%s" font-family="monospace">%s</text></g>`,
		itoa(codeX), itoa(chipY), itoa(codeW), itoa(chipH), actCodeBG,
		itoa(codeX+actCodePadX), itoa(codeBaseline), itoa(actCodeFont), actCodeFill,
		partials.EscapeXML(code))
}
