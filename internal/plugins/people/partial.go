package people

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// peopleOcticon is the upstream `<%- octicon "people" %>` 16x16 path
// used in the per-type section header (EJS lines 5-9, 24-28).
const peopleOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5.5 3.5a2 2 0 100 4 2 2 0 000-4zM2 5.5a3.5 3.5 0 115.898 2.549 5.507 5.507 0 013.034 4.084.75.75 0 11-1.482.235 4.001 4.001 0 00-7.9 0 .75.75 0 01-1.482-.236A5.507 5.507 0 013.102 8.05 3.49 3.49 0 012 5.5zM11 4a.75.75 0 100 1.5 1.5 1.5 0 01.666 2.844.75.75 0 00-.416.672v.352a.75.75 0 00.574.73c1.2.289 2.162 1.2 2.522 2.372a.75.75 0 101.434-.44 5.01 5.01 0 00-2.56-3.012A3 3 0 0011 4z"></path></svg>`

// labelForType returns the upstream per-type header label
// (EJS lines 26-30). The mapping mirrors the upstream object literal:
//
//	followers              → "N follower(s)"
//	following              → "N followed"
//	sponsorshipsAsSponsor  → "N sponsored"
//	sponsorshipsAsMaintainer → "N sponsor(s)"
//	membersWithRole / members → "N member(s)"
//	contributors            → "N contributor(s)"
//	stargazers              → "N stargazer(s)"
//	watchers                → "N watcher(s)"
//	thanks                 → "Special thanks"
//	(any other)            → "N <type>"
func labelForType(t string, n int) string {
	switch t {
	case "thanks":
		return "Special thanks"
	case "followers":
		return fmt.Sprintf("%d follower%s", n, pluginutil.Plural(n))
	case "following":
		return fmt.Sprintf("%d followed", n)
	case "sponsorshipsAsSponsor", "sponsors":
		return fmt.Sprintf("%d sponsored", n)
	case "sponsorshipsAsMaintainer":
		return fmt.Sprintf("%d sponsor%s", n, pluginutil.Plural(n))
	case "membersWithRole", "members":
		return fmt.Sprintf("%d member%s", n, pluginutil.Plural(n))
	case "contributors":
		return fmt.Sprintf("%d contributor%s", n, pluginutil.Plural(n))
	case "stargazers":
		return fmt.Sprintf("%d stargazer%s", n, pluginutil.Plural(n))
	case "watchers":
		return fmt.Sprintf("%d watcher%s", n, pluginutil.Plural(n))
	}
	return fmt.Sprintf("%d %s", n, t)
}

// Partial renders the classic SVG fragment for the people plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/people.ejs.
//
// Output structure (per-type):
//
//	<section data-section="people">
//	  [for each type with non-empty list]:
//	    <section>
//	      <h2 class="field"><svg octicon/>${label}</h2>
//	      <div class="row">
//	        <section class="people">
//	          <img class="avatar" src="..." width="${size}" height="${size}" alt="" />
//	          ...
//	        </section>
//	      </div>
//	    </section>
//	</section>
//
// The wrapping `<section data-section="people">` is our addition for
// downstream CSS/JS selectors; the inner per-type `<section>` blocks
// preserve upstream's flat structure.
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
	total := 0
	for _, list := range r.Types {
		total += len(list)
	}
	if total == 0 {
		return "", 0, nil
	}
	// Stable type ordering for deterministic SVG output. Upstream
	// iterates `plugins.people.types` (the requested list); we have
	// the map's keys, sorted for reproducibility.
	keys := make([]string, 0, len(r.Types))
	for k := range r.Types {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Avatar display size honors plugin_people_size (resolved in Run,
	// stored on the Result). Fall back to the metadata default (28)
	// when the Result predates the size field (e.g. golden fixtures).
	avatarSize := r.Size
	if avatarSize <= 0 {
		avatarSize = defaultPeopleSize
	}

	var b strings.Builder
	b.WriteString(`<section data-section="people">`)
	for _, k := range keys {
		list := r.Types[k]
		if len(list) == 0 {
			continue
		}
		// The label count is the true total (Counts[k], e.g. GraphQL
		// followers.totalCount), not the fetched-and-clipped slice
		// length (#470). Repo-mode REST types have no totalCount, so
		// fall back to len(list).
		count := len(list)
		if c, ok := r.Counts[k]; ok {
			count = c
		}
		b.WriteString(`<section>`)
		fmt.Fprintf(
			&b, `<h2 class="field">%s%s</h2>`,
			peopleOcticon,
			partials.EscapeXML(labelForType(k, count)),
		)
		b.WriteString(`<div class="row">`)
		fmt.Fprintf(&b, `<section class="people" data-type="%s">`, partials.EscapeXML(k))
		for _, p := range list {
			fmt.Fprintf(
				&b,
				`<img class="avatar" src="%s" width="%d" height="%d" alt=""/>`,
				partials.EscapeXML(p.AvatarURL), avatarSize, avatarSize,
			)
		}
		b.WriteString(`</section>`)
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}
	b.WriteString(`</section>`)
	return b.String(), 0, nil
}
