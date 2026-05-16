package habits

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

// Partial renders the habits fragment. DOM contract: <g
// class="habit-chart"> when there's data.
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
	totalDays := 0
	for _, n := range r.Charts.Days {
		totalDays += n
	}
	if totalDays == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="habits">`)
	b.WriteString(`<g class="habit-chart" data-chart="hours">`)
	for i, n := range r.Charts.Hours {
		fmt.Fprintf(&b, `<rect class="habit-hour" data-hour="%d" data-count="%d"></rect>`, i, n)
	}
	b.WriteString(`</g>`)
	b.WriteString(`<g class="habit-chart" data-chart="days">`)
	for i, n := range r.Charts.Days {
		fmt.Fprintf(&b, `<rect class="habit-day" data-day="%d" data-count="%d"></rect>`, i, n)
	}
	b.WriteString(`</g>`)
	fmt.Fprintf(&b, `<text class="habit-cpd" data-cpd="%.2f">%.2f commits/day</text>`,
		r.Facts.CommitsPerDay, r.Facts.CommitsPerDay)
	b.WriteString(`</section>`)
	return b.String(), nil
}
