package stargazers

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Partial returns "" in M4 (always Skipped). Future US3/N-task wiring
// will populate the charts DOM.
func Partial(_ context.Context, _ *templates.PartialContext) (string, error) {
	return "", nil
}
