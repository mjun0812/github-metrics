package calendar

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

// Partial renders the calendar fragment. DOM contract: <g
// class="calendar-year"> per year.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Years) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="calendar">`)
	for _, y := range r.Years {
		fmt.Fprintf(&b, `<g class="calendar-year" data-year="%d" data-total="%d">%d (%d)</g>`,
			y.Year, y.Total, y.Year, y.Total)
	}
	b.WriteString(`</section>`)
	return b.String(), nil
}
