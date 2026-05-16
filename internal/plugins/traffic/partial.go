package traffic

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

// Partial renders the traffic fragment. DOM contract: <text
// class="traffic-count"> (Skipped=false 時のみ).
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
	fmt.Fprintf(&b, `<text class="traffic-count" data-count="%d" data-uniques="%d">%d views (%d unique)</text>`,
		r.Total.Count, r.Total.Uniques, r.Total.Count, r.Total.Uniques)
	b.WriteString(`</section>`)
	return b.String(), nil
}
