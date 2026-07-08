package repositories

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// reposHeaderOcticon is the upstream `<%- octicon "repo" %>`-style 16x16
// path used in the repositories section header (EJS line 4).
const reposHeaderOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M0 2.75A2.75 2.75 0 012.75 0h10.5A2.75 2.75 0 0116 2.75v10.5A2.75 2.75 0 0113.25 16H2.75A2.75 2.75 0 010 13.25V2.75zM2.75 1.5c-.69 0-1.25.56-1.25 1.25v10.5c0 .69.56 1.25 1.25 1.25h10.5c.69 0 1.25-.56 1.25-1.25V2.75c0-.69-.56-1.25-1.25-1.25H2.75z"></path><path d="M8 4a.75.75 0 01.75.75V6.7l1.69-.975a.75.75 0 01.75 1.3L9.5 8l1.69.976a.75.75 0 01-.75 1.298L8.75 9.3v1.951a.75.75 0 01-1.5 0V9.299l-1.69.976a.75.75 0 01-.75-1.3L6.5 8l-1.69-.975a.75.75 0 01.75-1.3l1.69.976V4.75A.75.75 0 018 4z"></path></svg>`

// repoNonForkOcticon is the per-row icon for non-fork repos (EJS line 16).
const repoNonForkOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M2 2.5A2.5 2.5 0 014.5 0h8.75a.75.75 0 01.75.75v12.5a.75.75 0 01-.75.75h-2.5a.75.75 0 110-1.5h1.75v-2h-8a1 1 0 00-.714 1.7.75.75 0 01-1.072 1.05A2.495 2.495 0 012 11.5v-9zm10.5-1V9h-8c-.356 0-.694.074-1 .208V2.5a1 1 0 011-1h8zM5 12.25v3.25a.25.25 0 00.4.2l1.45-1.087a.25.25 0 01.3 0L8.6 15.7a.25.25 0 00.4-.2v-3.25a.25.25 0 00-.25-.25h-3.5a.25.25 0 00-.25.25z"></path></svg>`

// repoForkOcticon is the per-row icon for fork repos (EJS line 14).
const repoForkOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`

// rowStarOcticon11 is the smaller 11x11 star icon used in the infos row.
const rowStarOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25z"></path></svg>`

// rowForkOcticon11 is the smaller 11x11 fork icon used in the infos row.
const rowForkOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`

// rowLicenseOcticon11 is the smaller 11x11 license (law) icon used in the
// infos row (upstream repositories.ejs line 41, `octicon "law"`).
const rowLicenseOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M8.75.75a.75.75 0 00-1.5 0V2h-.984c-.305 0-.604.08-.869.23l-1.288.737A.25.25 0 013.984 3H1.75a.75.75 0 000 1.5h.428L.066 9.192a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.514 3.514 0 00.686.45A4.492 4.492 0 003 11c.88 0 1.556-.22 2.023-.454a3.515 3.515 0 00.686-.45l.045-.04.016-.015.006-.006.002-.002.001-.002L5.25 9.5l.53.53a.75.75 0 00.154-.838L3.822 4.5h.162c.305 0 .604-.08.869-.23l1.289-.737a.25.25 0 01.124-.033h.984V13h-2.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-2.5V3.5h.984a.25.25 0 01.124.033l1.29.736c.264.152.563.231.868.231h.162l-2.112 4.692a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.517 3.517 0 00.686.45A4.492 4.492 0 0013 11c.88 0 1.556-.22 2.023-.454a3.512 3.512 0 00.686-.45l.045-.04.01-.01.006-.005.006-.006.002-.002.001-.002-.529-.531.53.53a.75.75 0 00.154-.838L13.823 4.5h.427a.75.75 0 000-1.5h-2.234a.25.25 0 01-.124-.033l-1.29-.736A1.75 1.75 0 009.735 2H8.75V.75zM1.695 9.227c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L3 6.327l-1.305 2.9zm10 0c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L13 6.327l-1.305 2.9z"></path></svg>`

// rowIssueOcticon11 is the smaller 11x11 issue (issue-opened) icon used in
// the infos row (upstream repositories.ejs line 54).
const rowIssueOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`

// rowPullRequestOcticon11 is the smaller 11x11 pull-request icon used in the
// infos row (upstream repositories.ejs line 58).
const rowPullRequestOcticon11 = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`

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

