package stargazers

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// starOcticon is the upstream `<%- octicon "star" %>` 16x16 path used
// in the stargazers section header (EJS line 4).
const starOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`

// bgLevel maps a 0..1 share to the upstream
// `var(--color-calendar-graph-day-L{1..4}-bg)` ramp via Math.ceil(p/0.25).
func bgLevel(p float64) int {
	if p <= 0 {
		return 1
	}
	lvl := int(math.Ceil(p / 0.25))
	if lvl < 1 {
		lvl = 1
	}
	if lvl > 4 {
		lvl = 4
	}
	return lvl
}

// Partial renders the classic SVG fragment for the stargazers plugin.
// Mirrors upstream org_repo/source/templates/classic/partials/stargazers.ejs.
//
// Returns "" until stargazers.go's Run is wired with chart data (current
// M4 path stays Skipped). When data arrives, emits the upstream
// "Total stargazers" chart-bars block alongside the section header.
//
// Output structure:
//
//	<section data-section="stargazers">
//	  <h2 class="field"><svg star/>Stargazers</h2>
//	  <div class="row margin-bottom">
//	    <section class="column chart">
//	      <h3>Total stargazers</h3>
//	      <div class="chart-bars">
//	        [for each point]:
//	          <div class="entry"><span class="value">N</span><div class="bar"/><span class="label">DD</span></div>
//	      </div>
//	    </section>
//	  </div>
//	</section>
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
	series := r.Charts.Series
	if len(series) == 0 && len(r.List) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(`<section data-section="stargazers">`)
	fmt.Fprintf(&b, `<h2 class="field">%sStargazers</h2>`, starOcticon)

	if len(series) > 0 {
		// Find min/max for normalization, matching upstream's `(value-min)/(max-min)` ramp.
		minV, maxV := series[0].Count, series[0].Count
		for _, pt := range series[1:] {
			if pt.Count < minV {
				minV = pt.Count
			}
			if pt.Count > maxV {
				maxV = pt.Count
			}
		}
		denom := maxV - minV
		if denom == 0 {
			denom = 1
		}

		b.WriteString(`<div class="row margin-bottom">`)
		b.WriteString(`<section class="column chart">`)
		b.WriteString(`<h3>Total stargazers</h3>`)
		b.WriteString(`<div class="chart-bars">`)
		for _, pt := range series {
			share := 0.05 + 0.95*float64(pt.Count-minV)/float64(denom)
			day := pt.Date.UTC().Day()
			fmt.Fprintf(
				&b,
				`<div class="entry"><span class="value">%d</span><div class="bar" style="height: %.0fpx; background-color: var(--color-calendar-graph-day-L%d-bg)"></div><span class="label">%d</span></div>`,
				pt.Count, share*50, bgLevel(share), day,
			)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
		b.WriteString(`</div>`)
	}

	b.WriteString(`</section>`)
	return b.String(), nil
}
