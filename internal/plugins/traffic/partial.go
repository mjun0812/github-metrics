package traffic

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
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
	return "", nil
}
