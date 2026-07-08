package stars

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

var (
	nowMu   sync.RWMutex
	nowFunc = time.Now
)

// SetNowForTest overrides the stars partial clock and returns a restore function.
func SetNowForTest(fn func() time.Time) func() {
	nowMu.Lock()
	old := nowFunc
	nowFunc = fn
	nowMu.Unlock()
	return func() {
		nowMu.Lock()
		nowFunc = old
		nowMu.Unlock()
	}
}

func currentNow() time.Time {
	nowMu.RLock()
	fn := nowFunc
	nowMu.RUnlock()
	return fn()
}

// starOcticon is the upstream `<%- octicon "star" %>` 16x16 path used
// in the stars section header (EJS line 4).
const starOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// repoOcticon is the upstream `<%- octicon "repo" %>` icon used as the
// per-repository row icon (EJS line 16 — non-fork branch).
const repoOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2 2.5A2.5 2.5 0 014.5 0h8.75a.75.75 0 01.75.75v12.5a.75.75 0 01-.75.75h-2.5a.75.75 0 110-1.5h1.75v-2h-8a1 1 0 00-.714 1.7.75.75 0 01-1.072 1.05A2.495 2.495 0 012 11.5v-9zm10.5-1V9h-8c-.356 0-.694.074-1 .208V2.5a1 1 0 011-1h8zM5 12.25v3.25a.25.25 0 00.4.2l1.45-1.087a.25.25 0 01.3 0L8.6 15.7a.25.25 0 00.4-.2v-3.25a.25.25 0 00-.25-.25h-3.5a.25.25 0 00-.25.25z"></path></svg>`

// forkOcticon is the upstream non-fork repo icon reused for the fork
// icon, plus the per-row fork count icon (EJS line 50).
const forkOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`

// rowStarOcticon is the star icon used on per-row star counts. Upstream
// uses the full 16x16 glyph in the info row (EJS line 46).
const rowStarOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// licenseOcticon is the upstream `<%- octicon "law" %>` icon shown next
// to the SPDX license id in the info row (EJS line 41).
const licenseOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8.75.75a.75.75 0 00-1.5 0V2h-.984c-.305 0-.604.08-.869.23l-1.288.737A.25.25 0 013.984 3H1.75a.75.75 0 000 1.5h.428L.066 9.192a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.514 3.514 0 00.686.45A4.492 4.492 0 003 11c.88 0 1.556-.22 2.023-.454a3.515 3.515 0 00.686-.45l.045-.04.016-.015.006-.006.002-.002.001-.002L5.25 9.5l.53.53a.75.75 0 00.154-.838L3.822 4.5h.162c.305 0 .604-.08.869-.23l1.289-.737a.25.25 0 01.124-.033h.984V13h-2.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-2.5V3.5h.984a.25.25 0 01.124.033l1.29.736c.264.152.563.231.868.231h.162l-2.112 4.692a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.517 3.517 0 00.686.45A4.492 4.492 0 0013 11c.88 0 1.556-.22 2.023-.454a3.512 3.512 0 00.686-.45l.045-.04.01-.01.006-.005.006-.006.002-.002.001-.002-.529-.531.53.53a.75.75 0 00.154-.838L13.823 4.5h.427a.75.75 0 000-1.5h-2.234a.25.25 0 01-.124-.033l-1.29-.736A1.75 1.75 0 009.735 2H8.75V.75zM1.695 9.227c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L3 6.327l-1.305 2.9zm10 0c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L13 6.327l-1.305 2.9z"></path></svg>`

// issueOcticon is the upstream `<%- octicon "issue-opened" %>` icon
// shown next to the open-issue count in the info row (EJS line 54).
const issueOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`

// pullRequestOcticon is the upstream `<%- octicon "git-pull-request" %>`
// icon shown next to the pull-request count in the info row (EJS line 58).
const pullRequestOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`

