package traffic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// graphOcticon is the upstream `<%- octicon "graph" %>`-style 16x16 path
// used in the traffic section header. Upstream classic doesn't have a
// dedicated traffic.ejs (it's included via base.repositories) — we
// emit a standalone header for parity with other plugins.
const graphOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 1.75a.75.75 0 00-1.5 0v12.5c0 .414.336.75.75.75h14.5a.75.75 0 000-1.5H1.5V1.75zm14.28 2.53a.75.75 0 00-1.06-1.06L10 7.94 7.53 5.47a.75.75 0 00-1.06 0L3.22 8.72a.75.75 0 001.06 1.06L7 7.06l2.47 2.47a.75.75 0 001.06 0l5.25-5.25z"></path></svg>`

// pluralS mirrors upstream's `s()` helper.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Partial renders the classic SVG fragment for the traffic plugin.
// Upstream classic does not ship a standalone traffic.ejs (the data is
// merged into base.repositories); we emit a self-contained section with
// header + aggregate counts + per-repo breakdown so downstream consumers
// can see the data the plugin actually collects.
//
// Output structure:
//
//	<section data-section="traffic">
//	  <h2 class="field"><svg/>Traffic</h2>
//	  <div class="row"><section>
//	    <div class="field"><span class="label">N views (M unique)</span></div>
//	    [for each repo with views]:
//	      <div class="field"><span class="repo">${name}</span>: ${views} views (${uniques} unique)</div>
//	  </section></div>
//	</section>
//
// When `plugin_traffic_hide_empty` is true (default), repositories with
// Count == 0 are filtered before sorting/rendering so the per-repo
// breakdown only shows repos that actually received traffic.
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
	if r.Total.Count == 0 && len(r.Views) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="traffic">`)
	fmt.Fprintf(&b, `<h2 class="field">%sTraffic</h2>`, graphOcticon)
	b.WriteString(`<div class="row"><section>`)
	// Aggregate line.
	fmt.Fprintf(
		&b,
		`<div class="field"><span class="label">%d view%s (%d unique)</span></div>`,
		r.Total.Count, pluralS(r.Total.Count), r.Total.Uniques,
	)
	// Per-repo lines, sorted by view count desc (stable secondary by name).
	if len(r.Views) > 0 {
		type repoView struct {
			name    string
			count   int
			uniques int
		}
		entries := make([]repoView, 0, len(r.Views))
		for name, v := range r.Views {
			if r.HideEmpty && v.Count == 0 {
				continue
			}
			entries = append(entries, repoView{name: name, count: v.Count, uniques: v.Uniques})
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].count != entries[j].count {
				return entries[i].count > entries[j].count
			}
			return entries[i].name < entries[j].name
		})
		for _, e := range entries {
			fmt.Fprintf(
				&b,
				`<div class="field"><span class="repo">%s</span>: %d view%s (%d unique)</div>`,
				partials.EscapeXML(e.name), e.count, pluralS(e.count), e.uniques,
			)
		}
	}
	b.WriteString(`</section></div>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}