// Partial renders the classic SVG fragment for the repositories plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/repositories.ejs.
//
// Output (native SVG): a `<section data-section="repositories">` anchor
// wrapping a nested <svg> with a section header and one repository card
// per featured/pinned repo — the repo/fork octicon + linked blue name +
// "created <date>" row, the wrapped description, and the info row
// (language color, license, stars, forks, issues, PRs). Each card keeps
// its `class="repository"` / `data-stars` / `data-forks` DOM hooks and
// the partial reports its consumed pixel height (#409 Phase B2).
//
// #466: the per-card "created <date>" label plus the license / issue /
// pull-request counters mirror upstream repositories.ejs. The license
// label follows upstream `f.license` (nickname → spdxId → name) via
// plugins.RepositoryLicense.Label. REST-sourced starred repos leave
// CreatedAt / License zero, so those fragments are simply skipped.
func Partial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if pc == nil || pc.Data == nil {
		return "", 0, nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", 0, nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Featured) == 0 {
		return "", 0, nil
	}

	var body strings.Builder
	header, y := chrome.SVGSectionHeader(reposHeaderOcticon, "Featured repositories")
	body.WriteString(header)

	seen := make(map[string]struct{}, len(r.Featured)+len(r.Pinned))
	for _, repo := range r.Featured {
		seen[repo.NameWithOwner] = struct{}{}
		y += writeRepoCard(&body, repo, y)
	}
	// #555: when `plugin_repositories_pinned` is enabled, append the
	// pinned items after Featured, deduping by NameWithOwner. Mirrors
	// upstream `org_repo/source/templates/classic/partials/repositories.ejs`
	// which iterates a single `list` that accumulates featured + pinned
	// items (`plugins/repositories/index.mjs` push loop). The no-token
	// test path sets `r.Pinned = r.Featured`, which the dedupe collapses
	// to a no-op so legacy fixtures stay byte-identical.
	for _, repo := range r.Pinned {
		if _, dup := seen[repo.NameWithOwner]; dup {
			continue
		}
		seen[repo.NameWithOwner] = struct{}{}
		y += writeRepoCard(&body, repo, y)
	}

	height := int(y)
	return chrome.WrapSection("repositories", height, body.String()), height, nil
}

// writeRepoCard emits one native-SVG `<g class="repository">` card for a
// single repo starting at y=top, returning the height it consumes
// (including the 6px top/bottom card margins). Reused for the Featured +
// Pinned sections.
func writeRepoCard(body *strings.Builder, repo plugins.Repository, top float64) float64 {
	const (
		cardMargin = 6.0
		nameWidth  = 440.0
		infoLeft   = 38.0
		descWidth  = 440.0
		descFont   = 13.0
	)
	y := top + cardMargin
	var card strings.Builder

	icon := repoNonForkOcticon
	if repo.IsFork {
		icon = repoForkOcticon
	}
	date := ""
	if created := formatCreated(repo.CreatedAt, time.Now()); created != "" {
		date = "created " + created
	}
	nameRow, h := chrome.SVGRepoName(0, y, nameWidth, 14, icon, repo.NameWithOwner, repo.URL, date)
	card.WriteString(nameRow)
	y += h

	if repo.Description != "" {
		m, dh := chrome.SVGParagraph(infoLeft, y, descWidth, descFont, "#666666", repo.Description)
		card.WriteString(m)
		y += dh
	}

	segs := make([]chrome.SVGInfoSegment, 0, 6)
	if repo.Language != nil && repo.Language.Name != "" {
		segs = append(segs, chrome.SVGInfoSegment{
			Icon: langDotOcticon(repo.Language.Color), Text: repo.Language.Name, Class: "language",
		})
	}
	if label := repo.License.Label(); label != "" {
		segs = append(segs, chrome.SVGInfoSegment{Icon: rowLicenseOcticon11, Text: label})
	}
	segs = append(segs,
		chrome.SVGInfoSegment{Icon: rowStarOcticon11, Text: fmt.Sprintf("%d", repo.Stars)},
		chrome.SVGInfoSegment{Icon: rowForkOcticon11, Text: fmt.Sprintf("%d", repo.Forks)},
		chrome.SVGInfoSegment{Icon: rowIssueOcticon11, Text: fmt.Sprintf("%d", repo.Issues)},
		chrome.SVGInfoSegment{Icon: rowPullRequestOcticon11, Text: fmt.Sprintf("%d", repo.PullRequests)},
	)
	infoRow, ih := chrome.SVGInfoRow(infoLeft, y, segs)
	card.WriteString(infoRow)
	y += ih + cardMargin

	fmt.Fprintf(body, `<g class="repository" data-stars="%d" data-forks="%d">%s</g>`,
		repo.Stars, repo.Forks, card.String())
	return y - top
}

// formatCreated mirrors upstream repositories/index.mjs `format()` date
// logic, producing the per-card "created <date>" label. now is injected
// so the relative-age branches are deterministic in tests. A zero t
// (e.g. REST-sourced starred repos that never fetched createdAt) returns
// "" so the caller skips the span entirely.
//
// Branches (upstream parity):
//   - < 1 day  → "N hour(s) ago"   (hours rounded up)
//   - < 30 day → "N day(s) ago"    (days rounded down)
//   - else     → "<Mon> <D> <YYYY>" (JS Date.toDateString().substring(4))
func formatCreated(t, now time.Time) string {
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
		return fmt.Sprintf("%d hour%s ago", hours, plural(hours >= 2))
	case days < 30:
		n := int(math.Floor(days))
		return fmt.Sprintf("%d day%s ago", n, plural(days >= 2))
	default:
		// Matches JS `new Date(...).toDateString().substring(4)`,
		// e.g. "Oct 26 2024" (day zero-padded to two digits). GitHub
		// timestamps are UTC.
		return t.UTC().Format("Jan 02 2006")
	}
}

// plural returns "s" when the count warrants a plural suffix. The caller
// passes the upstream comparison directly so the singular/plural choice
// matches repositories/index.mjs exactly.
func plural(many bool) string {
	if many {
		return "s"
	}
	return ""
}
