package reactions

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

// Partial renders the reactions fragment. DOM contract: <text
// class="reaction-count">.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped || r.Total == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section data-section="reactions">`)
	fmt.Fprintf(&b,
		`<text class="reaction-count" data-issues="%d" data-comments="%d" data-discussions="%d" data-total="%d">%d reactions</text>`,
		r.Issues, r.Comments, r.Discussions, r.Total, r.Total)
	_ = partials.EscapeXML // silence import when unused above
	b.WriteString(`</section>`)
	return b.String(), nil
}