// langDotOcticon returns the per-repo primary-language color dot icon.
// Falls back to grey when no color is set (matches upstream's `#959DA5`).
func langDotOcticon(color string) string {
	if color == "" {
		color = "#959DA5"
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="%s" fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8z"></path></svg>`,
		partials.EscapeXML(color),
	)
}

// Partial renders the classic SVG fragment for the stars plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/stars.ejs.
//
// Output (native SVG): a `<section data-section="stars">` anchor
// wrapping a nested `<svg>` with a section header and one repository
// card per starred repo — the repo/fork octicon + blue name + "starred
// <date>" row, the wrapped description, and the info row (language
// color, license, stars, forks, open issues, PRs). Each card keeps its
// `class="repository"` / `data-stars` / `data-forks` DOM hooks and the
// partial reports its consumed pixel height (#409 Phase B2).
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

	const (
		cardMargin = 6.0   // .repository { margin: 6px 0 }
		nameWidth  = 440.0 // .repository .name { width: 440px }
		infoLeft   = 38.0  // .repository .infos / .description { margin-left: 38px }
		descWidth  = 440.0
		descFont   = 13.0
	)

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(starOcticon, "Recently starred repositories")
	body.WriteString(header)

	for _, s := range r.List {
		y += cardMargin
		var card strings.Builder

		date := ""
		if !s.StarredAt.IsZero() {
			date = "starred " + formatStarredAt(s.StarredAt, currentNow())
		}
		icon := repoOcticon
		if s.IsFork {
			icon = forkOcticon
		}
		nameRow, h := chrome.SVGRepoName(0, y, nameWidth, 14, icon, s.NameWithOwner, "", date)
		card.WriteString(nameRow)
		y += h

		if s.Description != "" {
			m, dh := chrome.SVGParagraph(infoLeft, y, descWidth, descFont, "#666666", s.Description)
			card.WriteString(m)
			y += dh
		}

		segs := make([]chrome.SVGInfoSegment, 0, 6)
		if s.Language != nil && s.Language.Name != "" {
			segs = append(segs, chrome.SVGInfoSegment{
				Icon: langDotOcticon(s.Language.Color), Text: s.Language.Name, Class: "language",
			})
		}
		if s.License != "" {
			segs = append(segs, chrome.SVGInfoSegment{Icon: licenseOcticon, Text: s.License})
		}
		segs = append(segs,
			chrome.SVGInfoSegment{Icon: rowStarOcticon, Text: partials.FormatCount(int64(s.Stars))},
			chrome.SVGInfoSegment{Icon: forkOcticon, Text: partials.FormatCount(int64(s.Forks))},
			chrome.SVGInfoSegment{Icon: issueOcticon, Text: partials.FormatCount(int64(s.Issues))},
			chrome.SVGInfoSegment{Icon: pullRequestOcticon, Text: partials.FormatCount(int64(s.PullRequests))},
		)
		infoRow, ih := chrome.SVGInfoRow(infoLeft, y, segs)
		card.WriteString(infoRow)
		y += ih + cardMargin

		fmt.Fprintf(&body, `<g class="repository" data-stars="%d" data-forks="%d">%s</g>`,
			s.Stars, s.Forks, card.String())
	}

	height := int(y)
	return chrome.WrapSection("stars", height, body.String()), height, nil
}

func formatStarredAt(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	days := now.Sub(t).Hours() / 24
	switch {
	case days < 1:
		hours := int(math.Ceil(days * 24))
		if hours < 0 {
			hours = 0
		}
		// Only "1 hour" is singular; "0 hours" (clamped future timestamp)
		// and "2+ hours" both take the plural suffix.
		return fmt.Sprintf("%d hour%s ago", hours, pluralSuffix(hours != 1))
	case days < 30:
		n := int(math.Floor(days))
		return fmt.Sprintf("%d day%s ago", n, pluralSuffix(days >= 2))
	default:
		return t.UTC().Format("Jan 02 2006")
	}
}

func pluralSuffix(many bool) string {
	if many {
		return "s"
	}
	return ""
}
