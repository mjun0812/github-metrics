package isocalendar

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

const (
	cellSize = 12
	cellGap  = 2
)

// Partial renders the classic SVG fragment for the isocalendar plugin.
// DOM contract: emits <g class="calendar"> containing one
// <rect class="calendar-day"> per (week, day) cell.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || len(r.Weeks) == 0 {
		return "", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<section data-section="isocalendar" data-duration="%s">`,
		partials.EscapeXML(r.Duration))
	b.WriteString(`<g class="calendar">`)
	for wi, w := range r.Weeks {
		for di, count := range w.Days {
			x := wi * (cellSize + cellGap)
			y := di * (cellSize + cellGap)
			fmt.Fprintf(
				&b,
				`<rect class="calendar-day" x="%d" y="%d" width="%d" height="%d" data-count="%d" data-date="%s"></rect>`,
				x, y, cellSize, cellSize, count, partials.EscapeXML(w.FirstDay),
			)
		}
	}
	b.WriteString(`</g>`)
	fmt.Fprintf(
		&b,
		`<text class="calendar-summary" data-sum="%d" data-streak-max="%d" data-streak-current="%d">%d contributions · streak %d (max %d)</text>`,
		r.Sum, r.Streak.Max, r.Streak.Current, r.Sum, r.Streak.Current, r.Streak.Max,
	)
	b.WriteString(`</section>`)
	return b.String(), nil
}
