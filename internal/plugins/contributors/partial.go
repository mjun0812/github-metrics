package contributors

import (
	"context"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// Partial returns an empty fragment in M4 because Result is always
// Skipped. classic.go's dispatcher then suppresses the wrapper.
func Partial(_ context.Context, _ *templates.PartialContext) (string, error) {
	return "", nil
}
