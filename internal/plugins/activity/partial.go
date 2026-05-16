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

// Partial renders the classic SVG fragment for the activity plugin.
// DOM contract (partial-classic-m4.md §5): emits one
// <text class="activity-event"> + <svg class="octicon"> per event.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Events) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="activity">`)
	b.WriteString(`<ul class="activity-events">`)
	for _, e := range r.Events {
		date := ""
		if !e.Date.IsZero() {
			date = e.Date.UTC().Format("2006-01-02")
		}
		fmt.Fprintf(
			&b,
			`<li class="activity-entry"><svg class="octicon" data-octicon=":octicon-%s-16:"></svg><text class="activity-event" data-type="%s" data-repo="%s" data-date="%s">%s in %s</text></li>`,
			partials.EscapeXML(octiconForEvent(e.Type)),
			partials.EscapeXML(e.Type),
			partials.EscapeXML(e.Repo),
			partials.EscapeXML(date),
			partials.EscapeXML(humanType(e.Type)),
			partials.EscapeXML(e.Repo),
		)
	}
	b.WriteString(`</ul>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

// octiconForEvent returns a primer/octicons name appropriate for the
// event type. The mapping mirrors upstream's classic activity panel.
func octiconForEvent(t string) string {
	switch t {
	case "PushEvent":
		return "git-commit"
	case "PullRequestEvent":
		return "git-pull-request"
	case "IssuesEvent":
		return "issue-opened"
	case "ReleaseEvent":
		return "tag"
	case "WatchEvent":
		return "star"
	case "CreateEvent":
		return "plus"
	case "ForkEvent":
		return "repo-forked"
	}
	return "broadcast"
}

func humanType(t string) string {
	switch t {
	case "PushEvent":
		return "Pushed"
	case "PullRequestEvent":
		return "Opened PR"
	case "IssuesEvent":
		return "Issue activity"
	case "ReleaseEvent":
		return "Release"
	case "WatchEvent":
		return "Starred"
	case "CreateEvent":
		return "Created"
	case "ForkEvent":
		return "Forked"
	}
	return t
}
