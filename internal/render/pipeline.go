package render

import (
	"fmt"
)

// PipelineStage is the function signature each decoration / optimization
// step exposes. Stages are composed by Apply.
type PipelineStage struct {
	// Name labels the stage in error wrapping and debug logs.
	Name string
	// Run takes the current SVG and returns the next SVG. Any error
	// is surfaced via Apply's []error return; the input string is
	// forwarded to the next stage unchanged (best-effort chain).
	Run func(in string) (string, error)
}

// Apply runs the stages in order. The returned string is the final SVG
// after all (successful) stages. Errors are returned alongside in
// registration order, each wrapped with the offending stage's name.
// Apply never panics on stage errors and never short-circuits on a
// failure: the caller decides whether to treat collected errors as
// fatal.
func Apply(stages []PipelineStage, in string) (string, []error) {
	var errs []error
	current := in
	for _, s := range stages {
		if s.Run == nil {
			continue
		}
		next, err := s.Run(current)
		if err != nil {
			errs = append(errs, fmt.Errorf("stage %q: %w", s.Name, err))
			// Leave `current` untouched so the next stage sees the
			// last known-good output (best-effort chain — see
			// FR-018).
			continue
		}
		current = next
	}
	return current, errs
}
